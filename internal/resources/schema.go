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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamline-platform/terraform-provider-streamline/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SchemaResource{}
var _ resource.ResourceWithImportState = &SchemaResource{}

// SchemaResource defines the schema resource implementation.
type SchemaResource struct {
	schemaRegistryClient *client.SchemaRegistryClient
}

// SchemaResourceModel describes the schema resource data model.
type SchemaResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Subject       types.String `tfsdk:"subject"`
	SchemaType    types.String `tfsdk:"schema_type"`
	Schema        types.String `tfsdk:"schema"`
	Version       types.Int64  `tfsdk:"version"`
	SchemaID      types.Int64  `tfsdk:"schema_id"`
	Compatibility types.String `tfsdk:"compatibility"`
	References    types.List   `tfsdk:"references"`
}

// SchemaReference represents a schema reference
type SchemaReference struct {
	Name    types.String `tfsdk:"name"`
	Subject types.String `tfsdk:"subject"`
	Version types.Int64  `tfsdk:"version"`
}

// NewSchemaResource creates a new schema resource
func NewSchemaResource() resource.Resource {
	return &SchemaResource{}
}

// Metadata returns the resource type name.
func (r *SchemaResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema"
}

// Schema defines the schema for the resource.
func (r *SchemaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Streamline Schema Registry schema.",
		MarkdownDescription: `
Manages a Streamline Schema Registry schema.

The Schema Registry provides a serving layer for your metadata. It provides RESTful interface for storing and retrieving Avro, JSON Schema, and Protobuf schemas.

## Example Usage

` + "```hcl" + `
# Avro schema for user events
resource "streamline_schema" "user_events_value" {
  subject     = "user-events-value"
  schema_type = "AVRO"

  schema = jsonencode({
    type      = "record"
    name      = "UserEvent"
    namespace = "com.example.events"
    fields = [
      { name = "user_id", type = "string" },
      { name = "event_type", type = "string" },
      { name = "timestamp", type = "long" },
      { name = "properties", type = { type = "map", values = "string" } }
    ]
  })

  compatibility = "BACKWARD"
}

# JSON Schema
resource "streamline_schema" "order_value" {
  subject     = "orders-value"
  schema_type = "JSON"

  schema = jsonencode({
    "$schema"    = "http://json-schema.org/draft-07/schema#"
    type         = "object"
    properties = {
      order_id   = { type = "string" }
      customer_id = { type = "string" }
      total      = { type = "number" }
    }
    required = ["order_id", "customer_id", "total"]
  })
}

# Protobuf schema
resource "streamline_schema" "metric_value" {
  subject     = "metrics-value"
  schema_type = "PROTOBUF"

  schema = <<-EOT
    syntax = "proto3";
    package metrics;

    message Metric {
      string name = 1;
      double value = 2;
      int64 timestamp = 3;
      map<string, string> tags = 4;
    }
  EOT
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier for the schema (subject:version).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"subject": schema.StringAttribute{
				Required:    true,
				Description: "The subject name for the schema (e.g., 'topic-value' or 'topic-key').",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("AVRO"),
				Description: "The schema type: 'AVRO', 'JSON', or 'PROTOBUF' (default: 'AVRO').",
				Validators: []validator.String{
					stringvalidator.OneOf("AVRO", "JSON", "PROTOBUF"),
				},
			},
			"schema": schema.StringAttribute{
				Required:    true,
				Description: "The schema definition as a string.",
			},
			"version": schema.Int64Attribute{
				Computed:    true,
				Description: "The version number of the schema.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"schema_id": schema.Int64Attribute{
				Computed:    true,
				Description: "The globally unique schema ID.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"compatibility": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("BACKWARD"),
				Description: "The compatibility level: 'BACKWARD', 'BACKWARD_TRANSITIVE', 'FORWARD', 'FORWARD_TRANSITIVE', 'FULL', 'FULL_TRANSITIVE', or 'NONE'.",
				Validators: []validator.String{
					stringvalidator.OneOf(
						"BACKWARD", "BACKWARD_TRANSITIVE",
						"FORWARD", "FORWARD_TRANSITIVE",
						"FULL", "FULL_TRANSITIVE",
						"NONE",
					),
				},
			},
			"references": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Schema references for complex schemas.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:    true,
							Description: "The name of the reference.",
						},
						"subject": schema.StringAttribute{
							Required:    true,
							Description: "The subject of the referenced schema.",
						},
						"version": schema.Int64Attribute{
							Required:    true,
							Description: "The version of the referenced schema.",
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *SchemaResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	if clients.SchemaRegistry == nil {
		resp.Diagnostics.AddError(
			"Schema Registry Not Configured",
			"The schema_registry_url must be set in the provider configuration to manage schemas.",
		)
		return
	}

	r.schemaRegistryClient = clients.SchemaRegistry
}

// Create creates the resource and sets the initial Terraform state.
func (r *SchemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SchemaResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating schema", map[string]any{
		"subject":     plan.Subject.ValueString(),
		"schema_type": plan.SchemaType.ValueString(),
	})

	// Build schema configuration
	schemaConfig := client.SchemaConfig{
		Subject:    plan.Subject.ValueString(),
		Schema:     plan.Schema.ValueString(),
		SchemaType: plan.SchemaType.ValueString(),
	}

	// Extract references if provided
	if !plan.References.IsNull() {
		var refs []SchemaReference
		resp.Diagnostics.Append(plan.References.ElementsAs(ctx, &refs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, ref := range refs {
			schemaConfig.References = append(schemaConfig.References, client.SchemaReference{
				Name:    ref.Name.ValueString(),
				Subject: ref.Subject.ValueString(),
				Version: int(ref.Version.ValueInt64()),
			})
		}
	}

	// Set compatibility level if specified
	if !plan.Compatibility.IsNull() {
		err := r.schemaRegistryClient.SetCompatibility(ctx, plan.Subject.ValueString(), plan.Compatibility.ValueString())
		if err != nil {
			tflog.Warn(ctx, "Failed to set compatibility level before registration", map[string]any{
				"error": err.Error(),
			})
		}
	}

	// Register the schema
	schemaID, err := r.schemaRegistryClient.RegisterSchema(ctx, schemaConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Register Schema",
			fmt.Sprintf("Unable to register schema for subject %s: %s", plan.Subject.ValueString(), err),
		)
		return
	}

	// Get the schema info to get the version
	schemaInfo, err := r.schemaRegistryClient.GetSchema(ctx, plan.Subject.ValueString(), 0) // 0 = latest
	if err != nil {
		tflog.Warn(ctx, "Failed to get schema info after registration", map[string]any{
			"error": err.Error(),
		})
		plan.Version = types.Int64Value(1)
	} else {
		plan.Version = types.Int64Value(int64(schemaInfo.Version))
	}

	// Set computed values
	plan.SchemaID = types.Int64Value(int64(schemaID))
	plan.ID = types.StringValue(fmt.Sprintf("%s:%d", plan.Subject.ValueString(), plan.Version.ValueInt64()))

	tflog.Info(ctx, "Created schema", map[string]any{
		"id":        plan.ID.ValueString(),
		"version":   plan.Version.ValueInt64(),
		"schema_id": plan.SchemaID.ValueInt64(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *SchemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SchemaResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading schema", map[string]any{
		"id": state.ID.ValueString(),
	})

	// Get schema from registry
	schemaInfo, err := r.schemaRegistryClient.GetSchema(ctx, state.Subject.ValueString(), int(state.Version.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Schema Not Found",
			fmt.Sprintf("Schema may have been deleted outside of Terraform: %s", err),
		)
		resp.State.RemoveResource(ctx)
		return
	}

	// Update state from server
	state.Schema = types.StringValue(schemaInfo.Schema)
	state.SchemaID = types.Int64Value(int64(schemaInfo.ID))
	state.Version = types.Int64Value(int64(schemaInfo.Version))
	if schemaInfo.SchemaType != "" {
		state.SchemaType = types.StringValue(schemaInfo.SchemaType)
	}

	// Get compatibility level
	compatibility, err := r.schemaRegistryClient.GetCompatibility(ctx, state.Subject.ValueString())
	if err == nil {
		state.Compatibility = types.StringValue(compatibility)
	}

	tflog.Info(ctx, "Read schema", map[string]any{
		"id":      state.ID.ValueString(),
		"version": state.Version.ValueInt64(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *SchemaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SchemaResourceModel
	var state SchemaResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating schema", map[string]any{
		"subject": plan.Subject.ValueString(),
	})

	// Update compatibility level if changed
	if !plan.Compatibility.Equal(state.Compatibility) && !plan.Compatibility.IsNull() {
		err := r.schemaRegistryClient.SetCompatibility(ctx, plan.Subject.ValueString(), plan.Compatibility.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to Update Compatibility",
				fmt.Sprintf("Unable to update compatibility level: %s", err),
			)
			return
		}
	}

	// Check if schema content changed - if so, register new version
	if !plan.Schema.Equal(state.Schema) {
		// Build schema configuration
		schemaConfig := client.SchemaConfig{
			Subject:    plan.Subject.ValueString(),
			Schema:     plan.Schema.ValueString(),
			SchemaType: plan.SchemaType.ValueString(),
		}

		// Extract references if provided
		if !plan.References.IsNull() {
			var refs []SchemaReference
			resp.Diagnostics.Append(plan.References.ElementsAs(ctx, &refs, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			for _, ref := range refs {
				schemaConfig.References = append(schemaConfig.References, client.SchemaReference{
					Name:    ref.Name.ValueString(),
					Subject: ref.Subject.ValueString(),
					Version: int(ref.Version.ValueInt64()),
				})
			}
		}

		// Register new schema version
		schemaID, err := r.schemaRegistryClient.RegisterSchema(ctx, schemaConfig)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to Update Schema",
				fmt.Sprintf("Unable to register new schema version: %s", err),
			)
			return
		}

		// Get the new version number
		schemaInfo, err := r.schemaRegistryClient.GetSchema(ctx, plan.Subject.ValueString(), 0) // 0 = latest
		if err != nil {
			plan.Version = types.Int64Value(state.Version.ValueInt64() + 1)
		} else {
			plan.Version = types.Int64Value(int64(schemaInfo.Version))
		}

		plan.SchemaID = types.Int64Value(int64(schemaID))
	} else {
		// Keep existing version and schema ID
		plan.Version = state.Version
		plan.SchemaID = state.SchemaID
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s:%d", plan.Subject.ValueString(), plan.Version.ValueInt64()))

	tflog.Info(ctx, "Updated schema", map[string]any{
		"id":      plan.ID.ValueString(),
		"version": plan.Version.ValueInt64(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *SchemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SchemaResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting schema", map[string]any{
		"id": state.ID.ValueString(),
	})

	// Soft delete the schema (can be permanently deleted with permanent=true)
	err := r.schemaRegistryClient.DeleteSchema(ctx, state.Subject.ValueString(), false)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Delete Schema",
			fmt.Sprintf("Unable to delete schema subject %s: %s", state.Subject.ValueString(), err),
		)
		return
	}

	tflog.Info(ctx, "Deleted schema", map[string]any{
		"id": state.ID.ValueString(),
	})
}

// ImportState imports an existing resource into Terraform state.
func (r *SchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("subject"), req, resp)
}
