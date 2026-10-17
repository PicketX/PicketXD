# M2C8：静态主机 strict 执行

> [English (default)](2026-10-17-strict-enforcement-design.md) | 简体中文

按持续授权内联推进可运行 strict 主机命令，保留 graceful 默认、原哈希及80项原生门禁。一次全新整分支评审。本轮不含 Controller/L7/容器转发、自动处理模式迁移、持久清理凭据，也不修复原延期 Minor。

strict 对每个保护报文重新检查来源/端点/绝对有效期与 deny；撤销或到期后，即使进程消失，旧连接报文也不再通过。不主动发送 TCP RST，不承诺对端立即收到断开通知。conntrack 精确清理是另一项现场协调：进程死亡不自主删除 conntrack。UDP 无连接，后续有效授权可恢复；成功清理后的 TCP 不得仅因重建无标记 conntrack 而复活；清理前若权限重新有效，尚存标记 TCP 可能恢复。本轮不证明未观察权限间隙已完成撤销。

Scope 接受严格/宽松两种显式 host-local/original-destination。graceful 映像 v1 JSON/哈希完全不变。strict 映像 v2 增加最多4096个排序、唯一的 KnownMarks，来自相同 Agent/context 的持久台账（含退休），须含当前目标 mark。Owner/清理构造器验证精确台账绑定，未知别名先拒绝；旧二进制不能静默读取 v2。日志仍 v4。

严格目标先 deny，再检查当前成员及内核绝对时间。UDP 授权写 mark 后接受。TCP 授权先跳转共享已拥有连接链，仅已知 mark 且 ESTABLISHED/RELATED 接受；否则返回，只有匹配授权的全新 SYN+NEW 可写当前 mark 并接受，其余丢弃。没有无条件 established 放行。共享链按历史 mark 增长，避免每成员乘每mark；8192规则/8MiB映像包含新增部分，超限效果前拒绝。退休旧mark允许仍独立授权的旧连接跨物理目标替换保留，不回收；空目标只留下 owner 结构，历史仍用于清理。

CLI 新增 --handling graceful|strict，默认 graceful；可信 facts 采用所选模式。runtimeKey 序列化不变，已有记录与所选模式不同则在 desired store 变化前拒绝，同 boot 模式/context 迁移仍不支持；不通过删旧状态绕过。支持全新 strict 部署、同模式更新/恢复及已验证新 boot。

严格 driver 每次回调先应用或完整核验规则，再 RevokeRecorded，最后重新读取一致 desired 状态，覆盖初启、更新、轮询、无效发布保留旧缓存及时间边界。复用已有边界驱动，每次完整双族扫描/回读；轮询可配置，代价明确。不完整历史、pending、扫描/删除/漂移或跨边界均致命且不报告严格清理确认，保留规则/日志，重开重验后重试。

JSON 可选 strict 元数据包含 enabled/cleanupConfirmed/observedAt/validUntil/imageDigest/范围计数。仅成功清理、残留0、最新代数/hash及现场证明匹配、未跨先前 NextChange 才确认；只表示该次观察，不持久，到 validUntil/新 desired 即失效。strict 顶层确认还要求清理成功；graceful 不调用清理。once 仅协调一次，未来到期仍由内核阻断，但未来 conntrack 删除需要进程再次运行。

文件映射详见英文表：hostpolicy范围；hostrules v2/严格链与限制；hostowner/hostrevoke台账校验与模式；hostagent/CLI模式、协调、状态；五项原生场景及双语证据。沿用所有依赖，无模块/配置文档 schema 迁移，新增可选 CLI 参数。映射已对照已验收 main 自审。

先观察单测 RED，再实现；验证 v1 不变、严格无旁路、别名损坏、限制、权限并集、调用顺序、失败/跨边界/代数状态。完整 unit/race/vet/build/smoke、integration 编译。新增五项必需原生门禁：严格成员/无关与独立授权保留、物理替换退休mark、拒绝/已应用pending、独立命令撤销更新、SIGKILL到期及恢复清理/重新授权。87项必需全过，核对原始场景/报告/摘要/版本/PR和main精确树；重要问题一次 RED→GREEN 修复，Minor延期。


## 反向重建与服务端点预留

真实内核 run39004655614 复现：精确删除后 IPv6 旧 TCP 的新数据仍回显，内核以服务端口为 original source、客户端端口为 original destination 重建 mark0 记录，绕过原始目标分发。strict 在全部原始目标分发之后，为每个受保护端点增加一条“实际目的端点且 conntrack reply 方向”的 DROP。正常独立当前授权先分发；original 方向的范围外 DNAT 别名仍按原始目标处理。该守卫不删除未标记或无关 conntrack 记录。

strict 的受保护本地地址/端口必须专用于服务，不能同时作为本地主动出站客户端的源端点，否则其入站回复可能被拒绝。这是明确的报文范围代价；graceful 规则和编码不变。不修改 namespace 全局 conntrack sysctl 或 OUTPUT 策略。每条守卫计入 MaxRules。新增双族反向未标记 UDP 拒绝且记录保留、五次隔离物理替换重复，以及 strict 原始 DNAT 保护/范围外别名回归。必需门禁总数更新为87，顶层记录108。
