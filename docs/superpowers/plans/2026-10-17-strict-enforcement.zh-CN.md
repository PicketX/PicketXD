# 静态主机 strict 执行计划

> 内联执行 executing-plans；沿用持续授权，一次全新整分支评审。

目标：内核逐包严格期限与持久历史定向清理闭环。绑定[设计](../specs/2026-10-17-strict-enforcement-design.zh-CN.md)。沿用 Go1.27 和全部固定模块，不改 graceful v1/默认/runtimeKey；strict imagev2、journalv4，KnownMarks4096、规则8192、映像/历史8MiB、日志64MiB；不含自动模式迁移/持久清理凭据，不修原延期 Minor。

1. 严格 Scope、映像、台账：先写缺失 API/严格范围 RED；Image.KnownMarks、ValidateMarks、共享拥有连接链与完整输入验证；保留 v1 序列化；覆盖退休mark保留/未知或错context别名、规则限制，完整单测绿色后提交。
2. 命令/driver：先观察 Config.Handling、RevokeRecorded、StrictStatus 缺失 RED；默认 graceful，显式 --handling strict；先应用/核验再清理再读最新代数，失败/残留/跨边界不确认，模式不匹配在 desired store 前拒绝；完整单测/race/vet/build/prepare smoke、integration 编译后提交。
3. 真实验收：增加五个 Owner、两个独立命令必需场景（成员与独立权限、物理替换退休mark、pending恢复、命令撤销、SIGKILL到期/恢复清理/重新授权、反向重建及原始 DNAT）；七项名称详见英文计划。一次全新审查，重要问题 RED→GREEN 一次修复，Minor延期；87项必需及全部原始场景通过，下载核对摘要/报告/版本/树；双语记录，最终 PR CI、预期head合并、main CI及归档复验、清空PR队列，收集全部决策和延期后仅删除本计划临时目录。

重点：退休旧mark仍独立授权保留；未知别名拒绝；IO期间边界/代数变更不能假确认；无进程到期及旧TCP不能借无标记状态复活；旧graceful兼容。
