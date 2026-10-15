# Static preparation and restart

> English (default) | [简体中文](README.zh-CN.md)

`picketxd prepare` is a real preparation/recovery loop. It **does not enforce traffic**. JSON reports desired preparation and always `enforcementConfirmed: false`. Host enforcement is currently exercised only in disposable kernel tests; do not use this binary to protect a service.

```sh
go run ./cmd/picketxd prepare --once \
  --policy examples/prepare/policy.yaml \
  --state /var/lib/picketx-prepare/accepted.json \
  --provider static-example --agent agent-a \
  --destination 192.0.2.100 --destination 2001:db8::100
```

Use a writable private directory; a missing final state directory is created with 0700, with 0600 lock/cache files. For local experiments replace the state path with a private directory under your own account. The process needs no root/network privileges for preparation. The destinations are explicit preparation bounds, not verified local enforcement observations. The example has absolute 2026-10-15 deadlines; an expired example remains valid desired state with zero active permission.

Omit `--once` to poll (default five seconds) and independently report future activation/expiry boundaries. Cancel with Ctrl-C/SIGTERM; the loop stops before releasing the exclusive state lock. Publish one complete YAML file by atomic rename. Empty/truncated/broken files are errors, not withdrawals; use the shared explicit empty Configuration form to remove responsibility.

Restart with identical provider, Agent, profile/destination settings and state path. The cache is restored, validated and recompiled at current time. Original deadlines are unchanged; `restored`, `sameBoot`, `generation`, `acceptedAt`, `lastAttemptAt` and `lastError` explain freshness and fallback. A rejected reload keeps the previous valid desired state and retains an error until a valid reload. Once returns nonzero for a bad current publication even when valid cached state is reported. Corrupt/foreign caches fail startup and require operator investigation; the program never silently overwrites them. Cache content is an internal canonical format, not a hand-editable policy API.

The directory, lock and cache must belong to the current user and must not be group/world accessible; symlink/special cache files fail. Only one process can own a state path. Switching provider/Agent or host configuration requires an explicitly separate state location; old/new sources are not merged. Boot metadata never proves current rule application. No Controller mode, listener, strict cleanup, directory watcher, Koanf process-file loader, L7 or administration is implemented here.
