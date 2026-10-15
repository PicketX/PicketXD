# M2B：静态准备流程与主机 INPUT 验证

> [English (default)](2026-10-15-static-lifecycle.md) | 简体中文

本架构增量按用户持续交付授权在本会话实施，结束一次独立审阅，不重复阶段确认。

交付 `picketxd prepare`：单文件完整 YAML 静态权威、共享严格编译替换、来源绑定的持久有效期望状态、重启恢复、有界读取、文件锁、可取消轮询和绝对时间投影。状态明确 prepared、enforcementConfirmed=false，不宣称生产执行。

同时在隔离命名空间真实验证主机 INPUT 路径：HostInputFixture 使用原始 conntrack 元组（含 DNAT），禁止混用原 NAT 夹具，先按当前受保护目标拒绝 invalid/untracked。既有 M0/transit 桥接不能改名冒充生产。生产表/mark 归属、严格清理崩溃恢复、失联时未来 deny、容器发布、传输/七层/管理端后续交付。

agentstate 固定 ProviderID、AgentRef、独立 Facts、RuntimeKey、BootID、cache、注入时钟；事实必须匹配权威及 Agent。命令事实来自显式主机目标范围/profile、当前 boot/namespace；只作准备，不证明执行能力。无 Controller 合并。运行指纹绑定主机/profile/目标，boot/namespace 是恢复元数据而非回执。

目录 0700，当前用户拥有的 lock/cache 为 0600 普通文件，拒绝符号链接，排他非阻塞 flock 直到 Close。缓存 v1 保存 static 类型/身份、Agent、运行指纹、boot、接受代次/时刻、规范策略哈希和完整原 YAML。规范 JSON 拒绝改写/重复/null/未知/尾随表示、来源/Agent/指纹不符、零代次/时刻、损坏哈希和无效策略。当前事实重新编译，不延期，不恢复 applied 成功；不同 boot 明确报告。无缓存允许首次启动，损坏/身份不符失败，不隐式重置权威。

完整解码/替换后，temp+fsync+rename+目录 fsync 成功才推进内存。已知发布前失败保留旧状态；发布后持久结果不确定则故障锁定直到重新打开，不声称旧磁盘仍权威。同哈希不增代次/重写，有效等价重载清除错误；拒绝候选保留绝对期限。尝试/错误与 accepted 时间/哈希/代次分开。每次状态显式时间求值，含未来边界，不缓存应用成功或伪造回执。

单一可取消 Timer 循环，不重叠重载；即使重载全失败，边界仍按最新候选投影，不捕获旧代次权限。取消并等待退出再关闭，Close 释放锁。

命令：`picketxd prepare --policy FILE --state FILE --provider ID --agent NAME --destination IP/CIDR [--destination ...] [--profile host] [--poll 5s] [--once]`。Cobra 显式参数，Koanf 进程文件加载仍待实现。无监听、内核修改或凭据。JSON 行 stdout、诊断 stderr；once 当前文件失败非零退出，即使显示有效缓存；持续模式可保留缓存继续，但持续报告错误直到修复。首次无缓存无效策略失败。仅 host-local/original-destination/graceful 准备能力，其余 Binding 由共享校验拒绝。

真实门禁从共享 YAML、已接受状态/恢复进入 server namespace，读回确认后声明应用；先证明双栈未过滤通路。新增必需 INPUT 默认拒绝、来源/deny、无效重载原生到期、恢复不延期、DNAT 原端口、untracked、INPUT/transit 分离；原 17 M0+19 bridge 不删，显式启用后环境失败不得 skip。

文件职责和精确执行顺序以[英文说明](2026-10-15-static-lifecycle.md)为主：状态 RED→GREEN/全量提交；循环命令 RED→GREEN/静态检查；host 表达式和真实场景；全量 test/race/vet/build、一次审阅、失败回归、推送及实际 CI。
