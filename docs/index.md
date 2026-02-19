# Streamline Provider

The Streamline provider enables Terraform to manage [Streamline](https://github.com/streamline-platform/streamline) streaming platform resources.

Streamline is a Kafka-compatible streaming platform with support for:
- Topics and partitions
- Access Control Lists (ACLs)
- Schema Registry
- Consumer Groups

## Example Usage

```hcl
terraform {
  required_providers {
    streamline = {
      source  = "streamline-platform/streamline"
      version = "~> 1.0"
    }
  }
}

provider "streamline" {
  bootstrap_servers = "localhost:9092"
}

# Create a topic
resource "streamline_topic" "events" {
  name       = "events"
  partitions = 6

  config = {
    "retention.ms" = "604800000"  # 7 days
  }
}

# Grant read access
resource "streamline_acl" "consumer_read" {
  resource_type   = "topic"
  resource_name   = streamline_topic.events.name
  pattern_type    = "literal"
  principal       = "User:consumer-app"
  operation       = "read"
  permission_type = "allow"
}
```

## Authentication

The provider supports multiple authentication methods:

### No Authentication (Development)

```hcl
provider "streamline" {
  bootstrap_servers = "localhost:9092"
}
```

### SASL/PLAIN

```hcl
provider "streamline" {
  bootstrap_servers = "kafka.example.com:9092"
  sasl_mechanism    = "PLAIN"
  sasl_username     = "admin"
  sasl_password     = var.kafka_password
}
```

### SASL/SCRAM

```hcl
provider "streamline" {
  bootstrap_servers = "kafka.example.com:9092"
  sasl_mechanism    = "SCRAM-SHA-256"
  sasl_username     = "admin"
  sasl_password     = var.kafka_password
}
```

### mTLS

```hcl
provider "streamline" {
  bootstrap_servers = "kafka.example.com:9092"
  tls_enabled       = true
  tls_ca_cert       = "/path/to/ca.crt"
  tls_client_cert   = "/path/to/client.crt"
  tls_client_key    = "/path/to/client.key"
}
```

## Environment Variables

The provider can be configured using environment variables:

| Variable | Description |
|----------|-------------|
| `STREAMLINE_BOOTSTRAP_SERVERS` | Comma-separated list of bootstrap servers |
| `STREAMLINE_SASL_MECHANISM` | SASL mechanism (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512) |
| `STREAMLINE_SASL_USERNAME` | SASL username |
| `STREAMLINE_SASL_PASSWORD` | SASL password |
| `STREAMLINE_TLS_ENABLED` | Enable TLS (true/false) |
| `STREAMLINE_TLS_CA_CERT` | Path to CA certificate |
| `STREAMLINE_TLS_CLIENT_CERT` | Path to client certificate |
| `STREAMLINE_TLS_CLIENT_KEY` | Path to client private key |
| `STREAMLINE_SCHEMA_REGISTRY_URL` | Schema Registry URL |

## Argument Reference

- `bootstrap_servers` - (Required) Comma-separated list of bootstrap servers.
- `sasl_mechanism` - (Optional) SASL mechanism for authentication.
- `sasl_username` - (Optional) SASL username.
- `sasl_password` - (Optional, Sensitive) SASL password.
- `tls_enabled` - (Optional) Enable TLS for connections.
- `tls_ca_cert` - (Optional) Path to CA certificate file.
- `tls_client_cert` - (Optional) Path to client certificate for mTLS.
- `tls_client_key` - (Optional) Path to client private key for mTLS.
- `connection_timeout` - (Optional) Connection timeout in seconds. Default: 30.
- `request_timeout` - (Optional) Request timeout in seconds. Default: 60.
- `schema_registry_url` - (Optional) Schema Registry URL for schema management (e.g., `http://localhost:8081`).
