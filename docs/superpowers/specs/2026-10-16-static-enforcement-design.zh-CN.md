# M2C5：独立静态主机执行

> [English (default)](2026-10-16-static-enforcement-design.md) | 简体中文

将静态已接受发布与 M2C4 引擎连接为可运行的 `picketxd enforce`，保留 prepare 不过滤。范围仍是双栈 host-local/original-destination/graceful TCP/UDP；不宣称 strict、控制器、NFQUEUE/TPROXY、L7、转发/容器、管理界面、服务打包或最终默认全功能启动已完成。

协调器采用构造器注入所有权/存储工厂、状态消费者和标准 slog，复用 Store.Run 的已验证重载、错误保留和状态边界。必须指定 policy、绝对非根 state-dir、provider、agent、destination、mark-first/mark-last；profile 默认 host，poll 默认 5s，once 默认关闭。mark 无默认值，完整非零 uint32 连续范围最多 4096。副作用前校验全部参数。owner.json 与 desired.json 同处强所有权层验证的私有目录，不做自动接管或缓存迁移；依赖受信文件系统和同 UID 特权写入者。

runtime key 包含规范化 Agent/profile/地址边界/mark 范围。Session 新增只读运行范围接口，从持有并重验的命名空间身份返回 BootID/NetworkSpaceID，避免稍后路径查找错绑。由此生成受信 facts，完整输入使用 CompileInput，并由 ApplyPolicy 执行持久替换来源与 mark 检查。

Store.Snapshot 在同一锁内返回状态及不可变候选，禁止把新候选标作旧 generation/hash。首次或版本变化时应用；版本不变只完整 Verify，不重复 CT 扫描。内核绝对时间独立执行未来生效/到期，Go 定时器只更新状态。

JSON 分开报告期望元数据、所有权、应用凭据、最后关联成功的 generation/hash 和顶层 enforcementConfirmed。只有当前已接受版本与成功现场凭据一致、且无缓存故障时顶层才为 true。新期望失败时旧应用可能保留，不能算新期望已生效。应用/核验错误先输出未确认状态再退出，不从磁盘布尔值恢复证明。

无效重载保留已接受状态并继续轮询/报告错误；once 可应用/核验缓存，但当前文件无效仍输出错误并非零退出。没有已接受状态则启动失败；首次坏发布可能留下不挂 hook 的所有权标记，不能放行流量。损坏/来源不符缓存和表拒绝，不替换。模糊应用/漂移报告错误后退出。SIGTERM/SIGINT 释放锁而保留过滤，SIGKILL 通过既有持久日志和完整回读恢复。

文件映射：hostowner/scope 负责持有身份；agentstate/store 增原子 Snapshot；hostagent/config/run 负责校验和组合；agentcli/enforce 与 main 暴露命令；hostagent/process_integration_test 启动真正预编译二进制并观测双栈包；runner/report 增第六份原始流和必需门禁；examples/enforce、README 和验收记录同步双语。依赖及日志模式不变，保留旧 64 项门禁。

先观察 Snapshot、范围生命周期、配置、应用/核验、无效发布保留和错误状态测试失败，再实现并通过完整 unit/race/vet/build/smoke/集成编译和新鲜评审。真实 CI 验证独立二进制双栈 TCP/UDP、重载、进程死亡后时间边界、损坏/来源不符状态、单例、撤销和漂移。显式 opt-in 后失败不跳过；核对全部原始流、产物摘要及最终 PR/main 精确树后完成。
