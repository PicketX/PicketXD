# M2C6：生产主机定向 conntrack 清理

> [English (default)](2026-10-17-host-revocation-design.md) | 简体中文

本轮交付生产扫描、选择、精确删除引擎，及持有 Session 的入口。命令仍为 graceful；持久累计历史、strict 准入/自动调度、进程失联强制断连与完整 strict 凭据在后续接入。本轮已获用户持续推进及扫描/选择/删除增量授权，按英文设计内联实现并以独立评审及实际 CI 合并。

不可变 Plan 校验并独立复制历史/当前映像与 mark 台账（最多 32 个历史映像、总 JSON 32 MiB）。authority/context 必须一致，所有物理 target/mark 匹配持久台账，当前 mark 不能退休。历史由调用者提供，不自动成为已实际发射或完整历史凭据；Session 强制加入其真实当前映像。所有权要求已知非零 mark、历史物理目标原始元组及历史允许来源，历史到期不抹除 graceful 旧连接所有权。当前允许跨 target/mark 取并集，deny 优先；NotBefore 包含、ExpiresAt 排除。保留未标记、其他来源/目标/mark 及仍被其他授权允许的连接。

Flow 保留双族 TCP/UDP 原始/回复完整元组、zone 和内核四字节不透明 ID。校验 NLA 长度/对齐、重复/缺失字段、族/协议/宽度及嵌套形状；无关元数据只做结构校验，非 TCP/UDP 排除。方向特定 tuple zone 明确不支持并失败，不默认为零。两个族完整 dump 后才删除；每族原始 dump 上限 64 MiB，类型化连接上限 262144。原有 Netlink DONE/序号/PID/截断/中断检查保留。

Run 在扫描前、逐条删除前及确认后重验真实当前映像；逐条刷新时钟与权限，拒绝倒退时钟或 IO 跨当前成员边界，要求重试，不静默成功。该操作不与壁钟构成原子事务，不证明无进程 strict 调度；不得与动态期望更新并发。删除后完整双族重扫，确认不完整 Remaining=-1，残留或删除错误均失败。ENOENT 单列为已不存在，不计实际确认删除。绝不提供全表 flush 或宽泛 mark 删除。

Session.RevokeWithdrawn 持有原互斥、重验 namespace，要求已确认应用及 mark 台账，仅用真实当前映像构造 Plan，不写 nft 或日志。删除通过原严格 socket，携带原始元组、zone、opaque CTA_ID，ID 不匹配时内核拒绝删除替代连接。Linux DELETE 不原子比较回复元组/mark/策略，仍依赖特权写入独占。清理错误使 Session 失效并要求重开；成功只返回限定范围计数，不持久证明 strict 完成。

上下文映射：hostrevoke 的 plan/run 为纯校验/选择/流程；wire_linux 为严格解析/编码；hostowner/cleanup_linux 适配持有的 socket/Session；cleanup_integration_test 在一次性 namespace 验证真实生产路径；runner/report 保留原 71 门禁并加四项。生产不导入 validation 夹具；模块、日志 schema、manifest/CLI 均不变。

验收按英文设计/计划：逐项 RED→GREEN，完整 unit/race/vet/build/prepare smoke、integration 编译、一次全新整分支评审，以及真实双栈 TCP/UDP、DNAT、zone/ID、漂移/预检零删除、部分删除/残留错误。核对最终 PR/main 精确树及下载原始证据。Linux v6.17 nf_conntrack_netlink.c 的 ctnetlink_del_conntrack 为 ID 保护依据；清理本身不阻止 graceful 准入下旧 TCP 重新形成跟踪。
