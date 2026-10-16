# M2C1：持久主机所有权

> [English (default)](2026-10-16-host-ownership-design.md) | 简体中文

用户已授权在清理全部积压 PR 后持续实施。本切片落实[生产简报](2026-10-16-production-l4-readiness.zh-CN.md)中的所有权前置条件，产生实际内核所有权，不执行流量保护。直接实施，结束后一次独立评审。

`internal/hostowner.Open(ctx, Config)` 获取 namespace 级锁、恢复私有所有权日志、观察实际表，持久记录尝试，在适当情况下排他创建仅含标记的表，再读回并持久确认。`Session.Close` 释放描述符/锁，不删除表；`Session.Verify(ctx)` 重新观察，过去检查成功不能持续证明当前内核状态。`Status` 始终报告 `enforcementConfirmed=false`。

配置固定 StatePath、ProviderID、AgentRef、RuntimeKey。持有 `/proc/thread-self/ns/net` 描述符，绑定 boot ID 与设备/inode。抽象 Unix datagram socket 将单实例绑定到网络 namespace，即使不同挂载 namespace 也无法绕过。运行文件锁名称也取决于 boot 和 namespace，不取决于状态文件路径；生产锁根目录固定 `/run/picketx`。同时锁住日志，防止不同 namespace 竞争同一状态文件。要求当前用户私有的常规文件/目录、受限非阻塞读取、管理目录组件不穿越符号链接、规范 JSON、完整目录/文件同步；沿用既有持久语义，不改期望状态日志格式。

内核标记是 `inet picketx` 内无 hook 的 `picketx__owner` chain 中唯一的 counter 规则。UserData 使用有界规范 ASCII 编码协议版本、随机安装 ID、范围哈希、随机 epoch token。范围哈希绑定 provider/Agent/runtime，epoch 绑定实际 boot/namespace。完整检查表：仅此 chain/rule，无 hook、set、object、flowtable，默认表 flags。出现未知额外结构则报错。标记是在可信 NET_ADMIN 下检测冲突，不能对抗其它特权管理器。本切片不分配 conntrack mark、不安装 hook/filter、不删除内核对象。

日志包含版本、明确 provider/Agent/runtime 身份、安装 ID、boot ID、namespace 设备/inode、epoch token，以及 `pending`/`confirmed` 阶段。使用前校验全部字段和规范编码。发送前先持久 pending，实际结构读回后才能保存 confirmed。持久结果不确定时停止并要求重新打开；pending 持久成功前不执行内核修改。create/ACK/读回失败保留可恢复 pending，不通过删除回滚。

| 恢复日志 | 实际表 | 结果 |
| --- | --- | --- |
| 不存在 | 不存在 | 生成并持久 pending，排他创建、读回、确认 |
| 不存在 | 存在 | 拒绝；不按表名认领 |
| Pending，范围/namespace 匹配 | 不存在 | 重试同一 epoch；发送前已有持久尝试 |
| Pending，范围/namespace 匹配 | 标记精确匹配 | 通过新读回确认，再持久 confirmed |
| Confirmed，范围/namespace 匹配 | 标记精确匹配 | 重新观察，不重写表；磁盘确认本身不够 |
| Confirmed，相同 boot/namespace | 不存在 | 拒绝漂移，不静默重建 |
| 任意 | 外来标记或不支持结构 | 不修改并拒绝 |
| 任意，相同 boot 不同 namespace | 任意 | 拒绝身份错配 |
| 任意，新 boot | 不存在 | 保留安装 ID，新建绑定当前 namespace 的 epoch，持久 pending 后只认领空标记表；不声称恢复策略或已应用状态 |
| 任意，新 boot | 存在 | 拒绝；旧 boot 身份不能授权接管 |

全部 dump 必须完整成功。在同一个锁定 OS 线程创建 namespace 描述符、抽象锁和持久 nftables 连接；socket 此后始终绑定该 namespace，无需 setns 或 CAP_SYS_ADMIN。使用排他 `CreateTable`，检查与发送间出现表时整批失败。Close 只报告资源释放错误，不声称移除保护。每个新阶段前检查取消；不承诺能立即中断正在执行的 Netlink 调用。结构检查前后两次观察表以减少普通漂移歧义，不承诺对抗特权写入者时具有线性一致性。

验收：真实文件系统锁/持久/重新打开，pending 保存、发送、读回和 confirmed 保存的失败注入；外来/同名/另一状态路径/并发调用；新 boot 区分；受限私有文件拒绝；独立进程强制退出释放锁；真实 namespace/表读回及排他创建竞争。保留既有 43 项必需内核场景，新增所有权必需证据门禁。`prepare` 不变；后续执行功能须先定义结构版本，才能加入 chain、mark 分配或清理。
