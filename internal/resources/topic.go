// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TopicResource{}
var _ resource.ResourceWithImportState = &TopicResource{}

// TopicResource defines the resource implementation.
type TopicResource struct {
	kafkaClient *client.StreamlineClient
}

// TopicResourceModel describes the resource data model.
type TopicResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Partitions      types.Int64  `tfsdk:"partitions"`
	ReplicationFactor types.Int64  `tfsdk:"replication_factor"`
	Config          types.Map    `tfsdk:"config"`
	CleanupPolicy   types.String `tfsdk:"cleanup_policy"`
	RetentionMs     types.Int64  `tfsdk:"retention_ms"`
	RetentionBytes  types.Int64  `tfsdk:"retention_bytes"`
	SegmentBytes    types.Int64  `tfsdk:"segment_bytes"`
	MinInSyncReplicas types.Int64  `tfsdk:"min_insync_replicas"`
	SemanticEmbed   types.Bool   `tfsdk:"semantic_embed"`
	SemanticModel   types.String `tfsdk:"semantic_model"`
	SemanticField   types.String `tfsdk:"semantic_field"`
}

// NewTopicResource creates a new topic resource
func NewTopicResource() resource.Resource {
	return &TopicResource{}
}

// Metadata returns the resource type name.
func (r *TopicResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_topic"
}

// Schema defines the schema for the resource.
func (r *TopicResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Streamline topic.",
		MarkdownDescription: `
Manages a Streamline topic.

Topics are the fundamental unit of organization in Streamline. Each topic is a category or feed name to which records are published.

## Example Usage

` + "```hcl" + `
resource "streamline_topic" "events" {
  name       = "events"
  partitions = 6

  config = {
    "retention.ms"    = "604800000"  # 7 days
    "cleanup.policy"  = "delete"
  }
}

resource "streamline_topic" "user_state" {
  name              = "user-state"
  partitions        = 12
  cleanup_policy    = "compact"
  retention_ms      = -1  # Infinite retention for compacted topics
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the topic (same as name).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the topic. Must be unique within the cluster.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"partitions": schema.Int64Attribute{
				Required:    true,
				Description: "The number of partitions for the topic. Can only be increased, not decreased.",
			},
			"replication_factor": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "The replication factor for the topic (default: 1).",
				PlanModifiers: []planmodifier.Int64{
					// Replication factor cannot be changed after creation
				},
			},
			"config": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Topic configuration overrides as key-value pairs.",
			},
			"cleanup_policy": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The cleanup policy for the topic: 'delete', 'compact', or 'compact,delete' (default: 'delete').",
			},
			"retention_ms": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Retention time in milliseconds. -1 for infinite retention.",
			},
			"retention_bytes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Retention size in bytes per partition. -1 for unlimited.",
			},
			"segment_bytes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Segment file size in bytes.",
			},
			"min_insync_replicas": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "Minimum number of in-sync replicas required for a write to succeed.",
			},
			"semantic_embed": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable automatic semantic embedding for messages on this topic (Moonshot M2).",
			},
			"semantic_model": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Embedding model to use when semantic_embed is enabled (e.g. 'bge-small', 'bge-large'). Default: 'bge-small'.",
			},
			"semantic_field": schema.StringAttribute{
				Optional:    true,
				Description: "JSON field path to embed when semantic_embed is enabled. When empty the entire message value is embedded.",
			},
		},
	}
}

// ProviderClients is a type alias to access the provider's client container
type ProviderClients struct {
	Kafka          *client.StreamlineClient
	SchemaRegistry *client.SchemaRegistryClient
	Moonshot       *client.MoonshotClient
}

// Configure adds the provider configured client to the resource.
func (r *TopicResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*ProviderClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderClients, got: %T", req.ProviderData),
		)
		return
	}

	r.kafkaClient = clients.Kafka
}

// Create creates the resource and sets the initial Terraform state.
func (r *TopicResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TopicResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating topic", map[string]any{
		"name":       plan.Name.ValueString(),
		"partitions": plan.Partitions.ValueInt64(),
	})

	// Build topic configuration
	topicConfig := client.TopicConfig{
		Name:              plan.Name.ValueString(),
		Partitions:        int(plan.Partitions.ValueInt64()),
		ReplicationFactor: int(plan.ReplicationFactor.ValueInt64()),
		Config:            make(map[string]string),
	}

	// Set optional config values
	if !plan.CleanupPolicy.IsNull() {
		topicConfig.Config["cleanup.policy"] = plan.CleanupPolicy.ValueString()
	}
	if !plan.RetentionMs.IsNull() {
		topicConfig.Config["retention.ms"] = fmt.Sprintf("%d", plan.RetentionMs.ValueInt64())
	}
	if !plan.RetentionBytes.IsNull() {
		topicConfig.Config["retention.bytes"] = fmt.Sprintf("%d", plan.RetentionBytes.ValueInt64())
	}
	if !plan.SegmentBytes.IsNull() {
		topicConfig.Config["segment.bytes"] = fmt.Sprintf("%d", plan.SegmentBytes.ValueInt64())
	}
	if !plan.MinInSyncReplicas.IsNull() {
		topicConfig.Config["min.insync.replicas"] = fmt.Sprintf("%d", plan.MinInSyncReplicas.ValueInt64())
	}

	// Set semantic configuration
	if !plan.SemanticEmbed.IsNull() && plan.SemanticEmbed.ValueBool() {
		topicConfig.Config["semantic.embed"] = "on"
	}
	if !plan.SemanticModel.IsNull() && !plan.SemanticModel.IsUnknown() {
		topicConfig.Config["semantic.model"] = plan.SemanticModel.ValueString()
	}
	if !plan.SemanticField.IsNull() && !plan.SemanticField.IsUnknown() {
		topicConfig.Config["semantic.field"] = plan.SemanticField.ValueString()
	}

	// Add any additional config from the config map
	if !plan.Config.IsNull() {
		var configMap map[string]string
		resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &configMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for k, v := range configMap {
			topicConfig.Config[k] = v
		}
	}

	// Create the topic
	err := r.kafkaClient.CreateTopic(ctx, topicConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Create Topic",
			fmt.Sprintf("Unable to create topic %s: %s", plan.Name.ValueString(), err),
		)
		return
	}

	// Read back the topic to get computed values
	topicInfo, err := r.kafkaClient.GetTopic(ctx, plan.Name.ValueString())
	if err != nil {
		tflog.Warn(ctx, "Failed to read back topic after creation, using plan values", map[string]any{
			"error": err.Error(),
		})
		// Set computed values from plan
		plan.ID = plan.Name
		if plan.CleanupPolicy.IsNull() {
			plan.CleanupPolicy = types.StringValue("delete")
		}
		if plan.RetentionMs.IsNull() {
			plan.RetentionMs = types.Int64Value(604800000) // 7 days default
		}
		if plan.RetentionBytes.IsNull() {
			plan.RetentionBytes = types.Int64Value(-1)
		}
		if plan.SegmentBytes.IsNull() {
			plan.SegmentBytes = types.Int64Value(1073741824) // 1GB default
		}
	} else {
		// Use values from server
		plan.ID = types.StringValue(topicInfo.Name)
		plan.Partitions = types.Int64Value(int64(topicInfo.Partitions))
		plan.ReplicationFactor = types.Int64Value(int64(topicInfo.ReplicationFactor))
		if v, ok := topicInfo.Config["cleanup.policy"]; ok {
			plan.CleanupPolicy = types.StringValue(v)
		} else if plan.CleanupPolicy.IsNull() {
			plan.CleanupPolicy = types.StringValue("delete")
		}
		if v, ok := topicInfo.Config["retention.ms"]; ok {
			if ms, err := parseint64(v); err == nil {
				plan.RetentionMs = types.Int64Value(ms)
			}
		} else if plan.RetentionMs.IsNull() {
			plan.RetentionMs = types.Int64Value(604800000)
		}
		if v, ok := topicInfo.Config["retention.bytes"]; ok {
			if b, err := parseint64(v); err == nil {
				plan.RetentionBytes = types.Int64Value(b)
			}
		} else if plan.RetentionBytes.IsNull() {
			plan.RetentionBytes = types.Int64Value(-1)
		}
		if v, ok := topicInfo.Config["segment.bytes"]; ok {
			if b, err := parseint64(v); err == nil {
				plan.SegmentBytes = types.Int64Value(b)
			}
		} else if plan.SegmentBytes.IsNull() {
			plan.SegmentBytes = types.Int64Value(1073741824)
		}
		if v, ok := topicInfo.Config["min.insync.replicas"]; ok {
			if i, err := parseint64(v); err == nil {
				plan.MinInSyncReplicas = types.Int64Value(i)
			}
		}
		// Semantic config
		if v, ok := topicInfo.Config["semantic.embed"]; ok {
			plan.SemanticEmbed = types.BoolValue(v == "on")
		} else if plan.SemanticEmbed.IsNull() {
			plan.SemanticEmbed = types.BoolValue(false)
		}
		if v, ok := topicInfo.Config["semantic.model"]; ok {
			plan.SemanticModel = types.StringValue(v)
		} else if plan.SemanticModel.IsNull() {
			plan.SemanticModel = types.StringValue("bge-small")
		}
		if v, ok := topicInfo.Config["semantic.field"]; ok {
			plan.SemanticField = types.StringValue(v)
		}
	}

	tflog.Info(ctx, "Created topic", map[string]any{
		"name": plan.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// parseint64 parses a string to int64
func parseint64(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// Read refreshes the Terraform state with the latest data.
func (r *TopicResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TopicResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading topic", map[string]any{
		"name": state.Name.ValueString(),
	})

	// Get topic information from Kafka
	topicInfo, err := r.kafkaClient.GetTopic(ctx, state.Name.ValueString())
	if err != nil {
		// Check if topic was deleted outside of Terraform
		resp.Diagnostics.AddWarning(
			"Topic Not Found",
			fmt.Sprintf("Topic %s may have been deleted outside of Terraform: %s", state.Name.ValueString(), err),
		)
		resp.State.RemoveResource(ctx)
		return
	}

	// Update state from server
	state.ID = types.StringValue(topicInfo.Name)
	state.Name = types.StringValue(topicInfo.Name)
	state.Partitions = types.Int64Value(int64(topicInfo.Partitions))
	state.ReplicationFactor = types.Int64Value(int64(topicInfo.ReplicationFactor))

	// Update config values
	if v, ok := topicInfo.Config["cleanup.policy"]; ok {
		state.CleanupPolicy = types.StringValue(v)
	}
	if v, ok := topicInfo.Config["retention.ms"]; ok {
		if ms, err := parseint64(v); err == nil {
			state.RetentionMs = types.Int64Value(ms)
		}
	}
	if v, ok := topicInfo.Config["retention.bytes"]; ok {
		if b, err := parseint64(v); err == nil {
			state.RetentionBytes = types.Int64Value(b)
		}
	}
	if v, ok := topicInfo.Config["segment.bytes"]; ok {
		if b, err := parseint64(v); err == nil {
			state.SegmentBytes = types.Int64Value(b)
		}
	}
	if v, ok := topicInfo.Config["min.insync.replicas"]; ok {
		if i, err := parseint64(v); err == nil {
			state.MinInSyncReplicas = types.Int64Value(i)
		}
	}
	// Semantic config
	if v, ok := topicInfo.Config["semantic.embed"]; ok {
		state.SemanticEmbed = types.BoolValue(v == "on")
	}
	if v, ok := topicInfo.Config["semantic.model"]; ok {
		state.SemanticModel = types.StringValue(v)
	}
	if v, ok := topicInfo.Config["semantic.field"]; ok {
		state.SemanticField = types.StringValue(v)
	}

	tflog.Info(ctx, "Read topic", map[string]any{
		"name":       state.Name.ValueString(),
		"partitions": state.Partitions.ValueInt64(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *TopicResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TopicResourceModel
	var state TopicResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating topic", map[string]any{
		"name": plan.Name.ValueString(),
	})

	// Check partition count - can only increase
	if plan.Partitions.ValueInt64() < state.Partitions.ValueInt64() {
		resp.Diagnostics.AddAttributeError(
			path.Root("partitions"),
			"Cannot Decrease Partition Count",
			fmt.Sprintf("Partition count can only be increased. Current: %d, Requested: %d",
				state.Partitions.ValueInt64(), plan.Partitions.ValueInt64()),
		)
		return
	}

	// Build update configuration
	updateConfig := client.TopicConfig{
		Name:       plan.Name.ValueString(),
		Partitions: int(plan.Partitions.ValueInt64()),
		Config:     make(map[string]string),
	}

	// Set config values
	if !plan.CleanupPolicy.IsNull() {
		updateConfig.Config["cleanup.policy"] = plan.CleanupPolicy.ValueString()
	}
	if !plan.RetentionMs.IsNull() {
		updateConfig.Config["retention.ms"] = fmt.Sprintf("%d", plan.RetentionMs.ValueInt64())
	}
	if !plan.RetentionBytes.IsNull() {
		updateConfig.Config["retention.bytes"] = fmt.Sprintf("%d", plan.RetentionBytes.ValueInt64())
	}
	if !plan.SegmentBytes.IsNull() {
		updateConfig.Config["segment.bytes"] = fmt.Sprintf("%d", plan.SegmentBytes.ValueInt64())
	}
	if !plan.MinInSyncReplicas.IsNull() {
		updateConfig.Config["min.insync.replicas"] = fmt.Sprintf("%d", plan.MinInSyncReplicas.ValueInt64())
	}

	// Semantic configuration
	if !plan.SemanticEmbed.IsNull() && plan.SemanticEmbed.ValueBool() {
		updateConfig.Config["semantic.embed"] = "on"
	} else if !plan.SemanticEmbed.IsNull() {
		updateConfig.Config["semantic.embed"] = "off"
	}
	if !plan.SemanticModel.IsNull() && !plan.SemanticModel.IsUnknown() {
		updateConfig.Config["semantic.model"] = plan.SemanticModel.ValueString()
	}
	if !plan.SemanticField.IsNull() && !plan.SemanticField.IsUnknown() {
		updateConfig.Config["semantic.field"] = plan.SemanticField.ValueString()
	}

	// Add any additional config from the config map
	if !plan.Config.IsNull() {
		var configMap map[string]string
		resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &configMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for k, v := range configMap {
			updateConfig.Config[k] = v
		}
	}

	// Update the topic
	err := r.kafkaClient.UpdateTopic(ctx, updateConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Update Topic",
			fmt.Sprintf("Unable to update topic %s: %s", plan.Name.ValueString(), err),
		)
		return
	}

	plan.ID = plan.Name

	tflog.Info(ctx, "Updated topic", map[string]any{
		"name": plan.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *TopicResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TopicResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting topic", map[string]any{
		"name": state.Name.ValueString(),
	})

	// Delete the topic
	err := r.kafkaClient.DeleteTopic(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Delete Topic",
			fmt.Sprintf("Unable to delete topic %s: %s", state.Name.ValueString(), err),
		)
		return
	}

	tflog.Info(ctx, "Deleted topic", map[string]any{
		"name": state.Name.ValueString(),
	})
}

// ImportState imports an existing resource into Terraform state.
func (r *TopicResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

