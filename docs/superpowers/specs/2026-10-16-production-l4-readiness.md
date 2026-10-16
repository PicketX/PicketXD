# M2C: Production L4 execution readiness

> English (default) | [简体中文](2026-10-16-production-l4-readiness.zh-CN.md)

Date: 2026-10-16 (Asia/Shanghai). Working implementation brief under the continuous-delivery mandate. This records the next execution boundary and unresolved runtime details; it does not claim a deployable enforcement service or finalize those details.

## Verified starting point

PicketXD PRs #2, #3 and #4 are merged. Main `c780c15d1156fbc26f40a420d987cc8f1f63ce1b` has exactly the source tree of tested M2B head `a5911f5256d97dcce510e4174d62f5741f946b94`: `1775dba66df34faa8af0b54954880441bfaf2c39`. Its 43 mandatory kernel cases and unit/race/vet/build/static-restart checks pass. The shared M2A configuration entry point is now merged in PicketX PR #10 after integration-conflict repair and successful CI `38769440110`.

`picketxd prepare` persists accepted desired state; it always reports `enforcementConfirmed=false`. The transit coordinator and host INPUT backend remain private validation fixtures. They cannot be deployed unchanged: they delete a fixed table without a production ownership claim, use the shared `TestMark`, and keep applied history only in memory.

## Intended next outcome

One static-authority host Agent can validate, apply, confirm, reload and recover a complete supported policy in its actual namespace. Start with explicit host INPUT/original-destination TCP/UDP and equal IPv4/IPv6 coverage. Keep the fixed `inet picketx` ownership boundary and existing Go Netlink dependencies. Controller transport, container-published/FORWARD paths, gateway routing, L7 and management remain later slices. Preserve the `prepare` command's read-only kernel behavior.

## Decisions required in the runtime implementation

| Boundary | Required behavior and review evidence |
| --- | --- |
| Namespace and process ownership | Hold a namespace descriptor and singleton lock for the entire Agent lifetime; revalidate the namespace/boot/runtime identity before effects. A lock beside one cache file does not prevent a second process using a different path from writing the same table. |
| Table ownership | Inspect before mutation. Refuse an existing foreign or unrecognized `inet picketx`; never treat the table name alone as adoption authority. A durable installation identity must match a kernel marker and a completely understood live structure. Do not add a blanket takeover/flush flag. |
| Conntrack ownership | Replace `TestMark` with explicit operator-reserved allocation and collision checks. Keep mark ownership distinct from policy identity. Define retirement/reuse before supporting same-tuple Resource/Endpoint recreation: old graceful flows must not silently become new authorization. |
| Persistence | Keep accepted desired state separate from applied history. Persist an application attempt before sending a transaction and preserve previous confirmed ownership. Persist confirmation only after ACK and semantic readback. Rename/fsync uncertainty is a fault, not success. |
| Ambiguous application | A crash between dispatch, ACK, readback and journal publication must remain observable as unknown/recovering. A pending desired snapshot alone never authorizes conntrack deletion. Recovery needs live ownership/structure evidence and must preserve potentially relevant old history. |
| Time boundaries | Native allow expiry must retain original absolute deadlines. A future deny or strict cutoff must remain effective if the process disappears before its timer fires; merely scheduling a Go timer is insufficient. Evaluate kernel-programmed bounds or an explicitly declared fail-closed target guard, without changing graceful semantics silently. |
| Strict recovery | Confirm the restrictive policy before scoped deletion; retain cumulative historical permissions across expiry, intervening apply and cleanup failure. Interrupted dumps, partial deletion and remaining targets remain errors. Reject unsupported strict requirements until the runtime path proves them. |
| Explicit withdrawal | Removing the last Grant preserves protection. Removing the last Binding removes only owned protection after validated replacement. Define mark retirement and same-name/UID recreation together; desired withdrawals are not installation receipts. |
| Status and shutdown | Separate desired generation/hash, applying/confirmed/unknown/recovering, cleanup-pending, observed time and errors. Restart never restores a successful live-confirmation boolean from disk. Normal stop does not silently unprotect services. |

The pinned nftables library exposes rule `UserData`; its table/chain models do not provide an equivalent ownership field. A dedicated unhooked owner chain and marker rule is a candidate, subject to structural readback and namespace tests. Treat the marker as collision detection under trusted NET_ADMIN ownership, not authentication against another privileged process.

## File and dependency map

| Area | Existing reference | Next responsibility |
| --- | --- | --- |
| Static desired input | `internal/agentstate` | Preserve accepted-state identity, complete replacement, private durable publication and absolute deadlines. |
| Runtime coordinator | `internal/validation/planbridge` | Introduce a production package with explicit ownership/persistence; do not import private fixture models into the production command. |
| nftables engine | `internal/validation/nftbackend` | Extract reusable mechanics with a reviewed runtime input/receipt contract; validation adapters retain all old gates. |
| Conntrack engine | `internal/validation/ctbackend` | Preserve exact original/reply/zone targeting and complete-dump/error handling; add runtime ownership only after its contract is fixed. |
| CLI assembly | `internal/agentcli`, `cmd/picketxd` | A separate enforcement command with typed runtime settings, injected effects and cancellable lifecycle. |
| Real acceptance | `internal/validation/hostbridge`, `scripts/test-kernel.sh`, report package | Exercise the actual standalone command and independent restart/crash processes in disposable namespaces. |

No code extraction should change the existing 17 M0 + 19 bridge + 7 host gates. Do not add a production command which only wraps `HostInputFixture` and calls that enforcement readiness.

## Implementation sequence and acceptance

1. **Ownership and crash contract:** fix the table marker, namespace lock, reserved mark lifecycle and durable attempt/confirmation schema. Tests must distinguish foreign table, foreign marker, missing journal, damaged journal, alternate state path, same-boot crash and new-boot recovery. Test each crash point before exposing a mutating command.
2. **Reusable kernel engine:** move only required mechanics behind runtime types; keep semantic readback, absolute dispatch-time membership and all existing fixture adapters/tests. Record real kernel evidence after extraction.
3. **Static host enforcement loop:** connect accepted configuration, application journal, apply/confirm and boundary driving. Invalid reload retains the last confirmed rules; ambiguous outcomes cannot report confirmed status or start destructive cleanup.
4. **Standalone process acceptance:** real dual-stack packets through the deployed command, restart before/after expiry, loss before a future deny, strict retry where supported, explicit empty withdrawal and unrelated-table/flow preservation. Require the new cases as well as all 43 existing cases.

The first runtime increment must state exactly which handling/time-boundary requirements it supports. A capability rejection is acceptable during staged development; silently accepting a contract the process-loss path cannot enforce is not. After this brief, resolve the runtime contract and produce its focused executable plan before product code.
