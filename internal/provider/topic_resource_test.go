// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTopicResource_basic(t *testing.T) {
	testAccPreCheck(t)
	topicName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccTopicResourceConfig(topicName, 3),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_topic.test", "name", topicName),
					resource.TestCheckResourceAttr("streamline_topic.test", "partitions", "3"),
					resource.TestCheckResourceAttr("streamline_topic.test", "replication_factor", "1"),
				),
			},
			// ImportState
			{
				ResourceName:      "streamline_topic.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update partitions (increase)
			{
				Config: testAccTopicResourceConfig(topicName, 6),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_topic.test", "partitions", "6"),
				),
			},
		},
	})
}

func TestAccTopicResource_withConfig(t *testing.T) {
	testAccPreCheck(t)
	topicName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTopicResourceConfigWithOptions(topicName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_topic.test", "name", topicName),
					resource.TestCheckResourceAttr("streamline_topic.test", "partitions", "6"),
					resource.TestCheckResourceAttr("streamline_topic.test", "cleanup_policy", "compact"),
					resource.TestCheckResourceAttr("streamline_topic.test", "retention_ms", "-1"),
				),
			},
		},
	})
}

func TestAccTopicResource_disappears(t *testing.T) {
	testAccPreCheck(t)
	topicName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTopicResourceConfig(topicName, 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_topic.test", "name", topicName),
				),
			},
		},
	})
}

func testAccTopicResourceConfig(name string, partitions int) string {
	return providerConfig() + fmt.Sprintf(`
resource "streamline_topic" "test" {
  name       = %[1]q
  partitions = %[2]d
}
`, name, partitions)
}

func testAccTopicResourceConfigWithOptions(name string) string {
	return providerConfig() + fmt.Sprintf(`
resource "streamline_topic" "test" {
  name           = %[1]q
  partitions     = 6
  cleanup_policy = "compact"
  retention_ms   = -1
}
`, name)
}
