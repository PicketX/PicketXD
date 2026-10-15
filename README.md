# PicketXD

> English (default) | [简体中文](README.zh-CN.md)

Linux enforcement Agent for PicketX. This repository contains **private M0 and shared-plan bridge validation infrastructure**, not a production Agent.

The approved [M0 plan](https://github.com/PicketX/PicketX/blob/cdf17f4af430fcebfdfa79dfe4ef907d77ade993/docs/superpowers/plans/2026-10-14-linux-data-plane-validation.md) defines nftables admission/expiry and scoped conntrack revocation validation before the full Agent/Controller workflow.

M1.3A's [`planbridge`](internal/validation/planbridge) validates complete shared plans against a fixed declarative fixture scope, materializes explicit-time TCP/UDP members, preserves separate Grant/Deny provenance and absolute deadlines, and validates full replacements including explicit Agent removal. Its shared dependency is pinned to `v0.0.0-20261015081000-e39f96e27871`. Scope construction does not prove a live topology, and compiler output does not prove application. This branch builds M1.3B/C on that same compiler: live topology guards, confirmed apply/readback, emitted history, strict/empty cleanup and boundary driving; new privileged acceptance is tracked below. See [slice findings](docs/validation/M1.3A.md), [implementation plan](docs/superpowers/plans/2026-10-15-planbridge-compiler.md) and the [L4/L7/management delivery route](docs/DELIVERY.md).

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

The [M1.3 bridge implementation](docs/validation/M13.md) pins the shared Core/planner, converts proven fixture intent, confirms actual emission, retains historical ownership and drives future boundaries. Its ordinary tests pass locally; **new privileged acceptance is pending**. The extended runner requires all 17 M0 cases plus 19 bridge cases and writes `bridge.json`. Existing M0 evidence does not prove the new bridge.

## License

GPL-3.0-or-later; see [LICENSE](LICENSE). Separate OEM/commercial terms require an agreement with the rights holder and are not granted by this repository. Dependency licenses are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
