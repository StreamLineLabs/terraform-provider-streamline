// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestManagedACLValuePlanModifierRejectsNewWildcards(t *testing.T) {
	t.Parallel()

	for _, attribute := range []string{"resource_name", "host"} {
		attribute := attribute
		t.Run(attribute, func(t *testing.T) {
			t.Parallel()

			resp := &planmodifier.StringResponse{}
			(managedACLValuePlanModifier{attribute: attribute}).PlanModifyString(
				context.Background(),
				planmodifier.StringRequest{
					Path:        path.Root(attribute),
					ConfigValue: types.StringValue("*"),
					PlanValue:   types.StringValue("*"),
					StateValue:  types.StringNull(),
				},
				resp,
			)
			if !resp.Diagnostics.HasError() {
				t.Fatalf("new %s wildcard should fail during planning", attribute)
			}
		})
	}
}

func TestManagedACLValuePlanModifierAllowsLegacyWildcardRefresh(t *testing.T) {
	t.Parallel()

	resp := &planmodifier.StringResponse{}
	(managedACLValuePlanModifier{attribute: "host", requireConfigured: true}).PlanModifyString(
		context.Background(),
		planmodifier.StringRequest{
			Path:        path.Root("host"),
			ConfigValue: types.StringNull(),
			PlanValue:   types.StringValue("*"),
			StateValue:  types.StringValue("*"),
		},
		resp,
	)
	if resp.Diagnostics.HasError() {
		t.Fatalf("legacy wildcard refresh was rejected: %v", resp.Diagnostics.Errors())
	}
}

func TestManagedACLValuePlanModifierRequiresHostForNewACL(t *testing.T) {
	t.Parallel()

	resp := &planmodifier.StringResponse{}
	(managedACLValuePlanModifier{attribute: "host", requireConfigured: true}).PlanModifyString(
		context.Background(),
		planmodifier.StringRequest{
			Path:        path.Root("host"),
			ConfigValue: types.StringNull(),
			PlanValue:   types.StringUnknown(),
			StateValue:  types.StringNull(),
		},
		resp,
	)
	if !resp.Diagnostics.HasError() {
		t.Fatal("new ACL without an exact host should fail during planning")
	}
}

func TestACLSchemaDoesNotDefaultHostToUnsupportedWildcard(t *testing.T) {
	t.Parallel()

	resp := &resource.SchemaResponse{}
	(&AclResource{}).Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", resp.Diagnostics.Errors())
	}

	host := resp.Schema.Attributes["host"].(schema.StringAttribute)
	if host.Default != nil {
		t.Fatal("host must not default to the unsupported wildcard")
	}
	if !host.Optional || !host.Computed || len(host.PlanModifiers) < 2 {
		t.Fatal("host must remain Optional+Computed for legacy refresh with plan-time create validation")
	}
}

func TestLegacyACLWildcardAllowsOnlyExactNoopPlan(t *testing.T) {
	t.Parallel()

	state := AclResourceModel{
		ID:             types.StringValue("legacy"),
		ResourceType:   types.StringValue("topic"),
		ResourceName:   types.StringValue("*"),
		PatternType:    types.StringValue("literal"),
		Principal:      types.StringValue("User:legacy"),
		Host:           types.StringValue("*"),
		Operation:      types.StringValue("read"),
		PermissionType: types.StringValue("allow"),
	}
	if !hasUnsafeLegacyACLWildcard(state) {
		t.Fatal("legacy wildcard state was not detected")
	}
	if !aclModelsEqual(state, state) {
		t.Fatal("identical legacy state must be a no-op")
	}
	if err := legacyACLPlanError(state, &state); err != nil {
		t.Fatalf("exact no-op refresh was rejected: %v", err)
	}

	changed := state
	changed.Operation = types.StringValue("write")
	if aclModelsEqual(changed, state) {
		t.Fatal("immutable-field change was incorrectly treated as a no-op")
	}
	if err := legacyACLPlanError(state, &changed); err == nil {
		t.Fatal("legacy wildcard immutable-field change should fail planning")
	}
	if err := legacyACLPlanError(state, nil); err == nil {
		t.Fatal("legacy wildcard destroy should fail planning")
	}
}
