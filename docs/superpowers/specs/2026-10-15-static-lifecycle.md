# M2B: Static preparation and host-input conformance

> English (default) | [简体中文](2026-10-15-static-lifecycle.zh-CN.md)

Architectural increment under the user's continuous-delivery mandate; implement inline, one final independent review. No repeated approval handoffs.

## Outcome and boundary

Add long-running `picketxd prepare`: one complete YAML publication is the static authority. Reuse strict shared decoding/compilation/replacement, persist provider-bound last-valid desired state, recover on restart, use bounded reads/file locking/cancellable polling/absolute-time projection. Report `prepared` and `enforcementConfirmed: false`; this is preparation and recovery, not production enforcement.

Also validate real host-local INPUT execution with accepted static configuration in isolated namespaces. Existing M0/bridge is transit-fixture-only; do not promote it to production by renaming it. Test-only `HostInputFixture` selects INPUT and original conntrack tuple matching, including DNAT. Reject mixed host/NAT-fixture options. Drop untracked/invalid packets to protected current destinations before original-tuple dispatch. Preserve all 36 prior required cases. Production table/mark ownership, strict cleanup recovery, fail-closed future restrictions during process loss, container-published paths, transport/L7/admin remain subsequent delivery.

## Static state

`internal/agentstate` fixes ProviderID, AgentRef, independently cloned Facts, RuntimeKey, BootID, cache path and injected clock. Facts must match authority/Agent. Command facts come from explicit host destination bounds/profile and current boot/namespace identity; preparation facts are not live enforcement capability evidence. No Controller fallback merge. RuntimeKey binds configured host/profile/destinations; boot/namespace are recovery metadata, never apply receipts.

Use a private 0700 state directory and current-owner 0600 regular lock/cache files; reject symlinks. Hold an exclusive nonblocking flock until Close. Cache v1 contains static provider type/identity, Agent, runtime fingerprint, boot, accepted generation/time, normalized policy hash and complete original YAML bytes. Canonical JSON rejects altered/duplicate/null/unknown/trailing representations, wrong provider/Agent/runtime, zero generation/time, corrupt hash and invalid policy. Recompile against current facts without renewing deadlines. Restore never confirms an applied revision; report a different boot explicitly. Missing cache permits bootstrap; corrupt/mismatched cache fails rather than resetting authority.

Decode/validate/replace completely, then temp file + fsync + rename + directory fsync before in-memory acceptance. Known pre-publication failure retains old state. Publication with uncertain durability faults the store until reopened; never claim the old disk snapshot is still authoritative. Identical semantic hashes do not advance generation or rewrite cache; a valid equivalent reload clears the last reload error. Rejected reloads retain all absolute deadlines. Attempts/errors are distinct from accepted-at/hash/generation. Status projects at an explicit instant, including future boundaries; no applied-success flag or reconstructed receipt is cached.

One cancellable timer loop, no overlapping reload. Boundary wakes freshly project current state even if every reload fails; never capture an old generation in a timer. Close releases lock; cancel and join before teardown. No restart or clock observation turns an absolute end into a duration.

## Command

`picketxd prepare --policy FILE --state FILE --provider ID --agent NAME --destination IP/CIDR [--destination ...] [--profile host] [--poll 5s] [--once]`. Cobra and explicit flags; Koanf runtime-file loading remains deferred. Only static preparation, no listener/kernel mutation/credentials. JSON lines stdout, diagnostics stderr. Once returns nonzero on rejected current publication even if it reports retained cached state. Long-running preparation may retain valid cache on reload failure, with visible errors until repaired. Invalid initial policy without cache fails. Only host-local/original-destination/graceful preparation is advertised; unsupported Bindings fail shared capability checks.

## Host evidence and file map

Use disposable topology namespaces, shared YAML compilation and accepted state/recovery. Apply in the server namespace and confirm receipts. Prove both-family unfiltered paths before interpreting denies. Required cases: INPUT default deny; source union/deny; native expiry with invalid reload; recovery/replay preserving deadline; DNAT original-port matching; untracked rejection; INPUT versus transit separation. Keep M0=17, bridge=19, add host required gate; opted-in setup failures fail, never skip.

| Files | Responsibility |
| --- | --- |
| internal/agentstate/{store,journal,driver}.go + tests | Candidate, private atomic cache/lock, recovery/status/timers |
| cmd/picketxd, internal/agentcli | Prepare flags, explicit host facts/identity, cancellation/JSON |
| internal/validation/nftbackend/* | Test-only INPUT/original tuples and untracked/invalid guard |
| internal/validation/hostbridge/* | Accepted static state to actual host fixture/packet evidence |
| report, kernel runner, m0-report | Preserve prior gates and add required host cases |
| go.mod/sum, notices, README/docs | Tested M2A pin, bilingual scope/evidence |

## Execution and review

1. RED→GREEN cache identity/lock/corruption/write-failure/copy/deadline tests; full suite; commit.
2. RED→GREEN latest-generation driver/cancellation/command tests; static binary smoke; commit.
3. RED→GREEN host expressions and actual kernel cases; retain every prior case.
4. Go/race/vet/build, one fresh reviewer, regression fixes, publish and inspect real Linux evidence.

Review focus: no cache authority mixing; uncertain persistence cannot advance acceptance; reload failures never renew deadlines; stale timers never revive permission; host matching uses original target under NAT and fails closed for untracked protected traffic.
