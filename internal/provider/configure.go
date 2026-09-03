// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

const (
	defaultConnectionTimeout = 30 * time.Second
	defaultRequestTimeout    = 60 * time.Second
)

// providerConfig holds the provider settings after the Terraform configuration,
// the environment variables, and the built-in defaults have been reconciled.
type providerConfig struct {
	bootstrapServers  string
	saslMechanism     string
	saslUsername      string
	saslPassword      string
	tlsEnabled        bool
	tlsCACert         string
	tlsClientCert     string
	tlsClientKey      string
	tlsSkipVerify     bool
	schemaRegistryURL string
	moonshotURL       string
	moonshotToken     string
	connectionTimeout time.Duration
	requestTimeout    time.Duration
}

// resolveProviderConfig applies the provider precedence rules: environment
// variables supply the defaults and any explicitly configured attribute wins.
func resolveProviderConfig(model StreamlineProviderModel) providerConfig {
	cfg := providerConfig{
		bootstrapServers:  os.Getenv("STREAMLINE_BOOTSTRAP_SERVERS"),
		saslMechanism:     os.Getenv("STREAMLINE_SASL_MECHANISM"),
		saslUsername:      os.Getenv("STREAMLINE_SASL_USERNAME"),
		saslPassword:      os.Getenv("STREAMLINE_SASL_PASSWORD"),
		tlsEnabled:        os.Getenv("STREAMLINE_TLS_ENABLED") == "true",
		tlsCACert:         os.Getenv("STREAMLINE_TLS_CA_CERT"),
		tlsClientCert:     os.Getenv("STREAMLINE_TLS_CLIENT_CERT"),
		tlsClientKey:      os.Getenv("STREAMLINE_TLS_CLIENT_KEY"),
		schemaRegistryURL: os.Getenv("STREAMLINE_SCHEMA_REGISTRY_URL"),
		moonshotURL:       os.Getenv("STREAMLINE_MOONSHOT_URL"),
		moonshotToken:     os.Getenv("STREAMLINE_MOONSHOT_TOKEN"),
		tlsSkipVerify:     model.TLSSkipVerify.ValueBool(),
		connectionTimeout: defaultConnectionTimeout,
		requestTimeout:    defaultRequestTimeout,
	}

	if !model.BootstrapServers.IsNull() {
		cfg.bootstrapServers = model.BootstrapServers.ValueString()
	}
	if !model.SaslMechanism.IsNull() {
		cfg.saslMechanism = model.SaslMechanism.ValueString()
	}
	if !model.SaslUsername.IsNull() {
		cfg.saslUsername = model.SaslUsername.ValueString()
	}
	if !model.SaslPassword.IsNull() {
		cfg.saslPassword = model.SaslPassword.ValueString()
	}
	if !model.TLSEnabled.IsNull() {
		cfg.tlsEnabled = model.TLSEnabled.ValueBool()
	}
	if !model.TLSCACert.IsNull() {
		cfg.tlsCACert = model.TLSCACert.ValueString()
	}
	if !model.TLSClientCert.IsNull() {
		cfg.tlsClientCert = model.TLSClientCert.ValueString()
	}
	if !model.TLSClientKey.IsNull() {
		cfg.tlsClientKey = model.TLSClientKey.ValueString()
	}
	if !model.SchemaRegistryURL.IsNull() {
		cfg.schemaRegistryURL = model.SchemaRegistryURL.ValueString()
	}
	if !model.MoonshotURL.IsNull() {
		cfg.moonshotURL = model.MoonshotURL.ValueString()
	}
	if !model.MoonshotToken.IsNull() {
		cfg.moonshotToken = model.MoonshotToken.ValueString()
	}
	if !model.ConnectionTimeout.IsNull() {
		cfg.connectionTimeout = time.Duration(model.ConnectionTimeout.ValueInt64()) * time.Second
	}
	if !model.RequestTimeout.IsNull() {
		cfg.requestTimeout = time.Duration(model.RequestTimeout.ValueInt64()) * time.Second
	}

	return cfg
}

func validateResolvedProviderConfig(cfg providerConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	if cfg.connectionTimeout <= 0 {
		diags.AddAttributeError(
			path.Root("connection_timeout"),
			"Invalid Connection Timeout",
			"connection_timeout must be at least 1 second.",
		)
	}
	if cfg.requestTimeout <= 0 {
		diags.AddAttributeError(
			path.Root("request_timeout"),
			"Invalid Request Timeout",
			"request_timeout must be at least 1 second.",
		)
	}

	if cfg.saslMechanism == "" {
		if cfg.saslUsername != "" || cfg.saslPassword != "" {
			diags.AddAttributeError(
				path.Root("sasl_mechanism"),
				"Missing SASL Mechanism",
				"Set sasl_mechanism when SASL credentials are configured.",
			)
		}
	} else {
		switch cfg.saslMechanism {
		case "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512":
		default:
			diags.AddAttributeError(
				path.Root("sasl_mechanism"),
				"Unsupported SASL Mechanism",
				"sasl_mechanism must be PLAIN, SCRAM-SHA-256, or SCRAM-SHA-512.",
			)
		}
		if cfg.saslUsername == "" {
			diags.AddAttributeError(
				path.Root("sasl_username"),
				"Missing SASL Username",
				"Set sasl_username when sasl_mechanism is configured.",
			)
		}
		if cfg.saslPassword == "" {
			diags.AddAttributeError(
				path.Root("sasl_password"),
				"Missing SASL Password",
				"Set sasl_password when sasl_mechanism is configured.",
			)
		}
	}

	if (cfg.tlsClientCert == "") != (cfg.tlsClientKey == "") {
		diags.AddAttributeError(
			path.Root("tls_client_cert"),
			"Incomplete TLS Client Certificate",
			"tls_client_cert and tls_client_key must be configured together.",
		)
	}

	validateURL := func(attribute, value string) {
		if value == "" {
			return
		}
		parsed, err := url.ParseRequestURI(value)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			diags.AddAttributeError(
				path.Root(attribute),
				"Invalid HTTP URL",
				fmt.Sprintf("%s must be an absolute HTTP or HTTPS URL.", attribute),
			)
		}
	}
	validateURL("schema_registry_url", cfg.schemaRegistryURL)
	validateURL("moonshot_url", cfg.moonshotURL)

	if cfg.moonshotToken != "" && cfg.moonshotURL == "" {
		diags.AddAttributeError(
			path.Root("moonshot_url"),
			"Missing Moonshot URL",
			"Set moonshot_url when moonshot_token is configured.",
		)
	}

	return diags
}

// validateBootstrapServers checks that bootstrap servers were supplied and that
// every address is a well-formed 'host:port' pair, returning the broker list.
func validateBootstrapServers(bootstrapServers string) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if bootstrapServers == "" {
		diags.AddAttributeError(
			path.Root("bootstrap_servers"),
			"Missing Streamline Bootstrap Servers",
			"The provider cannot create the Streamline client as there is a missing or empty value for the Streamline bootstrap servers. "+
				"Set the bootstrap_servers value in the configuration or use the STREAMLINE_BOOTSTRAP_SERVERS environment variable.",
		)
		return nil, diags
	}

	var brokers []string
	for _, server := range strings.Split(bootstrapServers, ",") {
		server = strings.TrimSpace(server)
		if _, _, err := net.SplitHostPort(server); err != nil {
			diags.AddAttributeError(
				path.Root("bootstrap_servers"),
				"Invalid Bootstrap Server Format",
				fmt.Sprintf("Invalid bootstrap server '%s': %s. Expected format: 'host:port'", server, err),
			)
			return nil, diags
		}
		brokers = append(brokers, server)
	}

	return brokers, diags
}

// newProviderClients builds the Kafka client plus the optional Schema Registry
// and Moonshot clients for the resolved configuration.
func newProviderClients(cfg providerConfig, brokers []string) (*client.Clients, diag.Diagnostics) {
	var diags diag.Diagnostics

	kafkaConfig := client.Config{
		Brokers:           brokers,
		ConnectionTimeout: cfg.connectionTimeout,
		Timeout:           cfg.requestTimeout,
		TLSEnabled:        cfg.tlsEnabled,
		TLSCACertPath:     cfg.tlsCACert,
		TLSCertPath:       cfg.tlsClientCert,
		TLSKeyPath:        cfg.tlsClientKey,
		TLSSkipVerify:     cfg.tlsSkipVerify,
	}

	// Configure SASL if specified
	if cfg.saslMechanism != "" {
		kafkaConfig.SASLMechanism = cfg.saslMechanism
		kafkaConfig.SASLUsername = cfg.saslUsername
		kafkaConfig.SASLPassword = cfg.saslPassword
	}

	kafkaClient, err := client.NewStreamlineClient(kafkaConfig)
	if err != nil {
		diags.AddError(
			"Failed to Create Kafka Client",
			fmt.Sprintf("Unable to create Streamline Kafka client: %s", err),
		)
		return nil, diags
	}

	clients := &client.Clients{
		Kafka: kafkaClient,
	}

	if cfg.schemaRegistryURL != "" {
		clients.SchemaRegistry = client.NewSchemaRegistryClient(client.SchemaRegistryConfig{
			URL:      cfg.schemaRegistryURL,
			Username: cfg.saslUsername, // Reuse SASL credentials
			Password: cfg.saslPassword,
			Timeout:  cfg.requestTimeout,
		})
	}

	if cfg.moonshotURL != "" {
		clients.Moonshot = client.NewMoonshotClient(client.MoonshotConfig{
			BaseURL: cfg.moonshotURL,
			Token:   cfg.moonshotToken,
			Timeout: cfg.requestTimeout,
		})
	}

	return clients, diags
}
