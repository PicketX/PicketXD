# 主机所有权实施计划

> 直接执行、测试驱动、结束后一次独立评审。沿用用户持续实施授权，不重复交接确认。

**目标：** 在生产入站代码之前，通过 pending/confirmed 持久日志认领绑定 namespace 的非过滤内核标记。

**设计：** [主机所有权](../specs/2026-10-16-host-ownership-design.zh-CN.md) / [English](../specs/2026-10-16-host-ownership-design.md)。

**约束：** Go 1.27.0、现有固定 Netlink 依赖、仅 GPL Agent、固定 `inet picketx`、不删除内核对象、不分配 conntrack mark、不声称执行保护。保留既有 43 项内核门禁。

**评审重点：** 另一状态路径不能绕过互斥；磁盘 confirmed 不能绕过实际观察；每次 create 前 pending 已持久；意外表对象拒绝认领；取消/关闭释放全部锁，不删除标记。

## 文件与接口

| 文件 | 职责 |
| --- | --- |
| `internal/hostowner/owner_linux.go`、`owner_test.go` | 配置/会话状态机、Open/Verify/Close 和状态；私有注入边界用于故障测试 |
| `internal/hostowner/files_linux.go`、`files_test.go` | Namespace 描述符、抽象 socket 锁、运行文件锁与状态锁、有界规范日志及原子发布 |
| `internal/hostowner/nft_linux.go`、`nft_test.go` | 绑定 namespace 的完整标记表观察及排他创建，不提供删除 API |
| `internal/hostowner/owner_integration_test.go` | 真实 namespace、外来对象、重启、进程锁/崩溃及排他竞争 |
| runner、report、`cmd/m0-report/main.go` | 执行所有权 binary、保留日志、强制新增场景 |
| 双语 README/验收记录 | 范围、实测证据及下一执行边界 |

任务 1 产出私有后端接口供任务 2 使用：`Observe(context.Context) (observation,error)`、`Create(context.Context, marker) error`、`Close() error`。观察区分不存在、完整支持的标记、不支持结构。仅故障边界注入副作用，仍测试真实文件/锁。`openWith` 是包私有；导出 `Open` 使用固定运行锁根目录和真实绑定 namespace 的 Netlink。

## 任务 1：持久会话状态机

1. 先写 `TestFreshClaimAndReopen`、`TestForeignAndDriftRefused`、`TestCrashBoundaries`、`TestNewBootAndNamespaceIdentity`、`TestAlternateStatePathExclusion`、`TestJournalPrivateCanonicalBounded`、`TestCanceledAndClosed`。仅加可编译声明桩，先观察断言失败。
2. 实现明确 Config/Session/Status、稳定安装身份、新 epoch、完整 scope/namespace 绑定、真实文件日志及全部锁。单独测试 rename 后不确定结果。
3. 运行 `go test ./internal/hostowner`，全部具名测试通过后运行全量 Go test/race/vet/build，再提交。

## 任务 2：真实内核后端与必需证据

1. 实现前先验证结构拒绝：完整读取 chains/rules/sets/objects/flowtables；dump 错误和漂移均失败。使用固定库的排他建表、单一无 hook chain、counter 标记规则。
2. 接入 Open 的 namespace 描述符、持久连接及抽象 socket 锁。新增必需案例 `TestOwnerKernelFreshAndRestart`、`TestOwnerKernelForeignTable`、`TestOwnerKernelUnexpectedStructure`、`TestOwnerKernelExclusiveCreate`、`TestOwnerKernelProcessLock`、`TestOwnerKernelPendingRecovery`、`TestOwnerKernelSocketExclusion`。全部使用隔离 namespace；明确开启后权限失败必须报错。进程锁还验证共享网络、但挂载全新 `/run` 的进程。
3. 扩展 runner 和证据解析，保留原门禁。运行单元/race/vet/build、本地集成编译、实际 Linux CI，检查全部必需结果；只有真实内核执行能证明兼容。
4. 独立全分支评审，用失败测试复现并修复阻塞问题，发布受测源码树和双语验收。本增量不宣传生产过滤命令。
