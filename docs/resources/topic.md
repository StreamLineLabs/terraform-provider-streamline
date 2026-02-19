---
page_title: "streamline_topic Resource - Streamline Provider"
subcategory: ""
description: |-
  Manages a Streamline topic.
---

# streamline_topic (Resource)

Manages a Streamline topic.

Topics are the fundamental unit of organization in Streamline. Each topic is a category or feed name to which records are published.

## Example Usage

### Basic Topic

```hcl
resource "streamline_topic" "events" {
  name       = "events"
  partitions = 6
}
```

### Topic with Configuration

```hcl
resource "streamline_topic" "user_state" {
  name              = "user-state"
  partitions        = 12
  cleanup_policy    = "compact"
  retention_ms      = -1  # Infinite retention for compacted topics

  config = {
    "min.insync.replicas" = "2"
  }
}
```

### Topic with Retention Policy

```hcl
resource "streamline_topic" "logs" {
  name           = "application-logs"
  partitions     = 3
  retention_ms   = 604800000  # 7 days
  retention_bytes = 1073741824  # 1 GB per partition
}
```

## Argument Reference

- `name` - (Required, Forces new resource) The name of the topic. Must be unique within the cluster.
- `partitions` - (Required) The number of partitions for the topic. Can only be increased, not decreased.
- `replication_factor` - (Optional) The replication factor for the topic. Defaults to `1`.
- `config` - (Optional) Topic configuration overrides as key-value pairs.
- `cleanup_policy` - (Optional) The cleanup policy: `delete`, `compact`, or `compact,delete`. Defaults to `delete`.
- `retention_ms` - (Optional) Retention time in milliseconds. `-1` for infinite retention.
- `retention_bytes` - (Optional) Retention size in bytes per partition. `-1` for unlimited.
- `segment_bytes` - (Optional) Segment file size in bytes.
- `min_insync_replicas` - (Optional) Minimum number of in-sync replicas required for a write to succeed. Defaults to `1`.

## Attribute Reference

- `id` - The ID of the topic (same as name).

## Import

Topics can be imported using the topic name:

```shell
terraform import streamline_topic.example my-topic
```
