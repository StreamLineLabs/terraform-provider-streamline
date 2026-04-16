# Terraform Provider for Streamline

[![CI](https://github.com/streamlinelabs/terraform-provider-streamline/actions/workflows/ci.yml/badge.svg)](https://github.com/streamlinelabs/terraform-provider-streamline/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://go.dev/)
[![Terraform](https://img.shields.io/badge/Terraform-1.5+-7B42BC.svg)](https://www.terraform.io/)
[![Release](https://img.shields.io/github/v/release/streamlinelabs/terraform-provider-streamline?label=release)](https://github.com/streamlinelabs/terraform-provider-streamline/releases)

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

## Architecture

Built on the [Terraform Plugin Framework v1.5+](https://developer.hashicorp.com/terraform/plugin/framework).
Resources are organized by domain:

```
internal/
├── provider/        # Provider config, schema, authentication
├── topic/           # streamline_topic resource + data source
├── acl/             # streamline_acl resource
├── schema/          # streamline_schema (Schema Registry) resource
├── cluster/         # streamline_cluster data source
└── client/          # Thin wrapper around the Streamline admin client
```

Each resource implements the standard Plugin Framework lifecycle:
`Schema`, `Configure`, `Create`, `Read`, `Update`, `Delete`, and (where it
makes sense) `ImportState`.

## Inner Loop

| Action | Command | Notes |
|--------|---------|-------|
| Build provider | `go build -o terraform-provider-streamline` | Output binary in repo root |
| Run unit tests | `go test ./...` | Fast (< 10 s); no external services |
| Run acceptance tests | `TF_ACC=1 go test ./... -v -run TestAcc` | Requires a running Streamline cluster |
| Lint | `golangci-lint run` | Same config as core repo |
| Format | `go fmt ./...` | Run before committing |
| Regenerate docs | `go generate ./...` | Uses [`terraform-plugin-docs`](https://github.com/hashicorp/terraform-plugin-docs) |
| Local install | `make install` | Drops binary into `~/.terraform.d/plugins/...` |

Acceptance tests follow [HashiCorp's TestAcc convention](https://developer.hashicorp.com/terraform/plugin/sdkv2/testing/acceptance-tests):
they create real resources against the configured `bootstrap_servers` and
clean them up at the end of each test. Run them against a throwaway
docker-compose cluster:

```bash
docker run -d --name streamline-test -p 9092:9092 -p 9094:9094 streamlinelabs/streamline:latest
TF_ACC=1 STREAMLINE_BOOTSTRAP_SERVERS=localhost:9092 go test -v ./... -run TestAcc
docker rm -f streamline-test
```

## Provider Configuration

```hcl
provider "streamline" {
  bootstrap_servers = "broker.example.com:9092"

  # Authentication (one of):
  sasl_mechanism = "SCRAM-SHA-512"
  sasl_username  = var.streamline_username
  sasl_password  = var.streamline_password

  # Or mTLS:
  # tls_ca_cert      = file("ca.pem")
  # tls_client_cert  = file("client.pem")
  # tls_client_key   = file("client-key.pem")

  # Operational tuning
  request_timeout_ms = 10000
  retries            = 3
}
```

All provider arguments may also be supplied via environment variables prefixed
with `STREAMLINE_` (e.g. `STREAMLINE_BOOTSTRAP_SERVERS`,
`STREAMLINE_SASL_USERNAME`). Environment variables take precedence over
HCL only when the HCL value is unset.

## Resource Reference (Summary)

### `streamline_topic`

```hcl
resource "streamline_topic" "events" {
  name              = "events"
  partitions        = 12
  replication_factor = 3

  config = {
    "retention.ms"           = "604800000"
    "min.insync.replicas"    = "2"
    "compression.type"       = "lz4"
  }
}
```

Importable: `terraform import streamline_topic.events events`.

### `streamline_acl`

```hcl
resource "streamline_acl" "app_writer" {
  resource_type = "TOPIC"
  resource_name = "events"
  pattern_type  = "LITERAL"
  principal     = "User:app-writer"
  host          = "*"
  operation     = "WRITE"
  permission    = "ALLOW"
}
```

### `streamline_schema`

```hcl
resource "streamline_schema" "user_event" {
  subject     = "user-events-value"
  schema_type = "AVRO"
  schema      = file("${path.module}/schemas/user_event.avsc")
  compatibility_level = "BACKWARD"
}
```

Full per-resource documentation lives under [`docs/`](docs/) (auto-generated
from schema descriptions; do not edit by hand — update the schema and run
`go generate`).

## Moonshot Resources

> ⚠️ **Experimental** — These resources require Streamline server 0.3.0+ with moonshot feature flags enabled.

### `streamline_semantic_topic`

Create a topic with built-in vector search capabilities.

```hcl
resource "streamline_semantic_topic" "logs" {
  name       = "logs"
  partitions = 6

  semantic = {
    embed           = true
    embedding_model = "bge-small"
    dimensions      = 384
  }

  config = {
    "retention.ms" = "604800000"
  }
}
```

### `streamline_memory`

Manage MCP-based agent memory namespaces.

```hcl
resource "streamline_memory" "chatbot" {
  namespace       = "chatbot-v1"
  retention_days  = 90
  semantic_search = true
  embedding_model = "bge-small"
}
```

### `streamline_branch`

Create topic branches for replay, A/B testing, or counterfactual analysis.

```hcl
resource "streamline_branch" "experiment" {
  source_topic = streamline_topic.events.name
  branch_name  = "experiment-v2"
  from_offset  = "latest"
}
```

### `streamline_contract`

Define data contracts with schema enforcement and attestation.

```hcl
resource "streamline_contract" "events" {
  topic       = streamline_topic.events.name
  schema_type = "json-schema"
  schema      = file("${path.module}/schemas/events.json")
  enforcement = "reject"

  attestation = {
    enabled        = true
    signing_key_id = var.signing_key_id
  }
}
```

## Importing Existing Resources

All resources support `terraform import` using the Streamline-native
identifier:

| Resource | Import ID | Example |
|----------|-----------|---------|
| `streamline_topic` | topic name | `terraform import streamline_topic.events events` |
| `streamline_acl` | `<resource_type>:<resource_name>:<principal>:<operation>` | `terraform import streamline_acl.x TOPIC:events:User:app:WRITE` |
| `streamline_schema` | `<subject>` | `terraform import streamline_schema.s user-events-value` |

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `Error: provider produced inconsistent result after apply` | Drift between desired and computed state | Inspect the resource's `Read`; ensure all computed attrs are populated post-create |
| `connection refused` on plan | `bootstrap_servers` unreachable from your machine | Verify with `nc -vz <host> 9092` |
| `unknown topic config: ...` | Streamline server version doesn't support that config key | Check server version; pin a compatible provider release |
| Acceptance tests hang | No broker on `localhost:9092` | Start one (see "Inner Loop" above) |
| `terraform import` fails with "not found" | ID format wrong | See the import ID table above |
| Docs out of date after schema change | Forgot to run `go generate` | Run it; commit `docs/` changes |

Verbose logs:

```bash
TF_LOG=DEBUG TF_LOG_PROVIDER_STREAMLINE=DEBUG terraform apply
```

## Releasing

Releases are produced by the GoReleaser config (`.goreleaser.yml`) and the
GitHub Actions workflow `.github/workflows/release.yml` on tag push
(`v*.*.*`). Artifacts are signed with a GPG key managed at the org level and
published to the [Terraform Registry](https://registry.terraform.io/) via the
auto-publish webhook.

Local dry-run:

```bash
goreleaser release --snapshot --clean
ls dist/
```

## Compatibility Matrix

| Provider | Terraform | Streamline server | Go |
|----------|-----------|-------------------|----|
| 0.1.x | ≥ 1.0 | 0.1.x – 0.2.x | 1.21+ |
| 0.2.x | ≥ 1.5 | 0.2.x – 0.3.x | 1.22+ |

Newer Streamline server versions are usually backward-compatible with older
provider releases, but new resource types or config keys require a provider
upgrade.

## Contributing

See the [org-wide CONTRIBUTING guide](https://github.com/streamlinelabs/.github/blob/main/CONTRIBUTING.md).
Provider-specific guidelines:

- Every new resource must include: schema test, acceptance test, example
  under `examples/`, and auto-generated docs page
- Prefer `types.String` / `types.Int64` (Plugin Framework) over plain Go
  types in schema definitions to preserve null/unknown semantics
- Validate user input in the schema (`Validators` field) when possible — it
  produces better error messages than runtime checks
- Never log credentials; the `client/` package strips sensitive fields before
  emitting logs

## Status

**Beta.** Schema and resource arguments may change between minor releases
until 1.0; all such changes will appear in `CHANGELOG.md` with migration
notes.
