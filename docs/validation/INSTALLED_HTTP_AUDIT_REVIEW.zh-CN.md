# 已安装 HTTP 审计独立评审

一次全新只读全分支评审，BASE665c9a4至本地dc5643f（发布72164f5，
相同树61b0cf78）；结论With fixes，0Critical/1Important/0Minor。无嵌套或二次评审。

root复现阻塞：同时替换receipt.grants、允许API行和DOM贡献列表，旧门禁仍通过。
这是公开凭据自引用验证缺口，不是真实请求造假。唯一修复轮增加独立的原始
Grant创建响应UID/revision与已知family/source记录report.grantIssuances，严格
关联两个连续创建revision、既有grant4/grant6执行场景及重启revision。
七项协调替换/无关UID/缺失记录/错误revision/source/case/隐私额外字段回归
均先Missing expected exception失败后通过；199针对性测试通过。新增revision1
确认最初令旧stale-generation变异成为无操作，现保留首项revision2、追加1，
恢复原断言真实拒绝；未删除旧断言。全套与最终真实Linux/归档/main仍须通过。

## 决策及误判成本

1. 真实Linux、归档、精确head/main由root在评审后验收，不从192项单元通过推断；
   成本：否则可能交付未验证原生行为，保留必需门禁防止接受。
2. 五项本地Unixgram EPERM仍为环境限制，不算回归或真实通过；成本：本地无法
   认证这些路径，真实Linux普通/race门禁仍必需。
3. 超出有界翻页的夹具失败关闭，不增搜索/导出或重做200行UI；成本：不能证明
   任意大历史，届时测试明确失败。
4. 使用固定真实可见DOM提取，不假设未来隐藏字段缺陷；成本：未来标记变化
   必须继续通过真实CI，也不构成通用无障碍认证。
5. 不为假设的context.close故障加特殊绕路，保留自有清理/OIDC外层错误抑制；
   成本：未来未见关闭故障可能替换固定诊断，目前无隐私泄漏实证。
6. 产品授权、内核、schema/UI重选不属于本段附加验收；成本：不新增拓扑、
   授权身份或生产容量保证，既有独立原生门禁继续约束。

本次无延后Minor；既有增量文档边界不变。
