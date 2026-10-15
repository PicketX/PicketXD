# Core 到 M0 桥接实施计划

> [English（详细接口与任务）](2026-10-15-core-m0-bridge.md)。沿用已选的会话内执行；用户已授权持续推进四层、七层及管理端，常规批次不重复确认。

目标：将共享 Core/物理规划器接入隔离 Linux 夹具，保留独立期限、真实应用确认及严格撤销的历史所有权。这一批是验证基础设施，生产配置提供者和 HTTP 鉴权后续单独推进。

依据：[主干 M1.3 设计](https://github.com/PicketX/PicketX/blob/9d53b13/docs/superpowers/specs/2026-10-15-core-m0-bridge-design.zh-CN.md)。依赖固定为共享仓库 `e39f96e2787154a8af8e7d996b2f65aa10b6c3eb`，Go 伪版本 `v0.0.0-20261015081000-e39f96e27871`。最低 Go 1.27.0，不更换已有 Netlink 依赖。

**执行状态：** 任务 1–4 已实现并通过本地验证。任务 5 的场景代码、脚本和独立评审已完成；用户已明确授权推送；真实特权验收及 PR 发布继续执行。[当前证据](../../validation/M13.zh-CN.md)。

**集成决定：** 复用已有 M1.3A PR #2 的编译器与全部测试，新 PR 以该分支为基线。不保留两套竞争接口，以免合并冲突和替换校验不一致；代价是增加一次编译器/协调器接口回归检查。

## 文件与依赖关系（已核对）

| 文件 | 职责 |
| --- | --- |
| `internal/validation/planbridge/{scope,compiler}.go`、`scope_linux.go` | 精确夹具范围校验、纯转换、Grant/Deny 分域溯源 |
| `internal/validation/nftbackend/{backend,receipt}_linux.go` | 派发时剩余期限、实际批次回执、读回确认、NAT 同事务、所属表删除 |
| `internal/validation/ctbackend/backend_linux.go` | 显式空 after 清理，保持原始/回复元组、地址族、zone 和 mark 限定 |
| `internal/validation/planbridge/{coordinator,driver}_linux.go` | 串行应用、累计已确认历史、失败重试、可取消时间边界驱动 |
| `internal/validation/kerneltest`、内核测试、报告与脚本 | 真实拓扑、443/22/UDP/NAT 报文、实际版本与结果证据门禁 |

不改变共享 API、数据库或公共配置；保持 M0 API 可用。GPL 实现只在 PicketXD；只导入共享 Core/planner，不引入 Controller/Web。所有操作仅在可销毁 transit 命名空间中，重新校验 Agent/context/epoch 和已证明的元组。

## 任务与验证

1. **纯转换**：先写失败测试，再实现 `NewScope`/`Prepare`。拒绝错误上下文、权威来源、未证明端口、范围扩大；保留所有 Binding、独立 Grant/Deny、完整非活动清单及原始期限。只允许显式永久权限得到零过期。运行转换包测试并提交。
2. **后端回执与确认**：先测锁等待、过期与不足毫秒过滤、读回不匹配、显式空状态。实现 `ApplyReceipt`/`Confirm`/`RemoveOwned`/`RevokeEmpty`；原生 timeout 以锁内新时刻计算，记录实际发出与省略成员。准入与已知 NAT 同批 Flush；准备失败丢弃批次。确认所属链、规则、集合与成员，仅容忍计数及原生到期。运行全套测试并提交。
3. **生命周期**：先测应用/确认/删除顺序、过期历史跨多次应用保留、清理失败重试、Binding/Agent 移除、拒绝候选和未知状态。实现 `NewCoordinator`/`Update`/`Refresh`/`RetryCleanup`，确认后先推进版本再清理。历史只记录实际发出并确认的成员，未知应用状态禁止删除。运行全套及竞态测试并提交。
4. **时间边界**：先测未来允许/拒绝、生效版本切换、取消及已跨越边界。实现测试专用 `Run(ctx)`，始终在串行区读取最新计划；更新唤醒，不重放旧闭包，慢步骤跨边界立即重算。运行竞态测试并提交。
5. **真实报文与证据**：证明 TCP/443、22、UDP 和 NAT 的未过滤路径及 transit 身份；新增独立期限、拒绝/撤销、原生过期、未来边界、临期重放、历史严格保留、应用/确认失败、部分清理、绑定/空 Agent 移除和 NAT 原始元组场景。记录真实时间、上下文、成员及选中/删除/剩余计数。脚本构建并执行新二进制，所有 Bridge 场景与原 17 项 M0 均为必需；缺失/跳过不得通过。全套测试、race、vet、build、特权 CI 及最终独立评审通过后同步双语状态并发布 PR。

## 重点评审

- 锁等待不能续期，也不能把有限期限变成永久。
- 过期的已发出权限可能仍拥有平滑连接，跨后续应用和失败清理仍需保留。
- ACK 后出现额外允许规则/集合也必须禁止撤销。
- 未知 Agent 错误不等于空状态；移除必须来自已验证的完整替换。
- 旧定时器不能在更新或取消后重放旧计划。
