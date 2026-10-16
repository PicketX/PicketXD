# Host admission implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Execute inline under the existing continuous-delivery authorization, with one fresh whole-branch reviewer.

**Goal:** Apply and recover actual bounded host admission rules through the owned namespace Session.

**Architecture:** Pure typed images generate deterministic expressions. Session journals pending and confirmed images around an atomic transaction and strict complete readback. Absolute kernel time handles future boundaries without a live process.

**Tech Stack:** Go 1.27, pinned Go nftables/Netlink libraries; no new dependencies.

**Spec:** [Design](../specs/2026-10-16-host-admission-design.md)

## Global constraints

- host-local/original-destination/graceful TCP/UDP, equal IPv4/IPv6.
- 4096 targets, 8192 generated rules, 8 MiB per image, 64 MiB journal.
- Preserve all 55 existing gates. No global conntrack flush or production imports of validation fixtures.
- No restored enforcement boolean without full live confirmation; preserve filtering on Close.

## Review focus

- Caller-mutated images or a forged active mark must not produce a usable receipt (Task 1/2 tests).
- Crash after pending, dispatch or confirmation publication must resolve only through exact live evidence (Task 2/3).
- New-boot old images and same-boot missing tables must not restore stale policy proof (Task 2).
- Unknown raw expression attributes and concurrent generation changes must reject adoption (Task 2).
- Delayed application and process loss before future deny must preserve original absolute time (Task 3).

### Task 1: typed image and deterministic rules

Files: new `internal/hostrules/{image,rules}_linux.go` and tests.

Interfaces: `Build(*hostpolicy.Policy, markstate.State) (Image,error)`, `Image.Validate() error`, `Image.Digest() string`, `Image.Layout(marker string) (Layout,error)`. Image carries full semantic Target/membership validity; Layout carries complete chains and rules, no mutable sockets.

- [ ] Write tests for full inventory future/revoked members, mark/shape mismatch, limits and unsupported dates, target isolation and absolute expression boundaries; observe missing-feature failure.
- [ ] Implement bounded canonical image and deterministic INPUT rules with deny precedence, SYN guard, INVALID/UNTRACKED drop and distinct marks.
- [ ] Run `go test ./...`; expect all pass. Commit.

### Task 2: durable Session apply and strict recovery

Files: new `hostowner/{application,inspect}_linux.go`; modify owner/nft/raw/marks/files and add unit tests.

Interfaces: `Session.ApplyPolicy(context.Context,*hostpolicy.Policy,markstate.Range) (ApplicationStatus,error)`, `Session.ApplicationStatus() ApplicationStatus`. Private backend configures validated known images before inspection and applies one atomic image on its held namespace connection.

- [ ] Write failing tests for pending-before-effects, dispatch/readback/save uncertainty, exact old/new recovery, new boot, no-op, scope mismatch and closed/faulted session.
- [ ] Implement v3 validation, image bounds, strict generation-bracketed raw inspection, pending/confirmed lifecycle and fault handling. Reconcile marks under the same mutex; refuse retiring marks still referenced by a confirmed image outside ApplyPolicy.
- [ ] Add malformed/unknown nested expression and generation-change regression tests; observe failures before corresponding implementation.
- [ ] Run `go test ./...`, race/vet/build; expect all pass. Commit.

### Task 3: native admission acceptance and delivery

Files: new `hostowner/admission_integration_test.go`; update report, README and bilingual validation record.

- [ ] Write real dual-stack default-deny/allow/deny, process-free future boundaries, restart recovery, atomic rejection, drift and explicit withdrawal tests before claiming native support.
- [ ] Compile integration binary; expect success. Fresh reviewer checks entire branch; fix Important/Critical findings with RED→GREEN and full green suite.
- [ ] Publish draft PR using the exact local tree; run existing and new mandatory native CI. Verify artifact digest, every raw test stream, and actual module/kernel/toolchain versions. Record evidence and sync Chinese documentation.
- [ ] Verify final PR head CI, merge expected SHA under standing authorization, verify main tree and post-merge CI. Remove only this plan's scratch workspace.
