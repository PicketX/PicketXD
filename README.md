# PicketXD

> English (default) | [简体中文](README.zh-CN.md)

Linux enforcement Agent for PicketX. This repository contains **static desired preparation/recovery plus private M0, shared-plan and host-input validation infrastructure**, not a production enforcement Agent.

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

[`internal/hostpolicy`](internal/hostpolicy) compiles shared plans into immutable host-local/original-destination/graceful intent for arbitrary dual-stack TCP/UDP targets. It validates all selected targets against trusted scope, preserves UID/binding provenance and independent absolute lifetimes, and retains complete inactive inventory. The host kernel test adapter consumes this production compiler; the engine remains a validation fixture. The gate adds one port scenario for 51 required cases; native acceptance is pending. See the [design](docs/superpowers/specs/2026-10-16-host-policy-design.md), [plan](docs/superpowers/plans/2026-10-16-host-policy.md) and [record](docs/validation/M2C2.md). No production filtering command or apply receipt is introduced.
