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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AclResource{}

// AclResource defines the ACL resource implementation.
type AclResource struct {
	kafkaClient *client.StreamlineClient
}

// AclResourceModel describes the ACL resource data model.
type AclResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ResourceType     types.String `tfsdk:"resource_type"`
	ResourceName     types.String `tfsdk:"resource_name"`
	PatternType      types.String `tfsdk:"pattern_type"`
	Principal        types.String `tfsdk:"principal"`
	Host             types.String `tfsdk:"host"`
	Operation        types.String `tfsdk:"operation"`
	PermissionType   types.String `tfsdk:"permission_type"`
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

## Example Usage

` + "```hcl" + `
# Allow user 'alice' to read from topic 'events'
resource "streamline_acl" "alice_read_events" {
  resource_type   = "topic"
  resource_name   = "events"
  pattern_type    = "literal"
  principal       = "User:alice"
  operation       = "read"
  permission_type = "allow"
}

# Allow user 'producer-service' to write to all topics starting with 'logs-'
resource "streamline_acl" "producer_write_logs" {
  resource_type   = "topic"
  resource_name   = "logs-"
  pattern_type    = "prefixed"
  principal       = "User:producer-service"
  operation       = "write"
  permission_type = "allow"
}

# Allow consumer group 'analytics' to read from any topic
resource "streamline_acl" "analytics_read_all" {
  resource_type   = "topic"
  resource_name   = "*"
  pattern_type    = "literal"
  principal       = "User:analytics"
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
				Description: "The type of resource: 'topic', 'group', 'cluster', 'transactional_id', or 'delegation_token'.",
				Validators: []validator.String{
					stringvalidator.OneOf("topic", "group", "cluster", "transactional_id", "delegation_token"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the resource. Use '*' for all resources of the type.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pattern_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("literal"),
				Description: "The pattern type: 'literal' (exact match), 'prefixed' (prefix match), or 'match' (wildcard match).",
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"host": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("*"),
				Description: "The host from which access is allowed. Use '*' for all hosts.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"operation": schema.StringAttribute{
				Required:    true,
				Description: "The operation: 'all', 'read', 'write', 'create', 'delete', 'alter', 'describe', 'cluster_action', 'describe_configs', 'alter_configs', 'idempotent_write'.",
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

// Configure adds the provider configured client to the resource.
func (r *AclResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	// Generate unique ID
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s",
		plan.ResourceType.ValueString(),
		plan.PatternType.ValueString(),
		plan.ResourceName.ValueString(),
		plan.Principal.ValueString(),
		plan.Host.ValueString(),
		plan.Operation.ValueString(),
		plan.PermissionType.ValueString(),
	))

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
	if err != nil {
		resp.Diagnostics.AddWarning(
			"ACL Not Found",
			fmt.Sprintf("ACL may have been deleted outside of Terraform: %s", err),
		)
		resp.State.RemoveResource(ctx)
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
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Delete ACL",
			fmt.Sprintf("Unable to delete ACL: %s", err),
		)
		return
	}

	tflog.Info(ctx, "Deleted ACL", map[string]any{
		"id": state.ID.ValueString(),
	})
}

