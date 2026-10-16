# M2C3：持久 conntrack mark 保留与分配

> [English (default)](2026-10-16-mark-reservations-design.md) | 简体中文

承接 PR #6 / 主分支 `201b79db07942f15bda8d473441d6aee248e75e7`，完成生产准入前的 mark 生命周期及持久 namespace 边界。沿用持续推进授权，当前会话实施，最后一次独立评审。

操作者明确保留连续非零的完整 32 位 conntrack mark 值，容量 1..4096，不共享 mask、不提供自动默认值。安装生命周期内单调分配，永久保存退休记录，耗尽明确报错。牺牲回收能力以防旧 graceful 连接在目标重建后重新取得权限；回收和更大库存另行实现。

沿用已有持锁、描述符相对的所有权日志，不增加可丢失/换路径的独立 mark 文件。兼容 v1；首次保留原子持久发布带完整库存的 v2。v2 必须带有效库存，v1 禁止带库存，旧二进制拒绝 v2。内核仍仅含 v1 owner 标记，日志版本变化不是实际规则结构契约。

## 纯生命周期接口

markstate 定义 Range{First,Last uint32}、Target{Key networkplan.TargetKey,Shape string}、Assignment{Target,Mark,Retired}、State{Range,Entries}。Shape 为规范小写 SHA-256；key 各身份非空、有效 UTF-8、至多 256 字节、无 NUL。

Reconcile(previous *State,reservation Range,targets []Target,occupied []uint32)(State,error) 完整校验后返回独立数据，不修改输入。拒绝目标 key 重复、非法历史、变更保留范围、范围内任何未分配值被占用及耗尽。已分配（含退休）mark 可有残存连接；范围外 mark 不干扰。首次使用要求整个保留范围空闲，而非仅所需第一个值。

按完整 key 排序分配。Entries 只能追加，Mark 等于 First+index，不允许空洞/重排/复用；每 key 至多一个未退休条目。同 key/shape 保留活动 mark；移除则退休，再引入相同目标也分配新值；物理 shape 改变也退休旧值并重新分配。失败不返回部分库存，不修改旧记录。

hostpolicy.Policy.MarkTargets() 从完整库存提取目标，hash 完整已解析上下文和物理匹配，不包含成员、期限和 Binding ID。普通续期、到期、叠加 Binding 不轮换 mark，Resource UID 保留在 key，UID 或物理匹配变化不复用旧权限。返回数据独立。

## 会话与持久边界

Session.ReconcileMarks(ctx,Range,[]Target)(MarkStatus,error) 在已持有的所有权会话内串行：校验 AgentRef，重新确认精确仅标记结构，完整扫描 IPv4 与 IPv6 conntrack，计算库存，再确认所有权，最后将 v2 原子持久发布后才返回分配。各阶段检查取消和已固定 boot/namespace。无变化也必须重新扫描两族和所有权，可不重写已持久库存。

Session 保留已打开目录的日志 writer。任何保存错误（含 rename 后 fsync 不确定）都使会话故障，不返回分配，须关闭重开。重启验证 scope、实际标记和库存，不仅信任磁盘确认。新 boot 保留安装的范围/退休历史，并正常生成新 epoch，不宣称恢复已应用策略。存在标记而日志缺失/损坏仍拒绝。日志上限增至 32 MiB，继续要求规范 JSON、私有常规文件、描述符相对发布。

MarkStatus 为独立 State 和新鲜 ObservedAt，不是应用凭据。执行确认始终 false。本批不安装准入规则、不写/删 conntrack mark、不加过滤命令或策略应用日志。后续引擎写 mark 前必须依赖本库存，并在扩展仅标记所有权前落实已应用规则结构与历史。

## 严格原生冲突读回

复用固定 namespace 的原始 Netlink inspector，仅提取公共 multipart 接收循环。NFT 保留 4 MiB 上限；conntrack 对 AF_INET、AF_INET6 依次发 CT_GET、读 CT_NEW，每族 64 MiB 上限，流式处理、不保留完整流表。验证 sequence/PID、内核发送方、截断、完成、状态、中断 data/DONE、响应类型、nfgen family/version。完整解析顶层属性封套，拒绝重复/非法 mark 属性，CTA_MARK 按大端 u32 读取，缺失视为 0；其他流字段内容保持不透明，只证明 mark 占用，不证明元组所有权。

即使第一族已发现冲突，也必须完成两族再返回占用或发布。上限/权限/不支持/取消/中断/畸形错误均失败，不能当空成功。每次 dump 仍有五秒期限。快照不能防御其他特权写入者，仍要求操作者保留范围和可信 NET_ADMIN 管理；相同数字 mark 不能认证连接创建者。

## 验收

覆盖确定性分配、u32 上界、重复/非法输入、全范围冲突、范围不匹配、退休/重建、shape 变更、耗尽、历史损坏和修改隔离。策略测试覆盖 Grant/Binding 稳定性与 UID/物理变更。真实文件测试覆盖 v1/v2 升级/重开、rename 前后保存故障、不返回部分结果、无变化仍扫描、取消/关闭/漂移、新 boot 保留退休记录、大型有界日志；wire 测试覆盖双族、零/缺失/范围外 mark、重复/畸形属性和中断完成。

保留原 51 项必需门禁，新增四项 TestOwnerKernelMarkLifecycle、TestOwnerKernelMarkCollision、TestOwnerKernelMarkProcessRecovery、TestOwnerKernelMarkDrift。真实双族 CT 插入/读回、无关流保留、退休/重启、独立 helper 未 Close 被强杀、漂移拒绝均在可丢弃 namespace 验证。显式 opt-in 后权限失败必须失败。真实 CI 与准确 head 归档下载校验后才能合并。
