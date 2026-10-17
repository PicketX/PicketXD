# Static strict host policy

> English (default) | [简体中文](README.zh-CN.md)

Use a fresh compatible owned network namespace and keep echo services in that same namespace for this loopback demo. The endpoints are TCP18443 and UDP18444 in IPv4/IPv6. Runtime uses Go Netlink, no nft/conntrack commands. Build from the repository root:

Strict reserves each protected local address/port for service traffic. Do not bind locally initiated client sessions to that source endpoint: incoming reply-direction packets to a protected endpoint can be dropped. This guard prevents an old connection from bypassing policy when conntrack reconstructs it with the server as original direction after deletion. Original-direction DNAT aliases keep original-destination scope. Unmarked/unrelated conntrack records remain preserved.

```sh
CGO_ENABLED=0 go build -o picketxd ./cmd/picketxd
sudo ./picketxd enforce --handling strict \
  --policy examples/strict/policy.yaml --state-dir /var/lib/picketxd-strict \
  --provider static-test --agent node \
  --destination 127.0.0.1 --destination ::1 \
  --mark-first 0x504a0000 --mark-last 0x504a003f --poll 5s
```

Reserve the complete mark range exclusively. Default handling is graceful; both the flag and Binding handling must explicitly select strict. Current source/endpoint/absolute-time permission gates every protected packet. Deny takes precedence. Removing the final Grant keeps protection/default deny and blocks existing TCP/UDP; explicit empty Configuration removes protection. Publish a complete YAML through atomic rename.

Each driver wake confirms the complete actual rules, scans both conntrack families, deletes exact withdrawn owned original tuples/zone/kernel IDs and confirms zero remaining. Independent current permissions, unmarked and unrelated flows are preserved. Every configured poll and known time boundary repeats this full scan; increase poll for lower periodic cost, while kernel absolute deadlines and boundary wakes remain independent of polling frequency.

JSON strict.cleanupConfirmed describes that reconcile only. Match applied/desired generation/hash and imageDigest, and honor validUntil. It is not persisted and cannot prove cleanup across unobserved intervals. Once reconciles once and exits; close/SIGKILL leave rules installed. Kernel packet deadlines still apply, but absent processes do not delete conntrack. Retained marked TCP may resume if permission returns before cleanup; successfully cleaned TCP cannot resume solely through unmarked conntrack reconstruction. UDP is connectionless and may resume under a new valid Grant. No TCP RST or immediate peer notification is promised.

Incomplete/legacy history, pending ambiguity, drift, incomplete dump, partial delete, remaining entries or a crossed boundary never produce strict confirmation; reopen reobserves and retries. Same-boot handling/context migration is refused. A different state-dir cannot bypass namespace ownership. Preserve old state and use an explicit deployment path; older binaries reject strict imagev2. Correct clock, filesystem durability and trusted privileged-writer exclusivity remain prerequisites.

See [M2C8 acceptance](../../docs/validation/M2C8.md) and the [design](../../docs/superpowers/specs/2026-10-17-strict-enforcement-design.md). Controller, durable completion recovery, container/FORWARD, L7 and management remain subsequent work.
