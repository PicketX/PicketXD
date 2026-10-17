# Static Host Strict Enforcement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Inline execution under standing authorization, one fresh whole-branch review.

**Goal:** Run strict host policy with kernel-enforced packet deadlines and observed recorded cleanup.
**Architecture:** Version2 strict images use a shared known-mark established chain behind current membership checks; the static driver composes confirmed apply/verify and complete-history cleanup at every wake.
**Tech Stack:** Go1.27, existing pinned pure-Go Netlink/Core modules.
**Spec:** [Design](../specs/2026-10-17-strict-enforcement-design.md)

## Global Constraints

Graceful v1 JSON/hash/default/runtimeKey unchanged. Strict imagev2, journalv4; KnownMarks4096, MaxRules8192, image8MiB/history8MiB/journal64MiB. No new modules. Preserve all80 native gates and three original plus M2C7 capacity-test deferred minors. No automatic mode migration or persisted strict cleanup receipt. Equal IPv4/IPv6 TCP/UDP, only owned disposable namespace effects, exact scoped delete, no flush.

## Review Focus

- Physical replacement retires still-authorized marks; strict current membership must preserve those flows: Task1/unit and Task3/native.
- Forged alias can bypass ownership if unchecked; reject unknown/out-of-context ledger aliases on startup and cleanup: Task1 tests.
- Boundary/latest generation may change during IO; strict status must not claim cleanup for a new membership: Task2 tests.
- Owner process dies before deadline; old TCP must stop and must not resume through unmarked reconstructed state: Task3 native.
- Existing graceful serialized image/state identity must remain identical: Task1/2 tests.

### Task1: strict scope, bounded image and held ledger validation

Files: hostpolicy/policy.go/tests; hostrules/image_linux.go/rules_linux.go/strict_linux.go/strict_linux_test.go; hostowner/application_linux.go; hostrevoke/plan.go.
Interfaces: Image.KnownMarks []uint32 (omitempty); Image.ValidateMarks(markstate.State) error. Build derives aliases only for strict; materialize v1/v2 from Handling, Validate semantic input plus alias list, strict layout one shared chain and current permission gating. All consumers call alias ledger validation.

- [x] Write TestStrictImageAndLedger and TestGracefulImageEncodingStable, strict scope/layout/corruption/known retired/unknown/out-of-context alias assertions. Run focused tests; expect missing API/strict scope RED.
- [x] Implement v2/validation/layout without changing graceful serialization/hash. Count shared alias and doubled TCP rules in existing bounds.
- [x] Focused and full uncached unit suite GREEN, commit.

### Task2: command mode, reconcile ordering and truthful status

Files: hostagent/config_linux.go/run_linux.go/strict_linux.go and tests; hostowner/application_linux.go (Handling metadata); agentcli/enforce_linux.go/tests.
Interfaces: Config.Handling networkplan.HandlingMode default graceful; Owner.RevokeRecorded(context.Context)(hostrevoke.Result,error); ApplicationStatus.Handling; Status.Strict *StrictStatus with enabled/cleanupConfirmed/observedAt/validUntil/imageDigest/result.

- [x] Missing Config/API/status RED tests: strict apply→cleanup, every verify→cleanup, graceful never cleanup, failure/remaining/generation/boundary no strict confirmation, mode mismatch before desired store, default/runtimeKey stable and unknown CLI flag mode rejected before owner.
- [x] Implement explicit flag/facts and mode check, strict cleanup on each driver wake, coherent final status and deadline check. No persisted receipt.
- [x] Full uncached unit/race/vet/build/static smoke GREEN; integration compiles, commit.

### Task3: native admission/recovery/process evidence and delivery

Files: hostowner/strict_integration_test.go; hostagent/strict_process_integration_test.go; report.go; bilingual README/examples/validation.

- [x] Add TestOwnerKernelStrictMembership, TestOwnerKernelStrictPhysicalReplacement, TestOwnerKernelStrictPendingRecovery, TestEnforceProcessStrictRevocation, TestEnforceProcessStrictExpiryWithoutProcess, TestOwnerKernelStrictReverseReconstruction, TestOwnerKernelStrictOriginalDNAT. Compile helpers using actual input compiler.
- [x] One fresh whole-branch reviewer on base accepted main; grade findings by user effect; Important/Critical one observed RED→GREEN pass plus full suite, Minor ledger/defer.
- [x] Publish exact code tree, native CI87 required plus actual raw counts, zero failures/skips; independently download ZIP and verify digest/raw/report/pins/build checkout/tree. Record bilingual acceptance.
- [ ] Final PR-head CI and artifact pass; expected-head merge; exact main push CI/artifact and no open PRs; collect exhaustive rulings/minors then delete only this plan scratch.
