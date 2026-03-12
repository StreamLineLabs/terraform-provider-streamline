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

var _ resource.Resource = &ConsumerGroupResource{}
var _ resource.ResourceWithImportState = &ConsumerGroupResource{}

// ConsumerGroupResource defines the consumer group resource implementation.
type ConsumerGroupResource struct {
	kafkaClient *client.StreamlineClient
}

// ConsumerGroupResourceModel describes the resource data model.
type ConsumerGroupResourceModel struct {
	ID      types.String `tfsdk:"id"`
	GroupID types.String `tfsdk:"group_id"`
	State   types.String `tfsdk:"state"`
	Members types.Int64  `tfsdk:"members"`
}

// NewConsumerGroupResource creates a new consumer group resource.
func NewConsumerGroupResource() resource.Resource {
	return &ConsumerGroupResource{}
}

func (r *ConsumerGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_consumer_group"
}

func (r *ConsumerGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages a Streamline consumer group.",
		MarkdownDescription: "Manages a Streamline consumer group. Allows reading state and deleting consumer groups via Terraform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier (same as group_id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Consumer group ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current state of the consumer group (e.g., Stable, Empty, Dead).",
			},
			"members": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of active members in the consumer group.",
			},
		},
	}
}

func (r *ConsumerGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	kafkaClient, ok := req.ProviderData.(*client.StreamlineClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.StreamlineClient, got: %T", req.ProviderData))
		return
	}
	r.kafkaClient = kafkaClient
}

func (r *ConsumerGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ConsumerGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := data.GroupID.ValueString()
	tflog.Trace(ctx, "managing consumer group", map[string]interface{}{"group_id": groupID})

	// Consumer groups are created implicitly when consumers join. Read current state.
	info, err := r.kafkaClient.DescribeConsumerGroup(ctx, groupID)
	if err != nil {
		// Group may not exist yet — that's fine for a managed resource
		data.ID = types.StringValue(groupID)
		data.State = types.StringValue("Empty")
		data.Members = types.Int64Value(0)
	} else {
		data.ID = types.StringValue(groupID)
		data.State = types.StringValue(info.State)
		data.Members = types.Int64Value(int64(info.Members))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ConsumerGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ConsumerGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := data.GroupID.ValueString()
	info, err := r.kafkaClient.DescribeConsumerGroup(ctx, groupID)
	if err != nil {
		// Group has been deleted externally
		resp.State.RemoveResource(ctx)
		return
	}

	data.State = types.StringValue(info.State)
	data.Members = types.Int64Value(int64(info.Members))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ConsumerGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ConsumerGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Consumer groups are immutable — group_id change requires replace.
	// State/members are computed.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ConsumerGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ConsumerGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := data.GroupID.ValueString()
	tflog.Trace(ctx, "deleting consumer group", map[string]interface{}{"group_id": groupID})

	if err := r.kafkaClient.DeleteConsumerGroup(ctx, groupID); err != nil {
		resp.Diagnostics.AddError("Failed to delete consumer group",
			fmt.Sprintf("Could not delete consumer group %s: %s", groupID, err))
		return
	}
}

func (r *ConsumerGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by group_id
	groupID := req.ID
	info, err := r.kafkaClient.DescribeConsumerGroup(ctx, groupID)
	if err != nil {
		resp.Diagnostics.AddError("Import failed",
			fmt.Sprintf("Could not find consumer group %s: %s", groupID, err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &ConsumerGroupResourceModel{
		ID:      types.StringValue(groupID),
		GroupID: types.StringValue(groupID),
		State:   types.StringValue(info.State),
		Members: types.Int64Value(int64(info.Members)),
	})...)
}
