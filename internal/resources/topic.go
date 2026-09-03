// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TopicResource{}
var _ resource.ResourceWithImportState = &TopicResource{}
var _ resource.ResourceWithValidateConfig = &TopicResource{}

// TopicResource defines the resource implementation.
type TopicResource struct {
	kafkaClient *client.StreamlineClient
}

// TopicResourceModel describes the resource data model.
type TopicResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Partitions        types.Int64  `tfsdk:"partitions"`
	ReplicationFactor types.Int64  `tfsdk:"replication_factor"`
	Config            types.Map    `tfsdk:"config"`
	CleanupPolicy     types.String `tfsdk:"cleanup_policy"`
	RetentionMs       types.Int64  `tfsdk:"retention_ms"`
	RetentionBytes    types.Int64  `tfsdk:"retention_bytes"`
	SegmentBytes      types.Int64  `tfsdk:"segment_bytes"`
	MinInSyncReplicas types.Int64  `tfsdk:"min_insync_replicas"`
	SemanticEmbed     types.Bool   `tfsdk:"semantic_embed"`
	SemanticModel     types.String `tfsdk:"semantic_model"`
	SemanticField     types.String `tfsdk:"semantic_field"`
}

type topicConfigMapValidator struct{}

func (topicConfigMapValidator) Description(context.Context) string {
	return "map keys and values must be supported by Streamline 0.3.0"
}

func (topicConfigMapValidator) MarkdownDescription(ctx context.Context) string {
	return (topicConfigMapValidator{}).Description(ctx)
}

func (topicConfigMapValidator) ValidateMap(
	_ context.Context,
	req validator.MapRequest,
	resp *validator.MapResponse,
) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	for key, element := range req.ConfigValue.Elements() {
		value, ok := element.(types.String)
		if !ok || value.IsNull() || value.IsUnknown() {
			continue
		}
		stringValue := value.ValueString()
		switch key {
		case "cleanup.policy":
			if stringValue != "delete" &&
				stringValue != "compact" &&
				stringValue != "delete,compact" &&
				stringValue != "compact,delete" {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid Topic Configuration",
					"cleanup.policy must be delete, compact, delete,compact, or compact,delete.",
				)
			}
		case "retention.ms", "retention.bytes":
			parsed, err := strconv.ParseInt(stringValue, 10, 64)
			if err != nil || parsed < -1 {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid Topic Configuration",
					fmt.Sprintf("%s must be a base-10 integer greater than or equal to -1.", key),
				)
			}
		case "segment.bytes":
			parsed, err := strconv.ParseUint(stringValue, 10, 64)
			if err != nil || parsed == 0 {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid Topic Configuration",
					"segment.bytes must be a positive base-10 integer.",
				)
			}
		case "storage.mode":
			if stringValue != "local" && stringValue != "hybrid" && stringValue != "diskless" {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid Topic Configuration",
					"storage.mode must be one of local, hybrid, or diskless.",
				)
			}
		case "message.ttl.ms",
			"message.timestamp.difference.max.ms",
			"delete.retention.ms",
			"min.compaction.lag.ms":
			if _, err := strconv.ParseInt(stringValue, 10, 64); err != nil {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid Topic Configuration",
					fmt.Sprintf("%s must be a base-10 integer: %s", key, err),
				)
			}
		case "min.cleanable.dirty.ratio":
			ratio, err := strconv.ParseFloat(stringValue, 64)
			if err != nil || ratio < 0 || ratio > 1 {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid Topic Configuration",
					"min.cleanable.dirty.ratio must be a number between 0 and 1.",
				)
			}
		default:
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Unsupported Topic Configuration",
				fmt.Sprintf(
					"%s is not applied by Streamline 0.3.0. Supported map keys are cleanup.policy, retention.ms, retention.bytes, segment.bytes, storage.mode, message.ttl.ms, message.timestamp.difference.max.ms, min.cleanable.dirty.ratio, delete.retention.ms, and min.compaction.lag.ms.",
					key,
				),
			)
		}
	}
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
  name           = "events"
  partitions     = 6
  retention_ms   = 604800000 # 7 days
  cleanup_policy = "delete"
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
				Validators: []validator.String{
					stringvalidator.All(
						stringvalidator.LengthBetween(1, 249),
						stringvalidator.RegexMatches(
							regexp.MustCompile(`^[A-Za-z0-9._-]+$`),
							"must contain only letters, digits, periods, underscores, and hyphens",
						),
						stringvalidator.NoneOf(".", ".."),
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"partitions": schema.Int64Attribute{
				Required:    true,
				Description: "The number of partitions for the topic. Can only be increased, not decreased.",
				Validators: []validator.Int64{
					int64validator.Between(1, math.MaxInt32),
				},
			},
			"replication_factor": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "The replication factor. Streamline 0.3.0 supports only 1.",
				Validators: []validator.Int64{
					int64validator.OneOf(1),
				},
			},
			"config": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Supported Streamline 0.3.0 topic configuration overrides applied at creation. Keys represented by dedicated attributes (cleanup.policy, retention.ms, retention.bytes, and segment.bytes) must be configured only through those attributes. Configured changes require replacement; removing configuration preserves its last known create-time state because readback is unavailable.",
				Validators: []validator.Map{
					topicConfigMapValidator{},
				},
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
					mapplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"cleanup_policy": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cleanup policy applied at creation. Configured changes require replacement; removing it preserves the create-time state because readback is unavailable.",
				Validators: []validator.String{
					stringvalidator.OneOf("delete", "compact", "delete,compact", "compact,delete"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"retention_ms": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Retention time applied at creation, in milliseconds. -1 means infinite retention. Configured changes require replacement.",
				Validators: []validator.Int64{
					int64validator.AtLeast(-1),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"retention_bytes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Retention size applied at creation, in bytes per partition. -1 means unlimited. Configured changes require replacement.",
				Validators: []validator.Int64{
					int64validator.AtLeast(-1),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"segment_bytes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Segment file size applied at creation, in bytes. Configured changes require replacement.",
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"min_insync_replicas": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Legacy minimum in-sync replicas setting. Streamline 0.3.0 ignores configured values; new configuration is rejected before creation.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"semantic_embed": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Legacy semantic embedding setting. Streamline 0.3.0 ignores configured values; new configuration is rejected before creation.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
					boolplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"semantic_model": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Legacy semantic model setting. Streamline 0.3.0 ignores configured values; new configuration is rejected before creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"semantic_field": schema.StringAttribute{
				Optional:    true,
				Description: "Legacy semantic field setting. Streamline 0.3.0 ignores configured values; new configuration is rejected before creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
		},
	}
}

// ValidateConfig rejects ambiguous topic configuration before planning. Without
// this check, map values silently overwrite dedicated attributes during Create
// while Terraform persists both contradictory inputs in state.
func (r *TopicResource) ValidateConfig(
	ctx context.Context,
	req resource.ValidateConfigRequest,
	resp *resource.ValidateConfigResponse,
) {
	var config TopicResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	configKeys, known := topicConfigKeys(config.Config)
	if !known {
		return
	}

	if key, attribute, conflict := duplicateDedicatedTopicConfig(configKeys, config); conflict {
		resp.Diagnostics.AddAttributeError(
			path.Root("config").AtMapKey(key),
			"Conflicting Topic Configuration",
			fmt.Sprintf(
				"config key %q duplicates the dedicated %q attribute. Configure this setting in exactly one place.",
				key,
				attribute,
			),
		)
	}
}

func duplicateDedicatedTopicConfig(
	configKeys map[string]struct{},
	config TopicResourceModel,
) (key, attribute string, conflict bool) {
	dedicated := []struct {
		key        string
		attribute  string
		configured bool
	}{
		{"cleanup.policy", "cleanup_policy", !config.CleanupPolicy.IsNull()},
		{"retention.ms", "retention_ms", !config.RetentionMs.IsNull()},
		{"retention.bytes", "retention_bytes", !config.RetentionBytes.IsNull()},
		{"segment.bytes", "segment_bytes", !config.SegmentBytes.IsNull()},
	}
	for _, setting := range dedicated {
		if _, exists := configKeys[setting.key]; exists && setting.configured {
			return setting.key, setting.attribute, true
		}
	}
	return "", "", false
}

func topicConfigKeys(config types.Map) (map[string]struct{}, bool) {
	if config.IsNull() {
		return map[string]struct{}{}, true
	}
	if config.IsUnknown() {
		return nil, false
	}
	keys := make(map[string]struct{}, len(config.Elements()))
	for key := range config.Elements() {
		keys[key] = struct{}{}
	}
	return keys, true
}

func hasConfiguredDedicatedTopicSetting(config TopicResourceModel) bool {
	return !config.CleanupPolicy.IsNull() ||
		!config.RetentionMs.IsNull() ||
		!config.RetentionBytes.IsNull() ||
		!config.SegmentBytes.IsNull()
}

func validateResolvedTopicConfigConflicts(
	config TopicResourceModel,
	plan TopicResourceModel,
) (key, attribute string, unresolved, conflict bool) {
	configMap := config.Config
	if configMap.IsUnknown() {
		configMap = plan.Config
	}
	keys, known := topicConfigKeys(configMap)
	if !known {
		return "", "", hasConfiguredDedicatedTopicSetting(config), false
	}
	key, attribute, conflict = duplicateDedicatedTopicConfig(keys, config)
	return key, attribute, false, conflict
}

func unsupportedTopicSetting(plan TopicResourceModel) (string, bool) {
	for name, configured := range map[string]bool{
		"min_insync_replicas": !plan.MinInSyncReplicas.IsNull() && !plan.MinInSyncReplicas.IsUnknown(),
		"semantic_embed":      !plan.SemanticEmbed.IsNull() && !plan.SemanticEmbed.IsUnknown(),
		"semantic_model":      !plan.SemanticModel.IsNull() && !plan.SemanticModel.IsUnknown(),
		"semantic_field":      !plan.SemanticField.IsNull() && !plan.SemanticField.IsUnknown(),
	} {
		if configured {
			return name, true
		}
	}
	return "", false
}

func clearUnknownTopicState(state *TopicResourceModel) {
	if state.Config.IsUnknown() {
		state.Config = types.MapNull(types.StringType)
	}
	if state.CleanupPolicy.IsUnknown() {
		state.CleanupPolicy = types.StringNull()
	}
	if state.RetentionMs.IsUnknown() {
		state.RetentionMs = types.Int64Null()
	}
	if state.RetentionBytes.IsUnknown() {
		state.RetentionBytes = types.Int64Null()
	}
	if state.SegmentBytes.IsUnknown() {
		state.SegmentBytes = types.Int64Null()
	}
	if state.MinInSyncReplicas.IsUnknown() {
		state.MinInSyncReplicas = types.Int64Null()
	}
	if state.SemanticEmbed.IsUnknown() {
		state.SemanticEmbed = types.BoolNull()
	}
	if state.SemanticModel.IsUnknown() {
		state.SemanticModel = types.StringNull()
	}
}

// Configure adds the provider configured client to the resource.
func (r *TopicResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create creates the resource and sets the initial Terraform state.
func (r *TopicResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TopicResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var config TopicResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if key, attribute, unresolved, conflict := validateResolvedTopicConfigConflicts(config, plan); unresolved {
		resp.Diagnostics.AddAttributeError(
			path.Root("config"),
			"Unresolved Topic Configuration Conflict",
			"The config map remained unknown when topic creation began while a dedicated topic configuration attribute was also configured. Terraform cannot prove the inputs do not overlap, so no remote request was sent.",
		)
		return
	} else if conflict {
		resp.Diagnostics.AddAttributeError(
			path.Root("config").AtMapKey(key),
			"Conflicting Topic Configuration",
			fmt.Sprintf(
				"config key %q duplicates the dedicated %q attribute. Configure this setting in exactly one place.",
				key,
				attribute,
			),
		)
		return
	}
	if attribute, configured := unsupportedTopicSetting(plan); configured {
		resp.Diagnostics.AddAttributeError(
			path.Root(attribute),
			"Unsupported Topic Setting",
			fmt.Sprintf("%s is ignored by Streamline 0.3.0 and cannot be managed safely.", attribute),
		)
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
	if !plan.CleanupPolicy.IsNull() && !plan.CleanupPolicy.IsUnknown() {
		topicConfig.Config["cleanup.policy"] = plan.CleanupPolicy.ValueString()
	}
	if !plan.RetentionMs.IsNull() && !plan.RetentionMs.IsUnknown() {
		topicConfig.Config["retention.ms"] = fmt.Sprintf("%d", plan.RetentionMs.ValueInt64())
	}
	if !plan.RetentionBytes.IsNull() && !plan.RetentionBytes.IsUnknown() {
		topicConfig.Config["retention.bytes"] = fmt.Sprintf("%d", plan.RetentionBytes.ValueInt64())
	}
	if !plan.SegmentBytes.IsNull() && !plan.SegmentBytes.IsUnknown() {
		topicConfig.Config["segment.bytes"] = fmt.Sprintf("%d", plan.SegmentBytes.ValueInt64())
	}
	if !plan.MinInSyncReplicas.IsNull() && !plan.MinInSyncReplicas.IsUnknown() {
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
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
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

	// Topic configuration cannot be read back through the Kafka API used by
	// this provider. Persist the exact create request in state; subsequent
	// configuration changes are replacement-only.
	clearUnknownTopicState(&plan)
	plan.ID = plan.Name

	tflog.Info(ctx, "Created topic", map[string]any{
		"name": plan.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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
	if handleReadError(ctx, resp, "Topic", state.Name.ValueString(), err) {
		return
	}
	if topicInfo == nil {
		resp.Diagnostics.AddError(
			"Failed to Read Topic",
			fmt.Sprintf("Unable to read topic %q: the client returned an empty response", state.Name.ValueString()),
		)
		return
	}

	// Update state from server
	state.ID = types.StringValue(topicInfo.Name)
	state.Name = types.StringValue(topicInfo.Name)
	state.Partitions = types.Int64Value(int64(topicInfo.Partitions))
	state.ReplicationFactor = types.Int64Value(int64(topicInfo.ReplicationFactor))

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
	if handleDeleteError(resp, "Topic", state.Name.ValueString(), err) {
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
