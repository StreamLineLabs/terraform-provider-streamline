// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

var _ datasource.DataSource = &ConsumerGroupDataSource{}

type consumerGroupClient interface {
	DescribeConsumerGroup(context.Context, string) (*client.ConsumerGroupInfo, error)
}

// ConsumerGroupDataSource reads a consumer group created by an application.
type ConsumerGroupDataSource struct {
	kafkaClient consumerGroupClient
}

// ConsumerGroupDataSourceModel describes the consumer group data source model.
type ConsumerGroupDataSourceModel struct {
	GroupID types.String `tfsdk:"group_id"`
	State   types.String `tfsdk:"state"`
	Members types.Int64  `tfsdk:"members"`
}

// NewConsumerGroupDataSource creates a consumer group data source.
func NewConsumerGroupDataSource() datasource.DataSource {
	return &ConsumerGroupDataSource{}
}

func (d *ConsumerGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_consumer_group"
}

func (d *ConsumerGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves a consumer group that was created by connecting consumers.",
		MarkdownDescription: "Retrieves a Streamline consumer group that already exists. " +
			"Consumer groups are created by applications when consumers connect; Terraform does not create them.",
		Attributes: map[string]schema.Attribute{
			"group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Consumer group identifier.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
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

func (d *ConsumerGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*client.Clients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Clients, got: %T", req.ProviderData),
		)
		return
	}

	d.kafkaClient = clients.Kafka
}

func (d *ConsumerGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ConsumerGroupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := data.GroupID.ValueString()
	tflog.Debug(ctx, "Reading consumer group", map[string]any{"group_id": groupID})

	info, err := d.kafkaClient.DescribeConsumerGroup(ctx, groupID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Read Consumer Group",
			fmt.Sprintf("Unable to read consumer group %q: %s", groupID, err),
		)
		return
	}

	data.GroupID = types.StringValue(info.GroupID)
	data.State = types.StringValue(info.State)
	data.Members = types.Int64Value(int64(info.Members))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
