# 静态主机服务

> [English (default)](README.md) | 简体中文

这是现有 `picketxd enforce` 的运维服务模板。示例在主机网络命名空间以 graceful 模式保护双栈回环 TCP 18443 / UDP 18444。参见[执行语义](../../examples/enforce/README.zh-CN.md)、[strict 限制](../../examples/strict/README.zh-CN.md)及[验收](../../docs/validation/M2C10.zh-CN.md)。Controller、L7 和开机防火墙尚未完成。

## 首次安装

使用验收记录中的 Linux/systemd 环境，以固定 Go 工具链构建；运行时不需要 Go、C 库或 nft/conntrack 命令。以下安装回环示例，显式启动后才启用保护。

```sh
CGO_ENABLED=0 go build -trimpath -o /tmp/picketxd ./cmd/picketxd
sudo install -d -m 0755 /usr/local/libexec
sudo install -m 0755 /tmp/picketxd /usr/local/libexec/picketxd
sudo install -d -m 0700 /etc/picketx
sudo install -m 0600 examples/enforce/policy.yaml /etc/picketx/policy.yaml
sudo install -m 0644 deploy/systemd/picketxd.service /etc/systemd/system/picketxd.service
sudoedit /etc/systemd/system/picketxd.service
```

首次启动前审查完整策略及 **ExecStart 每个参数**：稳定 provider/Agent/profile、精确目标、模式、轮询及运维独占保留的完整 mark 范围。模板范围仅为示例；Binding agent/profile/目标/模式须匹配。容器/FORWARD 不在主机 INPUT/原始目标覆盖内。strict 受保护地址/端口对须按 strict 指南专供服务使用。

**全新 strict 部署**须首次启动前同时选择 `--handling strict` 和匹配的 `examples/strict/policy.yaml`。已有/留存 graceful 部署不能通过改参数、身份或删除状态迁移；兼容迁移需另行规划。

```sh
sudo systemd-analyze verify /etc/systemd/system/picketxd.service
sudo systemctl daemon-reload
sudo systemctl enable --now picketxd.service
sudo systemctl status picketxd.service
sudo journalctl -u picketxd.service -o cat -n 30 --no-pager
```

`Type=exec` 仅确认进程执行。**active 不等于策略就绪。** 检查最新 Agent JSON：enforcementConfirmed 为 true，desired/applied generation 与 hash 对应，无 desired.lastError/lastApplyError，strict 还需当前清理确认。持续核对时间与后续错误，旧日志不证明当前健康。拒绝无效发布、保留已接受策略时进程仍可 active。目前没有 sd_notify 就绪协议、健康端点或受保护应用的就绪排序。

After=network.target 不证明地址、时钟同步或实际应用。新命名空间/重启到确认安装之间有窗口，部署流程须在真实确认前保持受保护应用不可用。留存内核规则不会自行跨系统重启保留。

## 状态与权限

systemd 创建 root 所有、0700 的 `/var/lib/picketx` 持久目录及 `/run/picketx` 锁目录。升级时保留 **owner.json 和 desired.json 整体**、路径及权限。不能通过重命名、删除、覆盖运行中状态绕过错误。RuntimeDirectoryPreserve 停止时保留锁；重启可清空 `/run`。有 Agent 使用时不得移除锁目录。

root 进程能力边界仅含 CAP_NET_ADMIN，设置 NoNewPrivileges，除声明状态/运行目录外文件系统只读，并有私有临时目录、内核文件保护及套接字族限制。AF_UNIX 用于命名空间所有权，AF_NETLINK 用于 nft/conntrack，另允许 IPv4/IPv6。进程共享主机网络命名空间，不要添加会改变执行命名空间/能力的 PrivateNetwork/PrivateUsers。仍依赖可信特权写入者、正确时钟及文件系统持久性。

## 发布与运维

审查 new-policy.yaml 后，在同一目录原子替换完整 YAML：

```sh
sudo install -m 0600 new-policy.yaml /etc/picketx/policy.next
sudo mv /etc/picketx/policy.next /etc/picketx/policy.yaml
sudo journalctl -fu picketxd.service -o cat
```

有效替换经轮询应用，无须重启。无效替换保留已接受状态及绝对截止时间；修复后核对新状态。没有 ExecReload，systemctl reload 不能发布策略。

```sh
sudo systemctl restart picketxd.service
sudo systemctl stop picketxd.service
```

重启核对留存镜像并打开相同状态，不延长截止时间。停止发送 SIGTERM、**保留过滤**，不撤回保护或清除旧 graceful 连接。撤回须运行时发布显式空 Configuration，确认应用后再停止，参见执行示例；不能全局清空防火墙/连接状态。

异常失败（含 SIGKILL）5 秒后重试，60 秒内最多启动 3 次。持续失败可使服务 failed、旧策略仍留存。修复日志指出的问题后 reset-failed 再 start。自动重启不提供有界撤权保证。strict 前置清理失败也阻止限制性候选，旧权限（可能永久）保留到成功重试；跨未观察间隔的持久撤销仍未完成。

升级先构建验证新二进制，安装至同级临时路径、原子重命名覆盖 `/usr/local/libexec/picketxd`，重启并检查新状态。保留策略、身份、目标、mark 与状态。模板编辑经审查后 daemon-reload。保留兼容旧二进制，旧版本可能拒绝 journal v4；只能回退到理解当前日志/镜像的程序。不能以旧日志覆盖新规则进行回退。

## 开发验证

```sh
scripts/verify-systemd.sh
# 具有真实系统管理器和 iproute2 的可丢弃特权主机：
CGO_ENABLED=0 go build -o /tmp/picketxd ./cmd/picketxd
sudo env PICKETX_SYSTEMD_TESTS=1 python3 scripts/test-systemd.py \
  --binary /tmp/picketxd --artifacts "$PWD/artifacts"
```

离线检查临时替换可执行文件路径，仅验证语法。原生验收复制模板为唯一临时服务，只替换测试路径/状态/轮询并加入全新隔离 NetworkNamespacePath，保留权限限制及重启设置，检查实际能力/命名空间、双栈 TCP/UDP、异常自动恢复、无效发布保留、拒绝性重载、停止保留及日志字节一致恢复。不安装生产服务、不更改宿主防火墙。显式选择后初始化/权限错误均失败、不跳过。CI 将服务报告/日志与既有原生证据归档。

参考上游 [systemd.service](https://www.freedesktop.org/software/systemd/man/systemd.service.html)、[systemd.exec](https://www.freedesktop.org/software/systemd/man/systemd.exec.html)。
