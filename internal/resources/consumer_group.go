// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

var _ resource.Resource = &ConsumerGroupResource{}
var _ resource.ResourceWithImportState = &ConsumerGroupResource{}

type consumerGroupResourceClient interface {
	DescribeConsumerGroup(context.Context, string) (*client.ConsumerGroupInfo, error)
	DeleteConsumerGroup(context.Context, string) error
}

// ConsumerGroupResource retains the legacy read/delete resource model.
// Applications, not Terraform, create consumer groups when consumers join.
type ConsumerGroupResource struct {
	kafkaClient consumerGroupResourceClient
}

type ConsumerGroupResourceModel struct {
	ID      types.String `tfsdk:"id"`
	GroupID types.String `tfsdk:"group_id"`
	State   types.String `tfsdk:"state"`
	Members types.Int64  `tfsdk:"members"`
}

func NewConsumerGroupResource() resource.Resource {
	return &ConsumerGroupResource{}
}

func (r *ConsumerGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_consumer_group"
}

func (r *ConsumerGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Legacy read/delete model for a Streamline consumer group.",
		MarkdownDescription: "Retains the legacy `streamline_consumer_group` resource schema for existing state. " +
			"Consumer groups are created only when application consumers connect, so Terraform cannot create them. " +
			"Use the `streamline_consumer_group` data source for new configurations.",
		DeprecationMessage: "Use the streamline_consumer_group data source; Terraform cannot create consumer groups.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier (same as `group_id`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Consumer group identifier.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current broker-reported group state.",
			},
			"members": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of active group members.",
			},
		},
	}
}

func (r *ConsumerGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*client.Clients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Clients, got: %T", req.ProviderData),
		)
		return
	}
	r.kafkaClient = clients.Kafka
}

func (r *ConsumerGroupResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"Consumer Group Creation Unsupported",
		"Consumer groups are created by applications when consumers connect. "+
			"Use the streamline_consumer_group data source to inspect an existing group.",
	)
}

func (r *ConsumerGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ConsumerGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := state.GroupID.ValueString()
	info, err := r.kafkaClient.DescribeConsumerGroup(ctx, groupID)
	if handleReadError(ctx, resp, "Consumer Group", groupID, err) {
		return
	}
	if info == nil {
		resp.Diagnostics.AddError(
			"Failed to Read Consumer Group",
			fmt.Sprintf("Unable to read consumer group %q: the client returned an empty response", groupID),
		)
		return
	}

	state.ID = types.StringValue(info.GroupID)
	state.GroupID = types.StringValue(info.GroupID)
	state.State = types.StringValue(info.State)
	state.Members = types.Int64Value(int64(info.Members))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ConsumerGroupResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Consumer Group Update Unsupported",
		"Consumer group identity is immutable and its runtime state is controlled by connected applications.",
	)
}

func (r *ConsumerGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ConsumerGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := state.GroupID.ValueString()
	if err := r.kafkaClient.DeleteConsumerGroup(ctx, groupID); handleDeleteError(resp, "Consumer Group", groupID, err) {
		return
	}
}

func (r *ConsumerGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	info, err := r.kafkaClient.DescribeConsumerGroup(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Consumer Group Import Failed",
			fmt.Sprintf("Unable to read consumer group %q: %s", req.ID, err),
		)
		return
	}
	if info == nil {
		resp.Diagnostics.AddError(
			"Consumer Group Import Failed",
			fmt.Sprintf("Unable to read consumer group %q: the client returned an empty response", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &ConsumerGroupResourceModel{
		ID:      types.StringValue(info.GroupID),
		GroupID: types.StringValue(info.GroupID),
		State:   types.StringValue(info.State),
		Members: types.Int64Value(int64(info.Members)),
	})...)
}
