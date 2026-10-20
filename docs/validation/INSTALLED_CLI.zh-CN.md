# 已安装 CLI 生命周期联合验收

PR29 扩展既有原生验收，不修改产品行为。Core 固定为
81ec5bc8a483010f99672f7ff5d982ad4ee6698f，源码树
1b04ceb9d86f4d72c30e12c540097aafd915b2d2。密码登录与真实正常校验 HTTPS
OIDC 两轮都执行实际已安装 CLI，空 PATH、私有管理密钥文件、正常证书信任。

公开报告必须包含 deployment.diagnostics.cases 的六个有序阶段：
before-agent、applied、grants-applied、agent-stopped、controller-restored、
agent-restarted。status 均退出0；新鲜匹配 APPLIED 的 doctor 退出0，Controller
就绪但 Agent 断连时退出1。CLI 始终 enforcement_confirmed:false，不替代独立
双栈真实流量、原始 Agent 状态、类型化 Controller 回执及重启进程边界。
类型化快照摘要与本地策略摘要属于不同域，不混为同一证明。

适配器不使用 shell：CLI5秒、子进程10秒、输出64KiB。启动失败、信号、超时、
溢出、异常 JSON/字段/退出码均返回固定脱敏错误，不能伪装成正常降级。
不公开密钥、私有路径、原始错误或 stderr。

开发证据（不是原生验收）：缺失模块先失败；旧报告缺少诊断时的拒绝测试先因
Missing expected exception 失败，再由必需门禁修复并通过。单元夹具不冒充
原生证明；真实子进程夹具覆盖0/1/2、启动、信号、JSON、溢出和实际10秒超时。
完整 Node303项在10:46Z通过，之后增加一项健康子进程测试，最终计数随精确
评审/CI回执记录。Python 停机状态真实文件系统12项通过，Go vet/build通过。
完整普通/race各有五项既有 NotifyEmitter Unixgram 因 operation not permitted
失败，不计为通过；等待真实 Linux CI。CoreWeb锁文件安装/typecheck/lint/build
通过。独立全分支评审、CI、四份实际归档和合并后 main 核验仍待完成。

范围裁定：本段无需 UI、产品 API、内核、配置、身份语义或依赖变更；不引入
新 UI 设计或运行时依赖。原有浏览器、Keycloak、Gitea、systemd、LKG恢复、GPL
源码/离线构建/许可证与112内核场景全部保留。本文不是总体完成或原生通过声明。
