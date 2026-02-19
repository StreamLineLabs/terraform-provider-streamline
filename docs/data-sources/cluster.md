---
page_title: "streamline_cluster Data Source - Streamline Provider"
subcategory: ""
description: |-
  Retrieves cluster metadata from a Streamline cluster.
---

# streamline_cluster (Data Source)

Retrieves cluster metadata from a Streamline cluster, including broker information and the controller ID.

## Example Usage

```hcl
data "streamline_cluster" "current" {}

output "cluster_id" {
  value = data.streamline_cluster.current.cluster_id
}

output "controller_id" {
  value = data.streamline_cluster.current.controller_id
}

output "broker_count" {
  value = length(data.streamline_cluster.current.brokers)
}
```

## Attribute Reference

- `cluster_id` - The cluster identifier.
- `controller_id` - The broker ID of the current controller.
- `brokers` - List of brokers in the cluster. Each broker has:
  - `id` - The broker ID.
  - `host` - The broker hostname.
  - `port` - The broker port.
  - `rack` - The rack ID of the broker (if configured).
