# Agent distribution: review, rulings and acceptance record

## Sole independent full-branch review
One fresh read-only agent_distribution_review (gpt-6-astra/high) reviewed the full
c02ab9600a3be2d64c4a987189726aa4e18040ec..226e9440a427ef9cb0773a98a53488f97a81d5c3
range, AGENTS/spec/plan/ledger and affected consumers. No nested reviewer or checkout/
index/HEAD/branch mutation. Ran full Node197/197, actual1395-file payload inspection,
static ELF/clean metadata checks and isolated real Go source-injection reproductions.
Verdict: no Critical, one Important, one deferred Minor. No second review/re-review
or fabricated GitHub approval. Subsequent root fix is proved by TDD and full CI.

## Important: exact source identity
Clean Git status and vcs.modified=false did not exclude ignored compiled input or
ambient GOFLAGS overlays/workspace/configuration. Real fixture programs reproduced
IGNORED_INPUT and OVERLAY_INPUT while metadata claimed the committed source, and
ambient GOAMD64=v3 replaced the intended portable baseline. Three new regressions
observed RED against the reviewed builder, then GREEN; complete Node200/200 passes.

Root one fix pass: capture the selected commit/tree, make a private isolated clone
of only that commit (no hardlinked objects), normalize Go input selection/target,
verify module-cache integrity, compile/vendor/retain notices from that snapshot.
The public builder checks actual commit/clean metadata/CGO0/AMD64v1. Verify the
snapshot remains clean, remove only its newly created private clone Git metadata,
seal every exported source/vendor/binary/notice file. Original worktree, refs and
branches are untouched. Offline rebuild has the same target controls, empty caches,
network/module/toolchain download disabled, and -buildvcs=false so it cannot inherit
the outer checkout's VCS metadata. Source snapshot Git tree is actually recomputed.

## Deferred Minor
Copied deploy/systemd guides have two relative links that no longer resolve at the
flattened distribution location. The intact source/PicketXD/deploy/systemd guides
and their referenced source docs/examples remain in the package. No safety model,
runtime or service default changed. Defer the copied-reference layout polish; no
claim all distribution hyperlinks are repaired in this increment.

## Rulings I made (in order, with costs)
1. Autonomous mandate waives routine spec/plan/execution/push/merge waits; preserve
   focused self-reviewed artifacts, inline TDD and one fresh review. Cost: executor
   makes scoped judgments without intermediate human feedback.
2. Reuse clean host-owned linked worktree/new exact-main branch, retain old branches.
   Cost: extra retained branches; no destroyed user state.
3. Official vendor + full committed Agent source, not a container or remote-only
   source URL. Cost: larger bundle, separate supported Go toolchain for rebuild;
   upstream dependency tests omitted by vendor's documented contract.
4. Build-only Node/Bash/Git/Go are allowed, shipped runtime empty PATH. Cost: this is
   not a universal bootstrap/root installer and does not install its own toolchain.
5. Add public distribution proof to existing console report/fixture/gate beyond
   initial context map. Otherwise manually built substitution could pass. Cost:
   additive private test-evidence schema; every acceptance consumer must carry proof.
6. Reviewer-declined privileged/native execution is not waived: independently inspect
   required Linux CI/raw/source/archives/status. Cost: missing evidence blocks merge.
7. Legal clearance/OEM sufficiency is outside engineering notice-retention mechanics.
   Preserve actual licenses/source and explicit limitation. Cost: no legal certification.
8. Bit-identical reproduction is not promised; prove actual offline build/tests and
   preparation equivalence instead. Cost: exported source lacks VCS metadata, so
   original/rebuilt executable bytes may differ.
9. Host boot/Controller service installation/backup-restore are separate. Cost:
   overall operational acceptance remains unfinished, do not disable automation.
10. Publisher authenticity/signing is not provided by self-contained SHA files;
    require trusted release-channel identity. Cost: no authenticity claim on a
    bundle obtained solely from an untrusted mirror.
11. Linux-amd64 only, now explicit AMD64v1. Cost: no other architecture/OS support.
12. Malicious compiler/toolchain/concurrent privileged filesystem replacement remain
    outside trusted build-environment scope. Ordinary Go configuration and ignored
    source are NOT excluded and are fixed. Cost: build host/toolchain must be trusted.
13. Replace working-tree Git archive/build with an isolated committed-source clone
    for this one Important fix. It preserves the exact-source intent while closing
    ignored input/overlay escapes. Cost: extra Git I/O, only newly created clone's
    private metadata removed; no original repository/worktree cleanup.
14. Ordinary CLI git push lacks credentials; use configured GitHub connector with
    expected_sha/no force, exact local/remote tree equality. Cost: distinct commit
    IDs share a verified identical tree; no credential extraction or auth bypass.

## Verification chronology / immutable integration gate
Baseline Node175, full Go normal only5 inherited TestNotifyEmitter* Unixgram EPERM.
TDD12 guard/integrity assertions RED against empty contracts (initial missing-module
errors not counted), then187/187 GREEN. Nine distribution proof failures RED against
no-op gate, then197/197 GREEN. Tests-first native39288604180 console118183053596
RED16:50:11Z 'actual Agent distribution proof required'; unit full Go passes, not
overall acceptance. Implementation local226e944/remotedfaa468/actualcf861118 share
tree6e4e0a660316448635b86285c19cfdee60d9d522. Native39288973797 all3jobs SUCCESS:
kernel118184312859, console118184313308, unit118184313309. This pre-review-fix run
does NOT establish final acceptance because the source-identity blocker remained.

Actual local1395-file bundle/static ELF/tar/SHA/exported tree and offline compilation
pass; full offline Go tests encounter exactly the same5 Unixgram EPERM restrictions,
so no full-local or privileged execution claim. Full local race has the same5 only,
no DATA RACE; vet/build pass. All5: FirstCleanConfirmationOnly,
DoesNotSignalWhenJSONWriteFails, FailureDoesNotConsumeReadiness, OffIgnoresEnvironment,
AbstractSocket under TestNotifyEmitter. Required Linux tests never bypass these.

Root source-identity regressions now200/200 full Node; actual fixed source still
requires complete native/offline/Go/Web/kernel/systemd CI and four independently
downloaded ZIPs, inner tar manifests/static/source trees/reports/status/screenshots,
expected-head merge/parents and full main verification. Final immutable local/remote/
actual checkout/tree, commit/PR23/CI/job IDs, archive hashes and immediate lease
release belong in PR23 metadata, avoiding self-referential commit loops. No overall
completion or fake approval. Full source and unchanged production behavior retained.

中文：一次独立全分支评审，1项构建来源Important已用真实Go回归RED→GREEN修复，
200项Node通过；1项复制的systemd文档链接Minor延后，完整源码内原文及引用仍保留。
不冒充法律认证/逐字节重现/宿主开机/备份恢复；全部取舍及代价如上，最终CI/源码/
归档/准确合并/main/立即释放以PR23最终元数据为准，前一轮全绿不替代修复后验收。
