# M1.3A Planner Fixture Compiler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> English (default) | [简体中文](2026-10-15-planbridge-compiler.zh-CN.md)

**Goal:** Convert an immutable shared plan into owned, explicit-time private M0 snapshots while rejecting targets outside the declared test fixture.

**Architecture:** A pure `internal/validation/planbridge` package consumes only shared Core/planner packages and the private fixture types. A validated scope fixes authority, epoch, Agent, context, namespace identity and fixture catalog; prepared candidates are immutable. A later coordinator must supply proven topology facts, decide which candidate is accepted, apply/read back the batch and retain actual emitted history.

**Tech Stack:** Go 1.27.0; standard library; PicketX at immutable `e39f96e2787154a8af8e7d996b2f65aa10b6c3eb`; existing Netlink versions unchanged.

**Spec:** [Approved M1.3 design](https://github.com/PicketX/PicketX/blob/9d53b13e2d8a805b2a7b2f3a8ad957fcf84ea12d/docs/superpowers/specs/2026-10-15-core-m0-bridge-design.md), sections 1–3 and candidate validation in 4–5. [Delivery route](../../DELIVERY.md) tracks the remaining mandatory work.

## Global Constraints

- GPL implementation stays in PicketXD; import only `pkg/core` and `pkg/networkplan` from Apache PicketX. No floating branch or committed local replace.
- Scope supports only `transit` / `original-destination` and one fixed `graceful` or `strict` mode; declarative identity is not topology proof.
- Ordinary destinations: `192.0.2.100/32`, `2001:db8:1:0:2::100/128`; TCP/443, TCP/22, UDP/8002. NAT VIPs: `192.0.2.101/32`, `2001:db8:1:0:2::101/128`; TCP/8000, TCP/8001. No implicit port rewrite.
- Preserve all Binding IDs, separate Grant/Deny IDs, independent absolute validity and same-family pairing. Only explicit permanent membership has zero M0 expiry.
- Reject complete invalid candidates without a partial result. Empty owned output is explicit intent, never a wildcard or permission to call nonempty M0 Apply.
- No clock, timer, namespace handle or backend operation in the compiler; no actual-installed history or production readiness claim.
- User's 2026-10-15 continuous-delivery instruction authorizes implementing routine slices without another plan-confirmation round. Preserve inline execution and one final independent review.

## Review Focus

- Forged zero scope or changed epoch/context must fail before a result is returned (Task 1/3).
- Future/expired/revoked membership must not become permanent access (Task 2).
- Removing an Agent must not reinterpret an arbitrary query error as empty ownership (Task 3).
- Returned provenance/inventory/withdrawals must not alias accepted or future candidates (Task 2/3).
- Unsupported unbound input must still undergo shared full validation; unsupported selected tuple must fail (Task 2/3).

## Context Map

| Files | Responsibility / change |
| --- | --- |
| `go.mod`, `go.sum`, `THIRD_PARTY_NOTICES.md` | Pin shared source/checksums; preserve existing Netlink versions; distinguish package imports from module graph |
| `internal/validation/planbridge/scope.go`, `scope_test.go` | Immutable declarative scope, fixed ordinary/NAT catalog; no topology attestation |
| `internal/validation/planbridge/compiler.go`, `compiler_test.go` | Compile/Replace, catalog compatibility, explicit-time snapshot/provenance |
| `internal/validation/planbridge/replacement_test.go` | Same-authority replacement, removed-Agent evidence, returned-value ownership/concurrent reads |
| `internal/validation/planbridge/doc.go`, `README.md`, `README.zh-CN.md` | Compiler-only status, next lifecycle and kernel gates |
| `pkg/networkplan` / `pkg/core` in shared module | Immutable ownership, resolved validity, replacement provenance; dependencies only |
| Existing `fixture/snapshot.go` and its tests | Nonempty private snapshot semantics; unchanged |
| Existing M0 nft/ct tests and kernel runner | Regression gate; unchanged, no new bridge packet claim |

Public APIs, database migrations and user configuration do not change. This map was checked against the approved design before implementation. Later production schemas and local topology proof remain separate work.

### Task 1: Pin dependency and validate immutable fixture scope

**Files:** `go.mod`, `go.sum`, `THIRD_PARTY_NOTICES.md`, `internal/validation/planbridge/{scope.go,scope_test.go}`.

**Interfaces:** Produces `Identity{AuthorityID, Epoch, AgentRef, NamespaceID string}`, `Catalog` (`Ordinary`, `NAT`), `NewScope(Identity, networkplan.ExecutionContext, Catalog) (*Scope, error)`; Scope has no public mutable fields.

- [ ] Resolve the immutable revision with Go, record its pseudo-version/checksum, and preserve selected Netlink versions. Verify actual package imports omit Echo/GORM/Web.
- [ ] Write `TestScopeRejectsUnsupportedFacts`: valid scope succeeds; empty identity fields, context Agent/namespace mismatch, empty context ID/profile, host/published/post-DNAT, invalid handling, unknown catalog and excessive/noncanonical destinations return nil/error. Write `TestScopeOwnsFacts`: modifying caller destinations after construction does not change later acceptance.
- [ ] Run `go test ./internal/validation/planbridge -run TestScope -count=1 -v`; expect the new validations to fail against a minimal constructor stub.
- [ ] Implement scope validation and clone destinations. Catalog uses exact fixture host destinations and declared protocol/port pairs; context metadata is input, never proof of a live namespace.
- [ ] Run `go test ./...`, then commit `feat(agent): pin shared core and validate bridge fixture scopes`.

### Task 2: Compile owned, explicit-time snapshots and provenance

**Files:** `internal/validation/planbridge/{compiler.go,compiler_test.go,doc.go}`.

**Interfaces:** Consumes Task 1 scope. Produces `Prepare(*Scope, *Prepared, networkplan.Input) (*Prepared, error)`, `(*Prepared).Materialize(time.Time) (Materialized, error)`. `Materialized` owns `Identity`, `At`, `NextChange`, `Snapshot`, `Provenance`, `Inventory`, `Withdrawals`, `AgentRemoved`. `Member` carries domain/ID/authority/target/BindingIDs/rule/validity/revoked state. Prepared has no public mutable fields or shared plan exposure.

- [ ] Write `TestMaterializeIndependentDualStackMembership`: a Gitea fixture produces four protections (TCP/443 and TCP/22 per family), a separate UDP fixture preserves 17/8002; TCP maps to 6; exact-host and wider-CIDR contributions keep literal absolute expiries and both Binding IDs; identical Grant/Deny IDs remain in separate domains. NAT has 6/8000 and 6/8001 at original VIPs.
- [ ] Write `TestMaterializeLifetimeBoundaries`: at explicit `2026-10-15T09:00:00Z`, start is inclusive, end is exclusive, future/revoked/expired are absent from rules but retained in inventory; NextChange is literal `09:30`; zero evaluation time errors; only permanent expiry is zero. At final expiry protections remain with no allow.
- [ ] Write `TestPrepareRejectsUnsupportedTargets`: an unsupported selected port/protocol/boundary/profile/class/stage/handling or unknown Agent fails with nil result; full shared validation rejects malformed unbound input. No implicit address/port rewrite.
- [ ] Run the named tests, observe missing behavior against minimal signatures, then implement Compile/Inventory/Projection consumption, exact scope checks and owned sidecars. Keep equivalent contributions separate; deduplicate only protection tuples.
- [ ] Run `go test ./...`, then commit `feat(agent): compile shared plans into owned fixture snapshots`.

### Task 3: Validate replacements, removals and ownership; publish slice

**Files:** `internal/validation/planbridge/replacement_test.go`, compiler adjustments, bilingual README and verification notes.

**Interfaces:** Task 2 `Prepare` calls shared Replace against the explicitly supplied accepted predecessor. Removed-Agent results are accepted only from same-scope complete replacement, explicit candidate Agent absence and old-target withdrawals, or a retained prior explicit empty state. Materialization does not catch arbitrary planner errors as empty output.

- [ ] Write `TestReplacementPreservesAcceptedCandidate`: malformed replacement, changed authority/epoch/context and retained-ID retarget fail; predecessor still materializes exactly as before. Removing one Binding retains the other Binding's provenance; removing all Grants keeps protection.
- [ ] Write `TestRemovedAgentRequiresReplacementEvidence`: initial unknown Agent errors; removal after a nonempty valid predecessor yields explicit empty/withdrawals with old tuples; repeated empty candidate retains removed status; reintroduced Agent resumes normal shared validation.
- [ ] Write `TestPreparedOutputsOwnData`: mutate returned Snapshot, Member.BindingIDs, inventory/context/matches/contributions and withdrawal fields, then re-materialize unchanged; concurrent readers pass race checking. Mutating caller input after Prepare cannot change results.
- [ ] Run named tests, observe assertion failures for unsupported replacement behavior, then implement the minimal removed-Agent path and independent output ownership.
- [ ] Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, `git diff --check`; compile all integration tests. Preserve M0's actual kernel gate and identify compiler-only evidence.
- [ ] Commit `test(agent): verify bridge replacement and ownership contracts`; perform independent whole-branch review, fix blocking findings with regression tests, publish PR and verify final-head CI.

## Self-review and handoff

All pure/candidate requirements in M1.3 sections 1–3 map above. The topology proof, apply ACK/readback/receipts, cumulative installed history, exact cleanup/empty-table deletion, timers and new bridge packet suite remain mandatory M1.3B/C work. This slice cannot complete M1.3 or the basic product. Runtime/HTTP/UI work follows the delivery route. User's continuing instruction preserves inline execution without a new review stop; decisions and actual evidence travel in the implementation PR.
