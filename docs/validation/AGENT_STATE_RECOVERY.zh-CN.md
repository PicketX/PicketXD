# Agent最新停服检查点恢复

> [English](AGENT_STATE_RECOVERY.md) | 简体中文

仅适用**最新停服检查点、同机器/启动/网络命名空间/原路径、兼容且验证过的二进制**。
不是旧备份回滚、宿主重启恢复或迁移。root控制文件，散列只能发现误复制，不能证明
快照未过时或抵抗恶意特权写入。若已有更新发布/撤销，禁止恢复旧快照；保持应用关闭，
另行解决权威/所有权恢复，不能删除journal强行接管。

## 独占维护

核对实际安装Controller模式unit的完整ExecStart和drop-in，保留authority/provider、
Agent/profile、双栈目标、独占mark范围、处理模式、传输/服务端名/CA/凭据路径。另存
兼容且已验证发布包，旧二进制可能拒绝新格式。维护期间其他操作员/自动任务不得启动/
更新服务、发布Controller策略或启动受保护应用。保留安全管理通路，记录revision/hash
和原始绝对期限。停服保留过滤；graceful不承诺结束已有连接。

## 完整私有备份

默认路径仅示例，替换为已核对的实际路径。可信root shell设置umask077，新建私有
备份目录，停止准确picketxd-controller.service（或实际unit），systemctl show确认
ActiveState=inactive且MainPID=0；failed、旧日志或发送信号成功不能代替停服证明。
整个复制期间保持停服。

用cp -a复制完整/var/lib/picketx，当前包含owner.json、desired.json及两个.lock。
不能只复制JSON或遗漏owner/mark/application历史。缺失/未知项、链接、错误所有者或
组/其他权限均拒绝；保留root、目录0700/文件0600。不删除/恢复共享/run/picketx，
运行时锁不是可迁移备份数据。同时复制原路径Token（0600）、实际unit和该部署需要的
drop-in、EnvironmentFile、凭据和CA。自定义部署可能多于测试的六个文件。私有清单
记录原路径、所有者、模式；停服逐一比较源和副本散列。备份/清单/Token/cache/journal
不得进入公开包、CI归档、日志或Git。按需使用访问控制/加密存储，这里不提供密钥管理。
字节一致不证明断电持久性。

## 原路径恢复

仅恢复该最新检查点，期间没有新发布或Agent运行。再次确认准确unit停服/MainPID0。
替换前验证完整备份散列/权限/所有者/原路径。停服原件移动到独立私有隔离目录以便
恢复，不覆盖；完整状态/凭据/unit复制到**不存在的原路径**，保留模式/所有者，独立
副本而非链接。目标存在、格式未知或身份不符立即停止，不能手改provider/runtime/
boot/namespace/owner/mark/generation/expiry绕过检查。逐个复核后daemon-reload，不能
隐式启动；失败保持应用关闭，备份和原件保留至确认恢复。

systemctl start --no-block异步启动，只读新MainPID最新状态，真实双栈TCP/UDP访问并独立
确认受保护应用健康。离线LKG可保留尚有效授权，不能续期；原期限到达后即使Agent
仍停止，新流也须拒绝。离线启动activating不冒充当前READY。重连原认证Controller后
要求最新匹配revision/hash的APPLIED、matches_desired、connected/fresh、Agent执行确认
及无应用错误，过期访问仍拒绝。审批/发布/active/旧APPLIED不代表当前应用。外国表/
漂移、启动/命名空间/配置/路径变化、较新权威或无安全检查点均停止，不全局flush
防火墙/conntrack、不删状态。

## 可执行证据

必需Linux脚本test-controller-service.py使用真实发布Agent/Core、受限notify unit和
一次性命名空间。停服备份四状态文件/Token/unit，隔离原件，原路径恢复独立、字节/
模式/所有者一致副本，真实36项IPv4/IPv6 TCP/UDP观测覆盖启动/崩溃/断连/停服/到期/
离线启动/重连。公开仅六文件数/布尔值/时间，不归档私有快照/清单。全部断言及清理
成功后才PASS。四个不同PID同时保留运行中断连跨越原期限及停服到期证明。状态原件
在原文件系统隔离；失败只停止/回收/注销运行时资源，保留私有原件/检查点/部分副本。
仅该测试root可在确认无相应服务/进程/命名空间后检查并清理打印的准确私有路径；禁止
上传或恢复至新命名空间。成功后才清理自有私有数据。准确源码/CI/归档/main验收见
对应PR最终回执；不宣称宿主启动证明。
