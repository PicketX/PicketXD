# M2C7：持久紧凑清理历史

> [English (default)](2026-10-17-durable-history-design.md) | 简体中文

目标是在重启后无需调用者旧映像即可清理历史连接。本轮仍不增加自动清理、strict 准入/调度/失联保证或 Controller/L7/管理端；保留三个已延期 Minor。持续授权涵盖内联实现、推送和通过验收后的合并。

采用 hostrevoke.History 保存 authority/context、按 mark 排序的物理身份/精确物理 context/保护端点和去重 `(source,endpoint)`，忽略所有权不需要的 Grant ID/期限/完整输入。当前完整映像与替换基线不变。只在实际现场确认后持久记入，已安装的未来 Grant 也可能无进程激活，须保留潜在所有权；不按到期或清理成功遗忘、不复用 mark。重复 UID/期限续期不增长。上限4096目标、65536成员、8MiB JSON，溢出在 mark/nft 效果前拒绝。

目标精确 Protection.Context 与可能更宽的可信映像范围分开；抽取与现有 MarkTargets 完全相同的物理哈希方法；校验规范前缀/族、保护端点、context 范围、排序去重及含退休实例的精确 mark 台账。Remember 返回独立值，NewRecorded 验证/克隆并复用当前权限选择。

日志 v4 在 application 增加内部 `{complete,state}`，继续读取 v1-v3；旧二进制拒绝 v4，不支持原地降级。首次新应用 complete=true；旧 v3 仅补已知确认映像，complete=false，同 boot 不自动变完整。no-op 可仅升级元数据，不增加应用次数/写 nft。真实新 boot 且旧表不在时，旧 conntrack 不可能跨启动保留，清空历史为完整新覆盖；mark 退休和逻辑替换基线保留。新 boot 行为通过注入单测验证，不声称 CI 重启主机。

效果前预览历史合并与上限；pending 仅持久保留旧已确认历史及独立 pending 映像。原子应用、完整回读/重验后，将候选历史与 confirmed 同一次持久发布。恢复认出 pending 才合并，认出旧映像则不记被拒候选。v4 确认映像须被历史覆盖，pending 预览必须合法可容纳；损坏先于启动效果拒绝。同 boot context 迁移在效果前拒绝，不损坏既有历史。

HistoryStatus 返回仅值元数据 Tracked/Complete/Targets/Members/Bytes/Digest；Complete 还要求开放、无故障、无 pending、当前现场已确认，不是 strict 清理凭据。hostagent JSON 加入此元数据。Session.RevokeRecorded 不接受外部历史，持有锁并要求 v4 完整覆盖及真实当前映像，复用原完整双族扫描/精确 ID 删除/重验流程。旧/不完整/歧义先于删除失败；不修剪历史、不写日志，不声称 strict 完成。被拒替换后现场当前映像可绑定已知退休 mark；记录式清理不回收 mark，仍确认真实现场。显式 RevokeWithdrawn 保留当前 mark 活跃限制。

文件映射与接口详见英文设计/计划：hostpolicy 身份助手；hostrevoke 历史/记录 Plan；hostowner 历史状态、v4 应用恢复、记录清理；hostagent 状态；五项真实门禁及双语文档。无依赖/manifest/CLI/映像 schema 变化，日志仍64MiB上限。

验收逐项 RED→GREEN，完整 unit/race/vet/build/prepare smoke、integration 编译、一次全新评审，五项新原生门禁（多代重启、pending成功/拒绝、旧历史拒绝、损坏拒绝、父进程持双栈连接时助手 SIGKILL）保留原75项。核验实际原始流/计数/摘要/模块/构建版本和最终 PR/main 精确树；不依据 mock 或编译宣称内核兼容。
