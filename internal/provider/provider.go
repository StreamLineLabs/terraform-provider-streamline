// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/datasources"
	"github.com/streamlinelabs/terraform-provider-streamline/internal/resources"
)

// Ensure StreamlineProvider satisfies various provider interfaces.
var _ provider.Provider = &StreamlineProvider{}

// StreamlineProvider defines the provider implementation.
type StreamlineProvider struct {
	version               string
	schemaResourceFactory func() resource.Resource
}

// StreamlineProviderModel describes the provider data model.
type StreamlineProviderModel struct {
	BootstrapServers  types.String `tfsdk:"bootstrap_servers"`
	SaslMechanism     types.String `tfsdk:"sasl_mechanism"`
	SaslUsername      types.String `tfsdk:"sasl_username"`
	SaslPassword      types.String `tfsdk:"sasl_password"`
	TLSEnabled        types.Bool   `tfsdk:"tls_enabled"`
	TLSCACert         types.String `tfsdk:"tls_ca_cert"`
	TLSClientCert     types.String `tfsdk:"tls_client_cert"`
	TLSClientKey      types.String `tfsdk:"tls_client_key"`
	TLSSkipVerify     types.Bool   `tfsdk:"tls_skip_verify"`
	ConnectionTimeout types.Int64  `tfsdk:"connection_timeout"`
	RequestTimeout    types.Int64  `tfsdk:"request_timeout"`
	SchemaRegistryURL types.String `tfsdk:"schema_registry_url"`
	MoonshotURL       types.String `tfsdk:"moonshot_url"`
	MoonshotToken     types.String `tfsdk:"moonshot_token"`
}

// New creates a new provider instance
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &StreamlineProvider{
			version: version,
		}
	}
}

// NewForSchemaAcceptanceTests creates a provider whose schema resource removes
// only Terraform state during test teardown. It must be used exclusively with
// a disposable Schema Registry fixture because production schema deletion
// remains intentionally unsupported.
func NewForSchemaAcceptanceTests(version string) func() provider.Provider {
	return func() provider.Provider {
		return &StreamlineProvider{
			version:               version,
			schemaResourceFactory: resources.NewSchemaResourceForAcceptanceTests,
		}
	}
}

// Metadata returns the provider type name.
func (p *StreamlineProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "streamline"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *StreamlineProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The Streamline provider enables Terraform to manage Streamline streaming platform resources.",
		MarkdownDescription: `
The Streamline provider enables Terraform to manage [Streamline](https://github.com/streamlinelabs/streamline) streaming platform resources.

Streamline is a Kafka-compatible streaming platform with support for:
- Topics and partitions
- Access Control Lists (ACLs)
- Schema Registry
- Consumer Groups

## Example Usage

` + "```hcl" + `
terraform {
  required_providers {
    streamline = {
      source  = "streamlinelabs/streamline"
      version = "~> 0.4.0"
    }
  }
}

provider "streamline" {
  bootstrap_servers = "localhost:9092"
}

resource "streamline_topic" "events" {
  name         = "events"
  partitions   = 3
  retention_ms = 604800000 # 7 days
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"bootstrap_servers": schema.StringAttribute{
				Description: "Comma-separated list of Streamline bootstrap servers (e.g., 'localhost:9092,localhost:9093')",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"sasl_mechanism": schema.StringAttribute{
				Description: "SASL mechanism for authentication (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"),
				},
			},
			"sasl_username": schema.StringAttribute{
				Description: "SASL username for authentication",
				Optional:    true,
			},
			"sasl_password": schema.StringAttribute{
				Description: "SASL password for authentication",
				Optional:    true,
				Sensitive:   true,
			},
			"tls_enabled": schema.BoolAttribute{
				Description: "Enable TLS for connections",
				Optional:    true,
			},
			"tls_ca_cert": schema.StringAttribute{
				Description: "Path to CA certificate file for TLS",
				Optional:    true,
			},
			"tls_client_cert": schema.StringAttribute{
				Description: "Path to client certificate file for mTLS",
				Optional:    true,
			},
			"tls_client_key": schema.StringAttribute{
				Description: "Path to client private key file for mTLS",
				Optional:    true,
			},
			"tls_skip_verify": schema.BoolAttribute{
				Description: "Skip TLS certificate verification (development only, NOT recommended for production)",
				Optional:    true,
			},
			"connection_timeout": schema.Int64Attribute{
				Description: "Connection timeout in seconds (default: 30)",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"request_timeout": schema.Int64Attribute{
				Description: "Request timeout in seconds (default: 60)",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"schema_registry_url": schema.StringAttribute{
				Description: "Schema Registry URL for schema management (e.g., 'http://localhost:8081')",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^https?://\S+$`),
						"must be an absolute HTTP or HTTPS URL",
					),
				},
			},
			"moonshot_url": schema.StringAttribute{
				Description: "Reserved Streamline Moonshot HTTP API base URL. Legacy Moonshot resources are retained only for state compatibility and do not issue API requests.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^https?://\S+$`),
						"must be an absolute HTTP or HTTPS URL",
					),
				},
			},
			"moonshot_token": schema.StringAttribute{
				Description: "Sensitive authentication value reserved for the Moonshot HTTP API.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

// Configure prepares a Streamline client for data sources and resources.
func (p *StreamlineProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Streamline provider")

	var model StreamlineProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config := resolveProviderConfig(model)
	resp.Diagnostics.Append(validateResolvedProviderConfig(config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	brokers, diags := validateBootstrapServers(config.bootstrapServers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clients, diags := newProviderClients(config, brokers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Created Streamline clients", map[string]any{
		"bootstrap_servers":   config.bootstrapServers,
		"sasl_mechanism":      config.saslMechanism,
		"tls_enabled":         config.tlsEnabled,
		"schema_registry_url": config.schemaRegistryURL,
		"connection_timeout":  config.connectionTimeout.String(),
	})

	resp.DataSourceData = clients
	resp.ResourceData = clients
}

// Resources defines the resources implemented in the provider.
func (p *StreamlineProvider) Resources(ctx context.Context) []func() resource.Resource {
	schemaResourceFactory := p.schemaResourceFactory
	if schemaResourceFactory == nil {
		schemaResourceFactory = resources.NewSchemaResource
	}
	return []func() resource.Resource{
		resources.NewTopicResource,
		resources.NewAclResource,
		schemaResourceFactory,
		resources.NewUserResource,
		resources.NewConsumerGroupResource,
		resources.NewBranchResource,
		resources.NewContractResource,
		resources.NewMemoryResource,
	}
}

// DataSources defines the data sources implemented in the provider.
func (p *StreamlineProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewClusterDataSource,
		datasources.NewConsumerGroupDataSource,
		datasources.NewTopicsDataSource,
	}
}
