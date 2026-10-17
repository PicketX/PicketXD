# Static host enforcement

> English (default) | [简体中文](README.zh-CN.md)

Build once, then run the static binary in the network namespace whose INPUT traffic it protects. The example only protects loopback TCP 18443 and UDP 18444, equally for IPv4 and IPv6. It grants loopback sources permanently; replace it with your actual complete publication and explicit destination bounds for deployment.

```sh
CGO_ENABLED=0 go build -o /tmp/picketxd ./cmd/picketxd
sudo /tmp/picketxd enforce \
  --policy "$PWD/examples/enforce/policy.yaml" \
  --state-dir /var/lib/picketx-demo \
  --provider static-demo --agent node \
  --destination 127.0.0.1 --destination ::1 \
  --mark-first 0x504a0000 --mark-last 0x504a003f
```

Reserve that entire mark range in your namespace before using it; the numbers above are an example, not a production default. Runtime requires Linux network-administration capability and access to a private state directory, not Go, C libraries or nft/conntrack executables. It installs only `inet picketx` after complete ownership checks and refuses foreign tables, corrupted/foreign journals and a second namespace owner, including one using a different state path. Startup may create a nonfiltering ownership marker before validating the first publication.

The private state directory holds `owner.json` (ownership, mark retirement, pending/confirmed image and replacement history) and `desired.json` (accepted static publication). Keep both. Same namespace, provider, Agent, profile, canonical destination bounds and mark range must be used on restart; changing them is not an automatic takeover/migration. Do not point another writer at these state files or rename the directory while running.

JSON lines distinguish `desired` acceptance/error/freshness, `ownership`, `application`, and `appliedGeneration`/`appliedPolicyHash`. Top-level `enforcementConfirmed=true` means the current accepted desired generation/hash is associated with a complete live application proof. A prior installed image can remain confirmed inside `application` while a newer desired generation fails; the top-level flag is then false. An invalid file is a rejected publication, not a withdrawal: the last accepted policy and its original absolute deadlines remain active. Unknown application or drift reports an error and exits, retaining the installed rules.

Publish a complete YAML by atomic rename. Polling defaults to 5s; `--poll` changes publication/live verification frequency. Kernel absolute validity rules handle future activation/expiry even if the process disappears. Removing the last Grant retains default deny and existing graceful marked connections. Remove the relevant Binding or publish the explicit empty Configuration below to withdraw all owned protection:

```yaml
apiVersion: picketx.io/v1alpha1
kind: Configuration
metadata: {name: empty, uid: empty-v1}
spec: {empty: true}
```

SIGINT/SIGTERM release process resources and locks but leave filtering installed; SIGKILL also leaves filtering. Restart strictly reobserves complete recorded rules before confirmation and never extends a deadline. `--once` applies/reobserves, prints one status and exits with filtering retained. If the current file is invalid but an accepted cache exists, once may confirm that cache but still exits nonzero and reports `desired.lastError`. `picketxd prepare` remains nonfiltering.

This example uses graceful static host-local/original-destination TCP/UDP. See the [strict example](../strict/README.md) for strict handling and [systemd deployment](../../deploy/systemd/README.md) for service packaging. Controller transport, NFQUEUE/TPROXY, L7, forwarding/container paths and management remain subsequent work. Kernel compatibility is limited to the actual acceptance record, not inferred from compilation.
