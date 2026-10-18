# Controller Provider implementation plan

> **For agentic workers:** Use superpowers:executing-plans, implementing inline with one independent whole-branch review.

**Goal:** Apply authenticated remote intent through the existing host executor.
**Architecture:** Adapt scoped wire snapshots to compiled configuration; retain the existing
owner/strict driver and atomic private journal. Serialize stream acceptance/application.
**Tech Stack:** Go 1.27, grpc-go/protobuf, Cobra, existing netlink backend.
**Spec:** ../specs/2026-10-18-controller-provider.md

## Global constraints
IPv4/IPv6 equal support; absolute deadlines; deny precedence; one authoritative source;
no mandatory shared library; do not trust remote namespace facts; expected-head merge.

## Review focus
- Unsupported dormant scope must fail complete acceptance.
- Replayed/equivocating revision cannot alter LKG.
- Endpoint/provider changes cannot adopt old cache.
- Outage/restart must not refresh absolute deadlines.
- Kernel failure must never report APPLIED.

### Task 1: Scoped conversion and durable acceptance
Files: internal/remotepolicy/{policy,policy_test}.go; internal/agentstate/{store,journal,remote}_linux.go and remote tests.
- [x] Write failing scope/deadline/revision/cache tests and run RED.
- [x] Implement strict conversion and atomic provider-bound remote acceptance.
- [x] Run targeted tests GREEN and commit.

### Task 2: Stream lifecycle and executable source selection
Files: internal/agentstate remote transport/loop; internal/hostagent config/run; internal/agentcli/enforce_linux.go; associated tests/examples.
- [x] Write failing real TCP/TLS, disconnect, report and CLI exclusivity tests.
- [x] Implement bounded/reconnecting stream, serialized execution reports and source flags.
- [x] Run targeted and complete unit/race/vet/build/static checks.

### Task 3: Native acceptance and integration
Files: internal/hostagent remote process integration; scripts/test-kernel.sh and report gate; bilingual docs.
- [x] Add required real dual-stack remote apply/revoke/outage/restart evidence.
- [x] Push PR; independent full-branch review; resolve findings and rerun checks.
- [ ] Verify exact remote tree/head and native raw evidence, merge with expected SHA.
- [ ] Verify main CI and persist next checkpoint.
