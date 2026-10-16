# Host Policy Compilation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline under the existing continuous execution authorization. Use checkbox steps and one fresh final reviewer.

**Goal:** Compile shared desired intent into an immutable production host-input policy without private fixture imports.

**Architecture:** A trusted immutable Scope bounds a shared Plan. A Policy validates every selected inventory target before time-based evaluation; snapshots preserve per-target provenance. Validation adapters consume snapshots without changing the fixture backend.

**Tech Stack:** Go 1.27.0, existing pinned shared Core/Netlink dependencies; no new modules.

**Spec:** [Host policy](../specs/2026-10-16-host-policy-design.md) / [中文](../specs/2026-10-16-host-policy-design.zh-CN.md).

## Global Constraints

- Production package never imports `internal/validation`.
- Host-local/original-destination/graceful only; arbitrary canonical dual-stack TCP/UDP destinations and ports.
- Preserve UID, binding provenance, absolute Validity and protection independently from active Grants.
- No mark allocation, kernel effects, apply receipt or enforcement claim; keep existing 50 gates.

## Review Focus

- A dormant target outside trusted scope must fail before it becomes active.
- Narrowed policy destinations must not allow a wider prefix than trusted facts.
- Mutation of returned nested slices must not affect concurrent evaluations.
- Future deny inventory must survive even though its active projection is empty.
- Last Binding removal must be distinguishable from last Grant expiry and unknown Agent.

## Context Map

### Files to Modify
| File | Purpose | Changes Needed |
| --- | --- | --- |
| `internal/hostpolicy/policy.go` | New immutable runtime compiler | Scope validation, selected-target capability checks, evaluation and inventory |
| `internal/hostpolicy/policy_test.go` | Compiler semantics | RED/GREEN tests including all review focus inputs |
| `internal/hostpolicy/configuration_test.go` | Actual static publication boundary | UID preservation and replacement/empty desired cases |
| `internal/validation/hostbridge/host_integration_test.go` | Existing actual INPUT cases | Consume production compiler via test-only conversion, add port case |
| `internal/validation/report/report.go` | Evidence gate | Require additional named host case |
| bilingual README/validation record | Honest scope and actual evidence | Record local and native CI outcomes |

### Dependencies
| File | Relationship |
| --- | --- |
| Shared `pkg/networkplan/plan.go`, `types.go`, `context.go` | Immutable compile/inventory/projection and trusted context normalization |
| Shared `pkg/configuration/candidate.go` | Existing accepted Candidate.Plan() provides input |
| `internal/agentstate/store.go` | Existing desired-state owner; remains unchanged |
| `internal/hostowner/*` | Independent marker ownership; remains unchanged |

### Test Files and Reference Patterns
| File | Coverage / pattern |
| --- | --- |
| `internal/validation/planbridge/compiler_test.go` | Independent lifetime/provenance checks; no production import |
| `internal/agentstate/store_test.go` | Real configuration facts and accepted replacement |
| `internal/validation/hostbridge/*` | Real packet observations and disposable namespaces |

Risk assessment: no public API break, migration, dependency or configuration change. New internal contract is not yet a kernel installation contract. Context map self-reviewed against imports and callers; continuous implementation authorization applies.

## Task 1: Immutable host compiler

**Files:** Create `internal/hostpolicy/policy.go`, `policy_test.go`, `configuration_test.go`.

**Interfaces:** Produces `NewScope(string,networkplan.ExecutionContext)(*Scope,error)`, `Compile(*networkplan.Plan,*Scope)(*Policy,error)`, `(*Policy).Evaluate(time.Time)(Snapshot,error)`, `(*Policy).Inventory()([]networkplan.TargetInventory,error)`. Snapshot contains Version, AuthorityID, Context, At, NextChange, Targets; each target contains Key, BindingIDs, Protected, Allow, Deny; each member retains ID, Source, Endpoint and Validity.

- [x] Write `TestDualStackArbitraryTargets`, `TestLifetimeAndInventory`, `TestScopeRejections`, `TestSelectedTargetRejections`, `TestIndependentReturnedData`, `TestConfigurationIdentityAndWithdrawal`. Assert exact target UID, protocols/ports/deadlines, future inventory, zero-time errors, nested mutation isolation and replacement outcomes.
- [x] Add only compiling signatures/stubs. Run `go test ./internal/hostpolicy -count=1`; expect assertion failures from missing behavior, not compile errors.
- [x] Implement the spec using shared normalized context, Inventory and Projection; validate full selected inventory at Compile, evaluate active members at explicit times, return independently owned data.
- [x] Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`; expect exit 0. Commit compiler.

## Task 2: Kernel adapter and evidence

**Files:** Modify host integration test/report; bilingual README and `docs/validation/M2C2.{md,zh-CN.md}`.

**Interfaces:** Consumes Task 1 Policy/Snapshot. Test adapter converts typed endpoints/members to fixture.Snapshot; production code remains fixture-free.

- [x] Add required `TestHostPolicyCompilerPorts` testing baseline reachability, dual-stack allowed sources, unrelated-source denial and unprotected controls through TCP/8001 and UDP/8002. Compile all integration packages; opted-in setup failures must fail.
- [x] Replace host fixture's hand-written shared projection conversion with hostpolicy compilation/evaluation using its trusted Facts. Preserve all existing gate names.
- [x] Run full local checks/static prepare smoke and integration compilation. Review whole branch once, fix Important/Critical findings with RED/GREEN regressions.
- [x] Publish branch/PR, require all 51 named kernel cases plus unit/race/vet/build/static smoke. Download and verify exact-head artifact and record actual environment/results. Merge only after review findings and CI are resolved, as explicitly authorized by the user.
