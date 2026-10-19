# Gitea application review and evidence ledger

## Scope and current gate
One fresh read-only whole-branch reviewer `gitea_application_review`, gpt-6-astra/high,
reviewed937b34c..71d20ab including spec/plan/Core contracts. Reviewer independently
ran Node124/124. No second review or fabricated GitHub approval. Native Gitea code
acceptance passed39249604347 at treeba4ec296dab18f47bd6763b3cec6a971333043e8;
local unit data does not establish packet/browser/service-UID proof. Final
documentation-head/tree/CI/archive/main acceptance and
immediate lease release will be recorded in PR21 metadata.

## Findings and single root fix pass
- Important R1: generic `Could not read from remote repository` also accompanies
  transport resets, unreachable networks and unusable keys. Four focused regressions
  observed RED, then GREEN after requiring pinned Gitea's actual repository-specific
  read-permission/not-found response. Generic SSH failures now remain fatal. HTTPS
  credential failures additionally require actual upstream application hits, not
  Nginx denial. Complete Node128/128 passes. Primary reference: upstream pinned
  [private serv](https://github.com/go-gitea/gitea/blob/v28.2.0/routers/private/serv.go)
  and [CLI serv](https://github.com/go-gitea/gitea/blob/v28.2.0/cmd/serv.go).
- Important R2: native39247791747 RED: service UID65534 cannot execute the binary
  beneath private runner workspace parents. Copy only the exact hash-verified public
  binary into the disposable service directory, chmod755 and rehash it there; leave
  the checkout/home permissions unchanged. This native-observed bootstrap fix remains
  gated on the next complete native run, not declared passing from Node tests.
- Minor deferred: optional Grant `not_before` is allowed but its value is not checked.
  Current pinned API emits a timestamp, so no actual secret leak was observed. Future
  timestamp/type/nested-value hardening is deferred; not part of the single fix pass.
- Visual minor deferred (root inspection): Gitea's375px viewport full-page PNG has an
  1118px canvas with excess blank area; visible repository content fits the375px
  region. Do not claim upstream mobile-layout perfection. No Gitea/UI runtime patch
  is included. PicketX Accounts mobile image is actually375px wide.

## Rulings made (including declined review boundaries)
1. Routine design/plan/push/merge approval waits are waived by the explicit autonomous
   mandate; focused spec/plan, inline TDD, one review and final native gates remain.
   Cost if wrong: autonomous scope judgment, no new production authority permitted.
2. Preserve old clean branch tips; reuse existing isolated worktrees on new task
   branches from exact fetched main. Cost: extra task branches, no lost user work.
3. Preserve the current React design and label-driven real forms; no redesign.
   Cost: no new visual-polish claim; actual responsive screenshots remain required.
4. Keep the original31-case evidence schema unchanged and make the additional Gitea
   validator mandatory at both native execution and external shell-report gate.
   Cost: calling the old validator alone never establishes Gitea acceptance.
5. Review complete code concurrently with native CI without calling native acceptance
   complete. Cost: later CI fixes are root TDD responsibilities; no second review.
6. Production deployment/release readiness is not inferred from disposable validation.
   Cost: a separate release/deployment acceptance increment remains required.
7. Graceful SSH revocation promises denial of new flows, not resetting established SSH.
   Cost: existing admitted SSH flows may remain; strict reset is not claimed.
8. Pin one Gitea Linux-amd64 release rather than claim a version/OS/architecture matrix.
   Cost: other platforms and upgrades require their own actual verification.
9. This workflow owns its isolated Controller/SQLite; unrelated concurrent operator
   publications are not simulated. Cost: no multi-operator acceptance follows.
10. Reviewer declined final native/post-merge judgment because bootstrap failed at its
    reviewed head. Root must verify exact final source/raw all-step evidence, download
    artifact bytes/hash, expected-head merge parents and full main CI before acceptance.
    Cost: native/main failure blocks merge/release; no mock or skip substitutes.
11. Git has no supported connectTimeout setting; native39248299904 showed its blackhole
    command killed without a protocol error. HTTPS transport-negative cases now use
    bounded curl with HTTP000, num_connects0 and exit7/28; connected TLS stalls and HTTP
    denial cannot count as transport denial. Admitted reads/clones and application
    credential denials remain native Git. Cost: this proves L4 admission denial, not
    Git's own timeout UX. Characterization RED→GREEN, full Node129/129.

## Verification chronology / limits
Baseline Node103/103. Missing Gitea validator asserted RED then16/16 GREEN; full
Node119, SSH host-key ordering RED→GREEN120, secret-field/redacted-failure RED→GREEN124,
review R1 four regressions RED→GREEN128. Native39247177382 passes original full Go
unit/kernel jobs and human conjunction, then RED at a case-sensitive version-output
assumption (actual `gitea version 28.2.0`); exact lowercase assertion corrected, same
published binary hash required. Failed/superseded runs are not acceptance evidence.
Native39247791747 reproduced service-UID EACCES;39248299904 then passed actual
Gitea migrate/startup, database/account/token/key and verified TLS setup, reaching
the new protected HTTPS connection probe. That phase evidence confirms the scoped
bootstrap fix, not the still-pending complete Gitea/merged-main acceptance.
Native39249045794 subsequently passed baseline/HTTPS-only/both entries, actual
dual-stack native Git application-auth checks/clones and strict wrong-host-key
rejection. Chromium accepted the real CA and reached Gitea sign-in, then RED on a
generic submit selector resolving to three forms. Use the actual accessible Sign In
button reported by Playwright; do not weaken strict selection or browser TLS.

Local official Go1.27.0 archive digest675c26c449cbb18fc24b74650de1eabbae6e16f64326fd85a283fb3b58280685
was verified. Full normal/race each fail only the known five notify Unixgram EPERM
tests: FirstCleanConfirmationOnly, DoesNotSignalWhenJSONWriteFails,
FailureDoesNotConsumeReadiness, OffIgnoresEnvironment and AbstractSocket. Vet/build
and Core locked npm ci/typecheck/lint/build pass. Local unshare is EPERM; attempted
actual pinned app bootstrap stops at chown65534 EINVAL in the local UID namespace.
No bypass or native-local success claim. Native unrestricted CI remains mandatory.

## Remaining
Code acceptance39249604347: all3 jobs/required steps SUCCESS, raw190top/482total
zero fail/skip/race; Node129; original31 +7 Gitea cases,22 actual confirmations,
112native/9systemd. Code local42a35c9/remote293ca7f/actualcheckout235e61484de7d96cc1ab28774faf94939a8cc346
share treeba4ec296dab18f47bd6763b3cec6a971333043e8. Downloaded console12184658921
SHA2568ee490fa7db42d8a71bbd1ec10cdcf68288b01699f5e8f545d39fe6974bdda89 and
kernel12184459114 SHA2569a7e3d62da4688343d976ccc5542a09b73f6853052048d6e8bb4b6ee7baee13d
match metadata; report/status/source/schema/private-state checks pass. Actual
desktop/mobile screenshots inspected. This proves R2 bootstrap GREEN and complete
application workflow, not deployment acceptance.

Complete final documentation-head gates, expected-head merge/main verification, then
deployment/static release and first-use acceptance. Overall goal not yet complete.

中文：一次独立全分支评审发现2项Important：SSH错误分类已4项RED→GREEN修复；
非root启动路径按真实CI失败修复，完整原生CI已确认。not_before字段加固与Gitea移动
截图留白延后。部署发布与整体完成不由本段推定，最终准确提交、CI、归档与租约释放见PR21。
