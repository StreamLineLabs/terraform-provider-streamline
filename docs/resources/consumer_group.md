---
page_title: "streamline_consumer_group Resource - Streamline Provider"
subcategory: ""
description: |-
  Manages a Streamline consumer group.
---

# streamline_consumer_group (Resource)

Manages a Streamline consumer group.

Consumer groups enable parallel consumption of topic partitions across multiple consumers. This resource allows you to inspect consumer group state and delete (reset) consumer groups via Terraform.

~> **Note:** Consumer groups are typically created automatically when consumers connect. This resource is primarily used for lifecycle management — reading state and deleting groups that are no longer needed.

## Example Usage

### Import an Existing Consumer Group

```hcl
resource "streamline_consumer_group" "order_processor" {
  group_id = "order-processor"
}
```

### Reference Group State in Other Resources

```hcl
resource "streamline_consumer_group" "analytics" {
  group_id = "analytics-pipeline"
}

output "analytics_group_state" {
  value = streamline_consumer_group.analytics.state
}

output "analytics_group_members" {
  value = streamline_consumer_group.analytics.members
}
```

## Argument Reference

- `group_id` - (Required, Forces new resource) The consumer group ID. Must be unique within the cluster.

## Attribute Reference

- `id` - The resource identifier (same as `group_id`).
- `state` - The current state of the consumer group. Possible values: `Stable`, `Empty`, `Dead`, `PreparingRebalance`, `CompletingRebalance`.
- `members` - The number of active members in the consumer group.

## Import

Consumer groups can be imported using the group ID:

```shell
terraform import streamline_consumer_group.example my-consumer-group
```
