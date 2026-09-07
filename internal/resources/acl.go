// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AclResource{}
var _ resource.ResourceWithImportState = &AclResource{}
var _ resource.ResourceWithModifyPlan = &AclResource{}

type aclClient interface {
	CreateACL(context.Context, client.ACLConfig) error
	GetACL(context.Context, client.ACLConfig) (*client.ACLConfig, error)
	DeleteACL(context.Context, client.ACLConfig) error
}

// AclResource defines the ACL resource implementation.
type AclResource struct {
	kafkaClient aclClient
}

// AclResourceModel describes the ACL resource data model.
type AclResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ResourceType   types.String `tfsdk:"resource_type"`
	ResourceName   types.String `tfsdk:"resource_name"`
	PatternType    types.String `tfsdk:"pattern_type"`
	Principal      types.String `tfsdk:"principal"`
	Host           types.String `tfsdk:"host"`
	Operation      types.String `tfsdk:"operation"`
	PermissionType types.String `tfsdk:"permission_type"`
}

type managedACLValuePlanModifier struct {
	attribute         string
	requireConfigured bool
}

func (m managedACLValuePlanModifier) Description(context.Context) string {
	if m.requireConfigured {
		return fmt.Sprintf(
			"%s must be configured with an exact value for new ACLs; legacy wildcard state remains refreshable",
			m.attribute,
		)
	}
	return fmt.Sprintf(
		"%s rejects new wildcard ACLs while preserving legacy wildcard state for refresh",
		m.attribute,
	)
}

func (m managedACLValuePlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m managedACLValuePlanModifier) PlanModifyString(
	ctx context.Context,
	req planmodifier.StringRequest,
	resp *planmodifier.StringResponse,
) {
	if req.ConfigValue.IsUnknown() {
		return
	}
	if req.ConfigValue.IsNull() {
		if m.requireConfigured && (req.StateValue.IsNull() || req.StateValue.IsUnknown()) {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Exact ACL Value Required",
				fmt.Sprintf(
					"%s must be explicitly configured for new ACLs because the broker cannot safely manage the wildcard value.",
					m.attribute,
				),
			)
		}
		return
	}
	if req.ConfigValue.ValueString() != "*" {
		return
	}
	if !req.StateValue.IsNull() &&
		!req.StateValue.IsUnknown() &&
		req.StateValue.ValueString() == "*" {
		return
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Unsupported ACL Wildcard",
		fmt.Sprintf(
			"%s cannot be %q for a new managed ACL because the broker treats it as a broad delete filter. Existing legacy wildcard state can still be refreshed.",
			m.attribute,
			req.ConfigValue.ValueString(),
		),
	)
}

// NewAclResource creates a new ACL resource
func NewAclResource() resource.Resource {
	return &AclResource{}
}

// Metadata returns the resource type name.
func (r *AclResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acl"
}

// Schema defines the schema for the resource.
func (r *AclResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Streamline Access Control List (ACL) entry.",
		MarkdownDescription: `
Manages a Streamline Access Control List (ACL) entry.

ACLs control who can perform operations on Streamline resources. Each ACL entry specifies a principal, resource, operation, and permission.

## Import

Import IDs contain all seven immutable fields separated by ` + "`|`" + `. Each
field is URL query escaped:

` + "```shell" + `
terraform import streamline_acl.example 'topic|literal|events|User%3Aalice|10.0.0.10|read|allow'
` + "```" + `

## Example Usage

` + "```hcl" + `
# Allow user 'alice' to read from topic 'events'
resource "streamline_acl" "alice_read_events" {
  resource_type   = "topic"
  resource_name   = "events"
  pattern_type    = "literal"
  principal       = "User:alice"
  host            = "10.0.0.10"
  operation       = "read"
  permission_type = "allow"
}

# Allow user 'producer-service' to write to all topics starting with 'logs-'
resource "streamline_acl" "producer_write_logs" {
  resource_type   = "topic"
  resource_name   = "logs-"
  pattern_type    = "prefixed"
  principal       = "User:producer-service"
  host            = "10.0.0.11"
  operation       = "write"
  permission_type = "allow"
}

# Allow the analytics principal to read from one topic
resource "streamline_acl" "analytics_read_all" {
  resource_type   = "topic"
  resource_name   = "analytics-events"
  pattern_type    = "literal"
  principal       = "User:analytics"
  host            = "10.0.0.12"
  operation       = "read"
  permission_type = "allow"
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier for the ACL entry.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"resource_type": schema.StringAttribute{
				Required:    true,
				Description: "The type of resource. 'delegation_token' is retained for legacy configuration compatibility but cannot be managed against Streamline 0.3.0.",
				Validators: []validator.String{
					stringvalidator.OneOf("topic", "group", "cluster", "transactional_id", "delegation_token"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_name": schema.StringAttribute{
				Required:    true,
				Description: "The exact resource name. New managed ACLs cannot use '*' because the broker treats it as a broad delete filter. Legacy wildcard state permits only an exact no-op refresh; changes, replacement, and destroy fail during planning.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					managedACLValuePlanModifier{attribute: "resource_name"},
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pattern_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("literal"),
				Description: "The pattern type: 'literal', 'prefixed', or legacy filter-only 'match'. New managed ACLs cannot use 'match'.",
				Validators: []validator.String{
					stringvalidator.OneOf("literal", "prefixed", "match"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"principal": schema.StringAttribute{
				Required:    true,
				Description: "The principal (user or service) in the format 'User:name' or 'Group:name'.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(User|Group):.+$`),
						"must use the form User:name or Group:name",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"host": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The exact host from which access is allowed. It must be explicitly configured for new ACLs. Legacy '*' state permits only an exact no-op refresh; changes, replacement, and destroy fail during planning.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					managedACLValuePlanModifier{attribute: "host", requireConfigured: true},
					stringplanmodifier.RequiresReplace(),
				},
			},
			"operation": schema.StringAttribute{
				Required:    true,
				Description: "The operation. Legacy 'all' remains readable, but new safely managed ACLs require an exact operation.",
				Validators: []validator.String{
					stringvalidator.OneOf(
						"all", "read", "write", "create", "delete", "alter", "describe",
						"cluster_action", "describe_configs", "alter_configs", "idempotent_write",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"permission_type": schema.StringAttribute{
				Required:    true,
				Description: "The permission type: 'allow' or 'deny'.",
				Validators: []validator.String{
					stringvalidator.OneOf("allow", "deny"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// ModifyPlan permits unsafe legacy wildcard ACLs only for a true no-op
// refresh. Destroying or replacing one would eventually call the broker with
// a broad wildcard filter, which this provider deliberately refuses.
func (r *AclResource) ModifyPlan(
	ctx context.Context,
	req resource.ModifyPlanRequest,
	resp *resource.ModifyPlanResponse,
) {
	if req.State.Raw.IsNull() {
		return
	}

	var state AclResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !hasUnsafeLegacyACLWildcard(state) {
		return
	}

	if req.Plan.Raw.IsNull() {
		err := legacyACLPlanError(state, nil)
		resp.Diagnostics.AddError(
			"Legacy Wildcard ACL Cannot Be Destroyed",
			err.Error(),
		)
		return
	}

	var plan AclResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := legacyACLPlanError(state, &plan); err != nil {
		resp.Diagnostics.AddError(
			"Legacy Wildcard ACL Cannot Be Changed",
			err.Error(),
		)
	}
}

func legacyACLPlanError(state AclResourceModel, plan *AclResourceModel) error {
	if !hasUnsafeLegacyACLWildcard(state) {
		return nil
	}
	if plan == nil {
		return fmt.Errorf("this ACL contains a wildcard resource_name or host that the broker interprets as a broad delete filter; refresh is allowed, but destroy/replacement must be performed through a verified external process followed by Terraform state removal")
	}
	if !aclModelsEqual(*plan, state) {
		return fmt.Errorf("this ACL contains a wildcard resource_name or host and can only be refreshed as an exact no-op; immutable-field changes or replacement would require an unsafe broad delete")
	}
	return nil
}

func hasUnsafeLegacyACLWildcard(model AclResourceModel) bool {
	return (!model.ResourceName.IsNull() &&
		!model.ResourceName.IsUnknown() &&
		model.ResourceName.ValueString() == "*") ||
		(!model.Host.IsNull() &&
			!model.Host.IsUnknown() &&
			model.Host.ValueString() == "*")
}

func aclModelsEqual(left, right AclResourceModel) bool {
	return left.ID.Equal(right.ID) &&
		left.ResourceType.Equal(right.ResourceType) &&
		left.ResourceName.Equal(right.ResourceName) &&
		left.PatternType.Equal(right.PatternType) &&
		left.Principal.Equal(right.Principal) &&
		left.Host.Equal(right.Host) &&
		left.Operation.Equal(right.Operation) &&
		left.PermissionType.Equal(right.PermissionType)
}

// Configure adds the provider configured client to the resource.
func (r *AclResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *AclResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AclResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating ACL", map[string]any{
		"resource_type": plan.ResourceType.ValueString(),
		"resource_name": plan.ResourceName.ValueString(),
		"principal":     plan.Principal.ValueString(),
		"operation":     plan.Operation.ValueString(),
	})

	// Build ACL config
	aclConfig := client.ACLConfig{
		ResourceType:   plan.ResourceType.ValueString(),
		ResourceName:   plan.ResourceName.ValueString(),
		PatternType:    plan.PatternType.ValueString(),
		Principal:      plan.Principal.ValueString(),
		Host:           plan.Host.ValueString(),
		Operation:      plan.Operation.ValueString(),
		PermissionType: plan.PermissionType.ValueString(),
	}

	// Create the ACL
	err := r.kafkaClient.CreateACL(ctx, aclConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Create ACL",
			fmt.Sprintf("Unable to create ACL: %s", err),
		)
		return
	}

	plan.ID = types.StringValue(aclID(aclConfig))

	tflog.Info(ctx, "Created ACL", map[string]any{
		"id": plan.ID.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *AclResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AclResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading ACL", map[string]any{
		"id": state.ID.ValueString(),
	})

	// Build ACL filter to check if ACL exists
	aclFilter := client.ACLConfig{
		ResourceType:   state.ResourceType.ValueString(),
		ResourceName:   state.ResourceName.ValueString(),
		PatternType:    state.PatternType.ValueString(),
		Principal:      state.Principal.ValueString(),
		Host:           state.Host.ValueString(),
		Operation:      state.Operation.ValueString(),
		PermissionType: state.PermissionType.ValueString(),
	}

	// Check if ACL exists
	acl, err := r.kafkaClient.GetACL(ctx, aclFilter)
	if handleReadError(ctx, resp, "ACL", state.ID.ValueString(), err) {
		return
	}
	if acl == nil {
		resp.Diagnostics.AddError(
			"Failed to Read ACL",
			fmt.Sprintf("Unable to read ACL %q: the client returned an empty response", state.ID.ValueString()),
		)
		return
	}

	// Update state from server response
	state.ResourceType = types.StringValue(acl.ResourceType)
	state.ResourceName = types.StringValue(acl.ResourceName)
	state.PatternType = types.StringValue(acl.PatternType)
	state.Principal = types.StringValue(acl.Principal)
	state.Host = types.StringValue(acl.Host)
	state.Operation = types.StringValue(acl.Operation)
	state.PermissionType = types.StringValue(acl.PermissionType)
	state.ID = types.StringValue(aclID(*acl))

	tflog.Info(ctx, "Read ACL", map[string]any{
		"id": state.ID.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *AclResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// ACLs are immutable - all changes require replacement
	// This should not be called due to RequiresReplace on all attributes
	resp.Diagnostics.AddError(
		"ACL Update Not Supported",
		"ACL entries are immutable. Any changes require destroying and recreating the resource.",
	)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *AclResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AclResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting ACL", map[string]any{
		"id": state.ID.ValueString(),
	})

	// Build ACL filter for deletion
	aclFilter := client.ACLConfig{
		ResourceType:   state.ResourceType.ValueString(),
		ResourceName:   state.ResourceName.ValueString(),
		PatternType:    state.PatternType.ValueString(),
		Principal:      state.Principal.ValueString(),
		Host:           state.Host.ValueString(),
		Operation:      state.Operation.ValueString(),
		PermissionType: state.PermissionType.ValueString(),
	}

	// Delete the ACL
	err := r.kafkaClient.DeleteACL(ctx, aclFilter)
	if handleDeleteError(resp, "ACL", state.ID.ValueString(), err) {
		return
	}

	tflog.Info(ctx, "Deleted ACL", map[string]any{
		"id": state.ID.ValueString(),
	})
}

// ImportState imports an ACL using its lossless seven-field composite ID.
func (r *AclResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	aclConfig, err := parseACLID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid ACL Import ID",
			fmt.Sprintf("Unable to parse ACL import ID %q: %s", req.ID, err),
		)
		return
	}
	if validationErr := client.ValidateManagedACLConfig(aclConfig); validationErr != nil {
		resp.Diagnostics.AddError(
			"Unsafe ACL Import Unsupported",
			fmt.Sprintf("ACL import ID %q cannot be managed safely: %s", req.ID, validationErr),
		)
		return
	}

	acl, err := r.kafkaClient.GetACL(ctx, aclConfig)
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"ACL Not Found",
				fmt.Sprintf("No ACL matched import ID %q.", req.ID),
			)
			return
		}
		resp.Diagnostics.AddError(
			"ACL Import Failed",
			fmt.Sprintf("Unable to read ACL %q: %s", req.ID, err),
		)
		return
	}
	if acl == nil {
		resp.Diagnostics.AddError(
			"ACL Import Failed",
			fmt.Sprintf("Unable to read ACL %q: the client returned an empty response", req.ID),
		)
		return
	}

	model := AclResourceModel{
		ID:             types.StringValue(aclID(*acl)),
		ResourceType:   types.StringValue(acl.ResourceType),
		ResourceName:   types.StringValue(acl.ResourceName),
		PatternType:    types.StringValue(acl.PatternType),
		Principal:      types.StringValue(acl.Principal),
		Host:           types.StringValue(acl.Host),
		Operation:      types.StringValue(acl.Operation),
		PermissionType: types.StringValue(acl.PermissionType),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func aclID(cfg client.ACLConfig) string {
	fields := []string{
		cfg.ResourceType,
		cfg.PatternType,
		cfg.ResourceName,
		cfg.Principal,
		cfg.Host,
		cfg.Operation,
		cfg.PermissionType,
	}
	for i := range fields {
		fields[i] = url.QueryEscape(fields[i])
	}
	return strings.Join(fields, "|")
}

func parseACLID(id string) (client.ACLConfig, error) {
	parts := strings.Split(id, "|")
	if len(parts) != 7 {
		return client.ACLConfig{}, fmt.Errorf(
			"expected 'resource_type|pattern_type|resource_name|principal|host|operation|permission_type'",
		)
	}
	for i := range parts {
		decoded, err := url.QueryUnescape(parts[i])
		if err != nil {
			return client.ACLConfig{}, fmt.Errorf("field %d is not valid URL encoding: %w", i+1, err)
		}
		parts[i] = decoded
	}

	cfg := client.ACLConfig{
		ResourceType:   parts[0],
		PatternType:    parts[1],
		ResourceName:   parts[2],
		Principal:      parts[3],
		Host:           parts[4],
		Operation:      parts[5],
		PermissionType: parts[6],
	}
	if err := client.ValidateACLConfig(cfg); err != nil {
		return client.ACLConfig{}, err
	}
	return cfg, nil
}
