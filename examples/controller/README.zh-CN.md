# Controller 主机执行

静态构建沿用主 README。先在独立 Core Controller 注册 Agent Token，并发布选择该节点、host profile
及本地目标范围的权威配置。Controller 端口应避开受保护业务端口。Token 存入当前用户所有、0600 的普通
文件；原子替换 Token 文件后重连生效。完整命令见 [English](README.md)，使用真实本地地址及预留 mark。

--controller 与 --policy 必须二选一，远程模式不支持 --once，也不回退读取本地静态配置。
TLS 正常验证服务名与证书；不提供 --controller-ca 时使用系统根证书。选择明文须移除 TLS 参数并显式
指定 --controller-plaintext，明文传输不保护 bearer Token；不要求 mTLS。
--provider 指预期 authority，与网络地址独立。缓存及 owner 身份还绑定 Agent、profile、目标、mark
及传输设置。改变这些内容属于部署迁移，不能借用旧缓存。

目前仅支持一个 host-local/original-destination 上下文，profile 和 handling 必须匹配本地配置。
远端命名空间及 context ID 不作为本地内核事实；远端目标边界和所有 match 必须位于明确的本地范围内。
未知字段、能力不支持、歧义目标、版本倒退/同版不同内容、贡献无效均整体拒绝。派生对象仍受共享配置
UID 及替换约束限制，复杂 inventory 须拆分为支持的部署。

空 targets 撤回保护；仅删除 Grant 仍保留默认拒绝。独立绝对期限、拒绝优先和授权并集跨重连/重启/断连
保留。私有 v2 desired 日志同时保存原始快照及派生配置，原子持久性语义不变；旧静态程序不理解该日志。
静态模式继续支持其 v1 缓存，禁止跨来源拷贝采用。

只有现有驱动确认当前原生执行及适用的 strict 清理后才报告 APPLIED，它仍只是 Agent 的已认证声明，
不等于 Controller 独立核验。RECEIVED 仅代表传输接收；拒绝/应用失败报告 FAILED。应用后报告传输可能
失败，因此缺少报告不代表未执行。alpha 协议没有报告确认或心跳，旧报告不能证明持续健康或连接状态。
网络失败每秒尝试重连，发送限时两秒；无缓存时等待有效快照后才可就绪，有缓存时继续执行原绝对期限。
撤销 Token 只阻止后续同步，不会自动撤销缓存的永久权限；须先发布并确认收紧策略后取消注册，或使用有界 Grant。

--systemd-notify 仍可选；现有服务模板是静态示例，远程部署须明确替换来源参数并保留沙箱。
本段不提供全局开机防火墙或跨未观察间隔的完整持久撤权保证。
