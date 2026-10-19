# Host policy services

> English (default) | [简体中文](README.zh-CN.md)

Operator-reviewed system services for existing `picketxd enforce`. The demonstrations protect loopback TCP 18443 / UDP 18444, both IP families, graceful mode, in the host network namespace. See [enforcement](../../examples/enforce/README.md), [strict limits](../../examples/strict/README.md) and [Controller pairing](../controller/README.md). These units do not establish a boot-time firewall or complete deployment acceptance.

## Select exactly one source

`picketxd.service` reads the static policy file. `picketxd-controller.service`
uses authenticated official gRPC with a private Agent Token file and no policy
file. Both share `/var/lib/picketx` and `/run/picketx`; **never enable both**.
They are operator-reviewed examples, not automatic enrollment or a source-mode
migration. Do not delete retained state or change its provider to force a switch.

For a fresh Controller deployment, first follow the pairing guide. Install the
verified Agent binary as below, but use the separate unit instead of the static
policy/unit commands:

```sh
sudo install -d -m 0700 /etc/picketx
# Copy only the generated Agent credential, never the bootstrap/ingress key.
sudo install -m 0600 /private/controller/agent-token /etc/picketx/agent-token
sudo install -m 0644 deploy/systemd/picketxd-controller.service /etc/systemd/system/picketxd-controller.service
sudoedit /etc/systemd/system/picketxd-controller.service
```

Replace `controller:REPLACE_WITH_GENERATED_AUTHORITY` with the exact public
`agent.authority` from the initialized Controller; review Agent ref, state path,
destinations, exclusive mark range and handling against actual managed facts and
Binding. The shipped plaintext transport is for trusted same-host loopback only.
For a normally verified TLS listener, remove `--controller-plaintext`, set the
actual address and mandatory `--controller-server-name`; optionally use
`--controller-ca /etc/picketx/controller-ca.pem` for custom roots. Never disable certificate checks.
No mTLS is required. Check token/CA parent traversal and read permissions under
the sandbox; do not place required files beneath protected home directories.

Verify, daemon-reload and explicitly enable/start `picketxd-controller.service`
only after this review. Use that same unit name in status/journal/application
dependency commands below. Publication belongs to the authenticated management
API, not local YAML polling or `systemctl reload`.

On Controller outage the provider-bound accepted LKG can retain permission only
until its original absolute deadline. A restarted process may confirm that cache
but remains activating until a clean current remote publication is confirmed;
LKG alone is not READY. Reconnect does not renew a Grant or resurrect expired
permission. Inspect both fresh local JSON and current Controller APPLIED revision/
hash; approval, delivery, an active service or an old receipt is not application.
`desired.remoteState` independently distinguishes `awaiting`, `current` and
`disconnected`; it is process-local, never restored as current from disk.
`desired.lastError` remains publication rejection, not transport health.

## First static installation

Build with the pinned Go toolchain; runtime needs no Go, C library or nft/conntrack executable. Use the Linux/systemd environment recorded in acceptance. These commands install the loopback demonstration; protection starts only when explicitly started.

```sh
CGO_ENABLED=0 go build -trimpath -o /tmp/picketxd ./cmd/picketxd
sudo install -d -m 0755 /usr/local/libexec
sudo install -m 0755 /tmp/picketxd /usr/local/libexec/picketxd
sudo install -d -m 0700 /etc/picketx
sudo install -m 0600 examples/enforce/policy.yaml /etc/picketx/policy.yaml
sudo install -m 0644 deploy/systemd/picketxd.service /etc/systemd/system/picketxd.service
sudoedit /etc/systemd/system/picketxd.service
```

Before first start, review the complete policy and **every ExecStart argument**: stable provider/Agent/profile, exact destination bounds, handling, poll and an exclusively operator-reserved full mark range. The shipped range is only an example. Binding agent/profile/destinations/handling must match. Container/FORWARD is outside this host INPUT/original-destination scope. Strict protected address/port pairs must be reserved for services as described in the strict guide.

For a **fresh strict deployment**, select `--handling strict` and matching `examples/strict/policy.yaml` before first start. An existing/retained graceful deployment cannot migrate through a flag change, identity change or state deletion; compatible migration needs separate planning.

```sh
sudo systemd-analyze verify /etc/systemd/system/picketxd.service
sudo systemctl daemon-reload
sudo systemctl enable --now picketxd.service
sudo systemctl status picketxd.service
sudo journalctl -u picketxd.service -o cat -n 30 --no-pager
```

`Type=notify` with explicit `--systemd-notify` completes startup only after the Agent emits its first clean current-policy confirmation and systemd receives READY=1. The driver proof includes current desired/applied generation/hash, ownership, complete live application and current strict cleanup when enabled. A rejected current publication never releases startup, even if a valid cached image remains confirmed. Startup waits at most 60 seconds, then times out and follows the failure-restart policy. Start-rate limits bound rapid failures; sufficiently spaced timeouts can keep retrying.

**Readiness is a one-time startup event, not ongoing health.** After startup, rejected publications can leave an active service retaining accepted policy; STATUS changes to explain this. Check fresh Agent JSON timestamps, enforcementConfirmed, matching desired/applied generation/hash, desired.lastError/lastApplyError and current strict cleanup continuously. An old successful line or active alone proves no current health. There is no watchdog or health endpoint.

To make a protected application's systemd start wait, add an operator-reviewed drop-in to that application's unit, then daemon-reload:

```ini
[Unit]
Requires=picketxd.service
After=picketxd.service
```

Both directives are required: After orders startup, while Requires pulls the Agent into the transaction. A failed Agent start blocks this ordered application start. Configure every relevant application and inspect its start result; this does not prevent manual/external starts or stop applications automatically on a later rejected publication. Existing applications are not restarted by installing this drop-in.

After=network.target does not prove address availability, synchronized clock or application. Fresh namespace/reboot has a window before confirmed installation: keep protected applications unavailable until actual confirmation through your deployment procedure. Retained kernel rules do not survive reboot by themselves.

## State and privilege

systemd creates private root-owned 0700 `/var/lib/picketx` for persistent state and `/run/picketx` for locks. Preserve **owner.json and desired.json together**, path and permissions across upgrades. Never rename/delete/overwrite live state to bypass errors. RuntimeDirectoryPreserve retains locks on stop; reboot may clear `/run`. Do not remove the lock directory while any Agent uses it.

The root process has only CAP_NET_ADMIN in its bounding set, NoNewPrivileges, read-only filesystem except declared state/runtime directories, private temporary files, protected kernel files and restricted socket families. AF_UNIX serves namespace ownership, AF_NETLINK serves nft/conntrack; IPv4/IPv6 sockets are permitted. It shares the host network namespace. Do not add PrivateNetwork/PrivateUsers, which change enforcement namespace/capabilities. This privileged firewall still depends on trusted privileged writers, correct clock and filesystem durability.

## Publish and operate

Publish complete YAML atomically in the same directory, after reviewing new-policy.yaml:

```sh
sudo install -m 0600 new-policy.yaml /etc/picketx/policy.next
sudo mv /etc/picketx/policy.next /etc/picketx/policy.yaml
sudo journalctl -fu picketxd.service -o cat
```

Valid replacements apply by polling, without restarting. Invalid replacements retain accepted state and absolute deadlines; repair and inspect fresh status. There is no ExecReload; systemctl reload is not a publication mechanism.

```sh
sudo systemctl restart picketxd.service
sudo systemctl stop picketxd.service
```

Restart verifies the retained live image, reopens the same private state and waits for fresh clean readiness without extending deadlines. Stop sends SIGTERM and **retains filtering**; it does not withdraw protection or remove old graceful connections. To withdraw, publish an explicit empty Configuration while running and confirm its application before stopping; see the enforcement example. Never globally flush firewall/conntrack state.

Unexpected failure, including SIGKILL, retries after five seconds, with at most three starts within 60 seconds. Persistent failure can leave failed service and old policy. Correct the logged cause, then reset-failed and start. Automatic restart provides no bounded revocation guarantee. Failed strict pre-replacement cleanup blocks even restrictions and retains old permissions, potentially permanent ones, until success; missed-interval durable revocation remains incomplete.

Upgrade by building/verifying a replacement, installing it to a sibling path, atomically renaming over `/usr/local/libexec/picketxd`, then restarting and inspecting fresh status. Preserve policy, identity, destinations, marks and state. Upgrade the binary and notify-capable unit together; the earlier M2C10 binary does not recognize --systemd-notify. Review unit edits and daemon-reload. Keep a compatible previous binary; older binaries may reject journal v4. Binary rollback requires understanding the current journal/image. Overwriting newer live state with an old journal is not rollback.

## Development verification

```sh
scripts/verify-systemd.sh
# Disposable capable host with real system manager and iproute2:
CGO_ENABLED=0 go build -o /tmp/picketxd ./cmd/picketxd
sudo env PICKETX_SYSTEMD_TESTS=1 python3 scripts/test-systemd.py \
  --binary /tmp/picketxd --artifacts "$PWD/artifacts"
```

Offline verification substitutes the executable path temporarily and checks syntax only. Native acceptance copies the shipped unit to a unique runtime unit, substitutes test paths/state/poll and adds a fresh disposable NetworkNamespacePath. It retains sandbox/restart settings and verifies actual capabilities/namespace, dual-stack TCP/UDP, automatic crash recovery, invalid reload retention, deny reload, stop retention and byte-identical journal recovery. Additional scenarios hold invalid-current/cached startup in activating, release an ordered dependent only after clean readiness, and verify timeout retains filtering/state. The timeout-only phase shortens TimeoutStartSec from 60s to 2s; notification access, sandbox and restart policy stay unchanged. It installs no production unit and never alters the runner host firewall. Opted-in setup failures fail, never skip. CI archives service report/journal alongside existing native evidence.

The separate required Controller-service gate runs `scripts/test-controller-service.py`
with explicit `PICKETX_CONTROLLER_SERVICE_TESTS=1`, actual installed Core/Agent
binaries and a real system manager. It requires current authenticated APPLIED,
three distinct real Agent processes, unchanged desired cache, dual-stack TCP/UDP
through startup/crash/outage/absolute expiry/offline restart/reconnect, independent
unprotected peer health and correct notify state. Public evidence contains only
allowlisted hashes/process/packet/receipt fields, never token/state/journal bodies.
It also verifies a complete latest stopped checkpoint restored at original paths,
with32 packet observations and no deadline renewal; see repository/source
`docs/validation/AGENT_STATE_RECOVERY.md` for procedure and limits. Same-boot recovery
is not old-snapshot rollback, migration, host reboot or boot-order proof.

References: upstream [systemd.service](https://www.freedesktop.org/software/systemd/man/systemd.service.html) and [systemd.exec](https://www.freedesktop.org/software/systemd/man/systemd.exec.html).

Notification is opt-in; standalone enforce and prepare do not send it just because NOTIFY_SOCKET is present. --systemd-notify rejects --once and missing/malformed Linux filesystem or abstract socket addresses before namespace effects. Delivery uses pure-Go Unix datagrams, one-second dial and write deadlines, and fixed STATUS text without publication/error interpolation; no libsystemd/C dependency. A delivery error stops the process with installed rules retained. Successful send is not manager acknowledgement; systemctl start completion and ordered-dependent acceptance establish manager processing.
