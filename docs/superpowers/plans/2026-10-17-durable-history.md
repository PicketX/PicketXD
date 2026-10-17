# Durable Compact Cleanup History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Native inline execution under standing user authorization; one fresh whole-branch review.

**Goal:** Recover complete confirmed historical ownership and clean old connections without caller images.

**Architecture:** Compact source/physical-target/mark history joins application journal v4. Preview before effects, remember only with durable live confirmation; recorded cleanup requires complete coverage and held current proof.

**Tech Stack:** Go1.27 and existing pinned dependencies.

**Spec:** [Design](../specs/2026-10-17-durable-history-design.md)

## Global constraints

No automatic strict claim or CLI/config/image-schema change. Preserve v1-v3 reads; older binaries reject v4. Compact limit4096 targets/65536 members/8MiB, journal64MiB unchanged. Preserve all75 native gates, equal IPv4/IPv6 TCP/UDP, no fixture imports/mark reuse/whole-table flush. No fixing existing deferred minor code in this pass.

## Review focus

- Pending image may have reached the kernel before history confirmation; status/cleanup must stay unconfirmed until live recovery: Task2/3 tests.
- Legacy v3 has lost source history even if current image is valid; no migration to complete=true: Task2/3 legacy tests.
- Historical scope may differ after context/namespace changes; same-boot reject vs real new-boot reset: Task1/2 tests.
- Repeated source renewal must not exhaust a full-image-count budget: Task1 repeated UID/deadline test.
- Cleaned/withdrawn sources and retired marks must remain recoverable across restart: Task2/3 tests.

### Task1: compact ownership representation

Files: hostpolicy/marks.go/tests; hostrevoke/history.go/history_test.go/plan.go.
Interfaces: `hostpolicy.TargetIdentity(key,context,matches)(markstate.Target,error)` preserves old hash. `Remember(previous *History,image hostrules.Image)(History,error)`, `History.Validate(marks markstate.State)error`, `NewRecorded(History,hostrules.Image,markstate.State)(*Plan,error)`.

- [ ] Observe missing APIs RED with actual compiled images, immutable/dedup/shape/family/mark/context/bounds tests.
- [ ] Implement exact identity helper, compact representation and recorded selection; current allow/deny remains existing Plan semantics.
- [ ] Full uncached tests GREEN; commit.

### Task2: v4 history lifecycle and runtime entry

Files: hostowner/history_linux.go/tests; application_linux.go; owner_linux.go; cleanup_linux.go/tests; hostagent/run_linux.go/tests.
Interfaces: internal cleanupHistory Complete/State; public HistoryStatus values; Session.HistoryStatus; Session.RevokeRecorded(ctx)(hostrevoke.Result,error); hostagent.Owner adds HistoryStatus.

- [ ] Observe RED fresh/legacy/no-op/recovery/failed-save/new-boot history tests and recorded cleanup/status API tests.
- [ ] Add v4 validation and bounded preflight, pending/confirmed atomic folding, legacy incomplete migration and new-boot empty coverage with retained retirement/baseline.
- [ ] Add held recorded cleanup and JSON metadata; test no destructive effects for incomplete/faulted/corrupt coverage, no journal mutation by cleanup, immutability/closed status.
- [ ] Full unit/race/vet/build/prepare smoke GREEN and integration compile; commit.

### Task3: native recorded lifecycle and delivery

Files: history_integration_test.go, existing owner helper, report requirements, bilingual README/validation.

- [ ] Add actual both-family TCP/UDP multi-generation restart cleanup; pending live success and rejected transaction; legacy incomplete refusal; corruption refusal; helper SIGKILL with parent-held A/B connections.
- [ ] Fresh whole-branch review; Important/Critical one RED→GREEN fix pass + full green suite; Minor ledger/defer.
- [ ] Publish exact code tree, require all80 native gates, independently check actual raw counts/artifact digest/modules/binary revisions; record bilingual evidence.
- [ ] Verify final PR-head CI/download, expected-head merge, exact main tree/push CI; no open PRs; collect rulings/deferred minors before deleting only this plan scratch.
