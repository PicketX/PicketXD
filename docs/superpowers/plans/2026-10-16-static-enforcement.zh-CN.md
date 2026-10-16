# 静态执行实施计划

> 完整接口与步骤：[English plan](2026-10-16-static-enforcement.md)。沿用持续交付授权内联执行，结束时一次新鲜完整分支评审。

目标：以真实独立命令运行和恢复双栈主机准入。显式 enforce，保留 prepare 不过滤；仅 host-local/original-destination/graceful TCP/UDP；绝对私有 state-dir、地址边界和 mark 范围均明确指定；最多 4096 mark，默认轮询 5s；依赖和日志模式不变，保留旧 64 项门禁；只在当前一致版本与现场凭据匹配时确认，关闭不撤销过滤。

1. 先为原子 Store.Snapshot 和持有身份 Session.RuntimeScope 编写失败测试，覆盖并发接受、关闭/故障/重验错误，再实现并通过全套提交。
2. 为配置、首次应用、版本不变核验、候选一致性、无效重载、once 缓存错误、应用/核验/缓存故障及资源关闭写失败测试，再实现注入式 hostagent.Run、JSON/slog 及 CLI；通过 unit/race/vet/build/prepare smoke 后提交。
3. 新增真实预编译二进制包观测：双栈 TCP/UDP、重载/撤销、SIGKILL/绝对时间/重启、范围/缓存/外表拒绝、漂移与单例。编译并新鲜评审，重要问题按 RED→GREEN 修复。发布精确树，核对全部真实门禁和原始产物摘要/版本，记录双语证据；验证最终头后合并并检查 main，清理本计划临时记录。

重点审查：状态回调期间新候选不能错标旧版本；新期望与旧现场必须可区分；once 无效发布保留证明但非零退出；状态范围和 mark 变更/损坏/漂移不能覆盖现场；进程死亡后未来拒绝/到期仍有效，重启不能续期。
