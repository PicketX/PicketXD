# Scoped Host Conntrack Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. User authorizes inline continued implementation, pushes and suitable merges; one fresh whole-branch reviewer.

**Goal:** Add production scoped conntrack cleanup under held ownership, with actual dual-stack evidence.

**Architecture:** Immutable validated images + mark ledger define historical ownership/current permission. Typed complete native dumps precede exact ID/zone deletion; held Session confirms actual current admission image before effects and after confirmation.

**Tech Stack:** Go 1.27, existing pinned Go Netlink/Core libraries.

**Spec:** [Design](../specs/2026-10-17-host-revocation-design.md)

## Global constraints

No fixture production imports, CLI/config/schema changes, automatic strict claim or mark reuse. Preserve existing 71 native gates. Equal IPv4/IPv6 TCP/UDP; only exact CT tuple/zone/opaque ID deletes. 32 history images/32 MiB JSON, 64 MiB native dump/family, 262144 typed flows. Any setup failure fails opted-in tests. Private durable filesystem and privileged-writer trust remain unchanged.

## Review focus

- New allowance in another target/mark must preserve an old flow: Task1 union test.
- A future activation or expiry crossed during IO cannot silently succeed: Task1 boundary/cancellation tests.
- Kernel CTA_ID is opaque, not a human-order integer; directional zones cannot silently become zone0: Task2 wire + actual ID/zone gates.
- Invalid caller history cannot widen physical scope beyond durable mark shapes: Task1/3 rejection tests.
- Partial native delete/dump or current-image drift cannot appear fully cleaned: Task3 actual/injected gates.

### Task1: immutable plan and cleanup protocol

Files: hostrevoke/plan.go, run.go, plan_test.go, run_test.go.
Interfaces: `New([]hostrules.Image,hostrules.Image,markstate.State)(*Plan,error)`, `Plan.Select([]Flow,time.Time)([]Flow,error)`, `Run(context.Context,*Plan,Effects)(Result,error)`. Flow has Family/Mark/Zone/ID/Original/Reply. Effects Confirm/Dump/Delete/Now; Result Selected/Deleted/Absent/Remaining.

- [ ] Write failing real image/ledger tests for historical source/physical mark scoping, union/deny/absolute times, immutable input, shape/scope/limits and invalid flow.
- [ ] Observe missing APIs RED; implement only pure plan/types.
- [ ] Write failing protocol tests for confirm ordering, both-family preflight, changing eligibility/boundaries, backwards clock, cancellation, delete errors/ENOENT and incomplete/remaining confirmation; implement required protocol.
- [ ] Full `go test ./... -count=1` GREEN; commit.

### Task2: typed native wire and held Session

Files: hostrevoke/wire_linux.go/tests, hostowner/cleanup_linux.go/tests.
Interfaces: `Decode(family uint8,messages []netlink.Message)([]Flow,error)`, `DeleteMessage(Flow)(netlink.Message,error)`; `Session.RevokeWithdrawn(context.Context,[]hostrules.Image)(hostrevoke.Result,error)`. Native socket lists complete flows and sends one exact delete through existing ACK helper.

- [ ] Observe RED wire tests for TCP/UDP/families/NAT/ID/zone and malformed/missing/duplicate/truncated attrs, unrelated protocols, directional zones.
- [ ] Implement strict decoder and original/zone/ID serializer; no wildcard/delete-filter API.
- [ ] Observe RED Session tests for missing application, extra/forged history, closed/faulted/drift and actual digest confirmation; implement held lock/current-image-only adapter.
- [ ] Full unit/race/vet/build + static prepare smoke GREEN; commit.

### Task3: native acceptance and integration

Files: cleanup_integration_test.go, report required names, bilingual README/validation.

- [ ] Add four native gates: cleanup/preservation TCP+UDP both families; DNAT/original and zone+ID safeguards; drift/preflight failure zero deletion; partial deletion/remaining error with complete confirmation.
- [ ] Compile integration, run fresh whole-branch reviewer, fix Important/Critical once RED→GREEN and full suite green.
- [ ] Publish exact tree; all 71 old +4 new required gates GREEN; inspect downloaded raw streams/digests/versions and record bilingual evidence.
- [ ] Final-head CI + artifact verified; expected-head merge; exact main tree/push CI verified; no open PRs; delete only this plan scratch, retain linked worktree.
