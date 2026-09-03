// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/streamlinelabs/terraform-provider-streamline/internal/provider"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"streamline": providerserver.NewProtocol6WithError(provider.New("test")()),
}

var testAccSchemaProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"streamline": providerserver.NewProtocol6WithError(provider.NewForSchemaAcceptanceTests("test")()),
}

func testAccPreCheck(t *testing.T) {
	t.Helper()

	if v := os.Getenv("STREAMLINE_BOOTSTRAP_SERVERS"); v == "" {
		t.Skip("STREAMLINE_BOOTSTRAP_SERVERS must be set for acceptance tests")
	}
}

func providerConfig() string {
	return fmt.Sprintf(`
provider "streamline" {
  bootstrap_servers = %q
}
`, os.Getenv("STREAMLINE_BOOTSTRAP_SERVERS"))
}

func TestProviderPreservesLegacyResourceModels(t *testing.T) {
	t.Parallel()

	p := provider.New("test")()
	var resourceNames []string
	for _, factory := range p.Resources(context.Background()) {
		resp := &resource.MetadataResponse{}
		factory().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "streamline"}, resp)
		resourceNames = append(resourceNames, resp.TypeName)
	}
	if !slices.Contains(resourceNames, "streamline_user") {
		t.Fatal("provider must preserve the legacy user resource schema")
	}
	if !slices.Contains(resourceNames, "streamline_consumer_group") {
		t.Fatal("provider must preserve the legacy consumer-group resource schema")
	}

	var dataSources []string
	for _, factory := range p.DataSources(context.Background()) {
		resp := &datasource.MetadataResponse{}
		factory().Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "streamline"}, resp)
		dataSources = append(dataSources, resp.TypeName)
	}
	if !slices.Contains(dataSources, "streamline_consumer_group") {
		t.Fatal("provider must advertise the read-only consumer-group data source")
	}
}
