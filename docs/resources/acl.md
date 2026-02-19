---
page_title: "streamline_acl Resource - Streamline Provider"
subcategory: ""
description: |-
  Manages a Streamline Access Control List (ACL) entry.
---

# streamline_acl (Resource)

Manages a Streamline Access Control List (ACL) entry.

ACLs control who can perform operations on Streamline resources. Each ACL entry specifies a principal, resource, operation, and permission. All attributes are immutable — any change requires replacing the resource.

## Example Usage

### Allow Read Access

```hcl
resource "streamline_acl" "alice_read_events" {
  resource_type   = "topic"
  resource_name   = "events"
  principal       = "User:alice"
  operation       = "read"
  permission_type = "allow"
}
```

### Prefixed Pattern

```hcl
resource "streamline_acl" "producer_write_logs" {
  resource_type   = "topic"
  resource_name   = "logs-"
  pattern_type    = "prefixed"
  principal       = "User:producer-service"
  operation       = "write"
  permission_type = "allow"
}
```

## Argument Reference

- `resource_type` - (Required, Forces new resource) The type of resource: `topic`, `group`, `cluster`, `transactional_id`, or `delegation_token`.
- `resource_name` - (Required, Forces new resource) The name of the resource. Use `*` for all resources of the type.
- `pattern_type` - (Optional, Forces new resource) The pattern type: `literal` (default), `prefixed`, or `match`.
- `principal` - (Required, Forces new resource) The principal in the format `User:name` or `Group:name`.
- `host` - (Optional, Forces new resource) The host from which access is allowed. Defaults to `*` (all hosts).
- `operation` - (Required, Forces new resource) The operation: `all`, `read`, `write`, `create`, `delete`, `alter`, `describe`, `cluster_action`, `describe_configs`, `alter_configs`, `idempotent_write`.
- `permission_type` - (Required, Forces new resource) The permission type: `allow` or `deny`.

## Attribute Reference

- `id` - The unique identifier for the ACL entry (composite of all fields).

## Import

ACL resources cannot be imported due to their composite key nature.
