# 主机策略编译实施计划

> 沿用持续实施授权，使用 executing-plans 在当前会话实施，最后一次独立全分支评审。[English (default)](2026-10-16-host-policy.md)

目标：将共享期望计划编译为不可变生产主机输入，不导入私有夹具。设计：[生产主机策略](../specs/2026-10-16-host-policy-design.zh-CN.md)。Go 1.27.0，现有固定依赖，不增加模块。

全局约束：仅 host-local/original-destination/graceful；双栈任意规范 TCP/UDP 目标；保留 UID、Binding 来源、绝对 Validity 及独立保护；不分配 mark、不操作内核、不提供执行凭据；保留原 50 项门禁。

## 上下文图与评审重点

| 文件 | 职责与变更 |
| --- | --- |
| `internal/hostpolicy/policy.go` | 新增 Scope、Policy、版本化 Snapshot，校验全库存并按时刻求值 |
| `internal/hostpolicy/policy_test.go` | 任意双栈目标、生命周期、来源、越界拒绝和并发隔离 |
| `internal/hostpolicy/configuration_test.go` | 真实配置 UID、替换和空期望输出 |
| `internal/validation/hostbridge/host_integration_test.go` | 测试转换器接入生产编译器，新增端口报文场景 |
| `internal/validation/report/report.go` | 增加必需命名场景 |
| README 与 `docs/validation/M2C2.*` | 同步范围与真实验收证据 |

直接依赖为共享 networkplan 的 Plan/Inventory/Projection/上下文规范化，以及 configuration.Candidate.Plan()。agentstate、hostowner 不变。参考 planbridge/compiler_test.go 的生命周期和来源检查、agentstate/store_test.go 的真实输入及 hostbridge 的可丢弃 namespace 报文测试。没有公开 API 破坏、迁移、依赖或配置变化。上下文图已对照导入和调用者自检。

评审重点：休眠越界目标提前拒绝；窄目标不能覆盖可信范围外地址；嵌套返回数据不影响后续并发求值；未来 deny 不丢库存；最后 Binding、最后 Grant 与未知 Agent 的结果明确区分。

## 任务 1：不可变编译器

- [x] 新增 NewScope(string,ExecutionContext)(*Scope,error)、Compile(*Plan,*Scope)(*Policy,error)、Policy.Evaluate(time.Time)(Snapshot,error)、Policy.Inventory()([]TargetInventory,error)。类型和字段以英文计划 Interfaces 为准。
- [x] 先写六组测试：TestDualStackArbitraryTargets、TestLifetimeAndInventory、TestScopeRejections、TestSelectedTargetRejections、TestIndependentReturnedData、TestConfigurationIdentityAndWithdrawal。对可编译桩运行，确认因缺行为而断言失败。
- [x] 使用共享 Inventory/Projection 实现；编译时验证完整选中库存，求值时使用明确时刻，返回独立数据。
- [x] 运行 go test ./...、go test -race ./...、go vet ./...、go build ./...，全部退出 0 后提交。

## 任务 2：内核适配与证据

- [x] 新增必需 TestHostPolicyCompilerPorts：双栈 TCP/8001、UDP/8002，基线可达、允许来源、无关来源拒绝和无关服务控制。
- [x] 用生产编译器替换手工共享投影转换，保留测试内的 fixture 转换和已有命名门禁；生产代码不导入 validation。
- [x] 全量本地检查、静态准备冒烟、集成编译；一次独立评审，Important/Critical 通过先红后绿回归修复。
- [x] 发布 PR，要求全部 51 项内核门禁及 unit/race/vet/build/static smoke，下载校验准确 head 归档，记录真实版本与结果；评审问题和 CI 解决后按用户明确授权合并。
