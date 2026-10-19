# Latest stopped Agent checkpoint recovery

> English (default) | [简体中文](AGENT_STATE_RECOVERY.zh-CN.md)

Only the **latest stopped checkpoint, same machine, boot, network namespace and
original paths**, with a compatible verified binary. NOT old-backup rollback,
reboot recovery or migration. Root controls files; hashes detect copy mistakes,
not malicious writers or stale checkpoints. If newer publication/revocation has
occurred, do not restore an old snapshot: keep applications closed and resolve
authority/ownership recovery separately. Never delete journals to force adoption.

## Exclusive maintenance

Inspect your actual installed Controller-mode unit's full ExecStart and drop-ins.
Preserve provider authority, Agent/profile, dual-stack destinations, reserved mark
range, handling, transport/server name/CA and credential path. Keep the compatible
verified distribution separately: old binaries may reject new state formats.
No other operator/automation may start/update the unit, publish Controller policy
or start protected applications during maintenance. Preserve a safe management
route and record current revision/hash and each original absolute Grant deadline.
Stop retains filtering; graceful stop does not terminate existing connections.

## Complete private capture

Default paths here are examples: substitute only your reviewed actual paths.
Use a trusted root shell, umask077 and a NEW private backup directory. Stop the
exact installed `picketxd-controller.service` (or actual owner unit); verify via
`systemctl show` both ActiveState=inactive and MainPID=0. Failed state, old journal
lines or successful signal delivery are insufficient. Keep it stopped throughout.

Use `cp -a` to copy the WHOLE `/var/lib/picketx`, currently owner.json, desired.json
and both .lock files. Do not cherry-pick JSON or omit owner/mark/application history.
Reject missing/unknown entries, links, wrong ownership or group/other permissions.
Retain root ownership, directory0700 and files0600. Do NOT delete/restore shared
`/run/picketx`: runtime-scoped locks are not portable backup data.

Copy the original Agent Token (0600), installed unit and YOUR unit's drop-ins,
EnvironmentFiles, credentials and CA. Custom deployments can require MORE than
the fixture's six files. Record original paths/owner/modes and compare every
source/copy hash in a PRIVATE manifest while stopped. No backup/manifest/token/
cache/journal in public bundles, CI artifacts, logs or Git. Use access-controlled,
encrypted storage as appropriate; no encryption/key management is supplied here.
Successful copies/byte comparison do not prove power-loss durability.

## Original-path restore

Only this latest checkpoint, with no intervening publication or Agent run. Verify
exact unit still inactive/MainPID0. Validate complete backup hashes/modes/owners/
path metadata BEFORE replacement. Move stopped originals into a separate private
quarantine, retaining recovery copies; never overwrite them. Copy complete state,
credentials and unit configuration to ABSENT ORIGINAL paths, preserving owner/mode
and using independent copies, not links. Abort on existing targets, unknown formats
or identity mismatch. Never edit provider/runtime/boot/namespace/owner/marks/
generation/expiry to bypass checks. Verify every restored file, then daemon-reload;
this must not start the Agent. On failure keep applications closed and retain both
quarantine and backup until recovery is confirmed.

Start exact unit with `systemctl start --no-block`. Inspect fresh JSON from its NEW
MainPID, and actual IPv4/IPv6 TCP/UDP outcomes with independently healthy protected
listeners. Offline LKG can retain still-valid permissions but cannot renew absolute
deadlines; new flows deny after original expiry even while Agent remains stopped.
Offline startup is activating, not current READY. Reconnect original authenticated
Controller; require fresh matching revision/hash APPLIED, matches_desired,
connected/fresh observation, Agent enforcement confirmation and no apply error.
Expired access stays denied. Approval/publication/active/cached APPLIED alone does
not prove application. Foreign table/drift, changed boot/namespace/config/paths,
newer authority state or missing safe checkpoint is a STOP, never permission to
flush firewall/conntrack or delete state.

## Executable proof

Required native `scripts/test-controller-service.py` uses shipped static Agent/Core,
sandboxed notify unit and disposable namespace. It captures all four state files,
token/unit while stopped, quarantines originals, restores independent byte/mode/
owner-identical copies at original paths, then observes32 real dual-stack TCP/UDP
cases across startup/crash/outage/stop/expiry/offline start/reconnect. Only sanitized
six-file count/booleans/times are public; no private backup/manifest is archived.
Public PASS follows ALL assertions and owned cleanup. Exact source/CI/archive/main
acceptance belongs in each PR final receipt; this is not a host-boot proof.
