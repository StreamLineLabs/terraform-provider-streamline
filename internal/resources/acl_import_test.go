// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

type fakeACLClient struct {
	getResult *client.ACLConfig
	getErr    error
}

func (f *fakeACLClient) CreateACL(context.Context, client.ACLConfig) error {
	return errors.New("unexpected CreateACL call")
}

func (f *fakeACLClient) GetACL(context.Context, client.ACLConfig) (*client.ACLConfig, error) {
	return f.getResult, f.getErr
}

func (f *fakeACLClient) DeleteACL(context.Context, client.ACLConfig) error {
	return errors.New("unexpected DeleteACL call")
}

func TestACLIDRoundTrip(t *testing.T) {
	t.Parallel()

	want := client.ACLConfig{
		ResourceType:   "topic",
		PatternType:    "literal",
		ResourceName:   "events|archive/2026",
		Principal:      "User:alice@example.com",
		Host:           "10.0.0.10",
		Operation:      "describe_configs",
		PermissionType: "allow",
	}
	id := aclID(want)
	got, err := parseACLID(id)
	if err != nil {
		t.Fatalf("parseACLID(%q) error = %v", id, err)
	}
	if got != want {
		t.Fatalf("parseACLID(%q) = %+v, want %+v", id, got, want)
	}
}

func TestParseACLIDRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, id := range []string{
		"too|few|fields",
		"topic|match|events|User%3Aalice|%2A|read|allow",
		"unknown|literal|events|User%3Aalice|%2A|read|allow",
		"topic|literal||User%3Aalice|%2A|read|allow",
	} {
		if _, err := parseACLID(id); err == nil {
			t.Fatalf("parseACLID(%q) expected error", id)
		}
	}
}

func TestACLImportStateReadsAndPopulatesAllFields(t *testing.T) {
	t.Parallel()

	acl := client.ACLConfig{
		ResourceType:   "topic",
		PatternType:    "literal",
		ResourceName:   "events",
		Principal:      "User:alice",
		Host:           "10.0.0.10",
		Operation:      "read",
		PermissionType: "allow",
	}
	r := &AclResource{kafkaClient: &fakeACLClient{getResult: &acl}}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	resp := &resource.ImportStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	r.ImportState(
		context.Background(),
		resource.ImportStateRequest{ID: aclID(acl)},
		resp,
	)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected import diagnostics: %v", resp.Diagnostics.Errors())
	}

	var got AclResourceModel
	resp.Diagnostics.Append(resp.State.Get(context.Background(), &got)...)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unable to decode import state: %v", resp.Diagnostics.Errors())
	}
	if got.ID.ValueString() != aclID(acl) ||
		got.ResourceName.ValueString() != acl.ResourceName ||
		got.Principal.ValueString() != acl.Principal ||
		got.Operation.ValueString() != acl.Operation {
		t.Fatalf("unexpected import state: %+v", got)
	}
}

func TestACLImportStateReportsConfirmedNotFound(t *testing.T) {
	t.Parallel()

	acl := client.ACLConfig{
		ResourceType:   "topic",
		PatternType:    "literal",
		ResourceName:   "events",
		Principal:      "User:alice",
		Host:           "10.0.0.10",
		Operation:      "read",
		PermissionType: "allow",
	}
	r := &AclResource{
		kafkaClient: &fakeACLClient{
			getErr: client.NewNotFoundError("ACL", aclID(acl), nil),
		},
	}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	resp := &resource.ImportStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	r.ImportState(
		context.Background(),
		resource.ImportStateRequest{ID: aclID(acl)},
		resp,
	)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected not-found import diagnostic")
	}
	if got := resp.Diagnostics.Errors()[0].Summary(); got != "ACL Not Found" {
		t.Fatalf("unexpected diagnostic summary: %q", got)
	}
}
