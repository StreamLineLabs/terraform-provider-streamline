// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &MemoryResource{}
var _ resource.ResourceWithImportState = &MemoryResource{}

// MemoryResource preserves the legacy partition schema. The broker exposes
// remember/recall operations, not a provisionable memory-partition object.
type MemoryResource struct {
	configured bool
}

type MemoryResourceModel struct {
	ID                      types.String  `tfsdk:"id"`
	AgentID                 types.String  `tfsdk:"agent_id"`
	Tenant                  types.String  `tfsdk:"tenant"`
	EpisodicRetentionDays   types.Int64   `tfsdk:"episodic_retention_days"`
	SemanticRetentionDays   types.Int64   `tfsdk:"semantic_retention_days"`
	ProceduralRetentionDays types.Int64   `tfsdk:"procedural_retention_days"`
	DecayHalfLifeDays       types.Float64 `tfsdk:"decay_half_life_days"`
	DecayThreshold          types.Float64 `tfsdk:"decay_threshold"`
	EncryptionEnabled       types.Bool    `tfsdk:"encryption_enabled"`
}

func NewMemoryResource() resource.Resource {
	return &MemoryResource{}
}

func (r *MemoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_memory"
}

func (r *MemoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Legacy Streamline memory-partition schema retained for state compatibility.",
		MarkdownDescription: "Retains the legacy `streamline_memory` schema so existing state remains readable. " +
			"The broker exposes remember/recall operations rather than CRUD for a memory-partition object, " +
			"so Terraform cannot provision this model.",
		DeprecationMessage: "streamline_memory is unsupported because the broker has no memory-partition CRUD API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Legacy resource identifier (`{tenant}/{agent_id}`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"agent_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Legacy agent identifier.",
				Validators: []validator.String{
					stringvalidator.All(
						stringvalidator.LengthAtLeast(1),
						stringvalidator.RegexMatches(regexp.MustCompile(`^[^/]+$`), "must not contain '/'"),
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tenant": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Legacy tenant namespace.",
				Validators: []validator.String{
					stringvalidator.All(
						stringvalidator.LengthAtLeast(1),
						stringvalidator.RegexMatches(regexp.MustCompile(`^[^/]+$`), "must not contain '/'"),
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"episodic_retention_days": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(30),
				MarkdownDescription: "Legacy episodic retention setting.",
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"semantic_retention_days": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(365),
				MarkdownDescription: "Legacy semantic retention setting.",
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"procedural_retention_days": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(-1),
				MarkdownDescription: "Legacy procedural retention setting.",
				Validators: []validator.Int64{
					int64validator.Any(
						int64validator.OneOf(-1),
						int64validator.AtLeast(0),
					),
				},
			},
			"decay_half_life_days": schema.Float64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Legacy memory decay half-life.",
				Validators: []validator.Float64{
					float64validator.AtLeast(0.000001),
				},
			},
			"decay_threshold": schema.Float64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Legacy memory decay threshold.",
				Validators: []validator.Float64{
					float64validator.Between(0, 1),
					float64validator.AlsoRequires(path.MatchRoot("decay_half_life_days")),
				},
			},
			"encryption_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Legacy encryption-at-rest setting.",
			},
		},
	}
}

func (r *MemoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configured = configureLegacyResource(req, resp)
}

func (r *MemoryResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_memory")
}

func (r *MemoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MemoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	addUnsupportedMoonshotReadWarning(&resp.Diagnostics, "streamline_memory")
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *MemoryResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_memory")
}

func (r *MemoryResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_memory")
}

func (r *MemoryResource) ImportState(_ context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	addUnsupportedMoonshotResourceDiagnostic(&resp.Diagnostics, "streamline_memory")
}
