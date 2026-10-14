# PicketXD

> [English (default)](README.md) | 简体中文

PicketX 的 Linux 执行 Agent。当前仓库仅包含**内部 M0 验证基础设施**，尚不是生产 Agent。

已评审的 [M0 计划](https://github.com/PicketX/PicketX/blob/cdf17f4af430fcebfdfa79dfe4ef907d77ade993/docs/superpowers/plans/2026-10-14-linux-data-plane-validation.zh-CN.md)要求先验证 nftables 准入/过期和 conntrack 定向撤销，再推进完整 Agent/Controller 流程。

## 开发

使用 Go 1.27.0 和 Linux。运行无特权检查：

```sh
go mod download
go test ./...
go vet ./...
go build ./...
```

内核验证需要显式启用、具备权限的 Linux 环境和可销毁的网络命名空间。跳过测试不构成内核证据；启用后缺少权限必须失败。测试辅助代码可以调用 `unshare`/`ip`；夹具及后端代码不能依赖运行时 nft/conntrack 命令。

在具备权限、可销毁的 Linux 主机运行真实内核场景：

```sh
PICKETX_KERNEL_TESTS=1 scripts/test-kernel.sh
```

已观察的版本、覆盖范围及边界见 [M0 发现与验收](docs/validation/M0.zh-CN.md)。

## 许可证

GPL-3.0-or-later，详见 [LICENSE](LICENSE)。独立 OEM/商业条款需要与权利人另行签订协议，本仓库不授予这些权利。依赖许可列于 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
