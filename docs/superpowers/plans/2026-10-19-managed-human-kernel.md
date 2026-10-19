# Managed Human Native Conjunction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline, then one fresh whole-branch reviewer.

**Goal:** Real database console accounts drive actual independent approval and Linux enforcement.
**Architecture:** Extend existing native fixture and fail-closed evidence gate; no runtime change.
**Tech Stack:** Existing Node/Playwright, Go, React, SQLite, gRPC, nftables, Nginx.
**Spec:** docs/superpowers/specs/2026-10-19-managed-human-kernel.md

## Global Constraints
- Pin Core31c7ac762ca0ea7d2d06ee2b30cb511c161cdad1; GPL Agent/Apache Core boundary.
- Preserve29 existing cases,112 kernel/9systemd cases, both address families.
- Explicit database users; no file human fallback; no secrets in public artifacts.
- Account disable/session invalidation is not independent Grant revocation.
- Full native CI, whole-branch review, exact source trees/head merge and main CI.

## Review Focus
- Account audit actor/revision/subject correlation, including coherent substitutions.
- Session-generation proofs must use old actual cookies, not already stale ones.
- Account lifecycle must neither extend original absolute deadlines nor republish.
- Actual provisioning must not silently reuse file users or API-only creation.
- New artifact fields must never include password/hash/token/cookie material.

### Task 1: Managed-account native workflow and evidence

**Files:** Modify integration/{console-kernel,human-requests,console-evidence,console-evidence.test}.mjs and .github/workflows/validation.yml; create integration/managed-humans.mjs; synchronized validation docs/checkpoint.
**Interfaces:** Consume Core GET/POST/PUT accounts, /me and existing runHumanRequests; produce provisionManagedHumans({page,password,evidence,report}), cycleRequester({page,requester,context,origin,report,confirmed,scenario,signIn}) and validateManagedHumans(report)→{requester,approver} IDs.

- [ ] Write independent directory/login/lifecycle fixture with two different32hex IDs; add missing-directory, file-mode, duplicate/mismatched ID, wrong role, disabled provision, wrong account-audit actor/subject/revision, revived-session, changed publication mutations.
- [ ] Run node --test integration/*.test.mjs: new schema mutations FAIL before gate implementation (existing fixture remains meaningful).
- [ ] Implement gate, actual Accounts create/edit/readback and login binding; update immutable pin and key-only runtime. Keep original cases, add humanAccountDisabled/humanAccountReenabled, both allow with unchanged approval6 revision.
- [ ] Run Node suite, syntax/shell checks, full Go normal/race/vet/build; locked Core npm ci/typecheck/lint/build. Expected Node/Web/vet/build green; only known local five Unixgram EPERM in full Go normal/race, mandatory native all green.
- [ ] Commit and create PR with expected source tree identity; inspect all native mandatory steps/raw logs/downloaded artifact digests and375/1440 screenshots. Fix genuine harness defects with observed RED→GREEN.
- [ ] One independent whole-branch review, root one Important/Critical fix pass with tests; record Minor/rulings. Final exact-head CI→merge→main CI and durable release metadata. Full task command: node --test integration/*.test.mjs.
