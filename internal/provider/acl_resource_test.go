// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAclResource_basic(t *testing.T) {
	testAccPreCheck(t)
	topicName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccAclResourceConfig(topicName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_acl.test", "resource_type", "topic"),
					resource.TestCheckResourceAttr("streamline_acl.test", "resource_name", topicName),
					resource.TestCheckResourceAttr("streamline_acl.test", "principal", "User:test-user"),
					resource.TestCheckResourceAttr("streamline_acl.test", "operation", "read"),
					resource.TestCheckResourceAttr("streamline_acl.test", "permission_type", "allow"),
					resource.TestCheckResourceAttr("streamline_acl.test", "pattern_type", "literal"),
					resource.TestCheckResourceAttr("streamline_acl.test", "host", "*"),
				),
			},
		},
	})
}

func TestAccAclResource_prefixed(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAclResourcePrefixedConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_acl.test", "pattern_type", "prefixed"),
					resource.TestCheckResourceAttr("streamline_acl.test", "resource_name", "logs-"),
					resource.TestCheckResourceAttr("streamline_acl.test", "operation", "write"),
				),
			},
		},
	})
}

func testAccAclResourceConfig(topicName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "streamline_acl" "test" {
  resource_type   = "topic"
  resource_name   = %[1]q
  principal       = "User:test-user"
  operation       = "read"
  permission_type = "allow"
}
`, topicName)
}

func testAccAclResourcePrefixedConfig() string {
	return providerConfig() + `
resource "streamline_acl" "test" {
  resource_type   = "topic"
  resource_name   = "logs-"
  pattern_type    = "prefixed"
  principal       = "User:producer-service"
  operation       = "write"
  permission_type = "allow"
}
`
}
