// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

// UserResource defines the user resource implementation.
type UserResource struct {
	kafkaClient *client.StreamlineClient
}

// UserResourceModel describes the user resource data model.
type UserResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Username  types.String `tfsdk:"username"`
	Mechanism types.String `tfsdk:"mechanism"`
}

// NewUserResource creates a new user resource
func NewUserResource() resource.Resource {
	return &UserResource{}
}

// Metadata returns the resource type name.
func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

// Schema defines the schema for the resource.
func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Streamline SASL/SCRAM user.",
		MarkdownDescription: `
Manages a Streamline SASL/SCRAM user.

Users are authenticated principals that can be granted access to Streamline resources via ACLs. This resource manages user credentials using the SASL/SCRAM mechanism.

## Example Usage

` + "```hcl" + `
resource "streamline_user" "alice" {
  username  = "alice"
  mechanism = "SCRAM-SHA-256"
}

resource "streamline_user" "producer_service" {
  username  = "producer-service"
  mechanism = "SCRAM-SHA-256"
}

# Grant the user access to a topic
resource "streamline_acl" "alice_read" {
  resource_type   = "topic"
  resource_name   = "events"
  principal       = "User:${streamline_user.alice.username}"
  operation       = "read"
  permission_type = "allow"
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the user (same as username).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "The username for the SASL/SCRAM user.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mechanism": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("SCRAM-SHA-256"),
				Description: "The SASL mechanism: 'SCRAM-SHA-256' (default) or 'SCRAM-SHA-512'.",
				Validators: []validator.String{
					stringvalidator.OneOf("SCRAM-SHA-256", "SCRAM-SHA-512"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *UserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating user", map[string]any{
		"username":  plan.Username.ValueString(),
		"mechanism": plan.Mechanism.ValueString(),
	})

	userConfig := client.UserConfig{
		Username:  plan.Username.ValueString(),
		Mechanism: plan.Mechanism.ValueString(),
	}

	err := r.kafkaClient.CreateUser(ctx, userConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Create User",
			fmt.Sprintf("Unable to create user %s: %s", plan.Username.ValueString(), err),
		)
		return
	}

	plan.ID = plan.Username

	tflog.Info(ctx, "Created user", map[string]any{
		"username": plan.Username.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading user", map[string]any{
		"username": state.Username.ValueString(),
	})

	userInfo, err := r.kafkaClient.GetUser(ctx, state.Username.ValueString())
	if err != nil {
		resp.Diagnostics.AddWarning(
			"User Not Found",
			fmt.Sprintf("User %s may have been deleted outside of Terraform: %s", state.Username.ValueString(), err),
		)
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(userInfo.Username)
	state.Username = types.StringValue(userInfo.Username)
	state.Mechanism = types.StringValue(userInfo.Mechanism)

	tflog.Info(ctx, "Read user", map[string]any{
		"username": state.Username.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All attributes require replacement, so Update should not be called
	resp.Diagnostics.AddError(
		"User Update Not Supported",
		"User resources are immutable. Any changes require destroying and recreating the resource.",
	)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting user", map[string]any{
		"username": state.Username.ValueString(),
	})

	err := r.kafkaClient.DeleteUser(ctx, state.Username.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Delete User",
			fmt.Sprintf("Unable to delete user %s: %s", state.Username.ValueString(), err),
		)
		return
	}

	tflog.Info(ctx, "Deleted user", map[string]any{
		"username": state.Username.ValueString(),
	})
}

// ImportState imports an existing resource into Terraform state.
func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("username"), req, resp)
}

// userMutex prevents concurrent user modifications.
var userMutex sync.Mutex
