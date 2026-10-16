# Host ownership implementation plan

> Implement inline with test-driven development and one fresh final review. User continuous execution authorization applies; no repeated handoffs.

**Goal:** Own a nonfiltering namespace-bound kernel marker through a durable pending/confirmed journal before production admission code exists.

**Spec:** [Host ownership](../specs/2026-10-16-host-ownership-design.md) / [中文](../specs/2026-10-16-host-ownership-design.zh-CN.md).

**Constraints:** Go 1.27.0, existing pinned Netlink dependencies, GPL Agent only, fixed `inet picketx`, no kernel deletion, no conntrack marks, no enforcement claim. Existing 43 kernel gates stay required.

**Review focus:** A second state path cannot evade exclusion; confirmed disk state never bypasses live inspection; pending durability precedes every create; unexpected table objects reject adoption; cancellation/close releases both locks without deleting the marker.

## Context map and preflight

| File | Responsibility |
| --- | --- |
| `internal/hostowner/owner_linux.go`, `owner_test.go` | Config/session state machine; `Open`, `Verify`, `Close`, owned status; private injected IO/kernel boundaries for fault tests |
| `internal/hostowner/files_linux.go`, `files_test.go` | Namespace pin, deterministic runtime lock plus state lock, bounded canonical journal and atomic publication |
| `internal/hostowner/nft_linux.go`, `nft_test.go` | Namespace-bound complete marker-table observation and exclusive creation; no delete API |
| `internal/hostowner/owner_integration_test.go` | Real namespaces, foreign objects, restart, process lock/crash and exclusive race evidence |
| `scripts/test-kernel.sh`, report package, `cmd/m0-report/main.go` | Run ownership binary, preserve logs and require new named cases |
| bilingual README/validation records | Scope, actual evidence and next enforcement boundary |

Task 1 produces the private backend interface consumed by Task 2: `Observe(context.Context) (observation,error)`, `Create(context.Context, marker) error`, `Close() error`. Observation distinguishes absent, complete supported marker, and unsupported structure. Tests use injected effects only for failure boundaries; real files/locks remain exercised. `openWith` is package-private; exported `Open` always uses the fixed runtime lock root and real namespace-bound Netlink.

## Task 1: Durable session state machine

1. Write tests `TestFreshClaimAndReopen`, `TestForeignAndDriftRefused`, `TestCrashBoundaries`, `TestNewBootAndNamespaceIdentity`, `TestAlternateStatePathExclusion`, `TestJournalPrivateCanonicalBounded`, `TestCanceledAndClosed`. Add only compiling type/signature stubs; run targeted tests and observe assertion failures before implementing behavior.
2. Implement explicit `Config`, `Session`, `Status`; stable installation identity, fresh epoch and complete scope/namespace binding. Implement real filesystem journal and both locks. Test ambiguous post-rename failure separately from pre-publication failure.
3. Run `go test ./internal/hostowner`; expect all named unit tests pass. Then full `go test ./...`, race, vet and build. Commit this coherent boundary.

## Task 2: Real kernel backend and required evidence

1. Test marker shape rejection before implementing real observation/creation. Inspect complete chains/rules/sets/objects/flowtables; fail on any dump error or drift. Use the pinned library's exclusive table creation, one unhooked chain and a counter-only marker rule.
2. Implement real `Open` namespace pin/lasting-connection and abstract socket lock setup. Add required kernel cases `TestOwnerKernelFreshAndRestart`, `TestOwnerKernelForeignTable`, `TestOwnerKernelUnexpectedStructure`, `TestOwnerKernelExclusiveCreate`, `TestOwnerKernelProcessLock`, `TestOwnerKernelPendingRecovery`, `TestOwnerKernelSocketExclusion`. Each uses disposable namespaces; an opted-in setup failure fails rather than skips. Process locking also covers a second mount namespace with a fresh `/run`.
3. Extend runner and evidence parser without weakening old requirements. Run unit/race/vet/build, compile integration suites locally, run actual Linux CI and inspect all required results. Tests added before backend implementation may initially compile with stubs; only real-kernel CI establishes compatibility.
4. Final independent whole-branch review, reproduce/fix blocking findings with failing tests, publish exact tested tree and bilingual validation. No production filtering command is advertised by this increment.
