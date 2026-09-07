# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).



## [Unreleased]

### Fixed

- Topic metadata now uses explicit Kafka Metadata requests with automatic topic
  creation disabled. Topic create/delete require exactly one result for the
  requested topic and surface empty, omitted, authorization, and broker-error
  responses instead of reporting success.
- Consumer-group describe/delete require the requested result. Empty responses
  are protocol errors, ListGroups top-level errors are surfaced, and ambiguous
  `CoordinatorNotAvailable` becomes not-found only after a successful
  independent ListGroups response confirms absence.
- Topic config conflict validation treats unknown dedicated values as
  configured, defers unknown maps safely, and reruns against resolved
  Create-time values before any remote request.
- Legacy wildcard ACLs now permit only a true no-op refresh. Destroy,
  replacement, and immutable-field changes fail during planning.
- Schema Registry responses that omit `schemaType` are canonicalized to
  `AVRO`, and schema acceptance import uses the subject rather than the
  generated `subject:version` state ID.
- `make testacc` and acceptance documentation require
  `STREAMLINE_SCHEMA_ACCEPTANCE_ALLOW_RETAINED_SUBJECTS=1`, preventing schema
  tests from silently skipping when state-only teardown was not acknowledged.
- Topic reads and deletes no longer treat a retriable
  `UnknownTopicOrPartition` response as confirmed absence. They retry unless a
  separate all-topic metadata request succeeds and confirms the topic is
  missing.
- Schema Registry subjects are escaped as individual URL path segments for
  registration, lookup, compatibility, and deletion endpoints.
- Topic plans reject configuration-map keys that duplicate dedicated
  attributes instead of silently overwriting one input while persisting both.
- New ACL plans reject wildcard `resource_name` and `host` values before apply,
  and `host` no longer defaults to the unsupported wildcard. Existing wildcard
  state remains refreshable.
- Schema acceptance tests use an explicit disposable-fixture provider that
  removes only test state during teardown and warns that the remote subject is
  retained; production schema deletion remains unsupported.
- Provider resources and data sources now receive the same configured client
  container, fixing runtime `Configure` type assertion failures.
- `connection_timeout` now controls Kafka connection establishment separately
  from `request_timeout`. The default effective dial timeout is now the
  documented 30 seconds; set `connection_timeout` explicitly for slower
  networks.

### Changed

- Internal Kafka administration and provider configuration responsibilities are
  split into focused modules without changing Terraform resource schemas.
- Deprecated `streamline_user` credential management and retained its schema
  only so existing state remains decodable. All credential operations now fail
  explicitly because ACL entries are not credentials.
- Added a read-only `streamline_consumer_group` data source. The legacy
  resource remains registered for state compatibility and supports only
  read/import/delete; creation fails explicitly because applications create
  groups when consumers connect.
- Removed the synthetic `cluster_id` and data-source `id` values from
  `streamline_cluster`; their released schema fields now remain as deprecated
  null compatibility values because the Kafka API does not expose a real
  cluster identifier. The topics data source uses the same approach for its
  released synthetic fields.
- Topic creation now accepts only settings Streamline 0.3.0 actually parses,
  fixes replication to 1, and rejects ignored semantic/arbitrary settings.
  Released computed state is preserved without destructive migration;
  configured changes require replacement while removal retains last known
  create-time state. Partition increases remain in-place.
- ACL enum conversion now matches the Streamline 0.3.0 broker wire contract,
  blocks unsafe wildcard-managed ACL creation/deletion/import, treats empty
  lite-broker creation results as errors, verifies exact describe/delete
  matches, and surfaces per-entry deletion errors. The released broad-value
  schema remains readable without triggering broad deletion.
- Resource refresh now removes Terraform state only for typed, confirmed
  not-found responses. Authorization, transport, and other errors preserve
  state and return diagnostics.
- Deprecated the legacy Moonshot branch, contract, and memory resource models.
  They remain registered for state compatibility but all operations fail
  explicitly instead of calling fictional or non-CRUD endpoints.
- Schema registration resolves the exact version for the returned schema ID,
  existing reference state remains readable while new/changed references fail
  explicitly, and deletion is blocked before Streamline's asynchronous
  subject-wide request can be sent.
- Added ACL import with a lossless URL-escaped seven-field composite ID and
  remote existence verification.
- Added plan-time validation for provider URLs/timeouts, topic limits and
  create-only configuration keys, ACL principals, schema references, Moonshot
  contracts/memory, topic filters, and consumer-group identifiers.
- Reconciled build documentation and the development container with the
  module's Go 1.25 minimum and Go 1.26.8 release toolchain.
- Removed the topic data source's synthetic ID and always-false `internal`
  attribute, and stopped inventing port 9092 when broker metadata omits a port.
- Upgraded reachable vulnerable gRPC, `x/net`, and `x/text` dependencies and
  pinned the release toolchain to Go 1.26.8 for standard-library fixes.
- Removed the colliding GOARM 6 release target; the `linux_arm` archive now
  consistently contains the standard GOARM 7 build.
- Terraform Registry release archives now contain versioned provider binaries,
  publish the protocol manifest, include it in checksums, emit archive SBOMs,
  and create provenance for the archives and manifest.


## [0.3.0] - 2026-04-20

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
