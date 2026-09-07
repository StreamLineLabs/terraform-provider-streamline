// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ContractResource{}
var _ resource.ResourceWithImportState = &ContractResource{}

// ContractResource preserves the legacy schema. The broker validates inline
// contracts but does not expose a contract registry object that Terraform can
// provision.
type ContractResource struct {
	configured bool
}

type ContractResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	SchemaJSON    types.String `tfsdk:"schema"`
	Compatibility types.String `tfsdk:"compatibility"`
}

type jsonObjectValidator struct{}

func (jsonObjectValidator) Description(context.Context) string {
	return "value must be a JSON object"
}

func (jsonObjectValidator) MarkdownDescription(context.Context) string {
	return "value must be a JSON object"
}

func (jsonObjectValidator) ValidateString(
	_ context.Context,
	req validator.StringRequest,
	resp *validator.StringResponse,
) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(req.ConfigValue.ValueString()), &object); err != nil || object == nil {
		detail := "The value must decode to a JSON object."
		if err != nil {
			detail = fmt.Sprintf("The value must decode to a JSON object: %s", err)
		}
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid JSON Object", detail)
	}
}

func NewContractResource() resource.Resource {
	return &ContractResource{}
}

func (r *ContractResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contract"
}

func (r *ContractResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Legacy Streamline contract registry schema retained for state compatibility.",
		MarkdownDescription: "Retains the legacy `streamline_contract` schema so existing state remains readable. " +
			"The broker accepts contracts inline during validation and does not expose contract registry CRUD, " +
			"so Terraform cannot provision a contract object.",
		DeprecationMessage: "streamline_contract is unsupported because the broker has no contract registry CRUD API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Legacy resource identifier (same as `name`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Legacy contract name.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Legacy JSON-encoded contract schema object.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(2),
					jsonObjectValidator{},
				},
			},
			"compatibility": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("BACKWARD"),
				MarkdownDescription: "Legacy compatibility policy.",
				Validators: []validator.String{
					stringvalidator.OneOf("BACKWARD", "FORWARD", "FULL", "NONE"),
				},
			},
		},
	}
}

func (r *ContractResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configured = configureLegacyResource(req, resp)
}

func (r *ContractResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_contract")
}

func (r *ContractResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ContractResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	addUnsupportedMoonshotReadWarning(&resp.Diagnostics, "streamline_contract")
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ContractResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_contract")
}

func (r *ContractResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_contract")
}

func (r *ContractResource) ImportState(_ context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_contract")
}
