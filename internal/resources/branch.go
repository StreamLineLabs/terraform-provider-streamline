// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

var _ resource.Resource = &BranchResource{}
var _ resource.ResourceWithImportState = &BranchResource{}

// BranchResource manages a Streamline branch (Moonshot M5).
type BranchResource struct {
	moonshot *client.MoonshotClient
}

// BranchResourceModel describes the resource data model.
type BranchResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Parent      types.String `tfsdk:"parent"`
	CreatedAtMs types.Int64  `tfsdk:"created_at_ms"`
}

// NewBranchResource builds a new branch resource.
func NewBranchResource() resource.Resource {
	return &BranchResource{}
}

func (r *BranchResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_branch"
}

func (r *BranchResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages a Streamline branch (time-travel / what-if data branch). Requires moonshot_url to be set on the provider.",
		MarkdownDescription: "Manages a Streamline branch (M5 time-travel). Branches are immutable refs to a topic state at creation time; readers consume `topic@branch=name`. Requires `moonshot_url` on the provider.",
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
				MarkdownDescription: "Branch name. Must be unique within the cluster.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parent": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Parent branch name. Defaults to the cluster's main branch.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at_ms": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp (Unix epoch milliseconds).",
			},
		},
	}
}

func (r *BranchResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
			"streamline_branch requires the provider's moonshot_url to be set (or STREAMLINE_MOONSHOT_URL env var).")
		return
	}
	r.moonshot = clients.Moonshot
}

func (r *BranchResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BranchResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()
	parent := ""
	if !data.Parent.IsNull() && !data.Parent.IsUnknown() {
		parent = data.Parent.ValueString()
	}
	tflog.Trace(ctx, "creating branch", map[string]interface{}{"name": name, "parent": parent})
	b, err := r.moonshot.CreateBranch(ctx, name, parent)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create branch",
			fmt.Sprintf("Could not create branch %q: %s", name, err))
		return
	}
	data.ID = types.StringValue(b.Name)
	if b.Parent != "" {
		data.Parent = types.StringValue(b.Parent)
	}
	data.CreatedAtMs = types.Int64Value(b.CreatedAtMs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BranchResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BranchResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()
	b, err := r.moonshot.GetBranch(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read branch",
			fmt.Sprintf("Could not read branch %q: %s", name, err))
		return
	}
	if b == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if b.Parent != "" {
		data.Parent = types.StringValue(b.Parent)
	}
	data.CreatedAtMs = types.Int64Value(b.CreatedAtMs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BranchResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All updatable attributes carry RequiresReplace; nothing to do here.
	var data BranchResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BranchResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BranchResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()
	tflog.Trace(ctx, "deleting branch", map[string]interface{}{"name": name})
	if err := r.moonshot.DeleteBranch(ctx, name); err != nil {
		resp.Diagnostics.AddError("Failed to delete branch",
			fmt.Sprintf("Could not delete branch %q: %s", name, err))
		return
	}
}

func (r *BranchResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name := req.ID
	b, err := r.moonshot.GetBranch(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Import failed",
			fmt.Sprintf("Could not find branch %q: %s", name, err))
		return
	}
	if b == nil {
		resp.Diagnostics.AddError("Import failed",
			fmt.Sprintf("Branch %q not found", name))
		return
	}
	model := BranchResourceModel{
		ID:          types.StringValue(b.Name),
		Name:        types.StringValue(b.Name),
		CreatedAtMs: types.Int64Value(b.CreatedAtMs),
	}
	if b.Parent != "" {
		model.Parent = types.StringValue(b.Parent)
	} else {
		model.Parent = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
