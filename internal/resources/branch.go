// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &BranchResource{}
var _ resource.ResourceWithImportState = &BranchResource{}

// BranchResource preserves the legacy schema. Its name/parent contract does
// not match the broker's current base_topic/base_offsets/created_by API.
type BranchResource struct {
	configured bool
}

type BranchResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Parent      types.String `tfsdk:"parent"`
	CreatedAtMs types.Int64  `tfsdk:"created_at_ms"`
}

func NewBranchResource() resource.Resource {
	return &BranchResource{}
}

func (r *BranchResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_branch"
}

func (r *BranchResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Legacy Streamline branch schema retained for state compatibility.",
		MarkdownDescription: "Retains the legacy `streamline_branch` schema so existing state remains readable. " +
			"Its `name`/`parent` model does not match the broker's current branch creation contract, " +
			"so provisioning is disabled until a versioned state migration can introduce the real schema safely.",
		DeprecationMessage: "streamline_branch provisioning is unsupported because the legacy schema does not match the broker API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Legacy resource identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Legacy branch name.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parent": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Legacy parent branch field; not part of the current broker API.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at_ms": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Legacy creation timestamp.",
			},
		},
	}
}

func (r *BranchResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configured = configureLegacyResource(req, resp)
}

func (r *BranchResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_branch")
}

func (r *BranchResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BranchResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	addUnsupportedMoonshotReadWarning(&resp.Diagnostics, "streamline_branch")
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BranchResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_branch")
}

func (r *BranchResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_branch")
}

func (r *BranchResource) ImportState(_ context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_branch")
}
