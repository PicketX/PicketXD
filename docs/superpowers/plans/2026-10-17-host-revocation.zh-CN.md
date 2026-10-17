# 主机定向连接清理实施计划

> [English (default)](2026-10-17-host-revocation.md) | 简体中文

依[设计](../specs/2026-10-17-host-revocation-design.zh-CN.md) 内联执行，使用 executing-plans；持续授权已涵盖实现、推送及通过门禁后的合并。本批不改 CLI/schema/依赖，不声称完整 strict。

1. 不可变 Plan 与 Run：真实映像/台账测试观察 RED；实现历史来源/目标/mark 精确所有权、当前并集/deny/绝对时间、输入不可变、预检/确认/删除/重扫、取消/边界/倒退时钟和残留错误；全套 GREEN 提交。
2. 严格 wire 与 Session：双族 TCP/UDP/NAT/zone/opaque ID、坏包/缺失/重复/嵌套/方向 zone 的 RED→GREEN；实现原始元组+zone+ID 请求，持有锁/当前映像确认，缺失应用/伪造历史/关闭/漂移拒绝；完整 unit/race/vet/build/prepare smoke GREEN 提交。
3. 四项实际内核门禁：双栈两协议清理与保留、DNAT+zone/ID、漂移/预检零删除、部分删除/残留失败。独立评审，Important/Critical 一轮 RED→GREEN；保留原 71 项并通过新增四项，下载核验全部原始流及摘要，记录中英文验收。最终 head CI 通过后精确合并，核对 main tree/CI、无开放 PR；只清理本计划临时目录并保留 worktree。

接口、上限、五项评审关注及对应断言完整列于英文计划；不得把纯计划或调用者历史当作已应用/完整历史凭据。
