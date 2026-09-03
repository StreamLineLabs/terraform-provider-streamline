// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

// UserResource preserves the legacy resource schema for state compatibility.
// Streamline does not expose an API that can manage SASL credentials.
type UserResource struct {
	configured bool
}

type UserResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Username  types.String `tfsdk:"username"`
	Mechanism types.String `tfsdk:"mechanism"`
}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

func (r *UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Legacy Streamline SASL/SCRAM user state. Credential management is unsupported.",
		MarkdownDescription: "Preserves legacy `streamline_user` state so provider upgrades do not make existing state unreadable. " +
			"Streamline does not expose an API for creating, reading, rotating, or deleting SASL credentials. " +
			"Manage credentials outside Terraform and use `streamline_acl` for authorization. " +
			"Remove legacy entries with `terraform state rm` after confirming external credential ownership.",
		DeprecationMessage: "streamline_user cannot manage credentials and is retained only for legacy state compatibility.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Legacy state identifier (same as username).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Legacy SASL/SCRAM username.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mechanism": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("SCRAM-SHA-256"),
				Description: "Legacy SASL mechanism.",
				Validators: []validator.String{
					stringvalidator.OneOf("SCRAM-SHA-256", "SCRAM-SHA-512"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configured = configureLegacyResource(req, resp)
}

func (r *UserResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"User Credential Management Unsupported",
		"Streamline does not expose an API for Terraform to create SASL credentials. "+
			"Manage credentials outside Terraform and grant authorization with streamline_acl.",
	)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Legacy User State Cannot Be Refreshed",
		"Streamline does not expose an API for reading SASL credentials. Terraform is preserving this legacy state unchanged. "+
			"After confirming credentials are managed externally, remove it with terraform state rm.",
	)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *UserResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"User Credential Management Unsupported",
		"Streamline does not expose an API for Terraform to update or rotate SASL credentials.",
	)
}

func (r *UserResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddError(
		"User Credential Deletion Unsupported",
		"Streamline does not expose an API for Terraform to delete SASL credentials. "+
			"Delete the credential through its external owner, then remove this legacy entry with terraform state rm.",
	)
}

func (r *UserResource) ImportState(_ context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"User Import Unsupported",
		"Cannot import "+req.ID+" because Streamline does not expose an API for reading SASL credentials.",
	)
}
