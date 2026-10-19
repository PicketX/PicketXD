# Installed Controller conjunction implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline, then one fresh whole-branch review.

**Goal:** Actual relocated Core installation drives full native conjunction and clean process recovery.
**Architecture:** Verify/init the static bundle, reuse real browser/kernel/Gitea
workflow and require correlated deployment/restart evidence.
**Tech Stack:** existing Go/Cobra/gRPC/Node/Playwright/Nginx/Gitea, no new dependency.
**Spec:** docs/superpowers/specs/2026-10-19-installed-controller.md

## Global Constraints
- Core93024b229c072ce1db8512323c3d7fa35e3df74e/tree64ed0dcc4b6a3232e093150d7fc100cc9a852e43.
- Agent base233f081; no kernel/protocol/UI change; preserve every previous gate.
- Generated private config/authority; explicit narrow caller registry edit only.
- Controller/Agent runtime empty PATH/absolute relocated paths/unrelated cwd.
- Process restart only; no host boot/systemd or Agent distribution claim.

## Review Focus
- Manifest traversal/duplicates/symlinks/missing/digests fail before execution.
- Raw keys never enter public evidence/logs/exceptions.
- Bootstrap audit identity stays generated; approval is not confirmation.
- Restart cannot reuse status or change DB/Grants/deadlines/provider.
- No checkout assets/binary or inherited runtime PATH substitution.

### Task 1: Deployment adapter and evidence gate
**Files:** integration/controller-install{,.test}.mjs, console-evidence{,.test}.mjs,
managed-humans.mjs and identity fixtures.
**Interfaces:** verifyControllerBundle(path) returns verified public source/digests;
initializeInstalledController(bundle,temp) returns private paths/keys/config only
to driver; validateInstalledEvidence(proof,report,nativeEvidence) gates public proof
against actual complete Agent history and restarted-process stdout.
- [x] Add missing/tampered proof rejection tests; node --test integration/*.test.mjs
  expected RED because old validator accepts invalid/missing deployment.
- [x] Add filesystem manifest unsafe/symlink/digest tests and implement minimal
  adapter/strict proof; full Node suite expected GREEN.
- [x] Commit tested adapter and limitations.

### Task 2: Native installed workflow and recovery
**Files:** integration/console-kernel.mjs, scripts/test-console-kernel.sh,
.github/workflows/validation.yml, deploy/controller/README{,.zh-CN}.md, checkpoints.
**Interfaces:** consume Task1 adapter/gate; produce real Linux report/source/binary
identities with all prior cases and fresh post-restart confirmation.
- [x] Wire required deployment gate first; native missing-proof RED required.
- [x] Build/tar/re-extract actual bundle, relocate/init, empty-PATH runtime; retain
  every prior human/Gitea/kernel/systemd scene.
- [x] Restart services with active dual-stack Grants, compare saved business views,
  retain raw history/fence restart cursor, require fresh actual receipt, reject old
  cookie and reauthenticate. Exclude only two endpoint-specific read clocks.
- [x] Full Go/race/vet/build + locked Web and mandatory native gates; inspect raw
  checkouts/reports/downloaded hashes/screenshots.
- [ ] One fresh independent full-branch review; one root Critical/Important TDD fix
  pass, defer Minors; final exact-head CI/merge/full main/source/archive verification.
  Review and fixes complete; final immutable integration gate recorded in PR22.

Final immutable PR/commit/CI/archive evidence and immediate lease release in PR
metadata. Preserve host worktree/branches; no automatic whole-goal completion claim.
中文：先失败后通过，实际安装输出接原生链路，再独立评审及准确head主线验收。
