// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserResource_basic(t *testing.T) {
	testAccPreCheck(t)
	username := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccUserResourceConfig(username),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_user.test", "username", username),
					resource.TestCheckResourceAttr("streamline_user.test", "mechanism", "SCRAM-SHA-256"),
					resource.TestCheckResourceAttr("streamline_user.test", "id", username),
				),
			},
			// ImportState
			{
				ResourceName:      "streamline_user.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccUserResource_sha512(t *testing.T) {
	testAccPreCheck(t)
	username := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceConfigWithMechanism(username, "SCRAM-SHA-512"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_user.test", "username", username),
					resource.TestCheckResourceAttr("streamline_user.test", "mechanism", "SCRAM-SHA-512"),
				),
			},
		},
	})
}

func testAccUserResourceConfig(username string) string {
	return providerConfig() + fmt.Sprintf(`
resource "streamline_user" "test" {
  username = %[1]q
}
`, username)
}

func testAccUserResourceConfigWithMechanism(username, mechanism string) string {
	return providerConfig() + fmt.Sprintf(`
resource "streamline_user" "test" {
  username  = %[1]q
  mechanism = %[2]q
}
`, username, mechanism)
}
