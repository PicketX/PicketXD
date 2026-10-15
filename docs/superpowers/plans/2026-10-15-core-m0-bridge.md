# Core to M0 bridge implementation plan

> Execute inline with superpowers:executing-plans. The user authorized continued implementation through working L4, L7 and administration; routine implementation batches do not need repeated approval.

**Goal:** connect the shared Core/planner to the isolated Linux validation backends, with truthful confirmation and independent deadlines.

**Architecture:** an owned, pure conversion package feeds a serialized fixture coordinator. Backend receipts describe acknowledged emission, readback confirms it, and cumulative ownership bounds strict cleanup. This is validation infrastructure; production providers and HTTP authorization follow separately.

**Tech stack:** Go 1.27.0 minimum, existing pinned Netlink libraries, immutable PicketX `e39f96e2787154a8af8e7d996b2f65aa10b6c3eb` (`v0.0.0-20261015081000-e39f96e27871`).

**Spec:** [approved continuation design](https://github.com/PicketX/PicketX/blob/9d53b13/docs/superpowers/specs/2026-10-15-core-m0-bridge-design.md).

**Integration ruling:** Reuse the existing M1.3A PR #2 compiler and all its tests, and target the new PR at that branch. Retaining competing compiler APIs would create merge conflicts and weaken replacement validation; the cost is one additional compiler/coordinator interface regression check.

**Execution status:** Tasks 1–5 implemented, reviewed and verified. PR #3 is published; real CI run 38749800479 passes all 17 M0 + 19 bridge required cases. See [current evidence](../../validation/M13.md).

## Global constraints

- GPL Agent code stays in PicketXD; import only Apache shared Core/planner packages.
- Disposable transit namespaces only; exact fixture Agent/context/epoch and tuple catalog.
- IPv4/IPv6, deny precedence, original absolute deadlines and independent contributions.
- Apply/confirm before destructive cleanup. Missing receipt or ambiguous confirmation faults the fixture; no invented rollback.
- Keep existing 17 M0 scenarios; never claim kernel acceptance from skipped tests.
- English and Chinese status stay synchronized.

## Context map and review

| Files | Responsibility / dependencies |
| --- | --- |
| `internal/validation/planbridge/{scope,compiler}.go`, `scope_linux.go` | Shared networkplan input, owned conversion, typed provenance |
| `internal/validation/nftbackend/{backend,receipt}_linux.go` | Dispatch-time expiry filtering, acknowledged receipt, full owned readback, atomic fixture NAT, exact removal |
| `internal/validation/ctbackend/backend_linux.go` | Explicit empty-after cleanup retaining exact tuple/mark selection |
| `internal/validation/planbridge/{coordinator,driver}_linux.go` | Accepted plan, confirmed history, cleanup/retry, cancellable boundaries |
| `internal/validation/kerneltest/{topology,nat}_linux.go` | Trusted topology facts, literal 443/22 and existing UDP/NAT listeners |
| `internal/validation/planbridge/*_test.go` | Unit ordering/failure tests and real dual-stack packet conformance |
| `scripts/test-kernel.sh`, report package, workflow | Run and require new evidence alongside M0 |

Reviewed against merged shared types and existing fixture/backends. No shared API, migration or public configuration change. Backend existing M0 APIs remain usable.

## Review focus

1. Delayed lock acquisition must not renew native TTL or turn bounded membership permanent.
2. A historical expired allow must survive intervening applies and cleanup failure.
3. Unexpected allow sets/rules must block revocation even after batch ACK.
4. Unknown Agent errors are not empty desired state; removed Agent cleanup needs validated replacement.
5. A stale timer must never apply a captured old plan after replacement or cancellation.

## Tasks

### Task 1: pure bounded conversion

Interfaces retained from M1.3A: `NewScope(Identity, networkplan.ExecutionContext, Catalog) (*Scope,error)`, `Prepare(*Scope,*Prepared,networkplan.Input) (*Prepared,error)` and `(*Prepared).Materialize(time.Time) (Materialized,error)`. The coordinator consumes this compiler; no second pure compiler is retained.

- [x] Write/run failing conversion tests: reject wrong context, authority, unsupported tuples and scope; preserve typed IDs, Binding IDs, inactive inventory and deadlines; zero expiry only for permanent.
- [x] Implement scope normalization, projection conversion and owned sidecars; validate complete shared candidates through Compile/Replace in the coordinator.
- [x] Run `go test ./internal/validation/planbridge`; commit.

### Task 2: backend receipts, confirmation and empty state

Interfaces: `ApplyReceipt(ctx,Snapshot,Options) (Receipt,error)`, `Confirm(ctx,Receipt) error`, `RemoveOwned(ctx) error`, `RevokeEmpty(ctx,before,mark,now) (Result,error)`.

- [x] Write/run failing deadline-filter/receipt, readback mismatch and explicit-empty tests.
- [x] Calculate remaining TTL at serialized dispatch; receipt records actual/omitted members; collect expected owned structure for readback. Append known NAT translation before the same Flush. Discard queued messages on preparation failure.
- [x] Confirm table/chains/rules/sets/membership, tolerate native expiry and counters only. Remove exact owned table and confirm absence. Add separate validated empty-after revocation.
- [x] Run `go test ./...`; commit.

### Task 3: confirmed lifecycle and strict history

Interfaces: injected `Effects` apply/confirm/remove/revoke functions, `NewCoordinator`, `Update`, `Refresh`, `RetryCleanup`, owned `State`.

- [x] Write/run failing tests for apply/confirm ordering, cumulative historical ownership, failed cleanup/retry, empty Binding/Agent withdrawal, rejected candidate and faulted ambiguity.
- [x] Serialize validation and effects, advance confirmed plan before cleanup, retain actual emission and typed provenance, distinguish known transaction rejection from unknown application.
- [x] Run `go test ./...` and race tests; commit.

### Task 4: boundary driver

Interface: `Run(context.Context) error` reads the latest accepted plan under coordinator serialization; Update signals a buffered wake channel.

- [x] Write/run failing future allow/deny, generation replacement, cancellation and overdue-boundary tests.
- [x] Implement cancellable test-only timer driver; no captured-plan replay, immediate reevaluation when a step crosses a boundary.
- [x] Run `go test -race ./...`; commit.

### Task 5: real packets and durable evidence

- [x] Extend topology with literal TCP/443 and TCP/22 and prove fixture identity/transit paths before policy.
- [x] Add required `TestBridge*` kernel cases: independent Gitea deadlines, deny/revocation/UDP, native expiry, future boundaries, replay/late apply, historical strict preservation, failed apply/confirmation, interrupted cleanup, Binding/empty-Agent withdrawal and atomic original-tuple NAT. Log actual time, context, membership and cleanup counts.
- [x] Compile bridge integration binary in the runner; require every named bridge case and all existing M0 cases. Clear stale artifacts; preserve failed-run evidence.
- [x] Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...` and privileged CI. Inspect dependency graph for unwanted Controller imports.
- [x] Final fresh branch review, resolve important findings, synchronize bilingual validation status, publish reviewable PR with observed evidence.
