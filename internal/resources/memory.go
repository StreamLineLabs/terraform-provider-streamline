// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

var _ resource.Resource = &MemoryResource{}
var _ resource.ResourceWithImportState = &MemoryResource{}

// MemoryResource manages a Streamline agent memory partition (Moonshot M1).
type MemoryResource struct {
	moonshot *client.MoonshotClient
}

// MemoryResourceModel describes the resource data model.
type MemoryResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	AgentID              types.String `tfsdk:"agent_id"`
	Tenant               types.String `tfsdk:"tenant"`
	EpisodicRetentionDays types.Int64  `tfsdk:"episodic_retention_days"`
	SemanticRetentionDays types.Int64  `tfsdk:"semantic_retention_days"`
	ProceduralRetentionDays types.Int64 `tfsdk:"procedural_retention_days"`
	DecayHalfLifeDays    types.Float64 `tfsdk:"decay_half_life_days"`
	DecayThreshold       types.Float64 `tfsdk:"decay_threshold"`
	EncryptionEnabled    types.Bool   `tfsdk:"encryption_enabled"`
}

// NewMemoryResource builds a new memory resource.
func NewMemoryResource() resource.Resource {
	return &MemoryResource{}
}

func (r *MemoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_memory"
}

func (r *MemoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages a Streamline agent memory partition (Moonshot M1). Provisions episodic, semantic, and procedural memory topics for an AI agent.",
		MarkdownDescription: "Manages a Streamline agent memory partition (M1 agentic memory). Provisions per-agent memory topics with configurable retention and decay. Requires `moonshot_url` on the provider.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier (`{tenant}/{agent_id}`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"agent_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique agent identifier that owns this memory partition.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tenant": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Tenant namespace for multi-tenant deployments.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"episodic_retention_days": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(30),
				MarkdownDescription: "Retention in days for episodic (event-based) memories. Default: 30.",
			},
			"semantic_retention_days": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(365),
				MarkdownDescription: "Retention in days for semantic (knowledge) memories. Default: 365.",
			},
			"procedural_retention_days": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(-1),
				MarkdownDescription: "Retention in days for procedural (skill) memories. -1 for infinite. Default: -1.",
			},
			"decay_half_life_days": schema.Float64Attribute{
				Optional:            true,
				MarkdownDescription: "Half-life in days for memory decay scoring. Controls how quickly memories lose relevance.",
			},
			"decay_threshold": schema.Float64Attribute{
				Optional:            true,
				MarkdownDescription: "Score threshold below which memories are eligible for garbage collection.",
			},
			"encryption_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Enable encryption at rest for memory topics.",
			},
		},
	}
}

func (r *MemoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
			"streamline_memory requires the provider's moonshot_url to be set (or STREAMLINE_MOONSHOT_URL env var).")
		return
	}
	r.moonshot = clients.Moonshot
}

func (r *MemoryResource) buildMemory(data MemoryResourceModel) client.Memory {
	m := client.Memory{
		AgentID:  data.AgentID.ValueString(),
		Tenant:   data.Tenant.ValueString(),
		Tiers: client.MemoryTiers{
			EpisodicRetentionDays:  data.EpisodicRetentionDays.ValueInt64(),
			SemanticRetentionDays:  data.SemanticRetentionDays.ValueInt64(),
			ProceduralRetentionDays: data.ProceduralRetentionDays.ValueInt64(),
		},
		EncryptionEnabled: data.EncryptionEnabled.ValueBool(),
	}
	if !data.DecayHalfLifeDays.IsNull() && !data.DecayHalfLifeDays.IsUnknown() {
		halfLife := data.DecayHalfLifeDays.ValueFloat64()
		m.Decay = &client.MemoryDecay{HalfLifeDays: halfLife}
		if !data.DecayThreshold.IsNull() && !data.DecayThreshold.IsUnknown() {
			m.Decay.Threshold = data.DecayThreshold.ValueFloat64()
		}
	}
	return m
}

func (r *MemoryResource) applyServerState(data *MemoryResourceModel, m *client.Memory) {
	data.ID = types.StringValue(fmt.Sprintf("%s/%s", m.Tenant, m.AgentID))
	data.AgentID = types.StringValue(m.AgentID)
	data.Tenant = types.StringValue(m.Tenant)
	data.EpisodicRetentionDays = types.Int64Value(m.Tiers.EpisodicRetentionDays)
	data.SemanticRetentionDays = types.Int64Value(m.Tiers.SemanticRetentionDays)
	data.ProceduralRetentionDays = types.Int64Value(m.Tiers.ProceduralRetentionDays)
	data.EncryptionEnabled = types.BoolValue(m.EncryptionEnabled)
	if m.Decay != nil {
		data.DecayHalfLifeDays = types.Float64Value(m.Decay.HalfLifeDays)
		data.DecayThreshold = types.Float64Value(m.Decay.Threshold)
	}
}

func (r *MemoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MemoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	memory := r.buildMemory(data)
	tflog.Trace(ctx, "creating memory", map[string]interface{}{"agent_id": memory.AgentID, "tenant": memory.Tenant})
	out, err := r.moonshot.CreateMemory(ctx, memory)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create memory",
			fmt.Sprintf("Could not create memory for agent %q: %s", memory.AgentID, err))
		return
	}
	r.applyServerState(&data, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MemoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MemoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.moonshot.GetMemory(ctx, data.Tenant.ValueString(), data.AgentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read memory",
			fmt.Sprintf("Could not read memory for agent %q: %s", data.AgentID.ValueString(), err))
		return
	}
	if out == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	r.applyServerState(&data, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MemoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data MemoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	memory := r.buildMemory(data)
	out, err := r.moonshot.CreateMemory(ctx, memory)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update memory",
			fmt.Sprintf("Could not update memory for agent %q: %s", memory.AgentID, err))
		return
	}
	r.applyServerState(&data, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MemoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MemoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.moonshot.DeleteMemory(ctx, data.Tenant.ValueString(), data.AgentID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete memory",
			fmt.Sprintf("Could not delete memory for agent %q: %s", data.AgentID.ValueString(), err))
		return
	}
}

func (r *MemoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: tenant/agent_id
	parts := splitImportID(req.ID)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Import failed",
			fmt.Sprintf("Expected import ID format 'tenant/agent_id', got %q", req.ID))
		return
	}
	tenant, agentID := parts[0], parts[1]
	out, err := r.moonshot.GetMemory(ctx, tenant, agentID)
	if err != nil {
		resp.Diagnostics.AddError("Import failed",
			fmt.Sprintf("Could not fetch memory for %q/%q: %s", tenant, agentID, err))
		return
	}
	if out == nil {
		resp.Diagnostics.AddError("Import failed",
			fmt.Sprintf("Memory for %q/%q not found", tenant, agentID))
		return
	}
	var model MemoryResourceModel
	r.applyServerState(&model, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// splitImportID splits a tenant/agent_id import ID.
func splitImportID(id string) []string {
	idx := -1
	for i, c := range id {
		if c == '/' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return []string{id}
	}
	return []string{id[:idx], id[idx+1:]}
}
