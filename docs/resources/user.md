---
page_title: "streamline_user Resource - Streamline Provider"
subcategory: ""
description: |-
  Manages a Streamline SASL/SCRAM user.
---

# streamline_user (Resource)

Manages a Streamline SASL/SCRAM user.

Users are authenticated principals that can be granted access to Streamline resources via ACLs. This resource manages user credentials using the SASL/SCRAM mechanism.

## Example Usage

### Basic User

```hcl
resource "streamline_user" "alice" {
  username = "alice"
}
```

### User with Specific Mechanism

```hcl
resource "streamline_user" "producer_service" {
  username  = "producer-service"
  mechanism = "SCRAM-SHA-512"
}
```

### User with ACL

```hcl
resource "streamline_user" "consumer_app" {
  username  = "consumer-app"
  mechanism = "SCRAM-SHA-256"
}

resource "streamline_acl" "consumer_read" {
  resource_type   = "topic"
  resource_name   = "events"
  principal       = "User:${streamline_user.consumer_app.username}"
  operation       = "read"
  permission_type = "allow"
}
```

## Argument Reference

- `username` - (Required, Forces new resource) The username for the SASL/SCRAM user.
- `mechanism` - (Optional, Forces new resource) The SASL mechanism: `SCRAM-SHA-256` (default) or `SCRAM-SHA-512`.

## Attribute Reference

- `id` - The ID of the user (same as username).

## Import

Users can be imported using the username:

```shell
terraform import streamline_user.example alice
```
