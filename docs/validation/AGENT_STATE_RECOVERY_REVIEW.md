# Stopped Agent recovery review and rulings

ONE independent complete c5d58ea..b514673 review by agent_backup_review; original
verdict With fixes,0Critical/3Important/1Minor. Not a fabricated GitHub approval.
Reviewer reran8 filesystem/12 evidence tests only, not native CI. Root checked
findings against real filesystem/control flow, then ONE regression-first pass.

## Findings and verification

1. Important: /var/lib state rename into /run fails EXDEV on ordinary split mounts.
   Real /tmp→/dev/shm reproduction errno18, original intact. Added actual separate-
   filesystem test: missing same-filesystem helper RED, sibling quarantine GREEN.
   Native fixture now quarantines on original filesystem and owns its cleanup path.
2. Important: unconditional teardown erased original checkpoint on failed recovery.
   Added real partial-restored-file/original/backup retention, runtime cleanup error
   and success-only deletion tests:4 added tests RED→GREEN. Stop/reap/unregister
   still mandatory; failure keeps private files for root inspection, never uploads.
   Invalid candidate report cannot mark cleanup successful before validation.
3. Important: former running/offline across deadline was replaced by stopped expiry.
   Missing live expiry evidence regression RED→GREEN. Restored third PID runs
   disconnected across original expiry; fourth PID restarts already expired. Public
   phase-specific actual PID/running state is strict and mutation-tested.36 packets,
   all former scenarios retained. No timeout/rate/authorization bypass.
4. Minor DEFERRED: independent file check compares inode only, not device+inode.
   Accidental same inode numbers across filesystems could reject a valid copy;
   fail-closed false rejection only, no invalid adoption or permission extension.
   Not folded into the blocker correction pass. No claim of universal portability.

After correction local filesystem12/12, evidence13/13, fullNode203/203 pass. Initial
Node attempt missing Go PATH caused3 existing bundle tests to fail; pinned PATH
passes. Full local Go normal/race retain ONLY the five inherited TestNotifyEmitter*
Unixgram EPERM failures; vet/build0. Actual native CI, exact tree/four archives,
expected-head merge and main proof remain mandatory. Immutable final IDs in PR25.

## Root rulings on every declined behavior (and costs)

| Behavior reviewer set aside | Root ruling and cost if wrong |
|---|---|
| Old checkpoint replay/malicious manifest replacement | Excluded; no root-proof external monotonic witness. Operator must select latest, otherwise stale authorization can resurrect. |
| Concurrent root writes/arbitrary starts | Exclusive maintenance required, repeated stopped checks are not a continuous lock. Cost: unsafe concurrent operations require a separate design. |
| Ancestor aliases/symlinks/pathological caller paths | Trusted private fixture caller only, not a privileged public API. Reuse needs additional path hardening. |
| Arbitrary group ownership | Root:root fixture; record mismatch fails closed. Other ownership may reject rather than restore. |
| Unusual private directory modes | Fixture0700 restored0700 only. No general directory metadata fidelity guarantee. |
| Power loss/fsync/reboot/migration/old binary compatibility | Explicitly excluded. This acceptance cannot justify disaster recovery or incompatible binary rollback. |
| Production drop-ins/CA/EnvironmentFiles/extra credentials | Runbook requires them; fixture's six files are complete only for its exact unit. Omissions in custom deployments prevent safe recovery. |
| Transient copy modes | Actual secret targets are in owned0700 directories, final modes checked. Cost: helper must not be reused with untrusted public parent directories. |
| Plaintext checkpoint | Private ownership/modes, encryption/key management external. Cost: root/storage compromise discloses credentials. |
| Graceful surviving sessions | New flows only, explicit scope. Existing application sessions may survive stop. |
| Before-deadline authorization | Correct intentional allow while still valid. Time synchronization remains an operator requirement. |
| Partial remnants block retry | Intentional fail-closed, retained originals; no automatic overwrite/retry. Cost: operator inspection/recovery needed. |
| Inherited runtime supervision/namespace cleanup/report | Preserve existing checks; fix new private-material lifecycle only. Cost: inherited runtime defects remain bounded by full native gates, not inferred fixed. |
| Native/full Go/Node/CI/archive provenance | Reviewer did NOT establish them. Root independently runs exact-source native/raw/download verification; no merge until full success. |

Other root rulings: routine process pauses waived by explicit autonomous mandate
(cost: focused reversible changes proceed without interim user review); inline
execution retains one independent final review. No general privileged backup CLI
(cost: operators use reviewed runbook); same latest stopped original-path scope
(cost: host reboot/migration remain separate). Private helper assumes trusted
exclusive maintenance (cost: no atomic multi-file/power-loss guarantee).

中文：一次独立全分支评审发现3个阻塞、1个小项。跨文件系统隔离、失败保留原件与
运行中离线跨期限场景已分别先RED后修复，改为四PID/36包，原有门禁不削弱。inode未
比较设备号小项延后，可能误拒绝有效副本，不会误放行。全部边界和代价如上；完整
原生/源码/实际归档/main证据以PR25最终回执为准，不把本地或评审当作原生成功。
