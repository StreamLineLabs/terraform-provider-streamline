# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).



## [Unreleased]

### Added
- **Moonshot resources** — first Terraform support for the Moonshot HTTP control plane (port 9094):
  - `streamline_branch` (M5: time-travel branches) — CRUD + import.
  - `streamline_contract` (M4: enforced contracts) — CRUD + import; schema body provided as JSON; supports `compatibility` policies.
- New provider attributes `moonshot_url` and `moonshot_token` (env vars `STREAMLINE_MOONSHOT_URL` / `STREAMLINE_MOONSHOT_TOKEN`).
- `internal/client/moonshot.go` — minimal HTTP client (`MoonshotClient`) with typed `MoonshotError`, branch + contract methods, bearer-token auth, configurable timeout.
- Tests: 5 client tests + 4 resource tests covering create/read/delete/import paths and JSON validation.

### Fixed
- Provider build was broken against `segmentio/kafka-go v0.4.47`:
  - `DescribeGroupsRequest` / `DeleteGroupsRequest` use field `GroupIDs`, not
    `Groups` (`internal/client/client.go`).
  - `ListGroupsResponseGroup` does not expose `ProtocolType`; the `state`
    attribute on `streamline_consumer_group` is now populated only from
    `DescribeGroups` (which exposes `GroupState`), not from `ListGroups`.
  - `provider.Configure` referenced an undefined `data` identifier when
    setting `TLSSkipVerify`; corrected to `config` (`internal/provider/provider.go`).
- `go build ./...`, `go vet ./...`, and `go test ./...` now all pass.

### Added
- `streamline_consumer_group` resource — manage consumer groups via Terraform (CRUD + import)
- Client: `ListConsumerGroups()`, `DescribeConsumerGroup()`, `DeleteConsumerGroup()` methods
- Consumer group resource supports `group_id`, computed `state` and `members` attributes

- refactor: extract common CRUD helpers (2026-03-06)
- fix: resolve state drift detection for ACL resources (2026-03-06)
- feat: add topic retention policy resource (2026-03-06)
- **Testing**: add plan-only tests for resource changes
- **Fixed**: correct import state for existing topics
- **Documentation**: regenerate provider documentation from schema
- **Added**: implement data source for cluster info
- **Fixed**: handle API timeout in resource read operations
- **Changed**: update terraform-plugin-framework dependency
- **Changed**: extract common CRUD patterns into helpers
- **Testing**: add acceptance tests for provider configuration
- **Fixed**: resolve state drift detection for ACL resources
- **Added**: add streamline_topic resource implementation

### Changed
- Extract connection pool into dedicated package


## [0.2.0] - 2026-02-18

### Added
- `streamline_topic` resource for topic management
- `streamline_schema` resource for schema registry management
- `streamline_acl` resource for access control management
- `streamline_cluster` data source for cluster information
- `streamline_topics` data source for listing topics
- Acceptance tests for all resources and data sources
- Terraform Plugin Framework v1.5 based provider
- fix: handle null values in resource plan comparison
- docs: add examples for topic resource configuration
- test: add resource import round-trip verification
- docs: update provider resource attribute documentation
