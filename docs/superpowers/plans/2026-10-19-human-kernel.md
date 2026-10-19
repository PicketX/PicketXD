# Human approval / kernel conjunction implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the accepted human workflow through actual dual-stack enforcement.
**Architecture:** Extend the existing namespace fixture with a focused human-session
browser helper and strict provenance gate. Retain every original case.
**Tech Stack:** Existing Go/React/Playwright/Node/Nginx/Linux/SQLite/gRPC.
**Spec:** docs/superpowers/specs/2026-10-19-human-kernel.md

## Global Constraints
Exact Core b79e242926c67d931537c160333008df699fa598; no runtime/dependency changes;
no skips, mock evidence or authority weakening; expected-head writes; native gates.
Inline execution under standing autonomous authorization, one independent review.

## Context Map (reviewed before implementation)
| Files | Purpose / changes |
| --- | --- |
| integration/console-{kernel,evidence}.mjs | Existing process/probe/confirmation contracts; extend cases and gate |
| integration/console-evidence.test.mjs | Reject absent, malformed or inconsistent human proof |
| integration/human-requests.mjs (new) | Actual human browser create/approve/reject/revoke/deadline helper |
| scripts/test-console-kernel.sh | Build actual password-hash CLI before elevation |
| .github/workflows/validation.yml | Pin accepted Core version |
| docs/validation/M2D2{,.zh-CN}.md | Bilingual reproduction/limits |
| docs/AUTONOMOUS_DELIVERY.md | Lease, exact evidence, handoff |

Dependencies/reference patterns: Core apps/web/e2e/access-requests.mjs and
human-login.mjs define real UI/API behavior; Core Requests.tsx defines labels and
ETag/draft ownership. Existing console-kernel confirmed()/scenario() own actual
Agent generation/hash and app-hit checks. No API/migration/configuration changes.

## Review Focus
Unhealthy services mistaken for denial; stale unrelated Agent confirmation;
self-approval or source/requester confusion; fixed deadlines silently extended;
credential leaks or namespace cleanup escaping fixture ownership.

### Task 1: Complete human approval conjunction
**Interfaces:** consumes existing scenario(name, options), confirmed(), api(path),
browser/process fixture. Produces runHumanRequests(context) plus report.humanRequests
consumed by validateEvidence(report). No production exports.

- [ ] Add evidence tests rejecting missing proof, self approval, changed sources,
  broadened endpoints, extended expiry, wrong Grant or unconfirmed revision.
- [ ] Run node --test integration/console-evidence.test.mjs. Expected RED because
  old gate accepts invalid human proof; record exact failures.
- [ ] Implement strict human gate and actual browser workflow; pin Core/build CLI.
- [ ] Run Node tests/syntax, full Go test/race/vet/build and locked Core Web checks.
  Expected Node/Web/vet/build pass; report known local five Unixgram EPERM failures.
- [ ] Publish draft, run complete native workflow, inspect raw logs/artifact bytes
  and screenshots. Expected all old/new cases and native/unit/systemd gates pass.
- [ ] Commit, independent whole-branch review, one TDD blocker fix pass if needed.
- [ ] Final exact-head CI/tree, safe merge, main CI and durable lease release.

Completion command: node --test integration/console-evidence.test.mjs; full native
CI and immutable tree/artifact verification remain additional mandatory gates.
