# Terraform Provider for Streamline

[![CI](https://github.com/streamlinelabs/terraform-provider-streamline/actions/workflows/ci.yml/badge.svg)](https://github.com/streamlinelabs/terraform-provider-streamline/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://go.dev/)
[![Terraform](https://img.shields.io/badge/Terraform-1.5+-7B42BC.svg)](https://www.terraform.io/)

This Terraform provider enables Infrastructure as Code (IaC) management of Streamline streaming platform resources.

## Features

- **Topic Management**: Create, configure, and manage topics
- **ACL Management**: Fine-grained access control
- **Schema Registry**: Manage Avro, Protobuf, and JSON schemas
- **Data Sources**: Query cluster and topic information

## Requirements

- Terraform >= 1.0
- Go >= 1.21 (for building from source)
- A running Streamline cluster

## Installation

### From Terraform Registry (Recommended)

```hcl
terraform {
  required_providers {
    streamline = {
      source  = "streamline-platform/streamline"
      version = "~> 1.0"
    }
  }
}
```

### Building from Source

```bash
cd terraform-provider-streamline
go build -o terraform-provider-streamline
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/streamline-platform/streamline/1.0.0/darwin_amd64/
mv terraform-provider-streamline ~/.terraform.d/plugins/registry.terraform.io/streamline-platform/streamline/1.0.0/darwin_amd64/
```

## Quick Start

```hcl
provider "streamline" {
  bootstrap_servers = "localhost:9092"
}

resource "streamline_topic" "events" {
  name       = "events"
  partitions = 6

  config = {
    "retention.ms" = "604800000"
  }
}
```

## Resources

| Resource | Description |
|----------|-------------|
| `streamline_topic` | Manages topics |
| `streamline_acl` | Manages access control lists |
| `streamline_schema` | Manages Schema Registry schemas |

## Data Sources

| Data Source | Description |
|-------------|-------------|
| `streamline_cluster` | Retrieves cluster metadata |
| `streamline_topics` | Lists topics with filtering |

## Authentication

The provider supports:
- No authentication (development)
- SASL/PLAIN
- SASL/SCRAM-SHA-256
- SASL/SCRAM-SHA-512
- mTLS (mutual TLS)

See [docs/index.md](docs/index.md) for detailed authentication examples.

## Examples

- [Basic Setup](examples/basic/) - Local development
- [Production Setup](examples/production/) - Enterprise with auth and ACLs

## Development

```bash
# Run tests
go test ./...

# Build
go build -o terraform-provider-streamline

# Generate documentation
go generate ./...
```

## License

Apache License 2.0
<!-- fix: 5c183633 -->
<!-- style: 001ff437 -->

