# 安装版 OIDC 联合验收

原有本地密码登录的完整原生流程继续强制执行；第二个临时网络命名空间运行同一套安装版
Controller/Agent/浏览器/Nginx/应用/Gitea场景，改用真实Keycloak26.8.1及正常HTTPS证书校验。
固定Core64c8cb8e6c92669873031ca45cff47dbb38d0224、tree4637c029a34c71ec04b66fcc205c288f1280dfb9。

## 运维边界
这是CI验收，不是生产IdP安装器，也不要求产品部署Keycloak/JDK/测试CA。保留既定栈、
协议和许可证边界。生产接入遵循Core OIDC_LOGIN.md：准确注册回调，显式绑定issuer/subject
到现有人类ID，保护私有文件/恢复密钥并使用可信入口。claims不授予角色或网络权限，
本地退出不等于IdP全局退出。

保留独立恢复密钥建号；实际OIDC申请人和审批人驱动既有独立审批流程，包括IdP SSO存活时
本地禁用/启用。Gitea发布前明确退出恢复会话，重新以OIDC审批人登录。每次必须观察真实
S256/nonce授权请求、准确本地ID/当前角色、新Secure/HttpOnly/Strict SCS及无浏览器令牌
存储；不允许密码回退。网络来源仍是独立审阅的IP/CIDR，不从IdP claims推导。

审批revision必须对应真实Agent applied generation/hash及Controller收到的APPLIED报告，
再观察双栈报文与拒绝时零转发。四份Gitea Grant审计actor必须是该OIDC审批人。独立并集、
绝对期限、已有连接撤销、应用自身认证和健康对照均保留。systemd/断连/LKG/停服恢复及
112个内核场景仍是独立强制门禁；不冒称新增IdP故障、MFA/HA、任意Provider或生产拓扑验收。

## 私有生命周期和证据
wrapper拥有独立0700目录，与Gitea的0711部署根分离；把固定原始Provider复制到私有副本，
写私有realm/client配置；在Node子进程启动前设置CA信任，Controller显式配置CA；仅添加/
删除唯一自有NSS标签，停止自有进程组/Provider后完整清理。绝不运行/修改/删除输入发行目录。
不关闭TLS校验，实际拒绝未知CA和错误SAN。子流程成功且清理检查通过后才写oidc-report.json。
失败只输出固定阶段，不输出IdP/子进程原异常、私有路径或原始Provider日志。新模式不发布
Controller/Nginx原始诊断及失败进程日志，仅保留成功的原始Agent日志核对重启边界。

四个产物名称不变，旧证据路径不变；新增完整报告/status/Agent日志及375/1440截图置于
console-kernel/oidc/。OIDC公开报告仅含准确源码/Provider描述、本地身份角色观测、关联的
审批/Gitea revision及TLS/清理证明。单元数据明确标注，不作为执行证明。

## 验证约束
25项新增测试已观察缺helper RED→GREEN，实际私有文件系统回归保护权限、绑定、DB权威及
符号链接拒绝；合计258 Node、12 Python状态测试、vet/build和Web锁文件检查通过。真实Core
归档/清单/静态CLI已在受控实例验证HTTPS browser origin、DB权威和空PATH，不是内核/运行证明。
完整本地normal/race仍有5项继承的TestNotifyEmitter Unixgram EPERM；只有准确head真实CI
能证明Provider/浏览器/内核联合闭环，设置/权限失败必须fail而非skip。必须一次独立全分支
评审、root先回归再修复、原始日志、四份实际归档/源码/静态/声明/截图、expected-head合并
及新main复核。最终准确证据和即时租约释放写PR28，优先于源码pending。是否完成整体目标
取决于实际端到端结果和既定基础功能清单，本文件不先行宣告完成。
