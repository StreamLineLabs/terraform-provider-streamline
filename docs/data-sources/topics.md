---
page_title: "streamline_topics Data Source - Streamline Provider"
subcategory: ""
description: |-
  Lists all topics in a Streamline cluster.
---

# streamline_topics (Data Source)

Lists all topics in a Streamline cluster with their metadata.

## Example Usage

```hcl
data "streamline_topics" "all" {}

output "topic_names" {
  value = [for t in data.streamline_topics.all.topics : t.name]
}

output "topic_count" {
  value = length(data.streamline_topics.all.topics)
}
```

## Attribute Reference

- `topics` - List of topics in the cluster. Each topic has:
  - `name` - The topic name.
  - `partitions` - The number of partitions.
  - `replication_factor` - The replication factor.
  - `internal` - Whether this is an internal topic.

## Argument Reference

- `name_pattern` - (Optional) Regex pattern to filter topics by name.
