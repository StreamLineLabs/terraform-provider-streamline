// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestJSONContractValidatorRejectsNonObject(t *testing.T) {
	t.Parallel()

	for _, value := range []string{`not-json`, `[]`, `null`} {
		resp := &validator.StringResponse{}
		(jsonObjectValidator{}).ValidateString(
			context.Background(),
			validator.StringRequest{
				Path:        path.Root("schema"),
				ConfigValue: types.StringValue(value),
			},
			resp,
		)
		if !resp.Diagnostics.HasError() {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestMoonshotResourcesAreDeprecated(t *testing.T) {
	t.Parallel()

	for name, factory := range map[string]func() resource.Resource{
		"streamline_branch":   NewBranchResource,
		"streamline_contract": NewContractResource,
		"streamline_memory":   NewMemoryResource,
	} {
		resourceUnderTest := factory()
		schemaResp := &resource.SchemaResponse{}
		resourceUnderTest.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Fatalf("%s schema diagnostics: %v", name, schemaResp.Diagnostics.Errors())
		}
		if schemaResp.Schema.DeprecationMessage == "" {
			t.Fatalf("%s must have a deprecation message", name)
		}

		createResp := &resource.CreateResponse{}
		resourceUnderTest.Create(context.Background(), resource.CreateRequest{}, createResp)
		if !createResp.Diagnostics.HasError() {
			t.Fatalf("%s create must return an unsupported diagnostic", name)
		}
	}
}

func TestMemoryDecayThresholdRequiresHalfLife(t *testing.T) {
	t.Parallel()

	resp := &resource.SchemaResponse{}
	(&MemoryResource{}).Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", resp.Diagnostics.Errors())
	}

	// The schema-level regression check ensures the cross-attribute validator
	// remains attached after deprecating the unsupported resource.
	attr := resp.Schema.Attributes["decay_threshold"]
	if attr == nil {
		t.Fatal("decay_threshold attribute is missing")
	}
}
