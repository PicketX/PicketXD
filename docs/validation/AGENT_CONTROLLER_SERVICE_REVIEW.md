# Controller-mode Agent service review and correction

ONE independent read-only full-branch review: base5373595..local4b2f3e4 (remote
37c5d92/tree47d62fe). Reviewer read all19files, spec/plan/ledger and actual Core /
runtime sources; no edits, no delegated/second review. Verdict **No**,0Critical,
3Important,2Minor. Python6/focusedNode27/offline syntax passed, explicitly NOT
native success. Root performs ONE correction pass; no fabricated platform approval.

## Findings and root rulings

1. Important: remote stream errors discarded; restored cache has no current-source
   distinction, so cache can release READY offline. Reviewer real Store reproduction
   emitted73 clean cached statuses/1.5s. First new-unit native39315655520 also fails
   at outage status wait (20:05:20Z), not accepted as success. Root three behavior
   tests RED→GREEN: restored cache before current acceptance, real official gRPC
   outage/reconnect before/after expiry with byte-identical cache, and notify clean
   predicate. Add process-local diagnostic remoteState awaiting/current/disconnected,
   propagated coherently to desired JSON; serialize disconnect with snapshot events.
   LastError remains publication rejection, not connection health. Only current
   remote acceptance can release initial notify; static behavior unchanged. Scope
   ruling supersedes original no-runtime-edit assumption, not the guarantee. Cost:
   additive diagnostic field/tighter remote startup; no durable/protocol/kernel
   schema or permission/deadline changes. All ordinary/race/native gates required.
2. Important: worker timeout kills only leader; stop timeout can bypass descendants
   and namespace cleanup. Root actual descendant/ignores-SIGTERM/unit-stop-timeout
   regressions RED→GREEN. Private worker is session/group-owned; Linux test driver
   acts as child subreaper, TERM/KILLs exact group and reaps leader/adopted descendants.
   Independent cleanup stages continue after errors and preserve original failure
   context. Exact owned runtime Agent unit remains separate; stop failure attempts
   scoped kill/stop, then remaining cleanup still executes and reports failure.
   Cost: Linux test harness uses libc prctl/subreaper; shipped Go runtime still
   CGO-free. Never touch host/global units, locks or other namespaces/process groups.
3. Important: a protected handler thread can die while unprotected health survives,
   turning service failure into false expiry denial. Root failed-protected-worker
   regression rejects health even while five other workers remain alive, and real
   TCP-reset/second-request regression proves ordinary disconnect does not kill the
   listener. All six workers supervised; unexpected exit/failure makes independent
   health return a non-echo failure marker. Cost: strict test-peer health can fail
   the scenario, intentionally not be accepted as network denial.
4. Minor TLS documentation: fixed both languages; verified server name is mandatory,
   only custom CA optional. No insecure/mTLS substitution. No deferral.
5. Minor stale report: exact prior report removed before top-level start; worker
   writes private provisional result, parent publishes only after all owned cleanup
   succeeds. Exact unrelated artifact sentinel preserved in regression. No deferral.

## Considered but declined — explicit root disposition and cost

- Host reboot/boot firewall: retain exclusion; no boot-recovery guarantee.
- Backup/relocation/restore execution: separate next increment; no disaster-recovery claim.
- Established graceful sessions: intentionally new flows only; existing flows can survive.
- Automatic enrollment/source migration: operator-managed; manual identity/source selection.
- Native final archives/main integration: reviewer cannot judge pending evidence;
  root MUST verify complete exact-head CI/raw/source/four actual downloads and main.

## Tests-first chronology and remaining proof

39312474772 (initial revision assumption),39312789768 (runtime-directory fixture),
39313499231 (ProtectHome hid /home artifact) are preliminary wiring failures only.
Actual target RED39314552719/job118272524446 at19:57:33Z: old static unit starts,
but actual connected Controller APPLIED receipt times out. Only then shipped new
unit/package-required content. Initial implementation native39315655520 is red due
to actual runtime readiness/connection gap above, not waived or relabeled green.
Local Go normal/race have five inherited Unixgram EPERM failures, never skipped;
full privileged Linux required. Exact corrected-source/final four ZIP digests /
raw logs / expected-head merge / main recheck / immediate release belong in PR24
metadata to avoid self-referential documentation CI cycles. Overall goal active;
same-boot service recovery is not host reboot or complete backup/restore acceptance.

中文：一次全分支独立评审发现3项Important、2项Minor、无Critical，原结论不可合并。
根执行器一次修复：独立远程状态与就绪门禁、超时子进程组/孤儿回收、全部处理线程
健康监督；实际失败回归先RED后GREEN。两项Minor一并修复，无延后。严格保留原失败
时间线、权限/期限/仓库边界，最终完整原生/四归档/准确源码/main证据及释放记录见PR24。
