# Agent Controller service implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline, then ONE independent whole-branch review.

**Goal:** Shipped Controller-mode example unit with actual systemd/remote LKG acceptance.
**Architecture:** Separate unit, existing CLI/remote source unchanged; real private
Controller, named disposable netns, actual shipped binary and packet peers.
**Tech Stack:** Go/Cobra/gRPC, systemd, Python stdlib, existing Node test runner.
**Spec:** docs/superpowers/specs/2026-10-20-agent-controller-service.md

## Global constraints
No runtime/API/kernel schema changes; IPv4/IPv6 equal; graceful new-flow guarantees
only; no host reboot/backup migration claim. Native setup failures fail, never skip.
Private token only file, public evidence allowlisted. All existing checks retained.

## Review focus
- A valid static deny sentinel cannot substitute for authenticated remote permission.
- A surviving READY/old journal line is not a fresh process/current APPLIED receipt.
- Dead echo peers cannot be mistaken for expiry denial; independent unprotected health.
- Offline LKG must neither extend deadlines nor release clean-start readiness.
- Cleanup/privacy must remain scoped to created namespace/unit/private directories.

### Task 1: Remote service and actual recovery gate
**Files:** all context-map files in spec and bilingual delivery checkpoints.
**Interfaces:** test-controller-service.py --binary PATH --controller-bundle PATH
--artifacts PATH --unit PATH; requires PICKETX_CONTROLLER_SERVICE_TESTS=1/root;
outputs public controller-service-report.json. --self-test runs evidence-validator
fixtures without kernel, --worker is internal only. Existing APIs/CLI consumed.
- [ ] Write native acceptance and evidence contract tests before unit; run real
  validator mutation negatives RED, then minimal validator GREEN. Expected reject
  missing/forged process/deadline/cache/source/current receipt/secret fields.
- [ ] Publish tests-first native with old static unit and valid deny sentinel;
  expected actual missing remote APPLIED/permission RED, retain original run/log.
- [ ] Add separate reviewed Controller unit, use it in native gate; offline verify
  both units. Preserve rate/restart/sandbox settings; substitute only test paths,
  provider identity and poll interval in disposable test copy.
- [ ] Actual native startup/packet allow; SIGKILL automatic restart/cache equality;
  real outage/deadline/new-flow denial with independent health; expired offline
  restart activating/not READY; reconnect READY/current APPLIED/still deny.
- [ ] Run all Node, full Go/race/vet/build, locked Web and all native jobs. Expected
  full passes natively; local inherited Unixgram EPERM explicitly reported.
- [ ] Commit; ONE independent complete-branch review, ONE root RED→GREEN correction
  pass for Critical/Important, record deferred Minors and all rulings/costs.
- [ ] Final exact head/tree/full CI/four actual downloaded archives verification,
  expected-head merge and full main recheck; immediate explicit lease release.
