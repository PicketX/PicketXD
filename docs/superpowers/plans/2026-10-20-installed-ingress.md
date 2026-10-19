# Installed ingress implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline. One fresh whole-branch review follows.

**Goal:** Consume shipped private ingress includes in installed joint acceptance.
**Architecture:** GPL acceptance adapter calls separate Apache Core executable;
existing Nginx fixture owns forwarding and TLS. No new product dependencies.
**Tech Stack:** Existing Node/native Nginx, static Go Controller/Agent, Linux CI.
**Spec:** docs/superpowers/specs/2026-10-20-installed-ingress.md

## Global Constraints
- Keep exact UID/caller scopes, privacy and every old native gate; no source template.
- Full raw/four-archive/source/expected-head/main evidence, no simulated execution.
- One independent whole-branch review, root ONE regression-first blocker pass.

## Review Focus
- Wrong location/server include context, route or callback collision.
- Private negative route accidentally enrolled or granted.
- Paths, errors, public report or upstream headers leaking credentials.
- TLS/health/keepalive/Host forwarding drift and removed existing native cases.
- Source pins, CLI identity and receipt hashes inconsistent with actual archives.

### Task 1: Installed generated ingress integration
**Files:** integration/installed-ingress.mjs; integration/controller-install.mjs;
integration/console-kernel.mjs; integration/gitea-application.mjs; integration/*.test.mjs;
scripts/test-controller-service.py; .github/workflows/validation.yml;
deploy/controller/README*.md; docs/AUTONOMOUS_DELIVERY*.md; docs/validation/INSTALLED_INGRESS.md.
**Interfaces:** prepareInstalledIngress(installed) -> { primary, privateRoute, gitea,
proof }; each private route exposes location/server include paths; proof has only
public hashes/modes/UIDs. Parent attaches two actual server config hashes.

- [ ] Add missing-ingress receipt test. Run Node test; Expected: missing rejection
      assertion fails against old validator. Add literal valid proof/mutations.
- [ ] Probe old CLI `nginx-init`; Expected: unsupported command. Implement adapter
      and strict validation; probe actual new bundle and run all Node tests.
      Expected: private generated files, new proof accepted, all malformed rejected.
- [ ] Replace two source-template consumers with generated includes; pin exact new
      Core source and update bilingual operations/checkpoint. Expected: all existing
      real cases unchanged and native report requires installed generation proof.
- [ ] Full local Node/Go normal/race/vet/build; Expected: Node green, inherited
      Unixgram failures recorded exactly; real Linux CI must be fully green.
- [ ] Commit/publish draft with short lease; one independent entire-branch review
      and regression-first root blocker pass; full native/raw/four archive checks.
- [ ] Expected-head merge and main re-verification; record receipt/release.
      Expected: exact tree/head, all jobs/steps and old/new actual evidence pass.
