# 静态准备与重启

> [English (default)](README.md) | 简体中文

`picketxd prepare` 是实际的准备/恢复循环，**不执行流量控制**。JSON 只报告期望状态准备，enforcementConfirmed 始终 false。主机执行目前仅在隔离内核测试验证，不能用本二进制保护服务。

```sh
go run ./cmd/picketxd prepare --once \
  --policy examples/prepare/policy.yaml \
  --state /var/lib/picketx-prepare/accepted.json \
  --provider static-example --agent agent-a \
  --destination 192.0.2.100 --destination 2001:db8::100
```

状态目录必须可写且私有；不存在的末级目录创建为 0700，lock/cache 为 0600。本地实验将 state 改到自己账户的私有目录；准备不需要 root/网络特权。destination 是显式准备边界，不是已验证的现场执行事实。示例使用 2026-10-15 绝对期限；过期配置仍可合法表示零有效权限。

去掉 once 后默认每 5 秒扫描，未来开始/到期边界独立触发状态求值。Ctrl-C/SIGTERM 先终止循环再释放排他锁。完整 YAML 文件应原子 rename 发布；空/截断/损坏不是撤回，完整撤回需共享显式空 Configuration。

用相同 provider、Agent、profile/目标参数及 state 路径重启。缓存重新校验、按当前时间重新编译，原期限不变；restored、sameBoot、generation、acceptedAt、lastAttemptAt、lastError 解释恢复和新鲜度。无效重载保留前一有效期望状态，错误持续到有效重载；once 当前文件失败非零退出，即使显示有效缓存。损坏/来源不符缓存直接启动失败，需要运维检查，不会静默覆盖。缓存是内部规范格式，不是手写策略 API。

目录及 lock/cache 必须由当前用户拥有，不能允许组/其他人访问，缓存符号链接/特殊文件失败。同一路径只能一个进程。切换权威、Agent 或主机配置应明确使用独立状态路径，不合并来源。boot 元数据不证明规则已应用。尚未实现 Controller 模式、监听、严格清理、目录监听、Koanf 进程文件加载、七层或管理端。
