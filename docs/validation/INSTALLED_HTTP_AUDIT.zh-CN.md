# 已安装链路 HTTP 决策审计联合验证

固定 Core `c67da10767d5cb92012366b6328baa31be8afa61`，树
`b2d6855d8406d70f98cc0d4c17fae9e31803a249`。Agent PR30 扩展既有密码与
真实校验证书 HTTPS Keycloak26.8.1 流程，不修改产品语义。

真实已安装 Controller/Agent 正常重启并取得新内核确认后，串行发送
allow4/deny4/allow6/deny6 四次真实 Nginx 请求。允许必须200且应用命中1，
拒绝必须403且应用命中0；每次请求前后读取真实鉴权审计 keyset，必须恰好
新增一条记录。来源族、caller、route、reason、当前/已应用 revision 和原始
Grant UID 逐项关联。原始 Agent/Controller 独立证明执行；审计不冒充内核
回执或 HTTP 请求用户身份。

独立自有浏览器上下文使用真实受管 approver 密码或 OIDC 登录，禁止回退
bootstrap。实际 /me 匹配账户及审计权限；经真实页面的有界翻页读取四条
记录，九个公开字段及两组贡献 ID 必须与实际 API 一致。保留1440/375截图，
无横向溢出、浏览器凭据存储或页面错误；finally关闭上下文，不影响原流程。

report.httpAudit 为严格公开凭据。缺失、旧/重复/不安全ID、跨族/revision/
Grant、API/DOM不一致、传输故障冒充403、额外隐私字段均以固定非隐私错误
拒绝。手工单元夹具仅用于测试，不能生成真实验收。既有31场景、Gitea7场景、
原始状态、CLI六阶段、systemd/LKG/停止备份、静态源码离线通知和四归档不减。

TDD先观察旧报告拒绝测试 Missing expected exception，再观察新模块缺失失败；
实现后192针对性与339全量Node均通过且无fail/skip。Python12、Go vet/build、
锁文件CoreWeb ci/typecheck/lint/build通过。本地普通/race Go均仅遇既有五个
Unixgram operation not permitted，不能当作真实Linux通过。仍须精确head CI、
一次独立全分支评审、原始日志、实际四归档和合并后main复核。

owner codex/installed-http-audit；租约13:40Z或更早明确验证main后释放。最终PR
不可变验收回执优先于此待验记录；未有最终实证前不宣称合并或基础目标完成。
