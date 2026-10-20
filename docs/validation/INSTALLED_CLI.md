# Installed CLI lifecycle conjunction

PR29 extends the existing installed native acceptance without product changes.
Core is pinned to 81ec5bc8a483010f99672f7ff5d982ad4ee6698f, source tree
1b04ceb9d86f4d72c30e12c540097aafd915b2d2. Both password and real verified-HTTPS
OIDC passes execute the shipped empty-PATH CLI with its private management key.

The public report now requires six ordered `deployment.diagnostics.cases`:
before-agent, applied, grants-applied, agent-stopped, controller-restored,
agent-restarted. Every status exits 0; doctor exits 0 for fresh matching APPLIED
assertions and 1 for disconnected Agents while the Controller remains ready.
The CLI always says `enforcement_confirmed:false`. Actual independent dual-stack
packet outcomes, raw Agent status, typed Controller receipt, and post-restart
process fence remain mandatory. Typed snapshot hashes and projected policy hashes
are separate domains. Diagnostics cannot replace prior native evidence.

Adapter limits: actual executable/no shell, private key file, normal trust,
CLI timeout5s, child deadline10s, output64KiB. Signal/launch/timeout/overflow,
invalid JSON/fields and unexpected exit are fixed private-safe failures, not
valid degraded diagnostics. No secrets, paths, raw errors or stderr are published.

## Development evidence (not native acceptance)

Missing module regression failed before implementation. The old-report rejection
failed with `Missing expected exception` before the installed gate required
diagnostics; both targeted suites then passed. Unit fixtures are explicitly not
native proof. Real child-process fixtures exercise 0/1/2 exits, launch, signal,
malformed output, overflow and an actual10s timeout. Native CI executes the actual
static Core binary against the real deployed Controller, twice.

Full local Node suite:303 passed at10:46Z before one additional healthy-child
boundary test; final counts are recorded with the immutable review/CI receipt.
Python stopped-state filesystem tests12 passed; Go vet/build passed. Full local
ordinary/race runs each fail only the five pre-existing TestNotifyEmitter Unixgram
cases with `operation not permitted`; no skipped test is counted as acceptance.
Locked CoreWeb npm ci/typecheck/lint/build passed. Mandatory native CI, exact-head
review, four actual archives and post-merge main verification are still pending.

## Scope rulings

No UI/product API/kernel/config/identity/dependency change is needed to connect
this existing operator CLI. Consequently no new UI design or additional runtime
dependency is introduced. Existing browser, Keycloak, Gitea, systemd, LKG recovery,
GPL source/offline build/notices and112 kernel cases are retained without waiver.
This documentation is not a final overall-completion or native-acceptance claim.
