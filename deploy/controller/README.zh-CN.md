# 安装版单节点 Controller 与 Agent

> [English](README.md) | 简体中文

Apache Core包与GPL Agent分开部署。Core固定版本cdee150201a5e33036cd3d73e3be5a8201000698
提供私有初始化、静态二进制/Web/部署文档及许可原文；保留LICENSE、notices、清单和
SOURCE。本指南不发布GPL Agent二进制/源码分发包，也不自动改宿主机。既有入口负责
TLS/转发，不得将回环明文Agent或本地决策端口暴露到不可信网络。

## 配对现有 host-local Agent

1. 按Core包说明准备非特权用户/私有父目录，部署包并初始化**全新**实例。选择真实
   浏览器origin及`--agent-ref`；`--route-id`仅限制初始入口凭据，不预置HTTPRoute或
   Grant。恢复、保留实例或初始化失败后的部分目录，均不得重新初始化覆盖。
2. 保留source与management.users的`database`权威模式，文件轮询不得覆盖API管理状态。
   bootstrap、Agent、入口密钥用途独立，0600文件/0700父目录；通过Accounts创建人员，
   私藏恢复密钥。应用密码、访问令牌及SSH密钥与这些授权凭据无关。
3. 单独构建/校验静态Agent并履行GPL源码/许可证要求。同一可信主机上，只把agent-token
   安全复制到root私有路径，如`/etc/picketx/agent-token`（0600）；不把值放到命令参数、
   环境变量、URL、日志或浏览器表单。
4. 从controller.yaml记录公开的`agent.authority`，作为Agent准确的`--provider`，
   `--agent`匹配已登记ref。独占预留完整mark范围，明确双栈目的范围、私有持久状态/
   锁目录、host profile及处理模式。默认同机回环使用`--controller 127.0.0.1:9090`、
   `--agent-token-file /etc/picketx/agent-token`、`--controller-plaintext`，不能再给
   `--policy`。远程需显式配置正常校验证书的服务端TLS或可信隧道，不跳过TLS校验。
5. 在现有Desired configuration/Resources管理流程配置实际能力、部署事实和Binding，
   匹配authority、Agent/profile/目的范围；再配Resource、HTTPRoute、独立审批/Grant及
   可信入口。声明式network-space ID不是本地命名空间身份。
6. 按[systemd指南](../systemd/README.zh-CN.md)审阅独立附带的`picketxd-controller.service`，
   不是静态`picketxd.service`。替换authority占位值，逐项核对state/Agent/destination/mark/
   传输参数，保留notify/沙箱/权限限制，检查后才显式启用。不能同时启用共用状态的两种
   服务；这不是自动登记或身份/来源迁移，不可盲目使用示例mark和目的范围。

入口启动前，用它自己的入口密钥生成私有配置。初始callers.json只授权一个准确route
UID；若增加Gitea等第二路由，显式修改私有registry的allowed routes，或登记独立缩限
caller，不得复用bootstrap/Agent密钥或默认扩大到所有路由。凭据登记不等于批准/应用。

## 恢复与检查

按Core停服备份规则一起保留SQLite（必要WAL/SHM）、生成authority/config及私有凭据；
Agent owner/desired/LKG和锁保持相同受保护路径。不得删除/改名状态、改provider或重新
初始化绕过错误；迁移/回滚另行审阅。计划重启先干净停止Agent及Controller，保留状态，
再按原设置启动Controller和Agent。内存浏览器会话失效，需重登；核对新鲜的desired/
applied修订/哈希、enforcementConfirmed及当前真实gRPC报告，不以active或旧日志代替。
真实双栈访问正常后，期限/撤销仍须拒绝。人员禁用不会撤销独立来源Grant，需显式撤销。

原生CI实际构建Core包、tar重解压/逐文件校验并迁离源码；Controller和Agent以空PATH、
独立cwd运行，携带活跃Grant重启，完整业务视图不变，要求重启后的新鲜Agent/Controller
确认和真实IPv4/IPv6请求，再运行全部既有人工作流/Gitea/撤销/期限场景。测试驱动、Nginx、
Gitea使用自己的依赖。公开证据保留源码/二进制/清单哈希、脱敏日志和完整Agent状态历史。

这仅是在一次性Linux命名空间中的进程恢复，**不是宿主机重启、正式Controller systemd
安装、公网TLS部署、实际备份恢复或完整发布验收**。独立必需的Controller模式Agent
systemd门禁覆盖同启动周期崩溃、provider绑定LKG、断连、绝对期限、离线重启未就绪及
重连后的当前APPLIED，不证明上述其他运维场景；全部既有静态systemd/内核门禁保留。
graceful SSH仅保证新连接拒绝，不承诺断开已建立SSH。
