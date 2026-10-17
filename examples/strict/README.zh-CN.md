# 静态 strict 主机策略

> [English (default)](README.md) | 简体中文

在全新兼容的自有 network namespace 中运行，同 namespace 准备 IPv4/IPv6 TCP18443、UDP18444 回显服务。本例只保护 loopback；运行时 Go Netlink，无必需 nft/conntrack 命令。仓库根目录构建并运行：

```sh
CGO_ENABLED=0 go build -o picketxd ./cmd/picketxd
sudo ./picketxd enforce --handling strict \
  --policy examples/strict/policy.yaml --state-dir /var/lib/picketxd-strict \
  --provider static-test --agent node \
  --destination 127.0.0.1 --destination ::1 \
  --mark-first 0x504a0000 --mark-last 0x504a003f --poll 5s
```

完整 mark 范围须独占预留。默认仍 graceful；参数和 Binding 均须选 strict。每个保护报文检查当前来源/端点/绝对有效期，deny 优先。移除最后一个 Grant 保留保护/默认拒绝并阻断已有 TCP/UDP；显式空 Configuration 才移除保护。完整 YAML 应原子 rename 发布。

driver 每次唤醒先完整确认实际规则，再完整双族扫描、原始元组/zone/内核ID精确删除撤回自有流、确认残留0。保留当前独立授权及 unmarked/无关项。每个轮询和已知时间边界均重复全量扫描；可增大 poll 降低周期成本，内核期限与边界唤醒不依赖轮询频率。

strict.cleanupConfirmed 仅表示该次协调，须同时匹配 desired/applied 代数/hash、imageDigest 和 validUntil。不持久，不能证明未观察区间已完成清理。once 只协调一次；关闭/SIGKILL 留下规则，内核仍按期阻断，但无进程不自主删除 conntrack。若清理前权限重新有效，尚存标记 TCP 可能恢复；成功清理后的 TCP 不能仅靠重建无标记 conntrack 复活。UDP 无连接，新有效授权下可恢复。不发送 TCP RST、不承诺对端即时收到断开通知。

旧/不完整历史、pending、漂移、不完整扫描、部分删除、残留或跨边界均不报告严格确认；重开重验/重试。同 boot 模式/context迁移拒绝，不同 state-dir 不能绕过 namespace 所有权；保留旧状态，采用显式部署路径。旧二进制拒绝 strict imagev2。正确时钟、持久文件系统及可信特权写入独占仍为前提。

见[M2C8验收](../../docs/validation/M2C8.zh-CN.md)和[设计](../../docs/superpowers/specs/2026-10-17-strict-enforcement-design.zh-CN.md)。Controller、持久完成恢复、容器/FORWARD、L7 和管理端随后推进。
