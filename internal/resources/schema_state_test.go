// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

func TestSchemaDefinitionChangedIncludesTypeAndReferences(t *testing.T) {
	t.Parallel()

	base := SchemaResourceModel{
		Schema:     types.StringValue(`{"type":"record"}`),
		SchemaType: types.StringValue("AVRO"),
		References: types.ListNull(types.ObjectType{AttrTypes: schemaReferenceAttrTypes}),
	}

	changedType := base
	changedType.SchemaType = types.StringValue("JSON")
	if !schemaDefinitionChanged(changedType, base) {
		t.Fatal("schema type changes must register a schema version")
	}

	changedReferences := base
	changedReferences.References = types.ListValueMust(
		types.ObjectType{AttrTypes: schemaReferenceAttrTypes},
		[]attr.Value{types.ObjectValueMust(schemaReferenceAttrTypes, map[string]attr.Value{
			"name":    types.StringValue("common.proto"),
			"subject": types.StringValue("common-value"),
			"version": types.Int64Value(1),
		})},
	)
	if !schemaDefinitionChanged(changedReferences, base) {
		t.Fatal("schema reference changes must register a schema version")
	}
}

func TestApplySchemaInfoRefreshesReferences(t *testing.T) {
	t.Parallel()

	model := SchemaResourceModel{
		Subject:    types.StringValue("orders-value"),
		References: types.ListNull(types.ObjectType{AttrTypes: schemaReferenceAttrTypes}),
	}
	var diags diag.Diagnostics
	applySchemaInfo(context.Background(), &model, &client.SchemaInfo{
		Version:    3,
		ID:         42,
		Schema:     `{"type":"record"}`,
		SchemaType: "AVRO",
		References: []client.SchemaReference{{
			Name:    "common.proto",
			Subject: "common-value",
			Version: 1,
		}},
	}, &diags)
	if diags.HasError() {
		t.Fatalf("applySchemaInfo diagnostics: %v", diags.Errors())
	}
	if model.ID.ValueString() != "orders-value:3" || model.SchemaID.ValueInt64() != 42 {
		t.Fatalf("unexpected schema identity: %+v", model)
	}
	if model.SchemaType.ValueString() != "AVRO" {
		t.Fatalf("missing schema type was not canonicalized to AVRO: %+v", model.SchemaType)
	}
	if model.References.IsNull() || len(model.References.Elements()) != 1 {
		t.Fatalf("references were not refreshed: %v", model.References)
	}
}

func TestResolveRegisteredSchemaUsesReturnedIDVersion(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/schemas/ids/42/versions":
			_ = json.NewEncoder(w).Encode([]client.SubjectVersionPair{
				{Subject: "orders-value", Version: 2},
				{Subject: "other-value", Version: 9},
			})
		case "/subjects/orders-value/versions/2":
			_ = json.NewEncoder(w).Encode(client.SchemaInfo{
				Subject: "orders-value",
				Version: 2,
				ID:      42,
				Schema:  `{"type":"record"}`,
			})
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	resource := &SchemaResource{
		schemaRegistryClient: client.NewSchemaRegistryClient(client.SchemaRegistryConfig{URL: server.URL}),
	}
	info, err := resource.resolveRegisteredSchema(context.Background(), "orders-value", 42)
	if err != nil {
		t.Fatalf("resolveRegisteredSchema() error = %v", err)
	}
	if info.Version != 2 || info.ID != 42 {
		t.Fatalf("resolved schema = %+v, want id 42 version 2", info)
	}
}

func TestSchemaDeleteRemainsUnsupportedOutsideAcceptanceHarness(t *testing.T) {
	t.Parallel()

	resp := runSchemaDelete(t, &SchemaResource{})
	if !resp.Diagnostics.HasError() {
		t.Fatal("production schema deletion must remain unsupported")
	}
}

func TestSchemaAcceptanceDeleteRemovesOnlyTestState(t *testing.T) {
	t.Parallel()

	resp := runSchemaDelete(t, &SchemaResource{acceptanceStateOnlyDelete: true})
	if resp.Diagnostics.HasError() {
		t.Fatalf("acceptance cleanup returned an error: %v", resp.Diagnostics.Errors())
	}
	if len(resp.Diagnostics) == 0 {
		t.Fatal("acceptance cleanup must warn that the remote schema was retained")
	}
}

func runSchemaDelete(t *testing.T, schemaResource *SchemaResource) *resource.DeleteResponse {
	t.Helper()

	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	schemaResource.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics.Errors())
	}

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, &SchemaResourceModel{
		ID:            types.StringValue("orders-value:1"),
		Subject:       types.StringValue("orders-value"),
		SchemaType:    types.StringValue("AVRO"),
		Schema:        types.StringValue(`{"type":"record"}`),
		Version:       types.Int64Value(1),
		SchemaID:      types.Int64Value(42),
		Compatibility: types.StringValue("BACKWARD"),
		References:    types.ListNull(types.ObjectType{AttrTypes: schemaReferenceAttrTypes}),
	})
	if diags.HasError() {
		t.Fatalf("state setup diagnostics: %v", diags.Errors())
	}

	resp := &resource.DeleteResponse{}
	schemaResource.Delete(ctx, resource.DeleteRequest{State: state}, resp)
	return resp
}
