# Static host service

> English (default) | [简体中文](README.zh-CN.md)

Operator-reviewed system service for existing `picketxd enforce`. The demonstration protects loopback TCP 18443 / UDP 18444, both IP families, graceful mode, in the host network namespace. See [enforcement](../../examples/enforce/README.md), [strict limits](../../examples/strict/README.md) and [acceptance](../../docs/validation/M2C10.md). Controller, L7 and a boot-time firewall remain incomplete.

## First installation

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

`Type=exec` confirms process execution. **Active is not policy readiness.** Inspect a fresh Agent JSON line: enforcementConfirmed true, matching desired/applied generation and hash, no desired.lastError/lastApplyError, and confirmed current cleanup in strict mode. Check timestamps and subsequent errors continuously; old logs prove no current health. Invalid publications can leave an active process retaining accepted policy. There is no sd_notify readiness, health endpoint or readiness ordering for protected applications.

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

Restart verifies the retained live image and reopens the same private state without extending deadlines. Stop sends SIGTERM and **retains filtering**; it does not withdraw protection or remove old graceful connections. To withdraw, publish an explicit empty Configuration while running and confirm its application before stopping; see the enforcement example. Never globally flush firewall/conntrack state.

Unexpected failure, including SIGKILL, retries after five seconds, with at most three starts within 60 seconds. Persistent failure can leave failed service and old policy. Correct the logged cause, then reset-failed and start. Automatic restart provides no bounded revocation guarantee. Failed strict pre-replacement cleanup blocks even restrictions and retains old permissions, potentially permanent ones, until success; missed-interval durable revocation remains incomplete.

Upgrade by building/verifying a replacement, installing it to a sibling path, atomically renaming over `/usr/local/libexec/picketxd`, then restarting and inspecting fresh status. Preserve policy, identity, destinations, marks and state. Review unit edits and daemon-reload. Keep a compatible previous binary; older binaries may reject journal v4. Binary rollback requires understanding the current journal/image. Overwriting newer live state with an old journal is not rollback.

## Development verification

```sh
scripts/verify-systemd.sh
# Disposable capable host with real system manager and iproute2:
CGO_ENABLED=0 go build -o /tmp/picketxd ./cmd/picketxd
sudo env PICKETX_SYSTEMD_TESTS=1 python3 scripts/test-systemd.py \
  --binary /tmp/picketxd --artifacts "$PWD/artifacts"
```

Offline verification substitutes the executable path temporarily and checks syntax only. Native acceptance copies the shipped unit to a unique runtime unit, substitutes test paths/state/poll and adds a fresh disposable NetworkNamespacePath. It retains sandbox/restart settings and verifies actual capabilities/namespace, dual-stack TCP/UDP, automatic crash recovery, invalid reload retention, deny reload, stop retention and byte-identical journal recovery. It installs no production unit and never alters the runner host firewall. Opted-in setup failures fail, never skip. CI archives service report/journal alongside existing native evidence.

References: upstream [systemd.service](https://www.freedesktop.org/software/systemd/man/systemd.service.html) and [systemd.exec](https://www.freedesktop.org/software/systemd/man/systemd.exec.html).
