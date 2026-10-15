# PicketXD

> [English (default)](README.md) | 简体中文

PicketX 的 Linux 执行 Agent。当前仓库包含**内部 M0 与共享计划桥接验证基础设施**，尚不是生产 Agent。

已评审的 [M0 计划](https://github.com/PicketX/PicketX/blob/cdf17f4af430fcebfdfa79dfe4ef907d77ade993/docs/superpowers/plans/2026-10-14-linux-data-plane-validation.zh-CN.md)要求先验证 nftables 准入/过期和 conntrack 定向撤销，再推进完整 Agent/Controller 流程。

M1.3A 的 [`planbridge`](internal/validation/planbridge)依据固定声明夹具范围校验完整共享计划，物化显式时间 TCP/UDP 成员，保留 Grant/Deny 分域来源及绝对期限，并校验包括明确移除 Agent 在内的完整替换。共享依赖固定为 `v0.0.0-20261015081000-e39f96e27871`。构造范围不证明实际拓扑，编译输出不证明实际应用。本分支沿用同一个编译器接入 M1.3B/C：真实拓扑校验、应用/读回确认、实际发出历史、严格/空状态清理和边界驱动；新增特权验收状态见下文。详见[切片记录](docs/validation/M1.3A.zh-CN.md)、[实施计划](docs/superpowers/plans/2026-10-15-planbridge-compiler.zh-CN.md)及[四层/七层/管理端交付路线](docs/DELIVERY.zh-CN.md)。

## 开发

[M1.3 桥接实现与状态](docs/validation/M13.zh-CN.md)固定共享 Core/planner 版本，将已证明的夹具策略转换为规则，确认实际发出的成员，保留历史所有权并驱动未来时间边界。[特权 CI](https://github.com/PicketX/PicketXD/actions/runs/38749800479) 在已测版本上通过原 17 项 M0 和新增 19 项桥接必需场景。扩展脚本要求原 17 项 M0 加 19 项桥接场景全部通过，并输出 `bridge.json`。原 M0 证据不能证明新桥接已验收。

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
