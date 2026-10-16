# Static enforcement implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Native inline execution under the user's continuous delivery authorization, with one fresh whole-branch review.

**Goal:** Run and recover static dual-stack host admission through an actual command.

**Architecture:** Coherent desired Snapshot + held namespace facts feed the existing compiled-input/owned-image engine. Reuse the store's polling/status boundaries, apply only accepted changes and verify unchanged images. Separate current desired confirmation from retained application proof.

**Tech Stack:** Go 1.27 and existing pinned dependencies.

**Spec:** [Design](../specs/2026-10-16-static-enforcement-design.md)

## Global constraints

- Explicit `enforce`; preserve nonfiltering prepare. Only host-local/original-destination/graceful TCP/UDP, equal families.
- Require absolute private state-dir, explicit destinations and nonzero uint32 mark range of at most 4096; poll defaults 5s.
- No dependency/journal-schema changes; preserve 64 existing native gates.
- Output current desired confirmation only after matching coherent generation/hash and live receipt. Close never removes filtering.

## Review focus

- Candidate acceptance during a status callback must not mislabel the installed generation/hash (Task 1/2).
- A newer desired cache and older recovered live image must remain distinguishable after failed application (Task 2/3).
- Invalid once startup with a cache must emit retained proof but exit nonzero (Task 2/3).
- Startup state scope/mark-range changes, corrupt cache, alternate state-dir and drift must not overwrite live rules (Task 3).
- Process loss before future deny/allow expiry must retain the original absolute time, and restart must not extend it (Task 3).

### Task 1: coherent desired snapshot and held runtime scope

Files: agentstate/store.go + snapshot_test.go; hostowner/scope_linux.go + scope_test.go.

Interfaces: `Store.Snapshot(time.Time) (Status,*configuration.Candidate,error)`; `Session.RuntimeScope(context.Context) (RuntimeScope,error)` with BootID/NetworkSpaceID.

- [ ] Write atomic snapshot/concurrent accept and namespace/closed/faulted/revalidate tests; run focused tests and observe missing-feature RED.
- [ ] Implement locked snapshot and namespace scope, without filesystem or kernel mutations.
- [ ] Run `go test ./...`; expect GREEN. Commit.

### Task 2: injected coordinator and explicit CLI

Files: new hostagent config/run and tests; agentcli/enforce and tests; main error wording.

Interfaces: `hostagent.Run(context.Context,Config,Dependencies) error`; dependencies supply Owner/Store factories, slog logger and status emitter. Owner exposes runtime scope/apply/verify/status/close; Store exposes Snapshot/Reload/Run/Close.

- [ ] Write failing tests for settings, first apply, unchanged verify, coherent current candidate, invalid reload retention, once fallback error, fatal apply/verify/cache faults and resource close.
- [ ] Implement complete-input compilation from pinned facts, runtime identity hash, status association and command required flags.
- [ ] Run unit/race/vet/build/static prepare smoke; expect GREEN. Commit.

### Task 3: standalone packet/process acceptance and delivery

Files: hostagent process integration tests; kernel runner/report; examples/enforce and bilingual README/validation.

- [ ] Add actual prebuilt binary gates for dual-stack TCP/UDP + reload/withdrawal, SIGKILL + absolute expiry/future denial + restart, startup scope/cache/foreign refusal, drift and singleton; all setup errors fail opted-in runs.
- [ ] Compile all integration binaries, review branch with a fresh most-capable reviewer, fix Important/Critical findings RED→GREEN with full green suite.
- [ ] Publish exact tree, run all old/new native gates, independently verify artifact/digest/raw streams and versions, record bilingual evidence.
- [ ] Verify final PR head CI, merge expected SHA under existing authorization, confirm main exact tree/CI and no open PRs. Delete only this plan's scratch.
