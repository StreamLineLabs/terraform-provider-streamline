# Terraform Provider for Streamline

[![CI](https://github.com/streamlinelabs/terraform-provider-streamline/actions/workflows/ci.yml/badge.svg)](https://github.com/streamlinelabs/terraform-provider-streamline/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8.svg)](https://go.dev/)
[![Terraform](https://img.shields.io/badge/Terraform-1.5+-7B42BC.svg)](https://www.terraform.io/)
[![Release](https://img.shields.io/github/v/release/streamlinelabs/terraform-provider-streamline?label=release)](https://github.com/streamlinelabs/terraform-provider-streamline/releases)

This Terraform provider enables Infrastructure as Code (IaC) management of Streamline streaming platform resources.

## Features

- **Topic Management**: Create, configure, and manage topics
- **ACL Management**: Fine-grained access control
- **Schema Registry**: Manage Avro, Protobuf, and JSON schemas
- **Data Sources**: Query clusters, topics, and existing consumer groups
- **Upgrade Safety**: Preserve legacy resource schemas without simulating unsupported APIs

## Requirements

- Terraform >= 1.5
- Go >= 1.25 (for building from source; releases use the Go 1.26.8 toolchain)
- A running Streamline cluster

## Installation

### From Terraform Registry (Recommended)

```hcl
terraform {
  required_providers {
    streamline = {
      source  = "streamlinelabs/streamline"
      version = "~> 0.4.0"
    }
  }
}
```

### Building from Source

```bash
cd terraform-provider-streamline
go build -ldflags "-X main.version=0.4.0" -o terraform-provider-streamline_v0.4.0
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/streamlinelabs/streamline/0.4.0/darwin_amd64/
mv terraform-provider-streamline_v0.4.0 ~/.terraform.d/plugins/registry.terraform.io/streamlinelabs/streamline/0.4.0/darwin_amd64/
```

## Quick Start

```hcl
provider "streamline" {
  bootstrap_servers = "localhost:9092"
}

resource "streamline_topic" "events" {
  name         = "events"
  partitions   = 6
  retention_ms = 604800000
}
```

## Resources

| Resource | Description |
|----------|-------------|
| `streamline_topic` | Manages topics |
| `streamline_acl` | Manages access control lists |
| `streamline_schema` | Manages Schema Registry schemas |
| `streamline_user` | Deprecated legacy state compatibility; credential operations are unsupported |
| `streamline_consumer_group` | Deprecated legacy read/delete model; use the data source |
| `streamline_branch` | Deprecated legacy state compatibility; provisioning is unsupported |
| `streamline_contract` | Deprecated legacy state compatibility; the broker has no registry CRUD |
| `streamline_memory` | Deprecated legacy state compatibility; the broker has no partition CRUD |

## Data Sources

| Data Source | Description |
|-------------|-------------|
| `streamline_cluster` | Retrieves cluster metadata |
| `streamline_consumer_group` | Reads an existing consumer group |
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
├── provider/        # Provider config, registration, and acceptance tests
├── resources/       # Supported resources plus deprecated compatibility schemas
├── datasources/     # Cluster, topics, and consumer-group data sources
└── client/          # Kafka, Schema Registry, and reserved Moonshot configuration
```

Each resource implements the standard Plugin Framework lifecycle:
`Schema`, `Configure`, `Create`, `Read`, `Update`, `Delete`, and (where it
makes sense) `ImportState`.

## Inner Loop

| Action | Command | Notes |
|--------|---------|-------|
| Build provider | `go build -o terraform-provider-streamline` | Output binary in repo root |
| Run unit tests | `go test ./...` | Fast (< 10 s); no external services |
| Run acceptance tests | `make testacc` | Requires broker/Schema Registry endpoints plus explicit retained-subject acknowledgement for a disposable fixture |
| Lint | `golangci-lint run` | Same config as core repo |
| Format | `go fmt ./...` | Run before committing |
| Regenerate docs | `go generate ./...` | Uses pinned [`terraform-plugin-docs`](https://github.com/hashicorp/terraform-plugin-docs) v0.25.0 |
| Release checks | `make release-check` | Validates the GoReleaser configuration without publishing |

Acceptance tests follow [HashiCorp's TestAcc convention](https://developer.hashicorp.com/terraform/plugin/sdkv2/testing/acceptance-tests):
they create real resources against the configured endpoint and clean them up
at the end of each test. This repository does not bundle a known-good fixture.
Use a disposable ACL-enabled deployment with a Schema Registry endpoint:

```bash
export STREAMLINE_BOOTSTRAP_SERVERS=fixture.example.test:9092
export STREAMLINE_SCHEMA_REGISTRY_URL=http://fixture.example.test:8081
export STREAMLINE_SCHEMA_ACCEPTANCE_ALLOW_RETAINED_SUBJECTS=1
make testacc
```

The retained-subject opt-in is mandatory: Streamline 0.3.0 cannot safely
delete one Terraform-managed schema version, so schema acceptance teardown
removes only Terraform test state. Use `make testacc` only with a disposable
Schema Registry fixture whose retained subjects are discarded with the
fixture.

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
  connection_timeout = 30
  request_timeout    = 60

  # Optional Schema Registry API
  schema_registry_url = "https://schemas.example.com"

  # Reserved for a future versioned Moonshot resource model
  moonshot_url        = "https://moonshot.example.com"
  moonshot_token      = var.moonshot_token
}
```

Bootstrap, SASL, TLS certificate, Schema Registry, and Moonshot settings may be
supplied through the documented `STREAMLINE_*` environment variables.
`connection_timeout`, `request_timeout`, and `tls_skip_verify` are HCL-only.
An HCL value takes precedence over its environment variable when both are set.

## Resource Reference (Summary)

### `streamline_topic`

```hcl
resource "streamline_topic" "events" {
  name         = "events"
  partitions   = 12
  retention_ms = 604800000

  config = {
    "message.timestamp.difference.max.ms" = "86400000"
  }
}
```

Topic configuration values are creation-time settings because the current
Kafka client cannot read them back or update them. Changing one replaces the
topic; increasing `partitions` remains an in-place operation.

Importable: `terraform import streamline_topic.events events`.

### `streamline_acl`

```hcl
resource "streamline_acl" "app_writer" {
  resource_type   = "topic"
  resource_name   = "events"
  pattern_type    = "literal"
  principal       = "User:app-writer"
  host            = "10.0.0.10"
  operation       = "write"
  permission_type = "allow"
}
```

Managed ACLs require exact resource, principal, host, and operation values.
Streamline 0.3.0 treats wildcard values as broad delete filters, so the
provider rejects new creation/import and deletion of those values rather than
risking unrelated ACLs. Existing broad ACL state remains readable.

### `streamline_schema`

```hcl
resource "streamline_schema" "user_event" {
  subject       = "user-events-value"
  schema_type   = "AVRO"
  schema        = file("${path.module}/schemas/user_event.avsc")
  compatibility = "BACKWARD"
}
```

Existing schema-reference state is preserved, but new or changed references
are unsupported against Streamline 0.3.0 because the registry does not return
them or include them in schema identity. Schema deletion is blocked before the
broker's asynchronous subject-wide deletion request is sent.

Full per-resource documentation lives under [`docs/`](docs/) (auto-generated
from schema descriptions; do not edit by hand — update the schema and run
`go generate`).

## Moonshot and Legacy Resources

`streamline_branch`, `streamline_contract`, and `streamline_memory` are
deprecated compatibility schemas. Their original Terraform models do not map
to provisionable objects in the current broker API, so all lifecycle and
import operations fail explicitly instead of simulating success or deleting
state. Do not use them in new configurations.

`streamline_user` is retained for the same upgrade-safety reason. Streamline
does not expose credential CRUD through the API used by this provider. Manage
credentials externally and use `streamline_acl` for authorization.

The legacy `streamline_consumer_group` resource can read, import, and delete an
existing group, but cannot create one. Use the identically named data source in
new configurations.

### Semantic topic settings

Legacy semantic topic attributes remain in the schema so old state can be
decoded, but Streamline 0.3.0 ignores them during topic creation. The provider
therefore rejects applying new semantic settings instead of recording
simulated success.

## Importing Existing Resources

Supported imports use these Streamline-native identifiers:

| Resource | Import ID | Example |
|----------|-----------|---------|
| `streamline_topic` | topic name | `terraform import streamline_topic.events events` |
| `streamline_acl` | URL-escaped `resource_type\|pattern_type\|resource_name\|principal\|host\|operation\|permission_type` | `terraform import streamline_acl.x 'topic\|literal\|events\|User%3Aapp\|10.0.0.10\|write\|allow'` |
| `streamline_schema` | `<subject>` | `terraform import streamline_schema.s user-events-value` |
| Deprecated `streamline_consumer_group` resource | group ID | `terraform import streamline_consumer_group.legacy analytics` |

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `Error: provider produced inconsistent result after apply` | Drift between desired and computed state | Inspect the resource's `Read`; ensure all computed attrs are populated post-create |
| `connection refused` on plan | `bootstrap_servers` unreachable from your machine | Verify with `nc -vz <host> 9092` |
| `unknown topic config: ...` | Streamline server version doesn't support that config key | Check server version; pin a compatible provider release |
| Acceptance tests are skipped or blocked | No disposable fixture is configured | Set `STREAMLINE_BOOTSTRAP_SERVERS`, `STREAMLINE_SCHEMA_REGISTRY_URL`, and `STREAMLINE_SCHEMA_ACCEPTANCE_ALLOW_RETAINED_SUBJECTS=1`, then use `make testacc` |
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
goreleaser release --snapshot --clean --skip=sign
```

Snapshot SBOM generation requires Syft. Snapshots do not publish or create
GitHub attestations.

## Compatibility Matrix

| Provider | Terraform | Streamline server | Go |
|----------|-----------|-------------------|----|
| 0.1.x | ≥ 1.0 | 0.1.x – 0.2.x | 1.21+ |
| 0.2.x | ≥ 1.5 | 0.2.x – 0.3.x | 1.22+ |
| 0.3.0 | ≥ 1.5 | 0.3.x | 1.25+ |
| 0.4.0 | ≥ 1.5 | 0.4.x | 1.25+ |

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
