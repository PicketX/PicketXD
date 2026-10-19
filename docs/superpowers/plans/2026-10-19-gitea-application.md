# Gitea application implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline, then one fresh independent whole-branch review.

**Goal:** Real dual-stack Gitea browser/Git HTTPS/SSH conjunction.
**Architecture:** Extend existing disposable native namespace harness; isolate app
authentication from source/endpoint Grants and ingress HTTP authorization.
**Tech Stack:** Existing Go/React/SQLite/gRPC/nftables/Nginx, Node/Playwright and pinned Gitea binary.
**Spec:** docs/superpowers/specs/2026-10-19-gitea-application.md

## Global Constraints
- Core298863f4e0cb1a72ef18e8abe77f8b8fb28464de; Giteav28.2.0 and exact published SHA256.
- Preserve31 conjunction cases and112kernel/9systemd; no required skip.
- Normal TLS verification, strict SSH host key, app-owned credentials; no secrets in artifacts.
- Existing isolated worktree; no overwrite/force; expected-head merge after native/review.
- UI remains real API-backed with existing design; no production/runtime/kernel changes.

## Review Focus
- Transport denial must not be a dead Gitea/ingress false positive.
- HTTPS and SSH/families must remain independent, including after revocation.
- Real outsider credentials must fail private repo reads while owner succeeds.
- Exact source/application confirmation and full persisted Grant scopes must correlate.
- Artifact serialization and failure logs must not expose app or trust private material.

### Task 1: Real application workflow and fail-closed evidence
**Files:** integration/gitea-application{,.test}.mjs, console-{kernel,evidence}.mjs,
console-evidence.test.mjs, .github/workflows/validation.yml.
**Interfaces:** consumes existing start(name,binary,args,options), publish(policy,reason),
confirmed() and api(path); produces runGiteaApplication(options) and
validateGiteaEvidence(proof, confirmations), stored at report.gitea.
- [ ] Write literal acceptance and corruption tests; run Node and observe missing gate RED.
- [ ] Implement real fixture, independent cases and evidence validation; full Node GREEN.
- [ ] Pin/download/hash actual Gitea and run unchanged complete native CI. Fix observed
  failures with characterization tests; never weaken application/authority checks.
- [ ] Run complete available Go normal/race/vet/build and locked Web checks (native CI
  mandatory when local capability missing); record limitations by name.
- [ ] Commit logical tested changes.

### Task 2: Independent review and integration evidence
**Files:** docs/validation/GITEA_APPLICATION_REVIEW.md, docs/AUTONOMOUS_DELIVERY.md.
**Interfaces:** consumes final code, actual CI/source/archive receipts from Task1;
produces durable acceptance and explicit lease release in final PR metadata.
- [ ] Fresh most-capable whole-branch review; one Critical/Important TDD fix pass,
  defer Minors. No re-review.
- [ ] Recheck final tree/head/full CI/raw evidence and downloaded SHA256 artifacts.
- [ ] Merge exact expected head; recheck merge parents/tree/main full native CI and archives.
- [ ] Release lease and record remaining deployment/release work. Do not pause automation.

中文：自主按计划内联执行；先失败后通过，再独立全分支评审及准确head合并。
