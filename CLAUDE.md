# CLAUDE.md — Terraform Provider for Streamline

## Overview
Terraform provider for managing [Streamline](https://github.com/streamlinelabs/streamline) resources as infrastructure-as-code. Built with the Terraform Plugin Framework.

## Build & Test
```bash
go build -o terraform-provider-streamline  # Build
go test ./...                              # Run tests
go generate ./...                          # Regenerate docs
go fmt ./...                               # Format
go vet ./...                               # Vet
```

## Architecture
```
├── main.go                    # Provider entry point
├── internal/
│   ├── provider/
│   │   └── provider.go        # Provider configuration (bootstrap servers, SASL, TLS)
│   ├── resources/
│   │   ├── topic.go           # streamline_topic resource
│   │   ├── acl.go             # streamline_acl resource
│   │   ├── schema.go          # streamline_schema resource
│   │   ├── user.go            # deprecated legacy user schema
│   │   ├── consumer_group.go  # deprecated legacy read/delete schema
│   │   ├── branch.go          # deprecated legacy branch schema
│   │   ├── contract.go        # deprecated legacy contract schema
│   │   └── memory.go          # deprecated legacy memory schema
│   ├── datasources/
│   │   ├── cluster.go         # streamline_cluster data source
│   │   ├── consumer_group.go  # streamline_consumer_group data source
│   │   └── topics.go          # streamline_topics data source
│   └── client/
│       ├── client.go          # Kafka-compatible client state
│       ├── schema_registry.go # Schema Registry HTTP client
│       └── moonshot.go        # reserved Moonshot client configuration
├── docs/                      # Generated provider documentation
└── examples/                  # Example Terraform configurations
```

## Coding Conventions
- **Terraform Plugin Framework** v1.5 patterns (not legacy SDKv2)
- **Context**: All CRUD operations accept `context.Context`
- **Diagnostics**: Use `resp.Diagnostics.AddError()` for Terraform-style errors
- **Validation**: Input validators via framework validators package
- **State management**: Import support on topic, ACL, schema, and the deprecated
  consumer-group resource

## Provider Configuration
```hcl
provider "streamline" {
  bootstrap_servers = "localhost:9092"
  sasl_mechanism    = "SCRAM-SHA-512"
  sasl_username     = "admin"
  sasl_password     = var.streamline_password
}
```

## Resources
- `streamline_topic` — Partitions, replication, retention, compression
- `streamline_acl` — Resource type, principal, operation, permission
- `streamline_schema` — Subject, schema type (Avro/JSON/Protobuf), compatibility
- `streamline_user` — Deprecated state compatibility; credential CRUD unsupported
- `streamline_consumer_group` — Deprecated read/import/delete model
- `streamline_branch` — Deprecated state compatibility; provisioning unsupported
- `streamline_contract` — Deprecated state compatibility; no registry CRUD
- `streamline_memory` — Deprecated state compatibility; no partition CRUD

## Data Sources
- `streamline_cluster` — Broker and controller metadata
- `streamline_topics` — Topic names, partitions, and replication
- `streamline_consumer_group` — Existing consumer-group state and members
