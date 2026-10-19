# Agent distribution implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline; one fresh whole-branch review at the end.

**Goal:** Relocatable source-backed static Agent bundle used by real native workflow.
**Architecture:** Clean Git source export + official Go vendor + actual static binary;
strict file manifest and offline rebuild acceptance before existing native workflow.
**Tech Stack:** existing Go1.27.0/Bash/Node, no product dependency or runtime change.
**Spec:** docs/superpowers/specs/2026-10-20-agent-distribution.md

## Global Constraints
- Base mainc02ab9600a3be2d64c4a987189726aa4e18040ec/tree73d8b4b5b6249c09e4c2a1c1bc879ed83d635499.
- Core93024b229c072ce1db8512323c3d7fa35e3df74e unchanged; GPL/Apache boundary retained.
- Existing31+7/23actual/112kernel/91required/9systemd and full Go/Web required.
- Runtime requires no Go/npm/shared library; offline rebuild needs supported Go.
- No legal clearance, bit-identical rebuild, host boot or backup/restore claim.

## Review Focus
- Output symlink/noncanonical path or dirty source cannot overwrite operator data.
- Complete manifest rejects unlisted/symlink/special/missing/tampered content.
- Source export/vendor/notices must correspond to actual clean binary identity.
- Offline rebuild must not use host module cache/network or auto-fetch toolchain.
- Native workflow must execute shipped binary, not silently rebuild a replacement.

## Context Map (reviewed inline before implementation)
| Files | Responsibility |
|---|---|
| scripts/build-agent-bundle.sh | exclusive output/preflight/static build/source+vendor |
| scripts/agent-bundle.mjs | upstream notice retention, manifest seal/verify |
| integration/agent-bundle.test.mjs | real filesystem/preflight integrity failures |
| scripts/test-agent-bundle.sh | actual tar/relocation/offline rebuild/CLI proof |
| scripts/test-console-kernel.sh | consumes verified shipped binary, existing driver unchanged |
| .github/workflows/validation.yml | retains3jobs, uploads additional Agent artifact |
| deploy/agent/README{,.zh-CN}.md | build/runtime/recovery limitations |
| docs/AUTONOMOUS_DELIVERY{,.zh-CN}.md | receipt/lease/next steps |
References: Core build-controller-bundle/collect-bundle-licenses; Agent test-prepare
and controller-install verifier. Consumers: console-kernel.mjs existing binary path;
no protocol, DB migration, management/API/UI or kernel configuration changes.

### Task 1: Source-backed bundle and integrity gate
**Files:** build-agent-bundle.sh, agent-bundle.mjs, agent-bundle.test.mjs, deploy/agent docs.
**Interfaces:** build-agent-bundle.sh NEW_ABSOLUTE_DIRECTORY; verifyAgentBundle(dir)
returns {source,manifestSHA256,binarySHA256,files}. source={commit,tree,platform,go}.
- [ ] Write real existing-file/dir/dangling-symlink/noncanonical/dirty source guard
  tests, and manifest mutation tests. Run node --test integration/agent-bundle.test.mjs;
  expect RED for missing builder/verifier, never a simulated successful artifact.
- [ ] Implement minimal strict preflight/build/notice/export/vendor/manifest behavior.
- [ ] Run full node --test integration/*.test.mjs; expect all pass/no skip. Commit.

### Task 2: Offline and native shipped-artifact acceptance
**Files:** test-agent-bundle.sh, test-console-kernel.sh, workflow, checkpoints.
**Interfaces:** test-agent-bundle.sh NEW_ABSOLUTE_DIRECTORY produces payload/, Agent
tar+sha and proof.json; test-console-kernel consumes payload/bin/picketxd verbatim.
- [ ] Write actual bundle/offline/empty-PATH test; run against missing implementation
  and observe RED, then implement and run GREEN from clean committed source.
- [ ] Integrate verified tar binary into existing native runner and upload artifact.
- [ ] Run full local Go/race/vet/build (report known Unixgram restriction), full Node;
  required real Linux Go/Web/kernel/native/offline gates must pass without skip.
- [ ] One fresh independent whole-branch review; fix Important/Critical with observed
  RED→GREEN in one pass, record deferred Minors/rulings/costs; exact-tree final CI,
  expected-head merge and full main proof. Final immutable receipt/release in PR.

Spec/plan self-reviewed, all5 Review Focus failures map to tests/actual native gate.
Host-owned worktree/branches preserved; recoverably archive only this plan's scratch.
