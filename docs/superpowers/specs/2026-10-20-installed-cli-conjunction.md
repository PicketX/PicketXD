# Installed CLI to native enforcement acceptance

Intent: finish the new operator status/doctor workflow against actual installed
Controller/Agent/ingress/Gitea, not only protocol fixtures. Existing native OIDC,
password, kernel, service, outage and recovery work is delivered and retained.
Entry Core81ec5bc8a483010f99672f7ff5d982ad4ee6698f/tree1b04ceb9d86f4d72c30e12c540097aafd915b2d2/
CI39405913826 and Agentb14b54d40b2e1c20af0d12735722202d494580dd/tree5b243935/CI39388341832
alljobs/steps green, no openPR; Core43 released10:17:59Z and Agent28 released07:53Z.
Existing clean isolated worktree/new exact-main branch, old branches/work preserved.
User autonomy waives routine spec/plan/execution/push/verified-merge handoffs only.

## Chosen approach and alternatives
Extend the existing shared native runner and strict public receipt (recommended):
same actual binaries/cases, one small CLI adapter and independent receipt validator.
Replacing either password/OIDC pass loses coverage; duplicating the runner drifts.
No product Go/kernel/API/UI/schema/dependency/licensing-boundary change. No generic
CLI framework, new deployment authority, privileged helper or topology expansion.

## Contract
Pin all active Core consumers to entry81ec5bc/tree1b04ceb9. Add
integration/cli-diagnostics.mjs with inspectInstalledCLI(installed,origin,expected)
async -> {phase,revision,status,doctor,statusExit,doctorExit,...correlation} and
validateCLIDiagnostics(proof,deployment,report,nativeEvidence). Fixed errors; never
include raw subprocess error/stdout/stderr/private paths or keys in thrown errors.
Execute shipped cliBinary with existing private bootstrap-key, exact --agent node,
--format json/--timeout5s; inherited configured normal trust, empty PATH, 10s child
deadline and bounded64KiB capture. No shell, redirect/TLS bypass or secret argument.
Only observed exits0/1 accepted, no signal/timeout/launch failure called degradation.
Actual CLI reports preserved as bounded public objects; exact keys/no private extras.

Six ordered phases in BOTH old password and real verified-HTTPS OIDC passes:
before-agent disconnected; applied reported-applied; grants-applied reported-applied;
agent-stopped disconnected; controller-restored disconnected; agent-restarted
reported-applied. Status0 each valid read; doctor0 only reported-applied, otherwise1.
Every report: scope controller-observations, exact initialized authority, exact
published decimal revision, valid observation timestamp, one exact node, matching
desired revision; enforcement_confirmed=false ALWAYS. Doctor ready=true because
the actual Controller remains ready. Healthy reports require positive boundedTTL
and exact reported revision; disconnectedTTL0, no requirement to invent a report.

Native runner captures actual Controller APPLIED receipt and raw Agent status index
for each healthy phase. Validator independently correlates exact revision, typed
snapshotSHA and actual applied Agent policySHA (different hash domains), actual
confirmation and raw status index. Restart phase must use a post-restart index.
Disconnected phases cannot carry a fabricated enforcement correlation. Also retain
the independent dual-stack packet observations; CLI is NOT kernel verification.
Whole-proof source/cliSHA/runtimePATH match the exact installed bundle. Require
new diagnostics via deployment.diagnostics inside existing reports, no new artifact
names, no removal/loosening of old31+7cases/23confirmations/112kernel/9systemd/
ControllerAgent4process36observations/six-file stopped-state proof.

## Context Map
| File | Purpose/change |
| --- | --- |
| integration/cli-diagnostics.mjs (new) | Bounded actual CLI subprocess adapter and strict correlation validator |
| integration/cli-diagnostics.test.mjs (new) | Missing/forged/stale/private reports, wrong exits/process failures; unit-only data |
| integration/console-kernel.mjs | Six actual lifecycle calls/shared public proof in both modes |
| integration/controller-install.mjs | Core pin and required diagnostics gate |
| integration/console-evidence.test.mjs, controller-install.test.mjs | Updated exact source and complete unit fixtures, missing diagnostics rejected |
| integration/oidc-identity.mjs and test | Same new exact Core source, identity semantics unchanged |
| scripts/test-controller-service.py, .github/workflows/validation.yml | Same exact Core pin, every existing native gate retained |
| docs/AUTONOMOUS_DELIVERY*.md, docs/validation/INSTALLED_CLI*.md | Short lease/exact receipts/review/rulings/completion audit |
Dependencies: existing installed proof/config/private key, console confirmed(),
latestControllerReceipt, status/restart fences, report confirmations and strict
validateInstalledEvidence. Reference installed-ingress.mjs and CoreCLI diagnostics.
Tests exercise validator behavior, not grep/source snapshots. Unit process fixtures
are explicitly not native proof; actual shipped binaries/native CI are mandatory.
Risk: additive test-evidence interface only; no public productAPI/DB/config change.

Self-review: single coherent deliverable, no placeholders or shared-task interfaces;
six-phase exits/correlation/privacy/source bounds pinned, actual proof distinct.
