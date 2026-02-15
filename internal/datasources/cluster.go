// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamline-platform/terraform-provider-streamline/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ClusterDataSource{}

// ProviderClients is a type alias to access the provider's client container
type ProviderClients struct {
	Kafka          *client.StreamlineClient
	SchemaRegistry *client.SchemaRegistryClient
}

// ClusterDataSource defines the data source implementation.
type ClusterDataSource struct {
	kafkaClient *client.StreamlineClient
}

// ClusterDataSourceModel describes the data source data model.
type ClusterDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	ClusterID    types.String `tfsdk:"cluster_id"`
	ControllerID types.Int64  `tfsdk:"controller_id"`
	Brokers      types.List   `tfsdk:"brokers"`
}

// BrokerModel represents a broker in the cluster
type BrokerModel struct {
	ID   types.Int64  `tfsdk:"id"`
	Host types.String `tfsdk:"host"`
	Port types.Int64  `tfsdk:"port"`
	Rack types.String `tfsdk:"rack"`
}

// NewClusterDataSource creates a new cluster data source
func NewClusterDataSource() datasource.DataSource {
	return &ClusterDataSource{}
}

// Metadata returns the data source type name.
func (d *ClusterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

// Schema defines the schema for the data source.
func (d *ClusterDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves information about the Streamline cluster.",
		MarkdownDescription: `
Retrieves information about the Streamline cluster.

This data source provides metadata about the connected cluster including broker information and the controller node.

## Example Usage

` + "```hcl" + `
data "streamline_cluster" "current" {}

output "cluster_id" {
  value = data.streamline_cluster.current.cluster_id
}

output "controller_id" {
  value = data.streamline_cluster.current.controller_id
}

output "broker_count" {
  value = length(data.streamline_cluster.current.brokers)
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The data source identifier.",
			},
			"cluster_id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier for the cluster.",
			},
			"controller_id": schema.Int64Attribute{
				Computed:    true,
				Description: "The broker ID of the current controller.",
			},
			"brokers": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of brokers in the cluster.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed:    true,
							Description: "The broker ID.",
						},
						"host": schema.StringAttribute{
							Computed:    true,
							Description: "The broker hostname.",
						},
						"port": schema.Int64Attribute{
							Computed:    true,
							Description: "The broker port.",
						},
						"rack": schema.StringAttribute{
							Computed:    true,
							Description: "The rack ID of the broker (if configured).",
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *ClusterDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*ProviderClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ProviderClients, got: %T", req.ProviderData),
		)
		return
	}

	d.kafkaClient = clients.Kafka
}

// Read refreshes the Terraform state with the latest data.
func (d *ClusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ClusterDataSourceModel

	tflog.Debug(ctx, "Reading cluster metadata")

	// Get cluster metadata from Kafka
	metadata, err := d.kafkaClient.GetClusterMetadata(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Get Cluster Metadata",
			fmt.Sprintf("Unable to get cluster metadata: %s", err),
		)
		return
	}

	// Set state from metadata
	state.ID = types.StringValue("cluster")
	state.ClusterID = types.StringValue(metadata.ClusterID)
	state.ControllerID = types.Int64Value(int64(metadata.ControllerID))

	// Build brokers list
	brokerAttrTypes := map[string]attr.Type{
		"id":   types.Int64Type,
		"host": types.StringType,
		"port": types.Int64Type,
		"rack": types.StringType,
	}

	var brokerValues []attr.Value
	for _, broker := range metadata.Brokers {
		rackValue := types.StringNull()
		if broker.Rack != "" {
			rackValue = types.StringValue(broker.Rack)
		}

		brokerObj, diags := types.ObjectValue(brokerAttrTypes, map[string]attr.Value{
			"id":   types.Int64Value(int64(broker.ID)),
			"host": types.StringValue(broker.Host),
			"port": types.Int64Value(int64(broker.Port)),
			"rack": rackValue,
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		brokerValues = append(brokerValues, brokerObj)
	}

	brokersList, diags := types.ListValue(types.ObjectType{AttrTypes: brokerAttrTypes}, brokerValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Brokers = brokersList

	tflog.Info(ctx, "Read cluster metadata", map[string]any{
		"cluster_id":    state.ClusterID.ValueString(),
		"controller_id": state.ControllerID.ValueInt64(),
		"broker_count":  len(metadata.Brokers),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
