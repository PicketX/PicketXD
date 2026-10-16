# Durable Mark Reservations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline under existing continuous execution authorization. One fresh whole-branch review at the end.

**Goal:** Durably reserve nonreusable per-target conntrack marks inside the held namespace ownership boundary.

**Architecture:** Pure markstate handles deterministic lifecycle; hostpolicy derives physical target revisions. hostowner verifies marker structure and both CT families before persisting the complete ledger in its existing pinned journal. Raw receive mechanics are shared without weakening nft observation.

**Tech Stack:** Go 1.27.0, existing pinned shared Core / Go Netlink libraries; GPL Agent only.

**Spec:** [Durable marks](../specs/2026-10-16-mark-reservations-design.md) / [中文](../specs/2026-10-16-mark-reservations-design.zh-CN.md).

## Global Constraints

- Explicit complete nonzero 32-bit range, capacity 1..4096; monotonic marks, permanent retirement, no reuse.
- Preserve all 51 required kernel gates; require four new cases, total 55.
- v1 owner records remain readable, v2 requires ledger; marker table remains unchanged.
- Journal maximum 32 MiB; native nft dump maximum 4 MiB; CT maximum 64 MiB per family, five-second dump deadlines.
- No filtering command, mark writes/deletes, applied-policy recovery or enforcement claim.

## Review Focus

- An occupied unused value at the far end of a reservation must reject allocation even when the next mark is free.
- An unchanged target key whose physical shape changes must not inherit old graceful flow authority.
- A publish error after rename must return no assignments and block the current session until reopen.
- Second-family scan failure after a successful first scan must never publish partial occupancy as empty success.
- New boot, returned-slice mutation or duplicate desired keys must not resurrect retired marks.

## Context Map

### Files to Modify
| File | Purpose | Changes |
| --- | --- | --- |
| `internal/markstate/state.go`, `state_test.go` | New pure lifecycle | Range/target/history validation, collision/exhaustion, monotonic reconcile |
| `internal/hostpolicy/marks.go`, `marks_test.go` | Runtime mark identity | Full inventory and stable physical revision |
| `internal/hostowner/marks_linux.go`, `marks_test.go` | Held-session persistence | Both-family scan, pinned save, fault/reopen semantics |
| `internal/hostowner/owner_linux.go`, `files_linux.go` | Owner journal | v1/v2 validation, retained writer, boot history, 32 MiB bound |
| `internal/hostowner/raw_linux.go`, `conntrack_linux.go`, `conntrack_test.go` | Native observation | Common strict receive loop; bounded streaming CT occupancy |
| `internal/hostowner/marks_integration_test.go`, `owner_integration_test.go` | Native acceptance | Lifecycle/collision/process kill/drift cases and helper mode |
| `internal/validation/report/report.go` | Mandatory evidence | Add four named mark cases |
| bilingual design/plan/README/validation record | Scope and evidence | Actual outcomes, decisions, next apply boundary |

### Dependencies and References
| File | Relationship |
| --- | --- |
| `hostpolicy.Policy.Inventory` / shared `networkplan.TargetKey` | Complete desired targets, including inactive contributions |
| `hostowner.openWith`, `journal.write`, `Session.Verify` | Existing namespace locks, live readback and descriptor-relative durability |
| `hostowner/review_test.go`, `files_test.go`, `owner_test.go` | Existing strict raw completion/fault test patterns |
| `owner_integration_test.go` | Disposable helper process and forced exit patterns |

Risk assessment: private record schema upgrade intentionally prevents downgrade; no public manifest/config change, database migration or dependency update. Owner marker-only semantics remain. Context map and plan self-reviewed against constructors/imports and the spec.

## Task 1: Pure lifecycle and physical identities

**Interfaces:** `markstate.Reconcile(*State,Range,[]Target,[]uint32)(State,error)`, `State.Validate() error`, `State.Clone() State`, `Range.Validate() error`, `Target.Validate() error`, `Policy.MarkTargets()([]markstate.Target,error)`.

- [x] Write `TestMarkLifecycle`, `TestMarkCollisionAndValidation`, `TestMarkExhaustionAndHistory`, `TestMarkIsolation`, `TestPolicyMarkIdentity`. Assert sorted contiguous values, no reuse after removal/recreation/shape change, high-u32 exhaustion, far-end collision, corrupt historical rejection, stable permission/Binding updates and changed UID/tuple revisions.
- [x] Add compiling stubs and run `go test ./internal/markstate ./internal/hostpolicy -count=1`; expect assertion failures for missing behavior.
- [x] Implement immutable reconcile and physical shape hash exactly as spec, without kernel imports.
- [x] Run full unit/race/vet/build; expect exit 0. Commit lifecycle boundary.

## Task 2: Namespace-bound journal and strict occupancy

**Interfaces:** `Session.ReconcileMarks(context.Context,markstate.Range,[]markstate.Target)(MarkStatus,error)`; `MarkStatus{State markstate.State,ObservedAt time.Time}`. Private kernel mark reader exposes `OccupiedMarks(context.Context,markstate.Range)([]uint32,error)`; the real reader completes AF_INET then AF_INET6 before returning.

- [x] Write `TestDurableMarks`, `TestMarkPublicationFailures`, `TestMarkScopeAndScanFailures`, `TestMarkHistoryAcrossBoot`, `TestMarkJournalValidation`, `TestRawConntrackMarks`. Cover all five Review Focus classes, descriptor-pinned publication, v1 upgrade, v2 restart, no-op scans, malformed/duplicate/missing/zero/outside mark fields, interruption and second-family failure. Run against compiling stubs and observe expected assertions fail.
- [x] Implement retained journal writer, v2 ledger validation/boot carry, 32 MiB bound, fresh ownership before/after scans and sticky publication faults; retain existing owner behavior.
- [x] Generalize only the strict multipart receive loop; keep NFT limits/type/family behavior, stream CT mark scans with new bounds. No mark writes/deletes.
- [x] Run full uncached tests/race/vet/build/static prepare smoke and integration compilation; expect exit 0. Commit session boundary.

## Task 3: Native acceptance and delivery

**Interfaces:** Consumes Tasks 1/2, uses actual NETLINK_NETFILTER CT create solely in disposable tests. Adds four required case names from the spec.

- [x] Add mandatory lifecycle, dual-family collision, standalone killed-helper recovery and drift cases. Before trusting collisions, verify inserted marks with independent test handle; preserve unrelated flows. Existing old packet cases remain mandatory.
- [x] Compile all integration packages, run independent whole-branch review once, fix Critical/Important with RED/GREEN regressions and a green suite; ledger any declined judgments/minors.
- [x] Publish PR; inspect all 55 required native results and zero fail/skip, download exact-head evidence and verify SHA-256, update bilingual records. Final-head checks and post-merge main CI must pass before completion.
