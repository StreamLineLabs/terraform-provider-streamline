# Contributing to Terraform Provider for Streamline

Thank you for your interest in contributing! Please review the [organization-wide contributing guidelines](https://github.com/streamlinelabs/.github/blob/main/CONTRIBUTING.md) first.

## Development Setup

### Prerequisites

- Go 1.21+
- Terraform CLI 1.5+ (for acceptance tests)
- A running Streamline instance (for acceptance tests)

### Build & Test

```bash
# Build
go build -o terraform-provider-streamline

# Run unit tests
go test ./...

# Run acceptance tests (requires a running Streamline instance)
export STREAMLINE_BOOTSTRAP_SERVERS=localhost:9092
TF_ACC=1 go test ./internal/provider/ -v -timeout 30m

# Run acceptance tests with schema registry
export STREAMLINE_SCHEMA_REGISTRY_URL=http://localhost:8081
TF_ACC=1 go test ./internal/provider/ -v -run TestAccSchema -timeout 30m

# Generate documentation
go generate ./...
```

### Adding a New Resource

1. Create the resource in `internal/resources/`
2. Register it in `internal/provider/provider.go` → `Resources()`
3. Add acceptance tests in `internal/provider/`
4. Create documentation in `docs/resources/`
5. Add an example in `examples/`

### Adding a New Data Source

1. Create the data source in `internal/datasources/`
2. Register it in `internal/provider/provider.go` → `DataSources()`
3. Add acceptance tests in `internal/provider/`
4. Create documentation in `docs/data-sources/`

## Architecture

- `internal/provider/` — Provider configuration and registration
- `internal/resources/` — Resource implementations (topic, ACL, schema)
- `internal/datasources/` — Data source implementations (cluster, topics)
- `internal/client/` — Streamline/Kafka client wrapper

## License

By contributing, you agree that your contributions will be licensed under the Apache-2.0 License.
<!-- feat: b8b64dc0 -->
