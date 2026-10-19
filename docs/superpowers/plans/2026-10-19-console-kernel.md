# Console/kernel acceptance implementation plan

Spec: docs/superpowers/specs/2026-10-19-console-kernel.md
Strategy: inline execution, then one independent whole-branch review.
Global constraints: exact pinned Core source; real Linux CI; no skips; isolated
existing worktree; expected-head remote writes; no production or license changes.

## Context map (reviewed before implementation)
- AGENTS.md, docs/AUTONOMOUS_DELIVERY.md: boundaries and handoff.
- internal/hostagent/remote_process_integration_test.go: actual Core/Agent precedent.
- internal/hostagent/process_integration_test.go: local scope and probe precedent.
- internal/hostagent/run_linux.go: desired/applied generation/hash status contract.
- scripts/test-kernel.sh, .github/workflows/validation.yml: mandatory native gates.
- Core apps/web/e2e/console.mjs, ingress.mjs: real session/Grant UI and ingress trust.
- Core examples/http/nginx.conf.template: trusted auth_request header construction.
- New integration/console-{evidence,kernel}.mjs and evidence tests: conjunction harness.
- New scripts/test-console-kernel.sh: explicit opt-in, builds, namespace boundary.

## Task 1: fail-closed evidence contract
Produces: validateEvidence(report), requiredCases; consumed by Task 2.
1. Write Node tests rejecting missing/duplicate/failed/skipped cases, bad source
   identity, mismatched application confirmation and missing dual-stack observations.
2. Run node --test integration/console-evidence.test.mjs. Expected: RED missing module.
3. Implement strict validator, run same command. Expected: all tests pass.
4. Commit test(e2e): add fail-closed combined evidence contract.
Completion: same Node test command, all tests pass.

## Task 2: native browser/Controller/Agent/Nginx acceptance
Consumes Task 1 evidence contract. Produces CI artifacts and bilingual acceptance docs.
1. Implement actual subprocesses, private random credentials, dual-stack real probes,
   browser publication/Grant controls, confirmation waits and all spec scenarios.
2. Add explicit-opt-in build/namespace script and separately pinned Core CI checkout.
3. Run syntax checks, evidence tests, full Agent Go test/race/vet/build and locked
   Core Web typecheck/lint/build. Local Unix socket restrictions must be reported.
4. Run full Linux workflow; inspect raw logs and artifacts. Expected: all original
   unit/native/systemd gates and the new conjunction job pass without skips.
5. Record exact trees, heads, CI and remaining scope; commit focused changes.
Completion: full Linux workflow successful, exact-head evidence and review.

## Review focus
False-positive denial due to unhealthy services; established vs fresh sockets;
IPv6 source and host confusion; stale Agent acknowledgement; deadline extension;
cleanup outside the namespace; credential leakage in artifacts; CI source mismatch;
evidence gate accepting absent/duplicate/skipped cases; license boundary.

## Integration
Independent full-branch review; fix blockers RED→GREEN; exact-head merge only after
all required CI succeeds; inspect main CI and release lease with durable handoff.
