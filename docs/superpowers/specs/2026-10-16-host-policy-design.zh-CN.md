# M2C2：生产主机策略编译

> [English (default)](2026-10-16-host-policy-design.md) | 简体中文

所有权 PR #5 合并后，继续已授权的生产 L4 路线。本批提供独立于私有验证夹具的生产内核输入契约，不安装规则、不宣称生产执行就绪。沿用持续推进授权，在当前会话实施并做一次独立全分支评审。

## 范围与取舍

新增 `internal/hostpolicy`，消费不可变共享 `networkplan.Plan`，包括 `configuration.Candidate.Plan()`。沿用夹具编译器会保留写死的目录及测试类型；立即实现执行命令会跳过 mark、历史和进程消失处理前置条件。独立纯编译器提供可用接口并保留这些前置条件，不改依赖和公开配置格式。

`NewScope(authorityID string, context networkplan.ExecutionContext) (*Scope,error)` 校验并复制可信运行事实。身份非空、有效 UTF-8、至多 256 字节、无 NUL；明确要求 host-local/original-destination/graceful，以及规范化目的范围。选中上下文的身份/profile/namespace 必须与可信事实匹配；策略只能缩小目的范围。scope 不暴露可变字段。

`Compile(plan *networkplan.Plan, scope *Scope) (*Policy,error)` 拒绝空/未编译计划、未知 Agent、authority 不匹配及全部选中但不支持的目标，包括没有活跃 Grant 的目标。拒绝 strict、transit 和 published-ingress。其他 Agent 不在本地编译范围内。仍存在的 Agent 移除最后 Binding 可产生明确空的期望策略，Agent 缺失则报错；空期望输出不是安装或删除凭据。

`Policy.Evaluate(at time.Time) (Snapshot,error)` 要求明确非零时间，读取共享活跃投影，输出版本 1：authority、可信上下文、UTC 求值时间、下一期望边界，以及各目标 key/Binding ID/保护端点/allow 与 deny 成员。端点包含规范目的前缀、IP 协议号 6 或 17、1..65535 的单端口。保留 TargetKey.Entry.ResourceName 中的 Resource UID、贡献 UID、来源前缀和完整绝对 Validity，不合并权限并集、独立期限或 ID 域。最后 Grant 到期后仍保留保护。IPv4/IPv6 同等支持任意有效目的地址、TCP/UDP 和端口，不读取夹具目录。

`Policy.Inventory() ([]networkplan.TargetInventory,error)` 返回包含未来/过期/撤销贡献的完整期望来源。返回数据可独立修改，并发读取不修改内部状态。计划始终是求值事实来源，时钟和调度器在包外。

未来 deny 表示为期望库存和求值边界，不代表已具备进程消失后的可靠安装能力。版本 1 是内部编译契约，不是后续 nftables 结构所有权契约。本批不分配 mark、不增加生产 apply/confirm 命令；`prepare` 和仅标记的 `hostowner` 不变。

## 验收

覆盖任意双栈 TCP/UDP 目标、同 ID 的 Grant/Deny 独立域、独立期限、NotBefore/排他结束、永久成员、撤销、最后 Grant 后保护、空 Binding 撤回、未知 Agent、休眠的不支持目标、authority/namespace/profile/范围不匹配、返回数据修改和并发求值。通过真实 configuration.Decode/Compile 验证 UID。仅替换现有 host 内核夹具的编译输入；生产代码禁止导入 validation 包。保留全部 50 项必需门禁，新增必需 `TestHostPolicyCompilerPorts`，实测双栈 TCP/8001 与 UDP/8002。夹具执行不代表生产内核引擎验收。
