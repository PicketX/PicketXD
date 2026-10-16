# 持久 mark 保留实施计划

> [English (default)](2026-10-16-mark-reservations.md)；沿用持续推进授权，executing-plans 当前会话实施，最后一次独立全分支评审。

目标：在已持有的 namespace 所有权边界内，持久保留不复用的逐目标 conntrack mark。设计：[持久 mark](../specs/2026-10-16-mark-reservations-design.zh-CN.md)。Go 1.27.0，现有固定依赖，GPL Agent。

约束：显式完整非零 32 位范围，容量 1..4096，单调分配、永久退休；v1 可读、v2 必须带库存，内核标记不变；日志 32 MiB、NFT dump 4 MiB、CT 每族 64 MiB/五秒；不加过滤命令、不写/删 mark、不宣称恢复策略或确认执行；保留 51 项，新增四项共 55 项门禁。

## 上下文图与评审重点

| 文件 | 变更 |
| --- | --- |
| `internal/markstate/state.go` 及测试 | 纯范围/目标/历史校验，冲突、耗尽和单调分配 |
| `internal/hostpolicy/marks.go` 及测试 | 完整库存及稳定物理版本 |
| `internal/hostowner/marks_linux.go` 及测试 | 会话串行、双族扫描、固定目录持久化和故障重开 |
| `owner_linux.go`、`files_linux.go` | v1/v2、writer、跨 boot 历史及 32 MiB |
| `raw_linux.go`、`conntrack_linux.go` 及测试 | 共享严格接收，流式有界 CT 占用 |
| mark/owner 集成测试、report | 强杀恢复、真实 CT、漂移及四项必需门禁 |
| 中英文 README/设计/计划/验收记录 | 实测证据、裁定和剩余执行边界 |

依赖 Policy.Inventory/TargetKey、openWith/journal.write/Session.Verify；参考已有 raw 完成、文件故障和 helper 模式。私有日志升级有意阻止降级，无公开配置/迁移/依赖变化。已自检上下文图与接口。

评审五点：范围末端未分配 mark 被占用也拒绝；同 key 物理 shape 变化轮换；rename 后失败不返回分配并故障；第二族失败不发布；新 boot/外部修改/重复 key 不复活旧 mark。

## 任务 1：纯生命周期与物理身份

- [x] 实现前先写 TestMarkLifecycle、TestMarkCollisionAndValidation、TestMarkExhaustionAndHistory、TestMarkIsolation、TestPolicyMarkIdentity，对可编译桩观察断言失败。
- [x] 实现 Reconcile(*State,Range,[]Target,[]uint32)(State,error)、State.Validate/Clone、Range.Validate、Target.Validate、Policy.MarkTargets，接口字段以英文计划为准。
- [x] 全量 unit/race/vet/build 退出 0 后提交。

## 任务 2：会话日志与严格占用

- [x] 先写 TestDurableMarks、TestMarkPublicationFailures、TestMarkScopeAndScanFailures、TestMarkHistoryAcrossBoot、TestMarkJournalValidation、TestRawConntrackMarks，覆盖五个重点、固定目录、v1/v2、无变化扫描、异常 wire/第二族失败，并观察失败。
- [x] 实现 Session.ReconcileMarks(ctx,Range,[]Target)(MarkStatus,error)，真实私有 OccupiedMarks 完成两族后才返回；日志持久化后才返回分配，保存故障必须重开。
- [x] 仅提取公共严格 multipart 循环，保留 NFT 行为，增加有界流式 CT；不写/删 mark。
- [x] 全量无缓存 unit/race/vet/build/静态准备冒烟及集成编译退出 0 后提交。

## 任务 3：真实验收与交付

- [x] 新增必需 TestOwnerKernelMarkLifecycle、TestOwnerKernelMarkCollision、TestOwnerKernelMarkProcessRecovery、TestOwnerKernelMarkDrift，在可丢弃 namespace 验证双族 CT、无关流保留、退休/重开、独立进程强杀和漂移；先独立核对测试插入成功。
- [x] 全分支一次独立评审；Important/Critical 先红后绿修复，记录未判定事项和 minor。
- [x] 发布 PR，核对 55 项必需场景、零失败/跳过，下载准确 head 归档校验 SHA-256，同步中英文记录；最终提交和合并后主分支 CI 均通过再完成。
