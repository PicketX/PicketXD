# 持久紧凑历史实施计划

> [English (default)](2026-10-17-durable-history.md) | 简体中文

依据[设计](../specs/2026-10-17-durable-history-design.zh-CN.md)，executing-plans 内联执行，一次全新整分支评审；沿用用户持续推进、推送、合并授权。

1. 身份哈希助手及紧凑历史：真实映像/台账下观察缺失 API RED；实现不可变、UID/期限去重、物理范围/族/mark/context/上限及记录选择；完整不使用缓存测试 GREEN 后提交。
2. v4 生命周期：先写首次/旧版本/no-op/pending恢复/失败发布/新boot RED，再实现历史效果前预览、确认时合并、旧版本不完整、新boot清空覆盖但保留退休/基线。新增 HistoryStatus、RevokeRecorded 和 JSON 元数据；验证不完整/歧义不删除、清理不写日志、关闭失去 Complete；全套 unit/race/vet/build/prepare smoke、integration 编译 GREEN 后提交。
3. 五项真实门禁覆盖多代双栈 TCP/UDP 重启、pending 成功及原子拒绝、旧历史不完整拒绝、损坏拒绝、父进程持 A/B 连接时助手 SIGKILL。独立评审 Important/Critical 一轮 RED→GREEN，Minor 记录延期。全部80必需门禁通过、独立下载核对实际原始计数/摘要/模块/构建版本，记录双语证据；最终 head CI 通过后精确合并并验证 main tree/CI、无开放 PR，收集裁定/延期项后仅清理本计划临时目录。

完整接口、限制与五项评审关注见英文计划；不自动调用清理、不宣称完整 strict；不在本轮修复既有延期 Minor。
