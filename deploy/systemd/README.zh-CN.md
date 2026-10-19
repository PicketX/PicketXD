# 主机策略服务

> [English (default)](README.md) | 简体中文

这是现有 `picketxd enforce` 的运维服务模板。示例在主机网络命名空间以 graceful 模式保护双栈回环 TCP 18443 / UDP 18444。参见[执行语义](../../examples/enforce/README.zh-CN.md)、[strict 限制](../../examples/strict/README.zh-CN.md)及[Controller配对](../controller/README.zh-CN.md)。这些单元不代表开机防火墙或完整部署验收。

## 只选择一种来源

`picketxd.service`读取静态策略；`picketxd-controller.service`使用官方gRPC与私有
Agent Token文件，不读取策略文件。两者共用`/var/lib/picketx`和`/run/picketx`，
**绝不能同时启用**。模板需要运维审阅，不是自动登记或来源模式迁移；不能通过删除
留存状态、改变provider强行切换。

全新Controller部署先完成配对指南，按下节安装已验证Agent二进制，但以以下步骤替代
静态策略及静态unit安装：

```sh
sudo install -d -m 0700 /etc/picketx
# 仅复制生成的Agent凭据，不是bootstrap或入口密钥。
sudo install -m 0600 /private/controller/agent-token /etc/picketx/agent-token
sudo install -m 0644 deploy/systemd/picketxd-controller.service /etc/systemd/system/picketxd-controller.service
sudoedit /etc/systemd/system/picketxd-controller.service
```

将`controller:REPLACE_WITH_GENERATED_AUTHORITY`换成初始化后真实公开的`agent.authority`，
逐项核对Agent ref、状态路径、目的范围、独占mark及处理模式，与管理事实/Binding匹配。
模板明文仅限可信同机回环。正常校验证书的TLS监听须移除`--controller-plaintext`并设置
实际地址；必要时加`--controller-server-name`和`--controller-ca /etc/picketx/controller-ca.pem`，
不能绕过证书校验，不要求mTLS。检查沙箱中的Token/CA文件与父目录权限，不放到受保护home下。

审阅后才verify、daemon-reload并显式启用/启动`picketxd-controller.service`；下方状态、
日志和应用依赖命令使用同一服务名。发布走鉴权管理API，不是本地YAML轮询或reload。

断连时provider绑定的已接受LKG只能保留权限到原有绝对期限。重启可以确认缓存，但只有
干净的当前远程发布确认后才READY；仅有LKG仍是activating。重连不续期Grant、不复活
过期权限。核对新鲜本地JSON与当前Controller APPLIED修订/哈希；审批、下发、active或
旧回执都不等于应用。

## 首次静态安装

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

`Type=notify` 配合显式 `--systemd-notify`，只在 Agent 输出首次无错误的当前策略确认、且 systemd 收到 READY=1 后完成启动。驱动确认包含当前 desired/applied generation/hash、所有权、完整实际应用和启用时的 strict 当前清理。即便有效缓存镜像仍确认，当前发布被拒绝也不放行启动。最多等待 60 秒，然后超时并按失败重试策略恢复；速率限制只约束快速失败，间隔足够大的超时可能持续重试。

**就绪是一次启动事件，不是持续健康保证。** 启动后发布被拒绝仍可 active 并保留旧策略，STATUS 会解释状态。持续检查最新 JSON 时间、enforcementConfirmed、desired/applied generation/hash、desired.lastError/lastApplyError 及 strict 当前清理；旧成功日志或 active 本身不证明当前健康。没有 watchdog 或健康端点。

要让受保护应用等待策略就绪，在该应用服务中添加经运维审查的 drop-in，再 daemon-reload：

```ini
[Unit]
Requires=picketxd.service
After=picketxd.service
```

两项都需要：After 排序，Requires 将 Agent 纳入启动事务；Agent 启动失败会阻止该有序应用启动。每个相关应用都须配置并检查结果；这不能阻止手动/外部启动，也不会在后续发布被拒绝时自动停止应用。安装 drop-in 不会重启已运行应用。

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

重启核对留存镜像、打开相同状态并等待新的干净就绪，不延长截止时间。停止发送 SIGTERM、**保留过滤**，不撤回保护或清除旧 graceful 连接。撤回须运行时发布显式空 Configuration，确认应用后再停止，参见执行示例；不能全局清空防火墙/连接状态。

异常失败（含 SIGKILL）5 秒后重试，60 秒内最多启动 3 次。持续失败可使服务 failed、旧策略仍留存。修复日志指出的问题后 reset-failed 再 start。自动重启不提供有界撤权保证。strict 前置清理失败也阻止限制性候选，旧权限（可能永久）保留到成功重试；跨未观察间隔的持久撤销仍未完成。

升级先构建验证新二进制，安装至同级临时路径、原子重命名覆盖 `/usr/local/libexec/picketxd`，重启并检查新状态。保留策略、身份、目标、mark 与状态。二进制和支持 notify 的模板须一起升级，旧 M2C10 程序不认识 --systemd-notify。模板编辑经审查后 daemon-reload。保留兼容旧二进制，旧版本可能拒绝 journal v4；只能回退到理解当前日志/镜像的程序。不能以旧日志覆盖新规则进行回退。

## 开发验证

```sh
scripts/verify-systemd.sh
# 具有真实系统管理器和 iproute2 的可丢弃特权主机：
CGO_ENABLED=0 go build -o /tmp/picketxd ./cmd/picketxd
sudo env PICKETX_SYSTEMD_TESTS=1 python3 scripts/test-systemd.py \
  --binary /tmp/picketxd --artifacts "$PWD/artifacts"
```

离线检查临时替换可执行文件路径，仅验证语法。原生验收复制模板为唯一临时服务，只替换测试路径/状态/轮询并加入全新隔离 NetworkNamespacePath，保留权限限制及重启设置，检查实际能力/命名空间、双栈 TCP/UDP、异常自动恢复、无效发布保留、拒绝性重载、停止保留及日志字节一致恢复。新增场景检查无效当前发布/有效缓存使服务保持 activating，干净就绪后才放行有序依赖，并检查启动超时保留规则/状态。仅超时阶段将 TimeoutStartSec 从 60 秒缩为 2 秒，通知权限、沙箱和重启策略不变。不安装生产服务、不更改宿主防火墙。显式选择后初始化/权限错误均失败、不跳过。CI 将服务报告/日志与既有原生证据归档。

参考上游 [systemd.service](https://www.freedesktop.org/software/systemd/man/systemd.service.html)、[systemd.exec](https://www.freedesktop.org/software/systemd/man/systemd.exec.html)。

独立必需Controller服务门禁通过显式`PICKETX_CONTROLLER_SERVICE_TESTS=1`运行
`scripts/test-controller-service.py`，使用真实安装版Core/Agent及系统管理器，要求当前
鉴权APPLIED、三个不同Agent进程、desired缓存不变，以及启动/崩溃/断连/绝对期限/
离线重启/重连的双栈TCP/UDP、独立未保护端口健康和正确notify状态。公开证据只保留
白名单哈希/进程/包观测/回执，不含密钥、状态或日志正文。同启动周期命名空间恢复
不是宿主重启、备份恢复或开机排序证明。

通知须显式开启；独立 enforce 与 prepare 不会仅因 NOTIFY_SOCKET 存在就发送通知。--systemd-notify 在命名空间效果前拒绝 --once、缺失或格式错误的 Linux 文件路径/抽象套接字地址。采用纯 Go Unix 数据报，连接及写入各有一秒期限，STATUS 固定文本不插入发布/错误内容，不依赖 libsystemd/C。发送失败使进程停止、保留规则；发送成功不等于管理器确认，systemctl start 完成与有序依赖验收验证管理器实际处理。
