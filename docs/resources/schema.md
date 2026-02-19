---
page_title: "streamline_schema Resource - Streamline Provider"
subcategory: ""
description: |-
  Manages a Streamline Schema Registry schema.
---

# streamline_schema (Resource)

Manages a Streamline Schema Registry schema.

The Schema Registry provides a serving layer for metadata. It provides a RESTful interface for storing and retrieving Avro, JSON Schema, and Protobuf schemas.

~> **Note:** Requires `schema_registry_url` to be configured in the provider.

## Example Usage

### Avro Schema

```hcl
resource "streamline_schema" "user_events_value" {
  subject     = "user-events-value"
  schema_type = "AVRO"

  schema = jsonencode({
    type      = "record"
    name      = "UserEvent"
    namespace = "com.example.events"
    fields = [
      { name = "user_id", type = "string" },
      { name = "event_type", type = "string" },
      { name = "timestamp", type = "long" }
    ]
  })

  compatibility = "BACKWARD"
}
```

### JSON Schema

```hcl
resource "streamline_schema" "order_value" {
  subject     = "orders-value"
  schema_type = "JSON"

  schema = jsonencode({
    "$schema" = "http://json-schema.org/draft-07/schema#"
    type      = "object"
    properties = {
      order_id = { type = "string" }
      total    = { type = "number" }
    }
    required = ["order_id", "total"]
  })
}
```

## Argument Reference

- `subject` - (Required, Forces new resource) The subject name (e.g., `topic-value` or `topic-key`).
- `schema_type` - (Optional) The schema type: `AVRO` (default), `JSON`, or `PROTOBUF`.
- `schema` - (Required) The schema definition as a string.
- `compatibility` - (Optional) The compatibility level: `BACKWARD` (default), `BACKWARD_TRANSITIVE`, `FORWARD`, `FORWARD_TRANSITIVE`, `FULL`, `FULL_TRANSITIVE`, or `NONE`.
- `references` - (Optional) Schema references for complex schemas. Each reference has:
  - `name` - (Required) The name of the reference.
  - `subject` - (Required) The subject of the referenced schema.
  - `version` - (Required) The version of the referenced schema.

## Attribute Reference

- `id` - The unique identifier (`subject:version`).
- `version` - The version number of the schema.
- `schema_id` - The globally unique schema ID.

## Import

Schemas can be imported using the subject name:

```shell
terraform import streamline_schema.example my-topic-value
```
