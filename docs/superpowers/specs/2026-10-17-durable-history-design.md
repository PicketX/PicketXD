# M2C7: durable compact cleanup history

> English (default) | [简体中文](2026-10-17-durable-history-design.zh-CN.md)

## Intent

Persist enough confirmed historical ownership to clean withdrawn connections after restart, without caller-supplied old images. This increment does not add automatic cleanup, strict admission/configuration/scheduling, process-free strict guarantees, Controller/L7/admin, or fix the three previously deferred minors. `enforce` remains graceful. The user authorizes staged continued delivery, pushes and suitable merges; execute inline with one fresh whole-branch review and actual CI.

## History representation

Add immutable `hostrevoke.History` (version1, authority/context, sorted targets). Each historical target retains exact mark/physical identity/physical context/protected endpoints and deduplicated `(source,endpoint)` members. Grant IDs, absolute lifetimes and full source inputs are not needed to prove historical marked-flow ownership; full current image/input and replacement baseline remain unchanged. Only a complete image actually confirmed live is folded into the durable history. Future grants in an installed image count as potential ownership because the kernel can activate them without a process. No garbage collection, expiry-based forgetting or mark reuse. Repeated renewals/UID changes with identical physical/source scope do not grow history.

Bounds: 4096 historical target marks, 65536 source/endpoint members, 8 MiB compact history JSON. Reject overflow before mark publication/nft effects; retain the previous applied image. Derive physical identity using the same shared target-shape helper as hostpolicy.MarkTargets, preserving existing hashes byte-for-byte. Retain each target's exact Protection.Context separately from potentially broader trusted image bounds. Validate canonical prefixes, matching families, protected endpoint membership, context bounds, sorted unique targets/members and exact active/retired mark ledger bindings. `Remember(previous,image)` returns an independent value; `NewRecorded(history,current,marks)` validates/clones and creates the existing cleanup Plan. The low-level zero-value Plan and production rollback-detection minors are unchanged.

## Journal v4 and crash ordering

`applicationRecord.History` has internal `{complete,state}`. Journal v4 requires it; v1/v2/v3 remain readable with previous validation. Older binaries reject v4, so in-place binary downgrade is unsupported. Same-boot scope/context migration is rejected before effects if existing history would become incompatible. Module pins, nft image schema/digest/marker and manifests/CLI flags stay unchanged.

Fresh first application begins coverage `complete=true`. Legacy v3 application seeds only its known confirmed image and remains `complete=false`, permanently for that boot. A no-op application can durably upgrade metadata to v4 without changing attempt/image or issuing nft apply. Plain legacy reopen does not silently invent missing history. A verified actual new boot with absent old kernel table resets historical scope to empty/complete while retaining mark retirement and logical replacement baseline: previous boot conntrack entries cannot survive. This is unit-injected boot evidence, not a claim CI rebooted a host.

Before side effects, compute both the previous-history state and candidate-history preview, including bounds. Pending v4 publication retains only previous confirmed history plus the separate pending image. After atomic dispatch, complete live readback and revalidation, fold candidate into history in the same durable confirmed publication. On reopen, live pending recognition promotes it and folds exactly that image; live old recognition clears the pending candidate without remembering it. Any durability ambiguity faults the Session and no recorded cleanup proceeds until reopen. A valid v4 confirmed image must be covered by its history; pending preview must fit and have compatible context. Journal corruption/missing membership/shape/mark mismatch fails startup without replacement.

## Runtime API and status

`Session.HistoryStatus()` returns value-only metadata: Tracked, Complete, Targets, Members, Bytes, Digest. Complete additionally requires a live confirmed application, no pending ambiguity, and an open/unfaulted verified Session; it is not a strict-cleanup receipt. Add this metadata to `hostagent.Status` through the Owner interface. No cache/desired acceptance behavior changes.

`Session.RevokeRecorded(ctx) (hostrevoke.Result,error)` takes no historical input. Under the held mutex it requires complete v4 coverage and the actual current confirmed image, then uses `NewRecorded` and the same full-live-confirm/both-family-dump/exact-ID-delete/reconfirm protocol. Incomplete/legacy/pending/unverified history fails before any flow deletion. Recovered current images may retain known retired ledger entries after a rejected replacement; recorded cleanup accepts exact bindings while the held Session confirms the actual live image, without reclaiming marks. Scoped `RevokeWithdrawn` keeps its active-current check. Recorded cleanup never prunes history, writes journal or claims persisted strict completion; errors retain the existing fault/reopen behavior.

## Context map

| Files | Purpose/change |
| --- | --- |
| hostpolicy/marks.go + tests | Extract exact TargetIdentity helper with unchanged hashes. |
| hostrevoke/history.go + tests; plan.go | Compact immutable historical source/target/mark proof, bounded validation/dedup; recorded Plan selection. |
| hostowner/history_linux.go + tests | Coverage lifecycle, status, record validation and migration helpers. |
| hostowner/application_linux.go; owner_linux.go | v4 pending/confirmed recovery integration, legacy/no-op/new-boot behavior; no image schema changes. |
| hostowner/cleanup_linux.go + tests | Recorded-only held API using actual history/current image; shared run adapter. |
| hostagent/run_linux.go + tests | Value metadata in JSON; injected Owner method updated. |
| hostowner/history_integration_test.go; owner helper; report | Real restart, pending success/rejection, legacy refusal, corruption refusal, SIGKILL protocol; all 75 old gates retained plus5. |
| bilingual docs/spec/plan/README | Exact evidence, downgrade and incomplete-history limitations. |

Reference patterns: M2C4 pending image recovery, M2C6 exact cleanup engine and failure gates, M2C5 actual process helper. Production never imports fixture models. State max64 MiB remains ample for current/pending/baseline and compact8 MiB history. Context map self-review precedes implementation. Journal migration is additive v4, no database/config migration.

## Acceptance and review focus

RED→GREEN tests for immutable compact scopes and dedup across time/UID changes, malformed/cross-scope/oversize evidence, raw mark hash stability, current permission union. Observe missing-history/recorded API RED. Exercise fresh/legacy/no-op/actual pending promotion/old rollback/new-boot reset, disk-confirm failure, missing confirmed membership, no side effects on bounds/context rejection. Full unit/race/vet/build/static smoke and integration compile pass. One fresh reviewer covers coverage truthfulness, pointer aliasing, migration, mutable/private boundaries and no cleanup before confirmation. Real disposable namespace CI executes five required new gates: multi-generation restart recorded cleanup, actual pending recovery and atomic rejection, legacy incomplete refusal, corruption refusal, helper SIGKILL with parent-held dual-stack TCP/UDP flows. Confirm all80 required plus101 recorded top-level gates (subject to actual raw counts), zero fail/skip, downloaded digest/raw names and exact final PR/main tree/build revisions before merge completion.
