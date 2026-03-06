// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
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

func testAccPreCheck(t *testing.T) {
	t.Helper()

	if v := os.Getenv("STREAMLINE_BOOTSTRAP_SERVERS"); v == "" {
		t.Skip("STREAMLINE_BOOTSTRAP_SERVERS must be set for acceptance tests")
	}
}

func providerConfig() string {
	return `
provider "streamline" {
  bootstrap_servers = "localhost:9092"
}
`
}
