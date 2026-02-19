// Copyright (c) Streamline Platform
// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccSchemaPreCheck(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	if v := os.Getenv("STREAMLINE_SCHEMA_REGISTRY_URL"); v == "" {
		t.Skip("STREAMLINE_SCHEMA_REGISTRY_URL must be set for schema acceptance tests")
	}
}

func TestAccSchemaResource_avro(t *testing.T) {
	testAccSchemaPreCheck(t)
	subject := fmt.Sprintf("tf-acc-test-%s-value", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccSchemaResourceAvroConfig(subject),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_schema.test", "subject", subject),
					resource.TestCheckResourceAttr("streamline_schema.test", "schema_type", "AVRO"),
					resource.TestCheckResourceAttr("streamline_schema.test", "compatibility", "BACKWARD"),
					resource.TestCheckResourceAttrSet("streamline_schema.test", "schema_id"),
					resource.TestCheckResourceAttrSet("streamline_schema.test", "version"),
				),
			},
			// ImportState
			{
				ResourceName:      "streamline_schema.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{"schema"},
			},
		},
	})
}

func TestAccSchemaResource_json(t *testing.T) {
	testAccSchemaPreCheck(t)
	subject := fmt.Sprintf("tf-acc-test-%s-value", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSchemaResourceJsonConfig(subject),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_schema.test", "subject", subject),
					resource.TestCheckResourceAttr("streamline_schema.test", "schema_type", "JSON"),
				),
			},
		},
	})
}

func TestAccSchemaResource_updateCompatibility(t *testing.T) {
	testAccSchemaPreCheck(t)
	subject := fmt.Sprintf("tf-acc-test-%s-value", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSchemaResourceAvroConfig(subject),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_schema.test", "compatibility", "BACKWARD"),
				),
			},
			{
				Config: testAccSchemaResourceAvroConfigWithCompatibility(subject, "FULL"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("streamline_schema.test", "compatibility", "FULL"),
				),
			},
		},
	})
}

func schemaProviderConfig() string {
	return `
provider "streamline" {
  bootstrap_servers  = "localhost:9092"
  schema_registry_url = "http://localhost:8081"
}
`
}

func testAccSchemaResourceAvroConfig(subject string) string {
	return schemaProviderConfig() + fmt.Sprintf(`
resource "streamline_schema" "test" {
  subject     = %[1]q
  schema_type = "AVRO"

  schema = jsonencode({
    type      = "record"
    name      = "TestRecord"
    namespace = "com.example.test"
    fields = [
      { name = "id", type = "string" },
      { name = "name", type = "string" },
      { name = "timestamp", type = "long" }
    ]
  })

  compatibility = "BACKWARD"
}
`, subject)
}

func testAccSchemaResourceAvroConfigWithCompatibility(subject, compatibility string) string {
	return schemaProviderConfig() + fmt.Sprintf(`
resource "streamline_schema" "test" {
  subject     = %[1]q
  schema_type = "AVRO"

  schema = jsonencode({
    type      = "record"
    name      = "TestRecord"
    namespace = "com.example.test"
    fields = [
      { name = "id", type = "string" },
      { name = "name", type = "string" },
      { name = "timestamp", type = "long" }
    ]
  })

  compatibility = %[2]q
}
`, subject, compatibility)
}

func testAccSchemaResourceJsonConfig(subject string) string {
	return schemaProviderConfig() + fmt.Sprintf(`
resource "streamline_schema" "test" {
  subject     = %[1]q
  schema_type = "JSON"

  schema = jsonencode({
    "$schema" = "http://json-schema.org/draft-07/schema#"
    type      = "object"
    properties = {
      id   = { type = "string" }
      name = { type = "string" }
    }
    required = ["id", "name"]
  })
}
`, subject)
}
