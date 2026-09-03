// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SchemaResource{}
var _ resource.ResourceWithImportState = &SchemaResource{}

// SchemaResource defines the schema resource implementation.
type SchemaResource struct {
	schemaRegistryClient      *client.SchemaRegistryClient
	acceptanceStateOnlyDelete bool
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

var schemaReferenceAttrTypes = map[string]attr.Type{
	"name":    types.StringType,
	"subject": types.StringType,
	"version": types.Int64Type,
}

// NewSchemaResource creates a new schema resource
func NewSchemaResource() resource.Resource {
	return &SchemaResource{}
}

// NewSchemaResourceForAcceptanceTests returns the production schema resource
// with state-only teardown enabled. Streamline 0.3.0 cannot safely delete one
// managed schema version, so acceptance tests may use this only with a
// disposable registry fixture where retained subjects are discarded with the
// fixture.
func NewSchemaResourceForAcceptanceTests() resource.Resource {
	return &SchemaResource{acceptanceStateOnlyDelete: true}
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
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
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
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
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
				Optional:           true,
				Description:        "Legacy create-time schema references. Existing state is preserved, but Streamline 0.3.0 cannot round-trip or safely update references.",
				DeprecationMessage: "Schema references cannot be verified or updated safely against Streamline 0.3.0.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:    true,
							Description: "The name of the reference.",
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"subject": schema.StringAttribute{
							Required:    true,
							Description: "The subject of the referenced schema.",
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"version": schema.Int64Attribute{
							Required:    true,
							Description: "The version of the referenced schema.",
							Validators: []validator.Int64{
								int64validator.AtLeast(1),
							},
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

	clients, ok := req.ProviderData.(*client.Clients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Clients, got: %T", req.ProviderData),
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

func schemaConfigFromModel(
	ctx context.Context,
	model SchemaResourceModel,
	diags *diag.Diagnostics,
) client.SchemaConfig {
	config := client.SchemaConfig{
		Subject:    model.Subject.ValueString(),
		Schema:     model.Schema.ValueString(),
		SchemaType: model.SchemaType.ValueString(),
	}
	if model.References.IsNull() || model.References.IsUnknown() {
		return config
	}

	var refs []SchemaReference
	diags.Append(model.References.ElementsAs(ctx, &refs, false)...)
	if diags.HasError() {
		return config
	}
	if len(refs) > 0 {
		diags.AddAttributeError(
			path.Root("references"),
			"Schema References Unsupported",
			"Streamline 0.3.0 does not return references or include them in schema identity, so Terraform cannot manage them safely.",
		)
	}
	return config
}

func (r *SchemaResource) resolveRegisteredSchema(
	ctx context.Context,
	subject string,
	schemaID int,
) (*client.SchemaInfo, error) {
	version, err := r.schemaRegistryClient.GetSchemaVersionForID(ctx, subject, schemaID)
	if err != nil {
		return nil, err
	}
	info, err := r.schemaRegistryClient.GetSchema(ctx, subject, version)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf("registry returned an empty schema for %s version %d", subject, version)
	}
	if info.ID != schemaID {
		return nil, fmt.Errorf(
			"registry returned schema ID %d for %s version %d, expected %d",
			info.ID,
			subject,
			version,
			schemaID,
		)
	}
	return info, nil
}

func applySchemaInfo(
	ctx context.Context,
	model *SchemaResourceModel,
	info *client.SchemaInfo,
	diags *diag.Diagnostics,
) {
	model.Schema = types.StringValue(info.Schema)
	model.SchemaID = types.Int64Value(int64(info.ID))
	model.Version = types.Int64Value(int64(info.Version))
	model.ID = types.StringValue(fmt.Sprintf("%s:%d", model.Subject.ValueString(), info.Version))
	schemaType := info.SchemaType
	if schemaType == "" {
		// Confluent-compatible registries omit schemaType for Avro, whose
		// canonical wire default is AVRO. Persist the explicit value so import
		// and refresh do not leave an unknown/stale Terraform attribute.
		schemaType = "AVRO"
	}
	model.SchemaType = types.StringValue(schemaType)

	if len(info.References) == 0 {
		return
	}
	references := make([]SchemaReference, 0, len(info.References))
	for _, ref := range info.References {
		references = append(references, SchemaReference{
			Name:    types.StringValue(ref.Name),
			Subject: types.StringValue(ref.Subject),
			Version: types.Int64Value(int64(ref.Version)),
		})
	}
	value, valueDiags := types.ListValueFrom(
		ctx,
		types.ObjectType{AttrTypes: schemaReferenceAttrTypes},
		references,
	)
	diags.Append(valueDiags...)
	if !diags.HasError() {
		model.References = value
	}
}

func schemaDefinitionChanged(plan, state SchemaResourceModel) bool {
	return !plan.Schema.Equal(state.Schema) ||
		!plan.SchemaType.Equal(state.SchemaType) ||
		!plan.References.Equal(state.References)
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

	schemaConfig := schemaConfigFromModel(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set compatibility level if specified
	if !plan.Compatibility.IsNull() && !plan.Compatibility.IsUnknown() {
		err := r.schemaRegistryClient.SetCompatibility(ctx, plan.Subject.ValueString(), plan.Compatibility.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to Set Schema Compatibility",
				fmt.Sprintf("Unable to set compatibility for subject %s: %s", plan.Subject.ValueString(), err),
			)
			return
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

	schemaInfo, err := r.resolveRegisteredSchema(ctx, plan.Subject.ValueString(), schemaID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Read Registered Schema",
			fmt.Sprintf(
				"Schema ID %d was registered for subject %s, but its exact version could not be resolved: %s. Import the subject to recover it before retrying.",
				schemaID,
				plan.Subject.ValueString(),
				err,
			),
		)
		return
	}
	applySchemaInfo(ctx, &plan, schemaInfo, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

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
	if handleReadError(ctx, resp, "Schema", state.ID.ValueString(), err) {
		return
	}
	if schemaInfo == nil {
		resp.Diagnostics.AddError(
			"Failed to Read Schema",
			fmt.Sprintf("Unable to read schema %q: the client returned an empty response", state.ID.ValueString()),
		)
		return
	}

	// Update state from server
	applySchemaInfo(ctx, &state, schemaInfo, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get compatibility level
	compatibility, err := r.schemaRegistryClient.GetCompatibility(ctx, state.Subject.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Read Schema Compatibility",
			fmt.Sprintf("Unable to read compatibility for subject %s: %s", state.Subject.ValueString(), err),
		)
		return
	}
	state.Compatibility = types.StringValue(compatibility)

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

	if schemaDefinitionChanged(plan, state) {
		schemaConfig := schemaConfigFromModel(ctx, plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
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

		schemaInfo, err := r.resolveRegisteredSchema(ctx, plan.Subject.ValueString(), schemaID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to Read Updated Schema",
				fmt.Sprintf(
					"Schema ID %d was registered for subject %s, but its exact version could not be resolved: %s. Refresh or import the subject before retrying.",
					schemaID,
					plan.Subject.ValueString(),
					err,
				),
			)
			return
		}
		applySchemaInfo(ctx, &plan, schemaInfo, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
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

	if r.acceptanceStateOnlyDelete {
		resp.Diagnostics.AddWarning(
			"Schema Retained During Acceptance Cleanup",
			fmt.Sprintf(
				"Removed schema %q from Terraform acceptance-test state without deleting the remote subject. The acceptance provider is valid only with a disposable Schema Registry fixture.",
				state.ID.ValueString(),
			),
		)
		return
	}

	resp.Diagnostics.AddError(
		"Schema Deletion Unsupported",
		fmt.Sprintf(
			"Refusing to delete schema %q because Streamline 0.3.0 only exposes an asynchronous subject-wide deletion that can remove every version after Terraform has already received an ambiguous result. Delete it through a verified external process, then remove the Terraform state entry.",
			state.ID.ValueString(),
		),
	)
}

// ImportState imports an existing resource into Terraform state.
func (r *SchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("subject"), req, resp)
}
