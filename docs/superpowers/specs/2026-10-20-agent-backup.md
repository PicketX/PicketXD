# Stopped Agent original-path recovery

## Intent and scope
Execute an honest operational recovery procedure for the shipped Controller-mode
Agent. Extend PR24's private native systemd probe; no production Go, schema,
ownership or kernel change. Routine design/plan pauses are waived by the explicit
autonomous mandate. Compared alternatives: general privileged backup CLI is
premature; prose alone has no execution proof. Select private probe plus runbook.

## Contract
Verify exact Agent unit inactive/MainPID0 before capture/restore. Snapshot all four
retained state files, exact token and rendered unit in a private directory. Preserve
content/modes/owner and original path binding. Reject running unit, missing/extra
state, links, broad secret modes, tampered backup, changed paths or live targets.
Quarantine only this fixture's originals, restore to absent original paths, verify
identical independent copies. Never manipulate nft or conntrack to force recovery.

Keep boot/namespace, authority, Agent/profile/marks/destinations and transport/token
path unchanged. After Controller outage, stop Agent and capture while Grant remains
valid; restore exact paths and let original deadline expire. Actual IPv4/IPv6 TCP/UDP
must remain blocked while Agent stopped, on offline start (activating, not READY),
and after current APPLIED reconnect. No new revision/cache mutation/renewal.
Retain all existing cases: after restore, a fresh offline Agent remains RUNNING
across the original deadline; then stop and start again expired. Four distinct
PIDs and36 packet observations bind phase-specific live/stopped PID proof. Only
sanitized six-file count/booleans/times are public;
no token/cache/journal/backup or private manifest in artifacts.

Quarantine state in its own filesystem, not across /var/lib to /run. On worker,
timeout or runtime-cleanup failure, stop/reap/unregister owned runtime resources
but retain all private backup/quarantine/partial state for root inspection. Remove
only exact owned private paths after successful recovery and runtime cleanup.

## Boundaries
Latest stopped checkpoint, same boot/namespace only. Old root-controlled snapshots
have no external anti-rollback witness: never replay after newer policy/revocation.
Foreign table, unknown/newer state, machine/namespace migration and host reboot
require a separate recovery design, not deleting journals/relaxing checks. Graceful
mode proves new-flow behavior, not existing-session teardown. Overall completion
still requires packaged ingress/application operational walkthrough.

## Context map (self-reviewed)
| File | Purpose/change |
|---|---|
| scripts/stopped_agent_state.py | Private snapshot/restore helper, not product CLI |
| scripts/test-controller-service.py | Real stop/copy/quarantine/restore/expiry and strict public proof |
| integration/controller-service.test.mjs | Runs helper/evidence regressions in full Node suite |
| docs/validation/AGENT_STATE_RECOVERY*.md | Bilingual operational procedure and boundaries |
| docs/AUTONOMOUS_DELIVERY*.md | Lease/progress/next work |

Dependencies/reference patterns: unchanged hostowner journal and agentstate cache
readers, provider/runtime/boot/namespace checks, shipped systemd unit/native CI.
Tests: Python self-tests/202 Node, remote process/owner drift native tests and full
kernel gate. No public API, migration, configuration schema or stack change.
