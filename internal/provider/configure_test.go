// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
)

// providerEnvKeys lists every environment variable read while resolving the
// provider configuration.
var providerEnvKeys = []string{
	"STREAMLINE_BOOTSTRAP_SERVERS",
	"STREAMLINE_SASL_MECHANISM",
	"STREAMLINE_SASL_USERNAME",
	"STREAMLINE_SASL_PASSWORD",
	"STREAMLINE_TLS_ENABLED",
	"STREAMLINE_TLS_CA_CERT",
	"STREAMLINE_TLS_CLIENT_CERT",
	"STREAMLINE_TLS_CLIENT_KEY",
	"STREAMLINE_SCHEMA_REGISTRY_URL",
	"STREAMLINE_MOONSHOT_URL",
	"STREAMLINE_MOONSHOT_TOKEN",
}

// clearProviderEnv empties every provider environment variable for the duration
// of the test. t.Setenv restores the previous values and forbids t.Parallel, so
// tests using it never mutate the environment concurrently.
func clearProviderEnv(t *testing.T) {
	t.Helper()

	for _, key := range providerEnvKeys {
		t.Setenv(key, "")
	}
}

// configureProvider runs Configure against an in-memory configuration made of
// the supplied attribute values; every other attribute is null.
func configureProvider(t *testing.T, attrs map[string]tftypes.Value) *fwprovider.ConfigureResponse {
	t.Helper()

	// Environment variables act as defaults inside Configure; neutralise them
	// so the test only exercises the explicit configuration above.
	clearProviderEnv(t)

	ctx := context.Background()
	p := New("test")()

	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(ctx, fwprovider.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("provider schema: %v", schemaResp.Diagnostics.Errors())
	}

	objType, ok := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("expected an object type, got %T", schemaResp.Schema.Type().TerraformType(ctx))
	}

	values := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, attrType := range objType.AttributeTypes {
		if v, set := attrs[name]; set {
			values[name] = v
			continue
		}
		values[name] = tftypes.NewValue(attrType, nil)
	}

	resp := &fwprovider.ConfigureResponse{}
	p.Configure(ctx, fwprovider.ConfigureRequest{
		Config: tfsdk.Config{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(objType, values),
		},
	}, resp)

	return resp
}

func TestProviderConfigure_PublishesSharedClientContainer(t *testing.T) {
	resp := configureProvider(t, map[string]tftypes.Value{
		"bootstrap_servers":   tftypes.NewValue(tftypes.String, "localhost:9092"),
		"schema_registry_url": tftypes.NewValue(tftypes.String, "http://localhost:8081"),
		"moonshot_url":        tftypes.NewValue(tftypes.String, "http://localhost:9094"),
	})

	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure returned errors: %v", resp.Diagnostics.Errors())
	}

	clients, ok := resp.ResourceData.(*client.Clients)
	if !ok {
		t.Fatalf("ResourceData is %T, want *client.Clients", resp.ResourceData)
	}
	if resp.DataSourceData != resp.ResourceData {
		t.Fatalf("DataSourceData (%T) and ResourceData (%T) must be the same container",
			resp.DataSourceData, resp.ResourceData)
	}
	if clients.Kafka == nil {
		t.Fatal("Kafka client was not initialized")
	}
	if clients.SchemaRegistry == nil {
		t.Fatal("SchemaRegistry client was not initialized despite schema_registry_url being set")
	}
	if clients.Moonshot == nil {
		t.Fatal("Moonshot client was not initialized despite moonshot_url being set")
	}
}

// TestProviderConfigure_ProviderDataAcceptedEverywhere is the end-to-end guard
// for the wiring: every registered resource and data source must accept the
// exact value Configure publishes.
func TestProviderConfigure_ProviderDataAcceptedEverywhere(t *testing.T) {
	ctx := context.Background()

	resp := configureProvider(t, map[string]tftypes.Value{
		"bootstrap_servers":   tftypes.NewValue(tftypes.String, "localhost:9092"),
		"schema_registry_url": tftypes.NewValue(tftypes.String, "http://localhost:8081"),
		"moonshot_url":        tftypes.NewValue(tftypes.String, "http://localhost:9094"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure returned errors: %v", resp.Diagnostics.Errors())
	}

	p := New("test")()

	for _, newResource := range p.Resources(ctx) {
		res := newResource()

		metaResp := &resource.MetadataResponse{}
		res.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "streamline"}, metaResp)

		configurable, ok := res.(resource.ResourceWithConfigure)
		if !ok {
			t.Fatalf("%s does not implement resource.ResourceWithConfigure", metaResp.TypeName)
		}

		t.Run(metaResp.TypeName, func(t *testing.T) {
			configureResp := &resource.ConfigureResponse{}
			configurable.Configure(ctx, resource.ConfigureRequest{ProviderData: resp.ResourceData}, configureResp)

			if configureResp.Diagnostics.HasError() {
				t.Fatalf("Configure rejected the provider data: %v", configureResp.Diagnostics.Errors())
			}
		})
	}

	for _, newDataSource := range p.DataSources(ctx) {
		ds := newDataSource()

		metaResp := &datasource.MetadataResponse{}
		ds.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "streamline"}, metaResp)

		configurable, ok := ds.(datasource.DataSourceWithConfigure)
		if !ok {
			t.Fatalf("%s does not implement datasource.DataSourceWithConfigure", metaResp.TypeName)
		}

		t.Run(metaResp.TypeName, func(t *testing.T) {
			configureResp := &datasource.ConfigureResponse{}
			configurable.Configure(ctx, datasource.ConfigureRequest{ProviderData: resp.DataSourceData}, configureResp)

			if configureResp.Diagnostics.HasError() {
				t.Fatalf("Configure rejected the provider data: %v", configureResp.Diagnostics.Errors())
			}
		})
	}
}

func TestProviderConfigure_RequiresBootstrapServers(t *testing.T) {
	resp := configureProvider(t, nil)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error when bootstrap_servers is unset")
	}
	if resp.ResourceData != nil || resp.DataSourceData != nil {
		t.Fatal("no clients should be published when configuration is invalid")
	}
}

func TestProviderSchemaHasPlanTimeValidators(t *testing.T) {
	t.Parallel()

	resp := &fwprovider.SchemaResponse{}
	New("test")().Schema(context.Background(), fwprovider.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %v", resp.Diagnostics.Errors())
	}

	for _, name := range []string{"bootstrap_servers", "sasl_mechanism", "schema_registry_url", "moonshot_url"} {
		attr, ok := resp.Schema.Attributes[name].(providerschema.StringAttribute)
		if !ok || len(attr.Validators) == 0 {
			t.Fatalf("%s must have a plan-time string validator", name)
		}
	}
	for _, name := range []string{"connection_timeout", "request_timeout"} {
		attr, ok := resp.Schema.Attributes[name].(providerschema.Int64Attribute)
		if !ok || len(attr.Validators) == 0 {
			t.Fatalf("%s must have a plan-time integer validator", name)
		}
	}
}

// resolveProviderConfig, validateBootstrapServers, and newProviderClients are
// exercised directly below: Configure only wires them to the framework.

func TestResolveProviderConfig(t *testing.T) {
	// t.Setenv is used per case, so these subtests must not run in parallel.
	tests := []struct {
		name  string
		env   map[string]string
		model StreamlineProviderModel
		want  providerConfig
	}{
		{
			name: "environment only",
			env: map[string]string{
				"STREAMLINE_BOOTSTRAP_SERVERS":   "env-host:9092",
				"STREAMLINE_SASL_MECHANISM":      "SCRAM-SHA-512",
				"STREAMLINE_SASL_USERNAME":       "env-user",
				"STREAMLINE_SASL_PASSWORD":       "env-password",
				"STREAMLINE_TLS_ENABLED":         "true",
				"STREAMLINE_TLS_CA_CERT":         "/env/ca.pem",
				"STREAMLINE_TLS_CLIENT_CERT":     "/env/client.pem",
				"STREAMLINE_TLS_CLIENT_KEY":      "/env/client.key",
				"STREAMLINE_SCHEMA_REGISTRY_URL": "http://env-registry:8081",
				"STREAMLINE_MOONSHOT_URL":        "http://env-moonshot:9094",
				"STREAMLINE_MOONSHOT_TOKEN":      "env-token",
			},
			want: providerConfig{
				bootstrapServers:  "env-host:9092",
				saslMechanism:     "SCRAM-SHA-512",
				saslUsername:      "env-user",
				saslPassword:      "env-password",
				tlsEnabled:        true,
				tlsCACert:         "/env/ca.pem",
				tlsClientCert:     "/env/client.pem",
				tlsClientKey:      "/env/client.key",
				schemaRegistryURL: "http://env-registry:8081",
				moonshotURL:       "http://env-moonshot:9094",
				moonshotToken:     "env-token",
				connectionTimeout: 30 * time.Second,
				requestTimeout:    60 * time.Second,
			},
		},
		{
			name: "tls enabled requires the literal true",
			env: map[string]string{
				"STREAMLINE_BOOTSTRAP_SERVERS": "env-host:9092",
				"STREAMLINE_TLS_ENABLED":       "TRUE",
			},
			want: providerConfig{
				bootstrapServers:  "env-host:9092",
				connectionTimeout: 30 * time.Second,
				requestTimeout:    60 * time.Second,
			},
		},
		{
			name: "configuration overrides environment",
			env: map[string]string{
				"STREAMLINE_BOOTSTRAP_SERVERS":   "env-host:9092",
				"STREAMLINE_SASL_MECHANISM":      "PLAIN",
				"STREAMLINE_SASL_USERNAME":       "env-user",
				"STREAMLINE_SASL_PASSWORD":       "env-password",
				"STREAMLINE_TLS_ENABLED":         "true",
				"STREAMLINE_TLS_CA_CERT":         "/env/ca.pem",
				"STREAMLINE_TLS_CLIENT_CERT":     "/env/client.pem",
				"STREAMLINE_TLS_CLIENT_KEY":      "/env/client.key",
				"STREAMLINE_SCHEMA_REGISTRY_URL": "http://env-registry:8081",
				"STREAMLINE_MOONSHOT_URL":        "http://env-moonshot:9094",
				"STREAMLINE_MOONSHOT_TOKEN":      "env-token",
			},
			model: StreamlineProviderModel{
				BootstrapServers:  types.StringValue("hcl-host:9092"),
				SaslMechanism:     types.StringValue("SCRAM-SHA-256"),
				SaslUsername:      types.StringValue("hcl-user"),
				SaslPassword:      types.StringValue("hcl-password"),
				TLSEnabled:        types.BoolValue(false),
				TLSCACert:         types.StringValue("/hcl/ca.pem"),
				TLSClientCert:     types.StringValue("/hcl/client.pem"),
				TLSClientKey:      types.StringValue("/hcl/client.key"),
				TLSSkipVerify:     types.BoolValue(true),
				SchemaRegistryURL: types.StringValue("http://hcl-registry:8081"),
				MoonshotURL:       types.StringValue("http://hcl-moonshot:9094"),
				MoonshotToken:     types.StringValue("hcl-token"),
			},
			want: providerConfig{
				bootstrapServers:  "hcl-host:9092",
				saslMechanism:     "SCRAM-SHA-256",
				saslUsername:      "hcl-user",
				saslPassword:      "hcl-password",
				tlsEnabled:        false,
				tlsCACert:         "/hcl/ca.pem",
				tlsClientCert:     "/hcl/client.pem",
				tlsClientKey:      "/hcl/client.key",
				tlsSkipVerify:     true,
				schemaRegistryURL: "http://hcl-registry:8081",
				moonshotURL:       "http://hcl-moonshot:9094",
				moonshotToken:     "hcl-token",
				connectionTimeout: 30 * time.Second,
				requestTimeout:    60 * time.Second,
			},
		},
		{
			name: "explicit empty configuration wins over environment",
			env: map[string]string{
				"STREAMLINE_BOOTSTRAP_SERVERS":   "env-host:9092",
				"STREAMLINE_SCHEMA_REGISTRY_URL": "http://env-registry:8081",
				"STREAMLINE_MOONSHOT_URL":        "http://env-moonshot:9094",
			},
			model: StreamlineProviderModel{
				SchemaRegistryURL: types.StringValue(""),
				MoonshotURL:       types.StringValue(""),
			},
			want: providerConfig{
				bootstrapServers:  "env-host:9092",
				connectionTimeout: 30 * time.Second,
				requestTimeout:    60 * time.Second,
			},
		},
		{
			name: "timeout defaults apply when unset",
			model: StreamlineProviderModel{
				BootstrapServers: types.StringValue("localhost:9092"),
			},
			want: providerConfig{
				bootstrapServers:  "localhost:9092",
				connectionTimeout: 30 * time.Second,
				requestTimeout:    60 * time.Second,
			},
		},
		{
			name: "timeout overrides are seconds",
			model: StreamlineProviderModel{
				BootstrapServers:  types.StringValue("localhost:9092"),
				ConnectionTimeout: types.Int64Value(5),
				RequestTimeout:    types.Int64Value(120),
			},
			want: providerConfig{
				bootstrapServers:  "localhost:9092",
				connectionTimeout: 5 * time.Second,
				requestTimeout:    120 * time.Second,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearProviderEnv(t)
			for key, value := range test.env {
				t.Setenv(key, value)
			}

			if got := resolveProviderConfig(test.model); got != test.want {
				t.Errorf("resolveProviderConfig() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestValidateBootstrapServers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		bootstrapServers string
		wantBrokers      []string
		wantSummary      string
		wantDetail       string
	}{
		{
			name:             "single address",
			bootstrapServers: "localhost:9092",
			wantBrokers:      []string{"localhost:9092"},
		},
		{
			name:             "multiple addresses are trimmed",
			bootstrapServers: "localhost:9092, localhost:9093 ",
			wantBrokers:      []string{"localhost:9092", "localhost:9093"},
		},
		{
			name:             "missing value",
			bootstrapServers: "",
			wantSummary:      "Missing Streamline Bootstrap Servers",
			wantDetail: "The provider cannot create the Streamline client as there is a missing or empty value for the Streamline bootstrap servers. " +
				"Set the bootstrap_servers value in the configuration or use the STREAMLINE_BOOTSTRAP_SERVERS environment variable.",
		},
		{
			name:             "address without a port",
			bootstrapServers: "localhost",
			wantSummary:      "Invalid Bootstrap Server Format",
			wantDetail:       "Invalid bootstrap server 'localhost': address localhost: missing port in address. Expected format: 'host:port'",
		},
		{
			name:             "second address is invalid",
			bootstrapServers: "localhost:9092,localhost:9093:9094",
			wantSummary:      "Invalid Bootstrap Server Format",
			wantDetail:       "Invalid bootstrap server 'localhost:9093:9094': address localhost:9093:9094: too many colons in address. Expected format: 'host:port'",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			brokers, diags := validateBootstrapServers(test.bootstrapServers)

			if test.wantSummary == "" {
				if diags.HasError() {
					t.Fatalf("unexpected diagnostics: %v", diags.Errors())
				}
				if !reflect.DeepEqual(brokers, test.wantBrokers) {
					t.Fatalf("brokers = %v, want %v", brokers, test.wantBrokers)
				}
				return
			}

			if brokers != nil {
				t.Fatalf("brokers = %v, want none on error", brokers)
			}
			errs := diags.Errors()
			if len(errs) != 1 {
				t.Fatalf("got %d error diagnostics, want 1: %v", len(errs), errs)
			}
			if got := errs[0].Summary(); got != test.wantSummary {
				t.Errorf("summary = %q, want %q", got, test.wantSummary)
			}
			if got := errs[0].Detail(); got != test.wantDetail {
				t.Errorf("detail = %q, want %q", got, test.wantDetail)
			}

			withPath, ok := errs[0].(diag.DiagnosticWithPath)
			if !ok {
				t.Fatalf("diagnostic %T does not carry an attribute path", errs[0])
			}
			if got := withPath.Path(); !got.Equal(path.Root("bootstrap_servers")) {
				t.Errorf("path = %s, want %s", got, path.Root("bootstrap_servers"))
			}
		})
	}
}

func TestNewProviderClients(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		config             providerConfig
		brokers            []string
		wantSchemaRegistry bool
		wantMoonshot       bool
		wantSummary        string
	}{
		{
			name:    "kafka only",
			config:  providerConfig{requestTimeout: 60 * time.Second},
			brokers: []string{"localhost:9092"},
		},
		{
			name: "optional clients are created when their urls are set",
			config: providerConfig{
				saslMechanism:     "PLAIN",
				saslUsername:      "user",
				saslPassword:      "password",
				schemaRegistryURL: "http://localhost:8081",
				moonshotURL:       "http://localhost:9094",
				moonshotToken:     "token",
				requestTimeout:    60 * time.Second,
			},
			brokers:            []string{"localhost:9092"},
			wantSchemaRegistry: true,
			wantMoonshot:       true,
		},
		{
			name: "unsupported sasl mechanism fails",
			config: providerConfig{
				saslMechanism:  "NOPE",
				requestTimeout: 60 * time.Second,
			},
			brokers:     []string{"localhost:9092"},
			wantSummary: "Failed to Create Kafka Client",
		},
		{
			name:        "missing brokers fail",
			config:      providerConfig{requestTimeout: 60 * time.Second},
			wantSummary: "Failed to Create Kafka Client",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			clients, diags := newProviderClients(test.config, test.brokers)

			if test.wantSummary != "" {
				if clients != nil {
					t.Fatal("no clients should be returned when construction fails")
				}
				errs := diags.Errors()
				if len(errs) != 1 {
					t.Fatalf("got %d error diagnostics, want 1: %v", len(errs), errs)
				}
				if got := errs[0].Summary(); got != test.wantSummary {
					t.Errorf("summary = %q, want %q", got, test.wantSummary)
				}
				return
			}

			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags.Errors())
			}
			if clients.Kafka == nil {
				t.Error("Kafka client was not initialized")
			}
			if got := clients.SchemaRegistry != nil; got != test.wantSchemaRegistry {
				t.Errorf("SchemaRegistry initialized = %t, want %t", got, test.wantSchemaRegistry)
			}
			if got := clients.Moonshot != nil; got != test.wantMoonshot {
				t.Errorf("Moonshot initialized = %t, want %t", got, test.wantMoonshot)
			}
		})
	}
}

func TestValidateResolvedProviderConfig(t *testing.T) {
	t.Parallel()

	valid := providerConfig{
		bootstrapServers:  "localhost:9092",
		connectionTimeout: time.Second,
		requestTimeout:    time.Second,
	}
	if diags := validateResolvedProviderConfig(valid); diags.HasError() {
		t.Fatalf("valid configuration returned diagnostics: %v", diags.Errors())
	}

	tests := map[string]providerConfig{
		"non-positive timeouts": {
			connectionTimeout: 0,
			requestTimeout:    -time.Second,
		},
		"credentials without mechanism": {
			saslUsername:      "user",
			saslPassword:      "password",
			connectionTimeout: time.Second,
			requestTimeout:    time.Second,
		},
		"mechanism without credentials": {
			saslMechanism:     "SCRAM-SHA-256",
			connectionTimeout: time.Second,
			requestTimeout:    time.Second,
		},
		"unsupported mechanism": {
			saslMechanism:     "NOPE",
			saslUsername:      "user",
			saslPassword:      "password",
			connectionTimeout: time.Second,
			requestTimeout:    time.Second,
		},
		"incomplete client certificate": {
			tlsClientCert:     "client.pem",
			connectionTimeout: time.Second,
			requestTimeout:    time.Second,
		},
		"invalid service URLs": {
			schemaRegistryURL: "registry.example.com",
			moonshotURL:       "ftp://moonshot.example.com",
			connectionTimeout: time.Second,
			requestTimeout:    time.Second,
		},
		"token without URL": {
			moonshotToken:     "token",
			connectionTimeout: time.Second,
			requestTimeout:    time.Second,
		},
	}

	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diags := validateResolvedProviderConfig(config); !diags.HasError() {
				t.Fatal("expected validation diagnostics")
			}
		})
	}
}
