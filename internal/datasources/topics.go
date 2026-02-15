// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package datasources

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamline-platform/terraform-provider-streamline/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &TopicsDataSource{}

// TopicsDataSource defines the data source implementation.
type TopicsDataSource struct {
	kafkaClient *client.StreamlineClient
}

// TopicsDataSourceModel describes the data source data model.
type TopicsDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	NamePattern types.String `tfsdk:"name_pattern"`
	Topics      types.List   `tfsdk:"topics"`
}

// TopicInfo represents topic information
type TopicInfo struct {
	Name              types.String `tfsdk:"name"`
	Partitions        types.Int64  `tfsdk:"partitions"`
	ReplicationFactor types.Int64  `tfsdk:"replication_factor"`
	Internal          types.Bool   `tfsdk:"internal"`
}

// NewTopicsDataSource creates a new topics data source
func NewTopicsDataSource() datasource.DataSource {
	return &TopicsDataSource{}
}

// Metadata returns the data source type name.
func (d *TopicsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_topics"
}

// Schema defines the schema for the data source.
func (d *TopicsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves a list of topics from the Streamline cluster.",
		MarkdownDescription: `
Retrieves a list of topics from the Streamline cluster.

This data source provides information about topics including partition count and replication factor.

## Example Usage

` + "```hcl" + `
# Get all topics
data "streamline_topics" "all" {}

# Get topics matching a pattern
data "streamline_topics" "events" {
  name_pattern = "events-.*"
}

output "topic_names" {
  value = [for t in data.streamline_topics.all.topics : t.name]
}

output "total_partitions" {
  value = sum([for t in data.streamline_topics.all.topics : t.partitions])
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The data source identifier.",
			},
			"name_pattern": schema.StringAttribute{
				Optional:    true,
				Description: "Optional regex pattern to filter topics by name.",
			},
			"topics": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of topics in the cluster.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The topic name.",
						},
						"partitions": schema.Int64Attribute{
							Computed:    true,
							Description: "The number of partitions.",
						},
						"replication_factor": schema.Int64Attribute{
							Computed:    true,
							Description: "The replication factor.",
						},
						"internal": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether this is an internal topic.",
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *TopicsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *TopicsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config TopicsDataSourceModel
	var state TopicsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading topics", map[string]any{
		"name_pattern": config.NamePattern.ValueString(),
	})

	// Get all topics from Kafka
	topics, err := d.kafkaClient.ListTopics(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to List Topics",
			fmt.Sprintf("Unable to list topics: %s", err),
		)
		return
	}

	// Compile name pattern if provided
	var pattern *regexp.Regexp
	if !config.NamePattern.IsNull() && config.NamePattern.ValueString() != "" {
		var err error
		pattern, err = regexp.Compile(config.NamePattern.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid Name Pattern",
				fmt.Sprintf("Invalid regex pattern '%s': %s", config.NamePattern.ValueString(), err),
			)
			return
		}
	}

	// Build topics list
	topicAttrTypes := map[string]attr.Type{
		"name":               types.StringType,
		"partitions":         types.Int64Type,
		"replication_factor": types.Int64Type,
		"internal":           types.BoolType,
	}

	var topicValues []attr.Value
	for _, topic := range topics {
		// Filter by pattern if provided
		if pattern != nil && !pattern.MatchString(topic.Name) {
			continue
		}

		topicObj, diags := types.ObjectValue(topicAttrTypes, map[string]attr.Value{
			"name":               types.StringValue(topic.Name),
			"partitions":         types.Int64Value(int64(topic.Partitions)),
			"replication_factor": types.Int64Value(int64(topic.ReplicationFactor)),
			"internal":           types.BoolValue(topic.Internal),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		topicValues = append(topicValues, topicObj)
	}

	topicsList, diags := types.ListValue(types.ObjectType{AttrTypes: topicAttrTypes}, topicValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue("topics")
	state.NamePattern = config.NamePattern
	state.Topics = topicsList

	tflog.Info(ctx, "Read topics", map[string]any{
		"count": len(topicValues),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
