# 静态主机执行

> [English (default)](README.md) | 简体中文

先构建，再在需要保护 INPUT 流量的网络命名空间内运行独立静态二进制。示例仅保护 loopback TCP 18443 和 UDP 18444，双栈同等支持，永久允许本机来源；实际部署需替换完整策略及明确目标边界。

```sh
CGO_ENABLED=0 go build -o /tmp/picketxd ./cmd/picketxd
sudo /tmp/picketxd enforce \
  --policy "$PWD/examples/enforce/policy.yaml" \
  --state-dir /var/lib/picketx-demo \
  --provider static-demo --agent node \
  --destination 127.0.0.1 --destination ::1 \
  --mark-first 0x504a0000 --mark-last 0x504a003f
```

运行前明确预留整个 mark 范围；这些数字只是示例，无生产默认值。需要 Linux 网络管理能力及私有状态目录，运行不依赖 Go、C 库或 nft/conntrack 命令。只在完整核验所有权后修改 inet picketx，拒绝外表、损坏/来源不符日志及同命名空间第二所有者（包括不同状态路径）。首次坏发布可能留下不挂 hook、不能放行权限的所有权标记。

目录含 owner.json（所有权、mark 退休、待执行/确认映像与替换历史）和 desired.json（已接受静态发布），两份都应保留。重启需同命名空间、provider、Agent、profile、规范化地址范围及 mark 范围；变更不是自动接管或迁移。不要让其他写入者操作这些文件，运行中不重命名目录。

JSON 行分开报告 desired 的接受/错误/新鲜度、ownership、application，以及 appliedGeneration/appliedPolicyHash。顶层 enforcementConfirmed=true 表示当前已接受 generation/hash 与完整现场证明关联；旧应用可能仍在 application 内确认，但新期望应用失败时顶层为 false。坏文件是拒绝发布，不是撤销，旧策略及原始绝对有效期保留。模糊应用或漂移报告错误并退出，保留已有规则。

完整 YAML 应通过原子 rename 发布。默认每 5s 重载/现场核验，可用 poll 修改；内核绝对有效期保证进程消失后未来生效/到期仍有效。最后一个 Grant 消失仍默认拒绝，已标记 graceful 连接保留。撤销相关 Binding，或显式发布以下空 Configuration，才撤销全部自有保护：

```yaml
apiVersion: picketx.io/v1alpha1
kind: Configuration
metadata: {name: empty, uid: empty-v1}
spec: {empty: true}
```

SIGINT/SIGTERM 释放资源和锁而保留过滤；SIGKILL 同样保留过滤。重启严格完整回读后确认，不延长有效期。once 应用/核验并输出一份状态后退出，规则保留；若当前文件坏但缓存有效，once 可确认缓存，仍非零退出并报告 desired.lastError。prepare 保持不执行过滤。

当前仅静态 host-local/original-destination/graceful TCP/UDP。strict 撤销、控制器、NFQUEUE/TPROXY、L7、转发/容器、服务打包及管理界面后续接入。内核兼容性只以真实验收记录为准。
