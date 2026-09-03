// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package datasources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func TestClusterDataSourcePreservesNullIdentityFields(t *testing.T) {
	t.Parallel()

	resp := &datasource.SchemaResponse{}
	(&ClusterDataSource{}).Schema(context.Background(), datasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", resp.Diagnostics.Errors())
	}
	for _, name := range []string{"id", "cluster_id"} {
		attr, ok := resp.Schema.Attributes[name].(schema.StringAttribute)
		if !ok || attr.DeprecationMessage == "" {
			t.Fatalf("%s must remain as a deprecated compatibility field", name)
		}
	}
}

func TestConsumerGroupIsReadOnlyDataSource(t *testing.T) {
	t.Parallel()

	resp := &datasource.SchemaResponse{}
	(&ConsumerGroupDataSource{}).Schema(context.Background(), datasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", resp.Diagnostics.Errors())
	}
	if _, ok := resp.Schema.Attributes["group_id"]; !ok {
		t.Fatal("consumer group data source must require group_id")
	}
	groupID := resp.Schema.Attributes["group_id"].(schema.StringAttribute)
	if len(groupID.Validators) == 0 {
		t.Fatal("consumer group group_id must have a plan-time validator")
	}
}

func TestTopicsDataSourcePreservesNullCompatibilityFields(t *testing.T) {
	t.Parallel()

	resp := &datasource.SchemaResponse{}
	(&TopicsDataSource{}).Schema(context.Background(), datasource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", resp.Diagnostics.Errors())
	}
	id, ok := resp.Schema.Attributes["id"].(schema.StringAttribute)
	if !ok || id.DeprecationMessage == "" {
		t.Fatal("topics id must remain as a deprecated compatibility field")
	}
	topics := resp.Schema.Attributes["topics"].(schema.ListNestedAttribute)
	internal, ok := topics.NestedObject.Attributes["internal"].(schema.BoolAttribute)
	if !ok || internal.DeprecationMessage == "" {
		t.Fatal("topic internal must remain as a deprecated compatibility field")
	}
}
