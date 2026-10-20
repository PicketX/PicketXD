# Installed HTTP audit implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans inline; one fresh whole-branch reviewer. Steps use checkbox syntax.

**Goal:** Real installed audit-page events correlated to actual admitted/denied traffic.
**Architecture:** Extend existing dual native flows with one focused helper/receipt gate.
**Tech Stack:** Existing Go/Linux/Node/Playwright/React/npm; no new dependency.
**Spec:** docs/superpowers/specs/2026-10-20-installed-http-audit.md

## Global Constraints
- Exact Corec67da10767d5cb92012366b6328baa31be8afa61/treeb2d6855d8406d70f98cc0d4c17fae9e31803a249.
- No product/kernel/UI/DB/API/dependency changes, no mocked native success.
- Four actual post-restart dual-stack ingress probes; exact API/DOM/UID/revision correlation.
- Password and realHTTPSOIDC human approver, separate context/no bootstrap fallback.
- Keep all prior31cases+Gitea/rawAgent/CLI/systemd/restores/offline/static gates.
- One whole-branch review/root one blocker pass; fullnative/raw/fouractualarchives/main.

## Review Focus
- An old/unrelated audit row or duplicate ID must not satisfy a new request probe.
- HTTP403 must prove a live admitted path with zero application forwarding, not a dead service or L4 outage.
- Audit UI must use the actual managed approver session in each login mode, not bootstrap or a fabricated identity.
- Restored/current/applied revision and family-specific Grant UIDs must remain correlated across API, DOM and native evidence.
- Failures must not publish secrets or weaken existing native cases, artifact/source checks or cleanup ownership.

### Task 1: Deliver installed audit conjunction
**Files:** Spec Context Map.
**Interfaces:** validateHTTPAuditEvidence(receipt,report):void;
runInstalledHTTPAudit({browser,origin,password,externalSignIn,report,api,request,evidence,grants}):Promise<receipt>.
- [ ] Add old-report-without-httpAudit rejection to actual existing validator test;
  run node --test integration/console-evidence.test.mjs; Expected Missing expected exception RED.
- [ ] Add hand-derived valid receipt and negative mutations for all five focus
  classes to integration/http-audit.test.mjs; Expected missing implementation RED.
- [ ] Implement exact strict receipt validator and required console gate; update
  old unit fixture with explicit valid new receipt. Same tests Expected all PASS.
- [ ] Implement real probes/keyset/managed approver sign-in/DOM correlation and
 375/1440 screenshots in focused helper, call after restored confirmation. Pin all
  active Core consumers/fixtures to exact main. No route stubs or seeded records.
- [ ] Run full node --test integration/*.test.mjs (Expected all PASS); Go normal/
  race/vet/build (only inherited UnixgramEPERM local, actual native green mandatory),
  Python regressions and locked CoreWeb ci/typecheck/lint/build; Expected supported
  localgreen, no skipped mandatory native claims. Update bilingual checkpoint/docs,
  commit, task-done command node --test integration/*.test.mjs.
- [ ] ONE fresh fullbranch review; root ONE blocker RED→GREEN pass; ledger/rule
  every declined behavior/cost and defer minors. No second review.
- [ ] Exact source/head/tree/fullnative/raw/fouractualarchives/static/notices/images;
  expectedhead merge; repeat main proof; immediate durable release and next step.

Self-review: one task/no shared interfaces, five focus classes owned here. User
autonomy explicitly waives routine handoff pauses, not verification/review gates.
