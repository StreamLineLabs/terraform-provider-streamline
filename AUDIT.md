# Release Readiness Audit — Provider 0.4.0

## Scope

This audit tracks the programmatic P0/P1 release-readiness work for the
`streamlinelabs/streamline` Terraform provider. The provider source version
is now `0.4.0`.

## Resolved P0/P1 Items

| Area | Resolution |
|---|---|
| Registry identity | Provider address, examples, and generated documentation use `streamlinelabs/streamline` and the `0.4.0` release line. |
| Topic configuration | Creation accepts only settings parsed by the Streamline 0.3.0 broker, fixes replication to 1, rejects new ignored semantic/min-ISR settings, and rejects map keys that duplicate dedicated attributes before the API call. Unknown config maps/dedicated values are rechecked after plan resolution in Create. Released `Optional+Computed` state is preserved; explicitly configured changes require replacement, while removal retains the last known create-time value because readback is unavailable. Partition increases remain the only in-place update. |
| User credentials | `streamline_user` is retained only to keep legacy state decodable. Create/update/delete/import fail explicitly because Streamline exposes no credential CRUD; refresh preserves legacy state with a warning. |
| Consumer groups | A read-only `streamline_consumer_group` data source is available for new configurations. The deprecated resource remains registered for upgrade safety, supports read/import/delete, and rejects creation. |
| Cluster identity | Released `id` and `cluster_id` schema fields remain as deprecated null compatibility attributes; no synthetic identity is returned. The topics data source similarly retains deprecated null `id`/`internal` fields. |
| ACL mappings | The released schema values remain decodable. Creation and filter pattern codes match the Streamline 0.3.0 broker wire contract; new managed/imported ACLs require exact resource/principal/host/operation fields, wildcard resource names and hosts fail during planning, empty lite-broker results fail, reads require an exact match, and per-entry delete failures are surfaced. Existing broad ACLs permit only exact no-op refresh; destroy, replacement, or immutable changes fail before apply. |
| ACL import | ACL import uses a URL-escaped seven-field composite ID and verifies that the exact ACL exists before writing state. |
| Not-found handling | Clients return a typed `NotFoundError`. Resource refresh removes state only for that type; authorization, transport, empty responses, omitted requested results, and protocol failures preserve state and return diagnostics. Topic metadata disables auto-creation and retriable unknown-topic responses become not-found only after an independent successful metadata listing confirms absence. Ambiguous consumer-group coordinator errors use the same independent ListGroups confirmation rule. |
| Moonshot contracts | Branch, contract, and memory schemas remain registered and documented for state compatibility. Refresh preserves state with an explicit warning; create/update/delete/import fail. Their legacy models target fictional or non-CRUD APIs and are deprecated until a versioned migration is possible. |
| Schema Registry | Registration resolves the exact subject/version associated with the returned schema ID instead of reading `latest`, absent schema types canonicalize to AVRO, and every subject-bearing HTTP endpoint escapes the subject as one path segment. Existing reference state is preserved, but new/reference-changing operations fail because Streamline 0.3.0 cannot round-trip or identify references. Production delete fails before sending the broker's asynchronous subject-wide request. |
| Validation | Provider and resource schemas include plan-time validators for supported enums, ranges, names, URLs, regular expressions, JSON objects, and dependent memory settings. |
| Go requirements | Module, documentation, lint configuration, and development container consistently require Go 1.25 or newer; releases use the fixed Go 1.26.8 toolchain. |
| Generated docs | Documentation is generated with `terraform-plugin-docs` v0.25.0, pinned in `go:generate`; build-only docs dependencies are not included in the provider module. |
| Release configuration | GoReleaser v2 uses versioned provider binary names, verifies modules before release, publishes and checksums the Registry manifest, emits archive SBOMs through Syft, and publishes one non-colliding GOARM 7 artifact. |
| Provenance | The tag workflow creates GitHub provenance attestations for release ZIP archives and the Registry manifest. |
| Reproducible checks | GolangCI-Lint, govulncheck, GoReleaser, Syft, and go-licenses versions are pinned in CI/release workflows. |
| License policy | CI rejects forbidden or unknown runtime dependency licenses using `go-licenses`. |
| Vulnerabilities | Reachable gRPC, `x/net`, and `x/text` findings were upgraded to fixed versions; the release toolchain is pinned past the reported standard-library fixes. |
| Acceptance automation | A manual acceptance workflow requires an explicit readiness confirmation and a configured disposable fixture endpoint before tests can start. Schema tests use a test-only provider that removes Terraform state during teardown while retaining remote subjects for disposal with the fixture; production deletion behavior is unchanged. |

## Intentionally Unsupported or Limited

- Topic configuration cannot be read back or changed through the Kafka API
  currently used by the provider. Terraform retains supported create-time
  configuration in state and replaces the topic when that configuration
  changes or is removed.
- The provider does not manage user credentials. Principals referenced by ACLs
  must be provisioned by a supported external identity or broker mechanism.
  The deprecated resource exists only to preserve legacy state.
- Consumer groups cannot be created by Terraform; use the data source after an
  application has created the group. The deprecated resource can still read,
  import, or delete an existing group.
- A cluster ID is not exposed because returning a fabricated value would be
  misleading.
- Legacy branch, contract, and memory resources are not provisionable. Their
  schemas remain registered so upgrades do not make existing state unreadable.
- Existing schema-reference state remains readable, but new or changed
  references are not manageable against Streamline 0.3.0 because the registry
  omits them from version responses and schema identity.

## External Blockers

- No disposable Streamline acceptance fixture is configured in this worktree,
  so acceptance tests must remain skipped locally. The manual workflow fails
  closed until `fixture_ready` is confirmed and both
  `STREAMLINE_ACCEPTANCE_BOOTSTRAP_SERVERS` and
  `STREAMLINE_ACCEPTANCE_SCHEMA_REGISTRY_URL` are configured. The broker must
  be built with ACL support enabled.
- GPG signing, GitHub release upload, Registry publication, and GitHub
  attestations require the protected tag workflow. They are not exercised by a
  local snapshot and no publication was attempted.
- Streamline 0.3.0 exposes only an asynchronous subject-wide deletion that can
  later remove every version. The provider refuses to send that request and
  preserves state; cleanup requires a verified external process followed by
  explicit Terraform state removal.

## Verification

The implementation includes focused regression tests for unsupported modeling,
typed not-found behavior, ACL enum conversion/import/delete handling, exact
Schema Registry ID/version reconciliation, and validators.

The final local verification completed as follows:

- `gofmt`, `go fmt ./...`, and `git diff --check`: passed.
- Terraform 1.16.1 (official archive, SHA-256 verified):
  `terraform fmt -recursive` and `terraform fmt -check -recursive` passed.
- `go generate ./...`: passed repeatedly; normalized generated documentation
  was idempotent.
- Go 1.26.8: `go build ./...`, `go vet ./...`, `go test ./...`, and
  `go test -race ./...` passed.
- GolangCI-Lint 2.13.2 (official archive, SHA-256 verified): zero issues.
- `govulncheck` 1.1.4 with verbose module reporting: no symbol, package, or
  module vulnerabilities found.
- `go-licenses` 2.0.1: passed. It emitted informational warnings for assembly
  files that cannot be inspected as Go source.
- `goreleaser check` 2.17.0: passed.
- A clean-cache `go mod verify` passed. The host-global Go module cache contains
  an externally modified `golang.org/x/oauth2` directory, so it was not altered
  or relied on for the release snapshot.
- GoReleaser snapshot: passed serially for every configured platform, produced
  ZIP archives, checksums, and Syft 1.51.1 SPDX JSON SBOMs, and did not sign or
  publish. The separately verified before hook was skipped in the final
  snapshot because the host-global cache is outside this repository.
- Workflow YAML and repository JSON files parsed successfully.
- Acceptance tests were not run because no
  `STREAMLINE_BOOTSTRAP_SERVERS` fixture is configured.
- All repository-local formatter, lint, module-cache, release, and `dist`
  artifacts were removed after verification.
