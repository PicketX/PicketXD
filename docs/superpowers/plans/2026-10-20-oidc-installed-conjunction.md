# Installed OIDC Conjunction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline. Steps use checkbox syntax.

**Goal:** Actual real-OIDC human identities drive installed dual-stack enforcement/application workflows.
**Architecture:** Keep old native pass; private owned real provider wrapper runs the
same whole native suite again with HTTPS console and explicit OIDC identity hook.
**Tech Stack:** Existing Go/static bundles/Node/Playwright/Nginx/Gitea; CI-only pinned Keycloak26.8.1/JDK21.
**Spec:** docs/superpowers/specs/2026-10-20-oidc-installed-conjunction.md

## Global Constraints
- Core64c8cb8e6c92669873031ca45cff47dbb38d0224/tree4637c029a34c71ec04b66fcc205c288f1280dfb9.
- Keycloak26.8.1/SHA2564e2740b181faa2ea623a1c2ecf00855935311e546046e2f857615a47ed24cbe8.
- ConsoleHTTPS127.0.0.1:18081/backend18080; providerHTTPS127.0.0.2:19460; normal TLS.
- Provider private tree separate from Gitea0711 tree; stop then cleanup owned state/NSS.
- Existing default/password pass, every native/kernel/service gate and four artifact names retained.
- Explicit subject/local account mapping; no password fallback/claim roles/new network permission.
- No product/kernel/API/UI/dependency or licensing-boundary change.

## Review Focus
- Provider state/log/credential confidentiality on startup or child failure, including cleanup.
- A previous API-key/local-password session must not masquerade as OIDC success.
- HTTPS trust must not be disabled for Node, Playwright API clients or installed Controller.
- Public identity evidence must correlate independent local actors with actual approved/applied revisions.
- New evidence must not overwrite or weaken old password/kernel/Gitea/service proofs.

### Task 1: Complete installed OIDC conjunction
**Files:** Spec context map (one integrated deliverable, no shared-task interfaces).
**Interfaces:** initializeInstalledController(bundle,temp,{browserOrigin}={});
runHumanRequests({...existing,externalSignIn?});
configureOidcIdentity(installed,privateConfig) -> {bindAccounts(report),signIn(page,user,report)};
validateOidcEvidence(receipt,report) -> throws on incomplete/widened/private/stale data;
oidc-conjunction.mjs CORE KEYCLOAK_SOURCE -> nested native evidence +OIDC receipt.

- [ ] Write tests before helper: exact pinned provider/current source, distinct local
  identities, required observed OIDC logins, applied approval/Gitea audit correlation,
  false TLS/cleanup/bypass/privateextra rejected; actual private config/binding modes.
  Run node --test integration/oidc-identity.test.mjs; Expected missing-helper RED.
- [ ] Implement minimum strict helper and run GREEN; no unit fixture called native proof.
- [ ] Integrate private real-provider/HTTPS wrapper, before-start Node CA trust, exact
  installed browser origin/config/bindings, OIDC hook and human Gitea audit correlation.
  Retain all defaults and complete old evidence; fixed-phase failures/private cleanup.
- [ ] Update exact Core pins in all active consumers/tests, add mandatory CI second
  pass with checksum/JDK21, keep old3jobs/allsteps/fourartifacts. Sync EN/ZH docs.
- [ ] Full Node/Go normal/race/vet/build, locked Webci/typecheck/lint/build; local
  inherited UnixgramEPERM not green. Fresh native exacthead all3jobs/steps/no skip,
  all112kernelcases/old+newconjunction/service/outage/recovery required.
- [ ] ONE fresh whole-branch independent review; root ONE blockerfix RED→GREEN,
  defer minors, persist every declined-scope ruling/cost. No fake platform approval.
- [ ] Final source/raw/fouractualarchives/manifests/static/VCS/notices/images;
  expectedheadmerge/freshmain repeatproof/immediatePRlease release. Reassess whole
  goal against observed acceptance, not passing fixture booleans alone.

Self-review: spec/topology/interfaces and five review-focus conditions map to tests
or mandatory actual native assertions; single task justifies one final reviewer.
Final integration is separate from the task implementation completion contract.
