# Stopped Agent recovery implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline. One fresh whole-branch review follows.

**Goal:** Execute latest-stopped-checkpoint recovery at original paths.
**Architecture:** Private snapshot helper in existing real systemd/Controller/kernel
probe; sanitized evidence and bilingual runbook, no production dependency changes.
**Tech Stack:** Existing Python stdlib, Node runner, static Go Agent, Linux/systemd.
**Spec:** docs/superpowers/specs/2026-10-20-agent-backup.md

## Global Constraints
- Same boot/namespace/provider/runtime/paths; four state files plus token/unit.
- No live overwrite, old-checkpoint replay, foreign adoption or deadline renewal.
- Preserve every prior native gate; one independent whole-branch review.
- Exact source/raw/four archives/expected-head merge/main verification mandatory.

## Review Focus
- Running/auto-restarted unit prevents capture and restore.
- Links, missing/extra files and broad secret modes fail before copying.
- Tampered backup/changed destinations never restore silently.
- Partial failure cannot publish PASS or lose quarantined originals.
- Restored expired permission stays denied offline and after reconnect.

### Task 1: Complete stopped-checkpoint acceptance
**Files:** scripts/stopped_agent_state.py; scripts/test-controller-service.py;
scripts/test-stopped-agent-state.py; integration/controller-service.test.mjs;
.github/workflows/validation.yml; deploy/{agent,systemd}/README*.md;
docs/validation/AGENT_STATE_RECOVERY*.md; docs/AUTONOMOUS_DELIVERY*.md.
**Interfaces:** capture(state, token, unit, backup, stopped) -> private manifest;
restore(state, token, unit, backup, stopped) -> sanitized receipt. stopped() checks
exact real unit inactive/MainPID0; caller quarantines only its fixture originals.

- [ ] Write real filesystem roundtrip/rejection tests first; run missing-helper RED.
- [ ] Implement bounded helper; pass active/links/modes/extra/missing/tamper/path/
      existing-target tests without changing any rejected originals.
- [ ] Strict public validator rejects old report (RED); extend fixture and real
      systemd flow for36 packet observations/four PIDs, preserving running-offline
      expiry alongside stopped restoration. Same-filesystem quarantine; preserve
      private material after worker/runtime cleanup failure. Add real EXDEV and
      partial-recovery cleanup tests before implementing review corrections.
- [ ] Write bilingual runbook; full Node and Go normal/race/vet/build locally.
      Record inherited Unix EPERM; native full CI supplies required Linux proof.
- [ ] Commit; publish draft using expected remote SHA; full three-job native CI,
      actual four archive/source/hash inspections and one fresh full-branch review.
- [ ] One root regression-first blocker correction pass; final exact-head full CI,
      expected-head merge/main verification, immutable receipt and lease release.
