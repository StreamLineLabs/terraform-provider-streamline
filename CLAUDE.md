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
│   │   ├── user.go            # streamline_user resource
│   │   └── schema.go          # streamline_schema resource
│   ├── datasources/
│   │   ├── clusters.go        # streamline_clusters data source
│   │   └── topics.go          # streamline_topics data source
│   └── client/
│       └── client.go          # Kafka-compatible client (segmentio/kafka-go)
├── docs/                      # Generated provider documentation
├── examples/                  # Example Terraform configurations
└── tools/                     # Code generation tools
```

## Coding Conventions
- **Terraform Plugin Framework** v1.5 patterns (not legacy SDKv2)
- **Context**: All CRUD operations accept `context.Context`
- **Diagnostics**: Use `resp.Diagnostics.AddError()` for Terraform-style errors
- **Validation**: Input validators via framework validators package
- **State management**: Import support on topic, user, and schema resources

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
- `streamline_user` — Username, SASL mechanism, credentials
- `streamline_schema` — Subject, schema type (Avro/JSON/Protobuf), compatibility
