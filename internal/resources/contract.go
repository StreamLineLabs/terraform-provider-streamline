// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

var _ resource.Resource = &ContractResource{}
var _ resource.ResourceWithImportState = &ContractResource{}

// ContractResource manages a Streamline contract (Moonshot M4).
type ContractResource struct {
	moonshot *client.MoonshotClient
}

// ContractResourceModel describes the resource data model.
type ContractResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	SchemaJSON    types.String `tfsdk:"schema"`
	Compatibility types.String `tfsdk:"compatibility"`
}

// NewContractResource builds a new contract resource.
func NewContractResource() resource.Resource {
	return &ContractResource{}
}

func (r *ContractResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contract"
}

func (r *ContractResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages a Streamline data contract enforced at the broker (Moonshot M4). Requires moonshot_url on the provider.",
		MarkdownDescription: "Manages a Streamline data contract (M4 enforced contracts + provenance). Producers writing to topics bound to a contract must satisfy the schema. Requires `moonshot_url` on the provider.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier (same as `name`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Contract name (e.g. `orders.v1`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "JSON-encoded contract schema body.",
			},
			"compatibility": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Compatibility policy: `BACKWARD`, `FORWARD`, `FULL`, or `NONE`. Defaults to `BACKWARD` server-side.",
			},
		},
	}
}

func (r *ContractResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*ProviderClients)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderClients, got: %T", req.ProviderData))
		return
	}
	if clients.Moonshot == nil {
		resp.Diagnostics.AddError("Moonshot URL not configured",
			"streamline_contract requires the provider's moonshot_url to be set (or STREAMLINE_MOONSHOT_URL env var).")
		return
	}
	r.moonshot = clients.Moonshot
}

func (r *ContractResource) buildContract(data ContractResourceModel) (client.Contract, error) {
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(data.SchemaJSON.ValueString()), &schemaMap); err != nil {
		return client.Contract{}, fmt.Errorf("schema must be valid JSON: %w", err)
	}
	c := client.Contract{
		Name:   data.Name.ValueString(),
		Schema: schemaMap,
	}
	if !data.Compatibility.IsNull() && !data.Compatibility.IsUnknown() {
		c.Compatibility = data.Compatibility.ValueString()
	}
	return c, nil
}

func (r *ContractResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ContractResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	contract, err := r.buildContract(data)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			pathRoot("schema"), "Invalid schema JSON", err.Error())
		return
	}
	tflog.Trace(ctx, "registering contract", map[string]interface{}{"name": contract.Name})
	out, err := r.moonshot.RegisterContract(ctx, contract)
	if err != nil {
		resp.Diagnostics.AddError("Failed to register contract",
			fmt.Sprintf("Could not register contract %q: %s", contract.Name, err))
		return
	}
	r.applyServerState(&data, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContractResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ContractResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.moonshot.GetContract(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read contract",
			fmt.Sprintf("Could not read contract %q: %s", data.Name.ValueString(), err))
		return
	}
	if out == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	r.applyServerState(&data, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContractResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ContractResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	contract, err := r.buildContract(data)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			pathRoot("schema"), "Invalid schema JSON", err.Error())
		return
	}
	out, err := r.moonshot.RegisterContract(ctx, contract)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update contract",
			fmt.Sprintf("Could not update contract %q: %s", contract.Name, err))
		return
	}
	r.applyServerState(&data, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContractResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ContractResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.moonshot.DeleteContract(ctx, data.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete contract",
			fmt.Sprintf("Could not delete contract %q: %s", data.Name.ValueString(), err))
		return
	}
}

func (r *ContractResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	out, err := r.moonshot.GetContract(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import failed",
			fmt.Sprintf("Could not fetch contract %q: %s", req.ID, err))
		return
	}
	if out == nil {
		resp.Diagnostics.AddError("Import failed",
			fmt.Sprintf("Contract %q not found", req.ID))
		return
	}
	schemaBytes, err := json.Marshal(out.Schema)
	if err != nil {
		resp.Diagnostics.AddError("Import failed",
			fmt.Sprintf("Could not encode schema for contract %q: %s", req.ID, err))
		return
	}
	model := ContractResourceModel{
		ID:         types.StringValue(out.Name),
		Name:       types.StringValue(out.Name),
		SchemaJSON: types.StringValue(string(schemaBytes)),
	}
	if out.Compatibility != "" {
		model.Compatibility = types.StringValue(out.Compatibility)
	} else {
		model.Compatibility = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *ContractResource) applyServerState(data *ContractResourceModel, out *client.Contract) {
	data.ID = types.StringValue(out.Name)
	data.Name = types.StringValue(out.Name)
	if out.Compatibility != "" {
		data.Compatibility = types.StringValue(out.Compatibility)
	}
	// Re-canonicalize the schema so plan/state stay stable across runs.
	if buf, err := json.Marshal(out.Schema); err == nil {
		data.SchemaJSON = types.StringValue(string(buf))
	}
}
