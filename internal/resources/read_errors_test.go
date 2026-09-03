// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

func TestHandleReadErrorRemovesStateOnlyForTypedNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stateSchema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
		},
	}
	initial := tftypes.NewValue(
		stateSchema.Type().TerraformType(ctx),
		map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "managed")},
	)

	t.Run("typed not found removes state", func(t *testing.T) {
		resp := &resource.ReadResponse{State: tfsdk.State{Raw: initial, Schema: stateSchema}}
		err := fmt.Errorf("wrapped: %w", client.NewNotFoundError("topic", "events", nil))

		if !handleReadError(ctx, resp, "Topic", "events", err) {
			t.Fatal("expected error to be handled")
		}
		if !resp.State.Raw.IsNull() {
			t.Fatalf("state was not removed: %s", resp.State.Raw)
		}
		if resp.Diagnostics.HasError() {
			t.Fatalf("not-found should not emit an error diagnostic: %v", resp.Diagnostics.Errors())
		}
	})

	for name, err := range map[string]error{
		"authorization": errors.New("authorization failed"),
		"transient":     errors.New("connection reset by peer"),
		"untyped text":  errors.New("topic not found: events"),
	} {
		t.Run(name+" preserves state", func(t *testing.T) {
			resp := &resource.ReadResponse{State: tfsdk.State{Raw: initial, Schema: stateSchema}}

			if !handleReadError(ctx, resp, "Topic", "events", err) {
				t.Fatal("expected error to be handled")
			}
			if !resp.State.Raw.Equal(initial) {
				t.Fatalf("state changed for %s error: %s", name, resp.State.Raw)
			}
			if !resp.Diagnostics.HasError() {
				t.Fatalf("%s error must emit a diagnostic", name)
			}
		})
	}
}

func TestHandleDeleteErrorIgnoresOnlyTypedNotFound(t *testing.T) {
	t.Parallel()

	notFoundResp := &resource.DeleteResponse{}
	if handleDeleteError(notFoundResp, "Topic", "events", client.NewNotFoundError("topic", "events", nil)) {
		t.Fatal("typed not-found should be treated as already deleted")
	}
	if notFoundResp.Diagnostics.HasError() {
		t.Fatalf("typed not-found emitted diagnostics: %v", notFoundResp.Diagnostics.Errors())
	}

	transientResp := &resource.DeleteResponse{}
	if !handleDeleteError(transientResp, "Topic", "events", errors.New("connection reset")) {
		t.Fatal("transient delete error should be reported")
	}
	if !transientResp.Diagnostics.HasError() {
		t.Fatal("transient delete error did not emit a diagnostic")
	}
}
