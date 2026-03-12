// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
	"github.com/streamlinelabs/terraform-provider-streamline/internal/datasources"
	"github.com/streamlinelabs/terraform-provider-streamline/internal/resources"
)

// Ensure StreamlineProvider satisfies various provider interfaces.
var _ provider.Provider = &StreamlineProvider{}

// StreamlineProvider defines the provider implementation.
type StreamlineProvider struct {
	version string
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
	ConnectionTimeout types.Int64  `tfsdk:"connection_timeout"`
	RequestTimeout    types.Int64  `tfsdk:"request_timeout"`
	SchemaRegistryURL types.String `tfsdk:"schema_registry_url"`
}

// ProviderClients holds the initialized clients for Streamline and Schema Registry
type ProviderClients struct {
	Kafka          *client.StreamlineClient
	SchemaRegistry *client.SchemaRegistryClient
}

// New creates a new provider instance
const (
	defaultCreateTimeout = 30 * time.Second
	defaultReadTimeout   = 10 * time.Second
	defaultDeleteTimeout = 30 * time.Second
)

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &StreamlineProvider{
			version: version,
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
      source = "streamlinelabs/streamline"
    }
  }
}

provider "streamline" {
  bootstrap_servers = "localhost:9092"
}

resource "streamline_topic" "events" {
  name       = "events"
  partitions = 3

  config = {
    "retention.ms" = "604800000"  # 7 days
  }
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"bootstrap_servers": schema.StringAttribute{
				Description: "Comma-separated list of Streamline bootstrap servers (e.g., 'localhost:9092,localhost:9093')",
				Optional:    true,
			},
			"sasl_mechanism": schema.StringAttribute{
				Description: "SASL mechanism for authentication (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)",
				Optional:    true,
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
			"connection_timeout": schema.Int64Attribute{
				Description: "Connection timeout in seconds (default: 30)",
				Optional:    true,
			},
			"request_timeout": schema.Int64Attribute{
				Description: "Request timeout in seconds (default: 60)",
				Optional:    true,
			},
			"schema_registry_url": schema.StringAttribute{
				Description: "Schema Registry URL for schema management (e.g., 'http://localhost:8081')",
				Optional:    true,
			},
		},
	}
}

// Configure prepares a Streamline client for data sources and resources.
func (p *StreamlineProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Streamline provider")

	var config StreamlineProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Default values from environment variables
	bootstrapServers := os.Getenv("STREAMLINE_BOOTSTRAP_SERVERS")
	saslMechanism := os.Getenv("STREAMLINE_SASL_MECHANISM")
	saslUsername := os.Getenv("STREAMLINE_SASL_USERNAME")
	saslPassword := os.Getenv("STREAMLINE_SASL_PASSWORD")
	tlsEnabled := os.Getenv("STREAMLINE_TLS_ENABLED") == "true"
	tlsCACert := os.Getenv("STREAMLINE_TLS_CA_CERT")
	tlsClientCert := os.Getenv("STREAMLINE_TLS_CLIENT_CERT")
	tlsClientKey := os.Getenv("STREAMLINE_TLS_CLIENT_KEY")
	schemaRegistryURL := os.Getenv("STREAMLINE_SCHEMA_REGISTRY_URL")

	// Override with explicit configuration
	if !config.BootstrapServers.IsNull() {
		bootstrapServers = config.BootstrapServers.ValueString()
	}
	if !config.SaslMechanism.IsNull() {
		saslMechanism = config.SaslMechanism.ValueString()
	}
	if !config.SaslUsername.IsNull() {
		saslUsername = config.SaslUsername.ValueString()
	}
	if !config.SaslPassword.IsNull() {
		saslPassword = config.SaslPassword.ValueString()
	}
	if !config.TLSEnabled.IsNull() {
		tlsEnabled = config.TLSEnabled.ValueBool()
	}
	if !config.TLSCACert.IsNull() {
		tlsCACert = config.TLSCACert.ValueString()
	}
	if !config.TLSClientCert.IsNull() {
		tlsClientCert = config.TLSClientCert.ValueString()
	}
	if !config.TLSClientKey.IsNull() {
		tlsClientKey = config.TLSClientKey.ValueString()
	}
	if !config.SchemaRegistryURL.IsNull() {
		schemaRegistryURL = config.SchemaRegistryURL.ValueString()
	}

	// Validate required configuration
	if bootstrapServers == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("bootstrap_servers"),
			"Missing Streamline Bootstrap Servers",
			"The provider cannot create the Streamline client as there is a missing or empty value for the Streamline bootstrap servers. "+
				"Set the bootstrap_servers value in the configuration or use the STREAMLINE_BOOTSTRAP_SERVERS environment variable.",
		)
		return
	}

	// Validate bootstrap servers format and parse brokers
	var brokers []string
	for _, server := range strings.Split(bootstrapServers, ",") {
		server = strings.TrimSpace(server)
		if _, _, err := net.SplitHostPort(server); err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("bootstrap_servers"),
				"Invalid Bootstrap Server Format",
				fmt.Sprintf("Invalid bootstrap server '%s': %s. Expected format: 'host:port'", server, err),
			)
			return
		}
		brokers = append(brokers, server)
	}

	// Parse timeouts
	connectionTimeout := 30 * time.Second
	requestTimeout := 60 * time.Second
	if !config.ConnectionTimeout.IsNull() {
		connectionTimeout = time.Duration(config.ConnectionTimeout.ValueInt64()) * time.Second
	}
	if !config.RequestTimeout.IsNull() {
		requestTimeout = time.Duration(config.RequestTimeout.ValueInt64()) * time.Second
	}

	// Create Kafka client configuration
	kafkaConfig := client.Config{
		Brokers:        brokers,
		Timeout:        requestTimeout,
		TLSEnabled:     tlsEnabled,
		TLSCACertPath:  tlsCACert,
		TLSCertPath:    tlsClientCert,
		TLSKeyPath:     tlsClientKey,
	}

	// Configure SASL if specified
	if saslMechanism != "" {
		kafkaConfig.SASLMechanism = saslMechanism
		kafkaConfig.SASLUsername = saslUsername
		kafkaConfig.SASLPassword = saslPassword
	}

	// Create Kafka client
	kafkaClient, err := client.NewStreamlineClient(kafkaConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Create Kafka Client",
			fmt.Sprintf("Unable to create Streamline Kafka client: %s", err),
		)
		return
	}

	// Create clients container
	clients := &ProviderClients{
		Kafka: kafkaClient,
	}

	// Create Schema Registry client if URL is provided
	if schemaRegistryURL != "" {
		schemaRegistryConfig := client.SchemaRegistryConfig{
			URL:      schemaRegistryURL,
			Username: saslUsername, // Reuse SASL credentials
			Password: saslPassword,
			Timeout:  requestTimeout,
		}
		clients.SchemaRegistry = client.NewSchemaRegistryClient(schemaRegistryConfig)
	}

	tflog.Debug(ctx, "Created Streamline clients", map[string]any{
		"bootstrap_servers":   bootstrapServers,
		"sasl_mechanism":      saslMechanism,
		"tls_enabled":         tlsEnabled,
		"schema_registry_url": schemaRegistryURL,
		"connection_timeout":  connectionTimeout.String(),
	})

	resp.DataSourceData = clients
	resp.ResourceData = clients
}

// Helper to extract integer from environment variable
func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

// Resources defines the resources implemented in the provider.
func (p *StreamlineProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewTopicResource,
		resources.NewAclResource,
		resources.NewSchemaResource,
		resources.NewUserResource,
		resources.NewConsumerGroupResource,
	}
}

// DataSources defines the data sources implemented in the provider.
func (p *StreamlineProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewClusterDataSource,
		datasources.NewTopicsDataSource,
	}
}


// TLS configuration notes for v0.3.0:
// - tls_skip_verify: Skip TLS certificate verification (not recommended for production)
// - tls_ca_cert: Path to CA certificate for server verification
// - tls_client_cert: Path to client certificate for mutual TLS
// - tls_client_key: Path to client private key for mutual TLS
