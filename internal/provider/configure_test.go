// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/client"
	"github.com/streamlinelabs/terraform-provider-streamline/internal/provider"
)

// configureProvider runs Configure against an in-memory configuration made of
// the supplied attribute values; every other attribute is null.
func configureProvider(t *testing.T, attrs map[string]tftypes.Value) *fwprovider.ConfigureResponse {
	t.Helper()

	// Environment variables act as defaults inside Configure; neutralise them
	// so the test only exercises the explicit configuration above.
	for _, key := range []string{
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
	} {
		t.Setenv(key, "")
	}

	ctx := context.Background()
	p := provider.New("test")()

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

	p := provider.New("test")()

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
