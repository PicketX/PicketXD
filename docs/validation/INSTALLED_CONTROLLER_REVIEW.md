# Installed Controller conjunction: review, rulings and evidence

## Scope and sole independent review
One fresh read-only installed_conjunction_review (gpt-6-astra/high) reviewed full
233f081..1da3b2c, AGENTS/spec/plan/ledger and affected Agent/Core consumers. Ran full
Node159/159, diff whitespace check and malformed-index reproduction. No worktree,
index, branch or HEAD mutation, no nested reviewer. Verdict:2 Important, no Critical,
no Minor. Subsequent root remediation is not a second independently reviewed branch;
TDD and all exact final/native/main/source/archive gates remain mandatory.

## Important findings and one root TDD fix pass
1. Whole-response restart hashes included fresh read clocks accounts.observedAt
   and grants.observed_at. Verified exact pinned Core handlers assign current time
   on each GET; native39273387239 console118131275002 failed14:57:56Z at comparison.
   Realistic timestamp-only/unit mutation cases observed RED; remove only those
   endpoint-specific top-level observation clocks. Keep all account state, scopes,
   absolute deadlines, revision, resources and audit event timestamps. No query or
   production persistence/security semantics changed.
2. Fresh-index gate originally accepted nonexistent safe-integer offsets and only
   an aggregate confirmation, without raw history/current Controller receipt. The
   reviewer reproduced acceptance. Root tests observed missing Controller proof RED
   and added missing/truncated/out-of-bounds/stale/mismatched row/log/receipt cases.
   Gate now bounds selected index in actual raw history, matches the full suffix to
   restarted-process stdout, validates the selected fresh applied row, and requires
   current connected/fresh/matching APPLIED Controller receipt at exact revision.
   Preserve typed snapshot SHA vs projected Agent policy hash as separate domains.
   Clean stop waits for process close/I/O drain, preventing old output crossing the
   restart boundary. Complete status history remains archived, not discarded.

Local complete Node174/174 passes after this fix pass, zero failed/skipped. Both
Important findings now have corrected native proof below; no merge on unit data.
No deferred Minor. No fabricated GitHub approval or second reviewer/re-review.

## Rulings in order (including reviewer-declined boundaries)
1. Autonomous mandate waives routine design/plan/push/merge waits; focused artifacts,
   inline TDD and one fresh review retained. Cost: executor makes scoped judgments.
2. Reuse host-owned isolated worktree, new exact-main branch, preserve old branches.
   Cost: extra retained branch, no discarded user state.
3. Build exact pinned Core bundle instead of relying on expiring old CI downloads.
   Cost: build time/toolchain trust; actual committed packager/source/manifest checked.
4. Formal GPL Agent package and host reboot/Controller systemd acceptance separate;
   deploy actual static Agent executable in this native gate. Cost: no full-release
   or host-boot claim, no automatic overall-goal completion.
5. Explicitly enroll second Gitea route in private caller registry, preserve generated
   runtime config/authority. Cost: documented narrow operator credential step remains.
6. Known local5 Unixgram EPERM failures cannot be bypassed; full native mandatory.
   Cost: no full-local Go/kernel success claim.
7. Retain full stdout history and fence old receipts with restart cursor, rather
   than clearing all status. Cost: larger public archive; enables independent
   correlation of previous revisions and the restarted process.
8. Reviewer did not execute privileged native/main/archive gates: root independently
   fetches every mandatory step/raw checkout/source/archive/status. Cost: absent or
   failed evidence blocks acceptance; review verdict alone is not enough.
9. Host reboot, production Controller systemd, public TLS, actual backup/restore,
   other platforms and formal Agent release compliance outside this increment;
   bilingual runbooks explicitly limit claims. Cost: operations work remains.
10. Subsequent remediation not independently re-reviewed: one root TDD fix pass and
    complete final native evidence are the gate. Cost: no second judgment pass.

## Verification chronology and remaining gates
Baseline Node129 passes. Generated bootstrap audit actor RED→GREEN. Installed
proof negative cases old-validator RED→GREEN, actual filesystem manifest gate
RED undefined→GREEN. Real prior main Core tar locally verified/relocated and actual
CLI private init passes with empty PATH, without claiming local kernel execution.
Tests-first native39272603050 console118128557938 RED14:52:02Z at missing required
installed proof; unit full Go/race/vet/build/static passes, not whole acceptance.
First implementation39273387239 reaches real clean restart then observation-clock
RED; corrected local174 Node passes. Full local Go normal/race each retain exactly
5 inherited TestNotifyEmitter* Unixgram EPERM failures, no other failures/races;
vet/build pass. Mandatory Linux normal/race, full112 kernel/91 required,9systemd,
31 prior conjunction/7 Gitea, real installed restart and locked Web remain required.
Corrected native39274725190 all3jobs succeed: unit118135944993,
console118135945168, kernel118135945401. Independent actual download inspection
then REJECTS legacy Agent build vcs.modified=true from untracked pinned .ci-core
checkout. This green run is NOT final acceptance. Real git-ignore regression RED
then narrow root/.ci-core exclusion + full parent cleanliness gate GREEN; unknown
Agent Go source remains visible and Core retains own exact-pin/full-clean builder.
Complete Node175/175 passes. New exact-source CI still mandatory; no dirty-build
acceptance and no production-source change hidden. Additional ruling: intentionally
separate pinned checkout is not parent Agent source; cost: must continue verifying
that secondary source independently. Final exact immutable IDs, downloaded
ZIP/tar hashes, source/report/screenshots, expected-head merge/main and immediate
lease release belong in final PR22 metadata, avoiding self-referential commits.

## Corrected code acceptance — 2026-10-19T15:16Z

Localdeb9a247a5857846b7abfca0511cf9d4a7e201cf, remote176ab6862fb89fcee3d36cb304fadd90176a3a4e,
actual CI checkoutf9874376b4a910c5c5f24b19108f2880f27cac4c share tree
ef0a5c39901b29df409a4ae1c07ea19097e79928. CI39275649986 all3jobs/every step
SUCCESS: unit118139162088, console118139162562, kernel118139162891. Raw Go190top/
482total, normal/race/vet/build, Node175, locked Web install/typecheck/lint/build,
static bundle/runtime and all existing gates pass, no failed/skipped/race tests.
Independent archive inspection confirms actual Agent build vcs.modified=false and
exact checkout, Core93024b2/tree64ed0dc, Core static ELF/240-file manifest/private
init, actual31+7 scenes/23 confirmations,112kernel/91required/9systemd.

Downloaded ZIP bytes equal GitHub metadata SHA256:
- kernel12197003603:00973fa68805fe6c9738695365a51ccd97fd95a876bf870e2ff0da3bd435f655
- bundle12196673983:c6ab11a116f014fc047a5263d9efecf24fc67f950d7e268efb1891479d4d88df
- console12196619009:c269bf4eb5e8fc87f7dd64284aeb549225f51951089e04748eeb777b1ab7c5ff
Inner tar bytes checked against its SHA sidecar; source/manifest copies and
manifestc8116060c5f433bd2e16728b65629c442a17aabb4f89712819f7c05f90ceb2e6 agree.
Public evidence contains no archived private keys/DB/config. Known successful
pgx notice_response.go SHA manifest line triggers a GitHub error annotation; only
that exact ': OK' path is allowlisted, all other error annotations fail acceptance.

Restart revision4: both clean exits0, old cookie401, four business-view hashes
unchanged, selected fresh raw status126 after cursor123 in3614 actual rows; all
3491 restarted rows exactly match the raw suffix. Selected desired/applied revision
and policy hash agree; current Controller connected/fresh/matching APPLIED receipt
agrees on typed snapshot SHA, not conflated with projected policy hash. Actual
post-restart v4/v6 requests each200 with one upstream hit. All23 aggregate receipts
independently match actual raw Agent confirmation rows. Actual1440/375 Nodes
screenshots inspected: no overflow, generated authority/bootstrap identity and
separate stream/freshness/target match visible. Prior upstream Gitea mobile blank
canvas limitation remains deferred, not an assertion of upstream layout perfection.

This documentation-only successor still requires full final exact-tree CI,
expected-head merge and full main verification. Immutable final IDs/hashes and
immediate lease release are authoritative in PR22 metadata, not this historical
code receipt. No overall completion, formal Agent package or host boot claim.

中文：一次独立全分支评审，两个Important按真实失败修复；只排除查询层观测时钟，
不忽略期限/业务/审计字段；新鲜确认必须匹配原始状态、新进程日志和当前Controller
回执。175项Node及完整原生代码验收通过，准确最终head/main/归档门禁仍必需。不伪造二次评审，
无延后Minor；全部取舍及代价如上。正式Agent包、宿主开机等仍未据此宣称完成。
