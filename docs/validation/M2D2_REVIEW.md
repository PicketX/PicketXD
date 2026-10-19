# M2D2 independent review and integration rulings

Fresh independent whole-branch review of 2a735af..363abd7, 10 changed files.
One Important defect: inherited Object properties bypassed required-case membership.
Actual report mutation reproduced; toString/valueOf/toLocaleString regressions RED,
own-property guard GREEN, full evidence suite24/24. No Critical/Minor findings.
The actual native report has all18 proper names and remains valid. Final Linux CI
must be green after the fix; no second review substitutes for those regression tests.

The reviewer independently checked 18 original tests, downloaded archive digest,
18 cases/eight confirmations, all eight raw status matches, source tree, pinned
Controller build metadata, sanitized logs and screenshots. Native execution is CI,
not reviewer-local kernel testing.

## Rulings (including every behavior the reviewer set aside)

| Decision and reason | Cost if wrong / remaining boundary |
| --- | --- |
| Use graceful combined scenarios with existing strict native gates. | No new strict-mode conjunction guarantee. |
| Healthy-service controls are adequate: startup dual-stack listeners, positive traffic, live processes, established HTTP and unmanaged probes. | A fixture-specific service failure could otherwise masquerade as denial. |
| Fresh probes disable pooling; established probes assert socket identity. | Reuse assumptions would weaken transport-vs-request claims. |
| IPv6 remains allowed after IPv4 removal; both families exercise Host rejection. | No exhaustive source/Host permutation guarantee. |
| Sequential revisions, applied hashes and received reports reject stale acknowledgments. | No continuous kernel-proof claim between observations. |
| Fixed timestamps plus independent expiry establish ordinary reconciliation behavior; existing native suites cover restart/reconnect. | Combined browser run does not repeat restart/reconnect. |
| Namespace wrapper and unique temporary directory constrain cleanup. | Unsupported direct invocation is not a deployment interface. |
| Private random credentials, exact-value log redaction and inspected artifacts are adequate. | Future secret-bearing fields require renewed review. |
| Archive source identities/tree agree; nested Core/build artifacts explain dirty build metadata. | Identity must be checked again on every accepted CI head. |
| Required-name own-property guard fixes the concrete omission bypass; ordinary missing/duplicate/failed/skipped cases remain rejected. | Validator is evidence validation, not a signature or a trusted producer replacement. |
| Strict TCP resets remain separate native-suite scope. | No strict combined request/connection guarantee. |
| Current run covers dual-stack fresh revocation/expiry and IPv4 established revocation/overlap. | Separate IPv6 established-revocation/overlap permutations are not newly established. |
| Forged-header case combines headers with a denied path; retain that narrow assertion. | It does not independently prove every spoofing dimension. |
| Loopback fixture and screenshots establish only tested topology/responsiveness. | Production TLS, human identity/approval, deployment and exhaustive accessibility remain outside this increment. |
| Separate Apache Core checkout and unchanged GPL Agent module boundary stand. | Future cross-repository code moves need their own licensing review. |
| Final delivery facts belong to merge/main verification and PR17. | No release is established solely by this review or the earlier green code head. |

No deferred review minors. Initial catalog-refresh failure was a test workflow issue:
explicitly rebase the mounted Grants editor after advanced publication, as the existing
Core browser acceptance does. Draft retention and CAS semantics remain unchanged.

中文：一次独立全分支评审发现门禁继承属性绕过，已先复现失败再修复，24 项单元
测试通过。无遗留小项。上表逐项保留评审边界与误判代价；实际双栈流量、复用连接、
绝对期限、真实 gRPC 报告和源码身份由原始 CI 证据支持。严格撤销、生产 TLS、身份
审批及未覆盖排列不能据此扩大宣称。最终提交 CI 与主线 CI 仍是合并和交接的必需条件。
