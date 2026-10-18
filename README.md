# PicketXD

> English (default) | [简体中文](README.zh-CN.md)

Linux execution Agent for PicketX. `picketxd enforce` provides standalone static host INPUT/original-destination dual-stack TCP/UDP enforcement and recovery, with default graceful handling and explicit strict mode. See the [graceful example](examples/enforce/README.md), [strict example](examples/strict/README.md) and [systemd deployment](deploy/systemd/README.md). The full Agent/Controller workflow, container forwarding, L7 and management remain in progress.

The approved [M0 plan](https://github.com/PicketX/PicketX/blob/cdf17f4af430fcebfdfa79dfe4ef907d77ade993/docs/superpowers/plans/2026-10-14-linux-data-plane-validation.md) defines nftables admission/expiry and scoped conntrack revocation validation before the full Agent/Controller workflow.

M1.3A's [`planbridge`](internal/validation/planbridge) validates complete shared plans against a fixed declarative fixture scope, materializes explicit-time TCP/UDP members, preserves separate Grant/Deny provenance and absolute deadlines, and validates full replacements including explicit Agent removal. Its shared dependency is pinned to `v0.0.0-20261015145159-81daf5abb831`. Scope construction does not prove a live topology, and compiler output does not prove application. This branch builds M1.3B/C on that same compiler: live topology guards, confirmed apply/readback, emitted history, strict/empty cleanup and boundary driving; new privileged acceptance is tracked below. See [slice findings](docs/validation/M1.3A.md), [implementation plan](docs/superpowers/plans/2026-10-15-planbridge-compiler.md) and the [L4/L7/management delivery route](docs/DELIVERY.md).

## Development

Use Go 1.27.0 and Linux. Run unprivileged checks:

```sh
go mod download
go test ./...
go vet ./...
go build ./...
```

Kernel validation requires an explicitly opted-in, capable Linux environment with disposable network namespaces. Skips are not kernel evidence; missing privileges in an opted-in run are failures. Test helpers may invoke `unshare`/`ip`; fixture and backend code must not require runtime nft/conntrack executables.

Run real kernel scenarios on a capable disposable Linux host:

```sh
PICKETX_KERNEL_TESTS=1 scripts/test-kernel.sh
```

See [M0 findings and acceptance](docs/validation/M0.md) for observed versions, test coverage and remaining limits.

The [M1.3 bridge implementation](docs/validation/M13.md) pins the shared Core/planner, converts proven fixture intent, confirms actual emission, retains historical ownership and drives future boundaries. [Privileged CI](https://github.com/PicketX/PicketXD/actions/runs/38749800479) passes all 17 M0 + 19 bridge required cases on the tested revision. The extended runner requires all 17 M0 cases plus 19 bridge cases and writes `bridge.json`. Existing M0 evidence does not prove the new bridge.

## License

GPL-3.0-or-later; see [LICENSE](LICENSE). Separate OEM/commercial terms require an agreement with the rights holder and are not granted by this repository. Dependency licenses are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## M2B static preparation and host validation

`picketxd prepare` adds bounded complete publication loading, shared validation/replacement, provider-bound private atomic cache, exclusive file locking, restart recovery, independent deadline projection and truthful error/freshness status. See the [runnable example](examples/prepare/README.md). It does not modify kernel state and always reports enforcement unconfirmed.

Test-only HostInputFixture verifies real INPUT, dual-stack original CT tuples including DNAT and invalid/untracked rejection. Seven required host cases join all 17 M0+19 bridge cases. The current branch real CI status is in its PR; unexecuted/skipped tests never establish compatibility. Production ownership/mark, strict recovery, process-loss future restrictions, container-published coverage and Controller/L7/admin remain incomplete. See the [design](docs/superpowers/specs/2026-10-15-static-lifecycle.md).

## M2C1 ownership foundation

[`internal/hostowner`](internal/hostowner) adds a nonfiltering ownership session: a pinned namespace, abstract socket exclusion across mount namespaces, private durable pending/confirmed journal, exclusive table creation and strict raw Netlink readback. It refuses foreign/unsupported tables and same-boot drift; close retains the marker. It installs no admission rules, allocates no conntrack marks and always reports enforcement unconfirmed. `picketxd prepare` is unchanged; there is no production enforcement command yet.

All 50 required real-kernel cases pass (the original 43 plus seven ownership cases), including process death and a different `/run` mount, on Linux `6.17.0-1022-azure` with Go `1.27.0`. CI also passes test/race/vet/build and static preparation smoke. See the [design](docs/superpowers/specs/2026-10-16-host-ownership-design.md), [plan](docs/superpowers/plans/2026-10-16-host-ownership.md) and [validation record](docs/validation/M2C1.md) for exact commits, run and artifact evidence. Compatibility is limited to the recorded environment.

## M2C2 production host policy compiler

[`internal/hostpolicy`](internal/hostpolicy) compiles shared plans into immutable host-local/original-destination/graceful intent for arbitrary dual-stack TCP/UDP targets. It validates all selected targets against trusted scope, preserves UID/binding provenance and independent absolute lifetimes, and retains complete inactive inventory. The host kernel test adapter consumes this production compiler; the engine remains a validation fixture. All 51 required kernel cases pass on Linux `6.17.0-1022-azure` / Go `1.27.0`, including the added port scenario; unit/race/vet/build/static smoke also pass. See the [design](docs/superpowers/specs/2026-10-16-host-policy-design.md), [plan](docs/superpowers/plans/2026-10-16-host-policy.md) and [record](docs/validation/M2C2.md). No production filtering command or apply receipt is introduced.

## M2C3 durable mark reservation

`hostpolicy.MarkTargets` and [`markstate`](internal/markstate) derive physical target identities and nonreusable marks from an explicit operator-reserved range. The held `hostowner` session verifies marker structure, completely scans both conntrack families, and durably upgrades the same private journal to v2 before returning assignments. Retirement/recreation never reuses an old mark; publication errors require reopen. No traffic rules or CT mark writes/deletes are introduced. All 55 required native cases pass on Linux `6.17.0-1022-azure` / Go `1.27.0`, including four new mark cases; unit/race/vet/build/static smoke also pass. See the [design](docs/superpowers/specs/2026-10-16-mark-reservations-design.md), [plan](docs/superpowers/plans/2026-10-16-mark-reservations.md) and [record](docs/validation/M2C3.md).

## M2C4 recoverable host admission

[`hostrules`](internal/hostrules) and `hostowner.Session.ApplyPolicy` add a separate production package path for real host INPUT/original-destination/graceful TCP/UDP. Complete typed images program future activation and absolute expiry in the kernel; journal v3 retains previous/pending images around atomic dispatch, strict generation-bracketed readback and durable confirmation. Reopen confirms only recognized complete live structure, including explicit empty withdrawal. Close leaves filtering installed. New native gates cover both families, TCP/UDP, process-free time boundaries, original DNAT ports, atomic rejection, drift, pending and SIGKILL recovery. All 64 mandatory native gates and 85 recorded top-level cases pass with zero failure/skip on Linux `6.17.0-1022-azure` / Go `1.27.0`; 120 local unit cases and race/vet/build/static smoke also pass. See the [exact acceptance record](docs/validation/M2C4.md).

This is a package engine increment. `prepare` remains nonfiltering; a production enforcement command, desired publication reload loop, strict cleanup, L7 and management are subsequent work. It depends on an explicitly reserved mark range, trusted privileged writer exclusivity, a correct system clock and filesystem durability. See the [design](docs/superpowers/specs/2026-10-16-host-admission-design.md) and [plan](docs/superpowers/plans/2026-10-16-host-admission.md).

Application requires `hostpolicy.CompileInput` so the journal can preserve and revalidate complete replacement provenance, including revoked/dormant members and unbound entries. The baseline survives withdrawal and reboot; rotating a mark alone never authorizes retained Grant IDs on a retargeted endpoint.

## M2C5 standalone static enforcement

`picketxd enforce` connects accepted static publications to the recoverable host engine. It requires an explicit policy path, private absolute state directory, provider/Agent identity, destination bounds and operator-reserved mark range. Atomic valid reloads apply complete replacements; invalid reloads retain accepted state and report errors. JSON distinguishes accepted desired state, actual application receipt and their generation/hash association. SIGTERM and SIGKILL leave filtering installed; kernel absolute-time rules continue without a process. Restart requires matching identity and complete live readback. `prepare` remains nonfiltering.

Use the [English example](examples/enforce/README.md) or [Chinese example](examples/enforce/README.zh-CN.md), and consult the [acceptance record](docs/validation/M2C5.md), [design](docs/superpowers/specs/2026-10-16-static-enforcement-design.md) and [plan](docs/superpowers/plans/2026-10-16-static-enforcement.md). This command is the explicit host-local/graceful slice; it depends on the recorded kernel environment, trusted privileged-writer exclusivity, correct clock, filesystem durability and a reserved mark range. It does not provide strict connection teardown, Controller synchronization, service packaging, container/FORWARD, L7 or management.

## M2C6 scoped connection cleanup engine

`hostowner.Session.RevokeWithdrawn` adds a production, namespace-bound conntrack cleanup API. Validated historical source/target/mark ownership and the actual confirmed current image determine withdrawn flows; current independent permission preserves connections. Complete dual-stack scans and full live image checks gate exact original-tuple/zone/kernel-ID deletion and final readback. Partial failure, drift or remaining entries return errors and require reopen. No global flush, nft mutation or journal write is performed. See [acceptance and limits](docs/validation/M2C6.md), [design](docs/superpowers/specs/2026-10-17-host-revocation-design.md) and [plan](docs/superpowers/plans/2026-10-17-host-revocation.md).

This image-based API remains available. M2C7 below adds recorded history; `enforce` remains graceful without automatic cleanup.


## M2C7 durable compact cleanup history

Journal v4 retains deduplicated confirmed source/endpoint/physical-mark ownership across generations and process restarts. `Session.RevokeRecorded(ctx)` uses it with the actual confirmed current image; callers need no old images. Pending candidates enter history only after complete live confirmation. Legacy v1–v3 remain readable, but lost history stays explicitly incomplete and recorded cleanup refuses it. JSON status reports coverage separately from enforcement. See [acceptance and recovery limits](docs/validation/M2C7.md), [design](docs/superpowers/specs/2026-10-17-durable-history-design.md) and [plan](docs/superpowers/plans/2026-10-17-durable-history.md).

Bounds are 4096 targets, 65536 source/endpoint members and 8 MiB JSON. Capacity/context incompatibility fails before mark/nft effects; history is never pruned after expiry or cleanup. Older binaries reject v4; no in-place downgrade. M2C8 below adds strict admission and scheduling; durable completion recovery remains subsequent work.


## M2C8 static strict enforcement

`picketxd enforce --handling strict` checks current source/endpoint/absolute-time permission for protected packets and runs recorded exact conntrack cleanup after confirming rules on every reconcile. Graceful remains the default. Known retired marks preserve independently authorized established flows across physical replacement. Kernel deadlines remain effective after process death; restart reobserves and retries cleanup. JSON strict confirmation applies only to the current observation and its validUntil, with no persisted completion receipt.

Strict reserves protected local address/port pairs for services; locally initiated clients must use other source endpoints. A final reply-direction guard blocks reversed conntrack reconstruction after deletion, while original-direction DNAT aliases retain original-destination scope. Full dual-family scans repeat each poll/boundary. If permission returns before cleanup, retained marked TCP may resume; successfully cleaned TCP remains blocked through unmarked reconstruction. No RST/peer-notification promise is made.

All87 mandatory native gates and108 recorded top-level cases pass with zero fail/skip on the recorded Linux6.17.0-1022-azure / Go1.27.0 environment. Use the [strict example](examples/strict/README.md) and [acceptance record](docs/validation/M2C8.md) for exact evidence, migration prerequisites and operational limits. Durable completion recovery, authenticated Controller synchronization, deployment/service packaging, container/FORWARD, L7 and management follow the delivery route.


## M2C9 cleanup before strict replacement

On strict restart or a changed desired generation, the driver first cleans withdrawn owned flows against the retained actual image, then applies the candidate and confirms its current cleanup. This prevents a fresh Grant from preserving already-expired old TCP. Cleanup failure or remaining flows stop replacement; continuous independent permissions are preserved. JSON `strict.previousCleanup` identifies only the previous image observation; current desired confirmation remains separate. Normal polls retain verify/current-cleanup behavior. Startup/replacement costs an extra full dual-family cleanup pass.

See [M2C9 acceptance](docs/validation/M2C9.md). This is ordering recovery for flows already withdrawn at observation, not a persisted revocation barrier across every missed permission gap. Journal/image schemas, dependencies and graceful behavior remain unchanged.

## M2C10 systemd deployment

The existing static command has a [service template and bilingual installation/upgrade guide](deploy/systemd/README.md), retaining private state and filtering on stop. Active is not policy readiness; boot protection and full M2 remain incomplete. See the [acceptance record](docs/validation/M2C10.md).

## M2C11 service startup readiness

The systemd template uses explicit --systemd-notify and Type=notify to wait for the first clean current-policy confirmation. During startup, an invalid current publication with accepted cache remains activating; an ordered Requires/After application can wait for readiness. Startup notification is not ongoing health or a global boot firewall. See the [deployment guide](deploy/systemd/README.md) and [acceptance](docs/validation/M2C11.md).

## Controller Provider

`enforce` also accepts an exclusive authenticated gRPC Controller source, with verified
optional server TLS, provider-bound remote LKG and native apply reports. See the
[deployment example](examples/controller/README.md). Acceptance is pending PR/native CI;
this does not complete management or Layer 7 authorization.
