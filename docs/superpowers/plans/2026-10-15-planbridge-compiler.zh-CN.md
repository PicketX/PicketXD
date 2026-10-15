# M1.3A 规划层夹具编译器实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> [English (default)](2026-10-15-planbridge-compiler.md) | 简体中文

**目标：** 将不可变共享计划转换为独立持有、显式时间的私有 M0 快照，拒绝声明测试夹具以外的目标。

**架构：** 纯 `internal/validation/planbridge` 包仅消费共享 Core/规划层及私有 fixture 类型。经过校验的范围固定权威、epoch、Agent、上下文、网络空间身份及夹具目录，候选保持不可变。后续协调器负责提供经过证明的拓扑事实、接受候选、应用/读回及维护实际发出历史。

**技术栈：** Go 1.27.0、标准库、PicketX 不可变版本 `e39f96e2787154a8af8e7d996b2f65aa10b6c3eb`；现有 Netlink 版本不变。

**设计依据：** [已批准 M1.3 设计](https://github.com/PicketX/PicketX/blob/9d53b13e2d8a805b2a7b2f3a8ad957fcf84ea12d/docs/superpowers/specs/2026-10-15-core-m0-bridge-design.zh-CN.md)第 1–3 节及第 4–5 节候选校验。[持续交付路线](../../DELIVERY.zh-CN.md)记录其余必需工作。

## 全局约束

- GPL 实现留在 PicketXD，仅从 Apache PicketX 导入 `pkg/core` 和 `pkg/networkplan`；不使用浮动分支或提交本地 replace。
- 仅支持 `transit` / `original-destination` 以及固定的 `graceful` 或 `strict`；声明身份不等于拓扑证明。
- 普通目的：`192.0.2.100/32`、`2001:db8:1:0:2::100/128`，TCP/443、TCP/22、UDP/8002。NAT VIP：`192.0.2.101/32`、`2001:db8:1:0:2::101/128`，TCP/8000、TCP/8001，不隐式改写端口。
- 保留全部 Binding ID、分域 Grant/Deny ID、独立绝对期限及同地址族配对；仅显式永久成员的 M0 到期为零。
- 完整非法候选不能返回部分结果。空自有输出是明确意图，不是通配符，也不授权调用非空 M0 Apply。
- 编译器无时钟、timer、命名空间句柄或后端动作；不声称实际安装历史或生产就绪。
- 用户 2026-10-15 的持续交付指令授权常规切片直接实现，不再新增计划确认轮次；保留本会话直接执行及最终一次独立评审。

## 评审重点

- 零值范围或变更 epoch/上下文须在返回结果前失败（任务 1/3）。
- 未来、到期、撤销成员不能变成永久权限（任务 2）。
- 移除 Agent 不能把任意查询错误解释为空所有权（任务 3）。
- 返回来源、清单及撤回项不能共享候选内部数据（任务 2/3）。
- 未绑定内容也须完整共享校验；不支持的已选 tuple 须失败（任务 2/3）。

## 上下文地图

| 文件 | 职责及变更 |
| --- | --- |
| `go.mod`、`go.sum`、`THIRD_PARTY_NOTICES.md` | 固定共享版本及校验和；保持 Netlink 版本，区分实际包导入与模块图 |
| `internal/validation/planbridge/scope.go`、`scope_test.go` | 不可变声明范围、普通/NAT 固定目录，不证明实际拓扑 |
| `internal/validation/planbridge/compiler.go`、`compiler_test.go` | Compile/Replace、目录兼容性、显式时间快照及来源记录 |
| `internal/validation/planbridge/replacement_test.go` | 同权威替换、Agent 移除依据、返回值所有权及并发读取 |
| `internal/validation/planbridge/doc.go`、双语 README | 编译器状态及下一生命周期/内核验收入口 |
| 共享模块 `pkg/networkplan` / `pkg/core` | 不可变所有权、期限及替换来源；仅依赖不修改 |
| 现有 `fixture/snapshot.go` 及测试 | 私有非空快照语义，不修改 |
| 现有 M0 nft/ct 测试及 runner | 回归门禁，不修改，不声称新增桥接报文证据 |

不修改公开 API、数据库迁移或用户配置。实现前已依据获批设计检查此地图；生产 Schema 与实际拓扑证明属于后续工作。

### Task 1: 固定依赖并校验不可变夹具范围

**文件：** `go.mod`、`go.sum`、`THIRD_PARTY_NOTICES.md`、`internal/validation/planbridge/{scope.go,scope_test.go}`。

**接口：** 产出 `Identity{AuthorityID, Epoch, AgentRef, NamespaceID string}`、`Catalog`（`Ordinary`、`NAT`）、`NewScope(Identity, networkplan.ExecutionContext, Catalog) (*Scope, error)`；Scope 没有公开可变字段。

- [ ] 用 Go 解析不可变版本、记录伪版本/校验和，保持既定 Netlink 版本；核对实际包导入不含 Echo/GORM/Web。
- [ ] 编写 `TestScopeRejectsUnsupportedFacts`：合法范围成功，空身份字段、上下文 Agent/网络空间不匹配、空 ID/profile、host/published/post-DNAT、非法处理模式、未知目录及过宽/非规范目的须返回 nil/error。`TestScopeOwnsFacts` 验证修改传入目的不能改变后续接受行为。
- [ ] 执行 `go test ./internal/validation/planbridge -run TestScope -count=1 -v`，在最小构造桩上观察新增校验失败。
- [ ] 实现范围校验及目的克隆；目录仅接受明确主机目的与协议/端口。上下文元数据是输入，不证明实际命名空间。
- [ ] 执行 `go test ./...`；提交 `feat(agent): pin shared core and validate bridge fixture scopes`。

### Task 2: 编译独立持有的显式时间快照及来源

**文件：** `internal/validation/planbridge/{compiler.go,compiler_test.go,doc.go}`。

**接口：** 消费任务 1 范围，产出 `Prepare(*Scope, *Prepared, networkplan.Input) (*Prepared, error)`、`(*Prepared).Materialize(time.Time) (Materialized, error)`。`Materialized` 独立持有 `Identity`、`At`、`NextChange`、`Snapshot`、`Provenance`、`Inventory`、`Withdrawals`、`AgentRemoved`；`Member` 包括 domain/ID/authority/target/BindingIDs/rule/validity/revoked。Prepared 无公开可变字段或共享计划暴露。

- [ ] 编写 `TestMaterializeIndependentDualStackMembership`：Gitea 产生四条保护（各地址族 TCP/443、TCP/22），独立 UDP 保留 17/8002，TCP 为 6；主机/较宽 CIDR 期限保持字面绝对值和两个 Binding ID；同名 Grant/Deny 保持分域。NAT 保持原 VIP 的 6/8000、6/8001。
- [ ] 编写 `TestMaterializeLifetimeBoundaries`：明确时刻 `2026-10-15T09:00:00Z` 起点包含、终点排除；未来/撤销/到期成员不进入当前规则，但保留在清单；NextChange 为字面 `09:30`；零求值时间报错，仅永久期限为零；最终到期仍保留保护。
- [ ] 编写 `TestPrepareRejectsUnsupportedTargets`：不支持的选中端口/协议/边界/profile/class/stage/handling 或未知 Agent 返回 nil/error；非法未绑定内容仍被共享完整校验拒绝；不隐式改地址/端口。
- [ ] 运行具名测试，在最小接口上观察缺失行为，再实现 Compile/Inventory/Projection、精确范围及独立来源数据；贡献不合并，仅保护 tuple 去重。
- [ ] 执行 `go test ./...`；提交 `feat(agent): compile shared plans into owned fixture snapshots`。

### Task 3: 验证替换、移除及所有权并发布切片

**文件：** `internal/validation/planbridge/replacement_test.go`、编译器调整、双语 README 及验证记录。

**接口：** 任务 2 `Prepare` 针对明确提供的已接受前任执行共享 Replace。移除 Agent 须具备同范围完整替换、候选明确不存在该 Agent 及旧目标撤回，或保留的此前明确空状态。不能捕获任意规划层错误并当空输出。

- [ ] 编写 `TestReplacementPreservesAcceptedCandidate`：非法候选、变更权威/epoch/上下文、保留 ID 改指向均失败，前任输出不变；移除单一 Binding 保留另一个的来源，移除全部 Grant 仍保持保护。
- [ ] 编写 `TestRemovedAgentRequiresReplacementEvidence`：首次未知 Agent 报错；非空合法前任之后移除，明确输出空及完整旧 tuple 撤回；重复空候选仍标记已移除；重新引入 Agent 恢复正常共享校验。
- [ ] 编写 `TestPreparedOutputsOwnData`：修改返回 Snapshot、Member.BindingIDs、inventory/context/matches/contributions 和 withdrawal 后重新物化不变；并发读取通过 race；修改 Prepare 的输入不能改变结果。
- [ ] 运行具名测试，观察尚未支持的替换行为断言失败，再实现最小移除路径及输出所有权。
- [ ] 执行 `go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./...`、`git diff --check` 并编译全部 integration 测试。保留 M0 实际内核门禁，明确本轮仅编译器证据。
- [ ] 提交 `test(agent): verify bridge replacement and ownership contracts`，完成独立全分支评审，以回归测试修复阻断项，发布 PR 并验证最终提交 CI。

## 自查与交接

M1.3 第 1–3 节纯转换/候选约束均有对应任务。拓扑证明、应用 ACK/读回/回执、累计安装历史、精确清理/空表删除、timer 及新增报文测试仍是 M1.3B/C 必需项。本切片不完成整个 M1.3 或基本产品；运行时/HTTP/UI 按交付路线推进。用户持续指令保留本会话直接执行，不新增确认停顿；裁定及实际证据记录在实现 PR。
