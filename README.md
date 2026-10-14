# PicketXD

> English (default) | [简体中文](README.zh-CN.md)

Linux enforcement Agent for PicketX. This repository currently contains **private M0 validation infrastructure**, not a production Agent or a completed network enforcement implementation.

The approved [M0 plan](https://github.com/PicketX/PicketX/blob/cdf17f4af430fcebfdfa79dfe4ef907d77ade993/docs/superpowers/plans/2026-10-14-linux-data-plane-validation.md) defines nftables admission/expiry and scoped conntrack revocation validation before the full Agent/Controller workflow.

## Development

Use Go 1.27.0 and Linux. Run unprivileged checks:

```sh
go mod download
go test ./...
go vet ./...
go build ./...
```

Kernel validation requires an explicitly opted-in, capable Linux environment with disposable network namespaces. Skips are not kernel evidence; missing privileges in an opted-in run are failures. Test helpers may invoke `unshare`/`ip`; fixture and backend code must not require runtime nft/conntrack executables.

## License

GPL-3.0-or-later; see [LICENSE](LICENSE). Separate OEM/commercial terms require an agreement with the rights holder and are not granted by this repository. Dependency licenses are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
