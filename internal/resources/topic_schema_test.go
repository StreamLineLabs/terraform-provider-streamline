// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestTopicSchemaPreservesLegacyStateWithoutDestructiveReplacement(t *testing.T) {
	t.Parallel()

	resp := &resource.SchemaResponse{}
	(&TopicResource{}).Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", resp.Diagnostics.Errors())
	}
	if resp.Schema.Version != 0 {
		t.Fatalf("schema version = %d, want released version 0", resp.Schema.Version)
	}

	for _, name := range []string{"cleanup_policy", "retention_ms", "retention_bytes", "segment_bytes"} {
		switch attr := resp.Schema.Attributes[name].(type) {
		case schema.StringAttribute:
			if !attr.Optional || !attr.Computed || len(attr.PlanModifiers) < 2 {
				t.Fatalf("%s must preserve Optional+Computed state with configured-only replacement", name)
			}
			if !strings.Contains(attr.PlanModifiers[1].Description(context.Background()), "configured") {
				t.Fatalf("%s must only replace when explicitly configured", name)
			}
		case schema.Int64Attribute:
			if !attr.Optional || !attr.Computed || len(attr.PlanModifiers) < 2 {
				t.Fatalf("%s must preserve Optional+Computed state with configured-only replacement", name)
			}
			if !strings.Contains(attr.PlanModifiers[1].Description(context.Background()), "configured") {
				t.Fatalf("%s must only replace when explicitly configured", name)
			}
		default:
			t.Fatalf("unexpected type for %s", name)
		}
	}

	config := resp.Schema.Attributes["config"].(schema.MapAttribute)
	if !config.Optional || !config.Computed || len(config.PlanModifiers) < 2 || len(config.Validators) == 0 {
		t.Fatal("config must preserve state, validate values, and replace only for configured changes")
	}

	replication := resp.Schema.Attributes["replication_factor"].(schema.Int64Attribute)
	if len(replication.Validators) == 0 {
		t.Fatal("replication_factor must be restricted to the supported value")
	}
}

func TestTopicConfigMapValidatorAcceptsReleasedSupportedKeys(t *testing.T) {
	t.Parallel()

	resp := &validator.MapResponse{}
	(topicConfigMapValidator{}).ValidateMap(
		context.Background(),
		validator.MapRequest{
			Path: path.Root("config"),
			ConfigValue: types.MapValueMust(types.StringType, map[string]attr.Value{
				"retention.ms":              types.StringValue("604800000"),
				"cleanup.policy":            types.StringValue("delete"),
				"min.cleanable.dirty.ratio": types.StringValue("0.5"),
			}),
		},
		resp,
	)
	if resp.Diagnostics.HasError() {
		t.Fatalf("supported topic configuration was rejected: %v", resp.Diagnostics.Errors())
	}
}

func TestTopicConfigMapValidatorRejectsIgnoredKeys(t *testing.T) {
	t.Parallel()

	resp := &validator.MapResponse{}
	(topicConfigMapValidator{}).ValidateMap(
		context.Background(),
		validator.MapRequest{
			Path: path.Root("config"),
			ConfigValue: types.MapValueMust(types.StringType, map[string]attr.Value{
				"compression.type": types.StringValue("lz4"),
			}),
		},
		resp,
	)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected ignored topic configuration key to be rejected")
	}
}

func TestDuplicateDedicatedTopicConfigRejectsAmbiguousSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key       string
		attribute string
		model     TopicResourceModel
	}{
		{
			key:       "cleanup.policy",
			attribute: "cleanup_policy",
			model:     TopicResourceModel{CleanupPolicy: types.StringValue("delete")},
		},
		{
			key:       "retention.ms",
			attribute: "retention_ms",
			model:     TopicResourceModel{RetentionMs: types.Int64Value(1000)},
		},
		{
			key:       "retention.bytes",
			attribute: "retention_bytes",
			model:     TopicResourceModel{RetentionBytes: types.Int64Value(1024)},
		},
		{
			key:       "segment.bytes",
			attribute: "segment_bytes",
			model:     TopicResourceModel{SegmentBytes: types.Int64Value(4096)},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()

			key, attribute, conflict := duplicateDedicatedTopicConfig(
				map[string]struct{}{tt.key: {}},
				tt.model,
			)
			if !conflict || key != tt.key || attribute != tt.attribute {
				t.Fatalf(
					"duplicateDedicatedTopicConfig() = %q, %q, %v; want %q, %q, true",
					key,
					attribute,
					conflict,
					tt.key,
					tt.attribute,
				)
			}
		})
	}
}

func TestDuplicateDedicatedTopicConfigAllowsSingleSourceOfTruth(t *testing.T) {
	t.Parallel()

	if key, attribute, conflict := duplicateDedicatedTopicConfig(
		map[string]struct{}{"retention.ms": {}},
		TopicResourceModel{RetentionMs: types.Int64Null()},
	); conflict {
		t.Fatalf("map-only configuration reported conflict for %q/%q", key, attribute)
	}
	if key, attribute, conflict := duplicateDedicatedTopicConfig(
		map[string]struct{}{"storage.mode": {}},
		TopicResourceModel{RetentionMs: types.Int64Value(1000)},
	); conflict {
		t.Fatalf("unrelated dedicated configuration reported conflict for %q/%q", key, attribute)
	}
}

func TestDuplicateDedicatedTopicConfigTreatsUnknownDedicatedValueAsConfigured(t *testing.T) {
	t.Parallel()

	key, attribute, conflict := duplicateDedicatedTopicConfig(
		map[string]struct{}{"retention.ms": {}},
		TopicResourceModel{RetentionMs: types.Int64Unknown()},
	)
	if !conflict || key != "retention.ms" || attribute != "retention_ms" {
		t.Fatalf("unknown dedicated value conflict = %q, %q, %v", key, attribute, conflict)
	}
}

func TestResolvedTopicConfigConflictUsesPlanForUnknownMap(t *testing.T) {
	t.Parallel()

	config := TopicResourceModel{
		Config:      types.MapUnknown(types.StringType),
		RetentionMs: types.Int64Unknown(),
	}
	plan := TopicResourceModel{
		Config: types.MapValueMust(types.StringType, map[string]attr.Value{
			"retention.ms": types.StringValue("1000"),
		}),
	}

	key, attribute, unresolved, conflict := validateResolvedTopicConfigConflicts(config, plan)
	if unresolved || !conflict || key != "retention.ms" || attribute != "retention_ms" {
		t.Fatalf(
			"resolved conflict = %q, %q, unresolved=%v conflict=%v",
			key,
			attribute,
			unresolved,
			conflict,
		)
	}
}

func TestResolvedTopicConfigConflictFailsClosedWhenMapRemainsUnknown(t *testing.T) {
	t.Parallel()

	config := TopicResourceModel{
		Config:      types.MapUnknown(types.StringType),
		RetentionMs: types.Int64Value(1000),
	}
	plan := TopicResourceModel{Config: types.MapUnknown(types.StringType)}

	_, _, unresolved, conflict := validateResolvedTopicConfigConflicts(config, plan)
	if !unresolved || conflict {
		t.Fatalf("unresolved=%v conflict=%v, want true/false", unresolved, conflict)
	}
}

func TestTopicConfigMapValidatorDefersUnknownElementValues(t *testing.T) {
	t.Parallel()

	resp := &validator.MapResponse{}
	(topicConfigMapValidator{}).ValidateMap(
		context.Background(),
		validator.MapRequest{
			Path: path.Root("config"),
			ConfigValue: types.MapValueMust(types.StringType, map[string]attr.Value{
				"retention.ms": types.StringUnknown(),
			}),
		},
		resp,
	)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unknown map value should be deferred until plan resolution: %v", resp.Diagnostics.Errors())
	}
}

func TestUnsupportedTopicSettingRejectsNewIgnoredConfiguration(t *testing.T) {
	t.Parallel()

	attribute, configured := unsupportedTopicSetting(TopicResourceModel{
		SemanticEmbed: types.BoolValue(false),
	})
	if !configured || attribute != "semantic_embed" {
		t.Fatalf("unsupported setting = %q, %v", attribute, configured)
	}
}

func TestClearUnknownTopicStateUsesNullInsteadOfFabricatedDefaults(t *testing.T) {
	t.Parallel()

	state := TopicResourceModel{
		Config:            types.MapUnknown(types.StringType),
		CleanupPolicy:     types.StringUnknown(),
		RetentionMs:       types.Int64Unknown(),
		RetentionBytes:    types.Int64Unknown(),
		SegmentBytes:      types.Int64Unknown(),
		MinInSyncReplicas: types.Int64Unknown(),
		SemanticEmbed:     types.BoolUnknown(),
		SemanticModel:     types.StringUnknown(),
	}
	clearUnknownTopicState(&state)

	if !state.Config.IsNull() ||
		!state.CleanupPolicy.IsNull() ||
		!state.RetentionMs.IsNull() ||
		!state.RetentionBytes.IsNull() ||
		!state.SegmentBytes.IsNull() ||
		!state.MinInSyncReplicas.IsNull() ||
		!state.SemanticEmbed.IsNull() ||
		!state.SemanticModel.IsNull() {
		t.Fatalf("unknown state was not normalized to null: %+v", state)
	}
}
