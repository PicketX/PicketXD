# PicketXD

> [English (default)](README.md) | 简体中文

PicketX 的 Linux 执行 Agent。当前仓库包含**静态期望状态准备/恢复流程，以及 M0、共享桥接和主机 INPUT 验证基础设施**，尚不是生产执行 Agent。

已评审的 [M0 计划](https://github.com/PicketX/PicketX/blob/cdf17f4af430fcebfdfa79dfe4ef907d77ade993/docs/superpowers/plans/2026-10-14-linux-data-plane-validation.zh-CN.md)要求先验证 nftables 准入/过期和 conntrack 定向撤销，再推进完整 Agent/Controller 流程。

M1.3A 的 [`planbridge`](internal/validation/planbridge)依据固定声明夹具范围校验完整共享计划，物化显式时间 TCP/UDP 成员，保留 Grant/Deny 分域来源及绝对期限，并校验包括明确移除 Agent 在内的完整替换。共享依赖固定为 `v0.0.0-20261015145159-81daf5abb831`。构造范围不证明实际拓扑，编译输出不证明实际应用。本分支沿用同一个编译器接入 M1.3B/C：真实拓扑校验、应用/读回确认、实际发出历史、严格/空状态清理和边界驱动；新增特权验收状态见下文。详见[切片记录](docs/validation/M1.3A.zh-CN.md)、[实施计划](docs/superpowers/plans/2026-10-15-planbridge-compiler.zh-CN.md)及[四层/七层/管理端交付路线](docs/DELIVERY.zh-CN.md)。

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

## M2B 静态准备与主机验证

新增 `picketxd prepare`：有界完整配置加载、共享校验替换、来源绑定的私有原子缓存、排他文件锁、重启恢复、独立期限求值和真实错误/新鲜度状态。参见[可运行示例](examples/prepare/README.zh-CN.md)。它不修改内核，状态始终明确未确认执行。

测试专用 HostInputFixture 验证实际 INPUT、双栈原始 conntrack 元组（包括 DNAT）、invalid/untracked 拒绝；新增 7 项 host 必需门禁，保留原 17 M0+19 bridge。当前分支真实 CI 状态见 PR；不得把未执行或 skip 当成兼容证据。生产所有权/mark、严格恢复、失联未来限制、容器发布、Controller/L7/管理端仍待完成。详见[设计](docs/superpowers/specs/2026-10-15-static-lifecycle.zh-CN.md)。

## M2C1 所有权基础

[`internal/hostowner`](internal/hostowner) 新增非过滤型所有权会话：持有 namespace 描述符、跨挂载 namespace 的抽象 socket 互斥、私有 pending/confirmed 持久日志、排他建表及严格原始 Netlink 读回。外来/不支持表及同 boot 漂移均拒绝，关闭保留标记。不安装入站规则、不分配 conntrack mark，始终报告未确认执行。`picketxd prepare` 不变，尚无生产执行命令。

全部 50 项必需真实内核场景（原 43 项加 7 项所有权案例）已在 Linux `6.17.0-1022-azure`、Go `1.27.0` 通过，包括进程退出及不同 `/run` 挂载。CI 的 test/race/vet/build 和静态准备冒烟也通过。准确提交、运行和归档证据见[设计](docs/superpowers/specs/2026-10-16-host-ownership-design.zh-CN.md)、[计划](docs/superpowers/plans/2026-10-16-host-ownership.zh-CN.md)、[验收记录](docs/validation/M2C1.zh-CN.md)。兼容声明仅限已记录环境。
