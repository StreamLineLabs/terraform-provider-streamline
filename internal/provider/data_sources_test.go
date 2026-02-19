// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccClusterDataSource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccClusterDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.streamline_cluster.test", "cluster_id"),
					resource.TestCheckResourceAttrSet("data.streamline_cluster.test", "controller_id"),
				),
			},
		},
	})
}

func TestAccTopicsDataSource_basic(t *testing.T) {
	testAccPreCheck(t)
	topicName := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTopicsDataSourceConfig(topicName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.streamline_topics.test", "topics.#"),
				),
			},
		},
	})
}

func testAccClusterDataSourceConfig() string {
	return providerConfig() + `
data "streamline_cluster" "test" {}
`
}

func testAccTopicsDataSourceConfig(topicName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "streamline_topic" "prerequisite" {
  name       = %[1]q
  partitions = 1
}

data "streamline_topics" "test" {
  depends_on = [streamline_topic.prerequisite]
}
`, topicName)
}
