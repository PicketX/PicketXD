# 安装入口失败隐私修复

2026-10-20T00:12Z，有界设计/上下文图/计划已自审。关闭PR26延后项：execFileSync的
原始异常带命令参数/令牌路径，且stderr可能早于外层脱敏直接转发。只修改
integration/installed-ingress.mjs准备边界，保留固定阶段/路由和受限数字退出状态，
新建Error，不携带原异常、cause、stdio、参数或文件路径；显式捕获子进程stdio。

上下文：controller-install和console-kernel消费准备结果，gitea-application是现有
外层值脱敏参考；新增installed-ingress.test.mjs实际执行Node子进程/文件失败。
四个场景：真实进程非零退出、无效cwd、缺失CLI读取、哈希不符。它们不是伪造CLI
成功或原生发布证明。原版四项均RED，修复后完整233Node通过且无skip。

没有生产Go/内核/API/配置/数据库/UI变更，不重选技术；不需要前端设计技能。
保留全部真实浏览器/锁定Web/Gitea/Agent/内核/备份/离线源码门禁。一轮独立全分支
评审，根代理单轮先回归后修阻塞，准确head完整原生CI/四归档/安全合并/main仍待验证。
最终准确回执在PR元数据中，不能据本地Node结果声称原生成功。

裁决：用户自主开发授权免除例行等待；这是已有流程的有界修复，不增架构子系统。
错误代价：诊断细节减少或配置漂移；保留安全阶段/退出状态、私有本地材料和原生门禁。

## 基础目标核对（入口时已接受main）

| 范围 | 实际证据 | 边界/剩余 |
| --- | --- | --- |
| L4/同步 | Agent524a62c/CI39344752833，112内核/23确认/9静态systemd/4 Controller模式PID及36观察，断连/期限/LKG/停服恢复 | 一次性Linux及明确作用域，不推广到任意宿主重启/迁移 |
| 基础L7 | 31既有+7 Gitea、真实浏览器/API/Agent/Nginx/HTTPS/SSH、伪造头/复用连接/撤销/期限及4私有路由拒绝，交付CLI实际片段 | 来源Grant与应用身份独立，不声称PicketX逐请求身份策略已实现 |
| 管理端 | 实际建号/账户生命周期/申请审批拒绝/Grant撤销/审计；Coreadfa1b1/CI39339061250真实Web/API | OIDC会话登录仍缺，不能用本地密码登录替代这一已列目标 |
| 单节点运维 | Core239文件/GPL Agent1415文件入口包、静态ELF、源码清单/离线重建、私有初始化、服务/停服恢复 | 不是任意生产拓扑或宿主回滚迁移承诺 |

下一实际功能：Core ARCHITECTURE的M1及BASIC_FUNCTION_ROADMAP第3项明确列OIDC，
HUMAN_LOGIN明确不含OIDC，现有internal/cmd/Web源码无issuer/callback集成。沿用
SCS/Casbin及本地/bootstrap恢复入口，后续做真实OIDC协议/浏览器验证，不再只叠打包。
不擅自从既定路线删除OIDC，也不宣告整体完成。账户生命周期已完成；PR12的已有行
UPDATE CAS由store_test和TestAccessRequestStaleRevisionOneUpdateCAS覆盖，不重做。
历史评审其他小项仍在原记录，不静默升级为阻塞或借此增加首期范围。
