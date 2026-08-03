# Clean Code and SRP Audit

## Summary

- **Highest-leverage split:** divide `internal/client/client.go` by Kafka admin
  actor while retaining the internal `StreamlineClient` facade and signatures.
- Provider configuration resolution, endpoint validation, and construction of
  three transports currently share one 159-line `Configure` method.
- The former incompatible provider/resource/data-source client containers were
  a P0 correctness defect and are fixed on the baseline branch by one
  `client.Clients` type.
- Schema Registry response cleanup is now shared, but request construction and
  response decoding still repeat across operations.
- Long Terraform `Schema` methods remain in place because each defines one
  public resource contract; splitting them would add navigation without
  removing an actor.

## Findings

| ID | Location | Category | Severity | Actors in conflict | Cost | Size | Behavior risk |
|---|---|---|---|---|---|---|---|
| TF-SRP-1 | `internal/client/client.go` | SRP, mixed module | P1 | connection/security; topic admin; ACL/security; cluster ops; user admin; consumer groups; shutdown ops | Unrelated Kafka actors edit one 800-line file and obscure field/method partitions. | L | Low |
| TF-SRP-2 | `internal/provider/provider.go:170-320` | SRP, mixed orchestration | P2 | Terraform configuration UX; environment/ops; endpoint validation; client construction | Precedence, validation, timeout policy, and transport factories change in one framework callback. | M | Medium |
| TF-CC-1 | `internal/client/schema_registry.go` | Duplication | P2 | Schema Registry contract; HTTP transport | Each operation repeats request creation, headers/auth, execution, and decoding despite shared error cleanup. | M | Medium |
| TF-CC-2 | `internal/client/client.go` shutdown fields and methods | Field/method partition | P2 | Kafka admin; shutdown ops | `mu`, `closing`, `inflight`, and `httpClient` are touched only by graceful shutdown; no operation registers with `inflight`. | S | Medium |

## Actor and Field Partition

### `StreamlineClient`

| Partition | Fields/methods | Actor/axis |
|---|---|---|
| Connection/security | brokers, dialer, Kafka transport, TLS, SASL, timeout, retry | platform/security |
| Topics | topic config/metadata and CRUD | topic administrators |
| ACLs | ACL config, CRUD, enum conversions | security/compliance |
| Cluster metadata | broker/controller discovery | operators |
| Users | user-as-principal lifecycle | identity administrators |
| Consumer groups | list/describe/delete | consumer operators |
| Shutdown | mutex, closing flag, wait group, idle HTTP client | process lifecycle |

Resulting files: `client.go` for state/construction, `connection.go`,
`topics.go`, `acls.go`, `cluster.go`, `users.go`, and
`consumer_groups.go`. No interfaces or forwarding types are introduced.

### Provider configuration

Resulting units:

- `resolveProviderConfig` owns HCL/environment precedence and timeout defaults.
- `validateBootstrapServers` owns address validation and broker parsing.
- `newProviderClients` owns Kafka, Schema Registry, and Moonshot construction.
- `Configure` remains the Terraform diagnostic/orchestration envelope.

## Ordered Refactor Sequence

1. Characterize constructor defaults, ACL conversions, and partition-count
   validation.
2. Move client domain types and methods unchanged into coherent files.
3. Validate and commit the pure move.
4. Extract provider configuration resolution and client construction.
5. Centralize Schema Registry request execution only if tests preserve exact
   paths, headers, status errors, and decoding.
6. Run format, tests, vet, GolangCI-Lint, build, and generated-doc checks after
   every commit.

## Deferred

- Graceful shutdown state is retained until real in-flight registration can be
  introduced and characterized; deleting it would change an internal lifecycle
  contract without adequate coverage.
- Acceptance tests still require an external Streamline deployment.

## Out of Scope

- Terraform resource/data-source `Schema` methods: one public contract actor.
- Per-resource CRUD files: framework lifecycle methods share one resource model
  and one backend actor.
- Language-specific HTTP/Kafka clients in other repositories: similar shapes,
  independent release actors.
