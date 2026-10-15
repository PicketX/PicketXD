# M2B 静态准备服务与主机 INPUT 验证

> [English（默认）](M2B.md) | 简体中文

[PR #4](https://github.com/PicketX/PicketXD/pull/4) 叠加于桥接 PR #3，提供可运行的静态准备服务，并新增七项必跑的真实主机 INPUT 场景。`picketxd prepare` 通过共享 M2A 编译器加载完整 YAML，持久化与提供者绑定的私有上次有效期望状态，支持重启恢复、轮询替换，以及不受重载错误影响的绝对时间边界投影。参见[运行示例](../../examples/prepare/README.zh-CN.md)。

命令报告 `prepared`、`enforcementConfirmed: false`，不修改内核。生产表/标记所有权、严格回收恢复、进程丢失期间未来 deny 的保证、容器路径、认证 Controller 同步、七层与管理端仍待后续实现。以下主机执行仅用于验证；缓存期望状态和审批都不是应用回执。

## 实测内核证据

[运行 38759897095 的 kernel 作业](https://github.com/PicketX/PicketXD/actions/runs/38759897095) 在源码 `04335f0d48555c9ceb5f38d1e795a0c0c9447986` 上通过全部 **17 项 M0 + 19 项桥接 + 7 项主机必跑场景**，共 **63 项顶层测试通过、零失败、零跳过**。这是一个实测 Linux 命名空间范围，不代表发行版矩阵或生产部署覆盖。该轮 unit 作业因旧 FIFO 测试将整个启动限制在 150ms 而失败；修正后单独检查日志非阻塞读取，再独立验证启动拒绝。标记就绪前还必须通过最终提交 CI。

实测 Linux **6.17.0-1022-azure**、Go **1.27.0**、共享配置 **v0.0.0-20261015145159-81daf5abb831**、nftables **v0.3.1-0.20250606091825-8a8ad2be8103**。[机器可读证据](M2B-evidence.json) 记录完整模块版本与场景状态。下载的制品 `11984658856` 含八份证据文件，ZIP SHA-256 已与 GitHub 元数据核对：`af035508311fa8282c005b3b1a2615eefb2c225d32dfa3bc3ba8e5e7b6c282c9`。

| 必跑主机场景 | 实测内容 |
| --- | --- |
| ConfigurationDefaultDeny | 双栈实际本机 INPUT TCP/UDP 拒绝，无关控制端口保持可达 |
| ConfigurationSourcesAndDeny | 精确地址与重叠 CIDR 并集、精确授权撤销、显式 deny 优先 |
| ConfigurationExpiryWithInvalidReload | 拒绝错误替换仍保留期望状态，不延长原生绝对到期；独立 IPv6 贡献保持有效 |
| ConfigurationRecoveryDeadline | 重开缓存与重放保留原始截止时间；恢复不报告执行成功 |
| OriginalDestinationDNAT | 转换到本机 TCP/8000 后仍按原始 TCP/8001 授权；直接访问未管理端口保持可达 |
| UntrackedProtectedTraffic | 已证明未过滤时可达的 NOTRACK TCP 流量，在受保护当前目标上被丢弃 |
| InputDoesNotClaimTransit | 路由器命名空间 INPUT 规则不声称覆盖或阻断转发服务 |

主机场景统一使用 `TestHost` 前缀。解释拒绝结果前先验证双栈未过滤 TCP/UDP/控制路径，包断言前读取核对实际应用回执。原有验收全部保留；缺失、失败、重复或跳过必跑场景均拒绝验收。策略内嵌进预编译测试程序，运行脚本以 `cmp` 防止它偏离用户示例。

## 恢复与审阅

一次独立审阅发现首次创建状态目录时缺少父目录持久化。现在启动同步状态目录及所有祖先目录，也覆盖创建后同步不确定的重试。确定性故障测试覆盖新建路径与真实 rename 后注入目录同步失败：内存代数不推进，状态存储进入故障状态，拒绝后续接受，重新打开后校验实际发布的规范缓存。这证明操作和错误语义，不代表真实断电试验。

本地 Go 测试、race、vet、build、静态 ELF 检查均通过，包含独立进程接受、重启、错误发布保留。以本地 overlay 移除 `O_NONBLOCK` 后，修正的 FIFO 回归测试确实失败；正常源码通过。nftables 升级修复上游将内核 u8 CT 方向属性误作 u32 解码的问题，并提供分地址族原始地址键。使用固定版本通过全部 43 项必跑内核场景，没有弱化回读核验。
