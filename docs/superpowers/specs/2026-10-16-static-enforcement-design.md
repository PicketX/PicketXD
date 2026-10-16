# M2C5: standalone static host enforcement

> English (default) | [简体中文](2026-10-16-static-enforcement-design.zh-CN.md)

## Intent

Connect the accepted static publication store and the proven M2C4 engine to a runnable `picketxd enforce` command. Preserve `prepare` as nonfiltering. This is the explicit host-local/original-destination/graceful TCP/UDP slice of the eventual Agent, with equal IPv4/IPv6 support. Do not claim strict cleanup, Controller transport, NFQUEUE/TPROXY, L7, forwarding/container paths, management UI, service packaging or the eventual full-feature default command.

## Choices and contract

Use a small coordinator with constructor-injected ownership/store factories, output consumer and standard slog. Reusing `agentstate.Store.Run` preserves validated publication replacement, invalid reload retention and independent status boundaries. Duplicating the publication loop would introduce another cache/acceptance contract; directly invoking the fixture backend would discard proven ownership. Neither is needed.

`enforce` requires `--policy`, `--state-dir`, `--provider`, `--agent`, at least one `--destination`, `--mark-first` and `--mark-last`. Profile defaults to `host`, poll to 5s, `--once` defaults false. Mark values are full nonzero uint32, at most 4096 contiguous values; no default reservation. Validate all flags before effects. State directory must be absolute, cleaned, non-root; owner journal is `owner.json`, accepted desired cache is `desired.json` under that same private directory. The ownership layer validates/pins the directory and safe ancestors before the desired store opens it. Filesystem and same-UID privileged writer trust remain prerequisites; no automatic takeover or cache migration.

Runtime key hashes canonical Agent/profile/destination bounds/mark range. A new read-only Session API returns BootID and a stable network-space ID from the held namespace identity, under the session lock/revalidation check. Build trusted configuration facts from that identity, not a later pathname namespace lookup. Complete input goes through `hostpolicy.CompileInput`; `ApplyPolicy` remains responsible for durable baseline replacement checks and mark reservation.

Add `Store.Snapshot(at)` to atomically return status and its immutable Candidate. A callback must never label a newer candidate with an older generation/hash. On first emission or accepted generation/hash change, compile and apply the latest coherent snapshot. On unchanged generation, Verify complete live structure without repeating conntrack mark scans. Native time expressions already enforce future activation/expiry; Go timers update status only.

Output JSON separates desired metadata, ownership, application receipt, last successfully associated generation/hash, and top-level `enforcementConfirmed`. The top-level flag is true only when the latest coherent accepted desired generation/hash has a successful live receipt, with no cache fault. A previously confirmed application may remain installed while newer desired state fails; it must not be reported as confirmation of that newer state. Emit a failure status before returning application/verification errors. Never infer proof from persisted booleans.

Invalid reload with accepted state: retain it, report/log the error and keep polling. `--once` applies/reobserves cached accepted state if current publication is invalid, emits truthful state and returns the reload error nonzero. No accepted state: startup fails. A fresh ownership claim may leave a nonfiltering marker after a bad initial publication; it cannot grant traffic. Corrupt/foreign caches or tables fail startup without replacement. An ambiguous application or drift terminates the process after reporting unconfirmed current desired state. SIGINT/SIGTERM stop and release locks; Close never withdraws filtering. SIGKILL is recovered using existing durable journal and complete readback.

## Context map

| Files | Purpose/change |
| --- | --- |
| `internal/hostowner/scope_linux.go`, tests | Read held boot/namespace identity, refuse closed/faulted/revalidation errors. No new effects. |
| `internal/agentstate/store.go`, tests | Atomic candidate/status Snapshot, preserve all existing cache behavior. |
| `internal/hostagent/{config,run}_linux.go`, tests | Validate/canonicalize runtime settings; compose owner/store, coherent apply/verify/status, error and shutdown behavior. |
| `internal/agentcli/enforce_linux.go`, command/main | Expose required flags and JSON/slog plumbing; preserve prepare behavior. |
| `internal/hostagent/process_integration_test.go` | Launch actual prebuilt binary, actual dual-stack TCP/UDP packets, reload/expiry/SIGKILL/drift/scope tests in disposable namespaces. |
| `scripts/test-kernel.sh`, report/command | Build binary and process test binary, collect sixth raw stream and require new gates. |
| `examples/enforce`, README, bilingual validation | Runnable permanent minimal example and explicit lifecycle/withdrawal instructions; exact native evidence. |

Dependencies stay pinned. Internal APIs only; no journal schema migration. Existing marker/image/conntrack fixtures and all 64 old native gates remain unchanged. Public command is explicit opt-in for actual mutation; no hidden filtering in prepare.

## Acceptance

Observe failing tests before implementation for coherent snapshot, scope lifecycle, configuration validation, apply vs verify, invalid publication retention and error receipt. Full unit/race/vet/build/static smoke and integration compilation must pass. A fresh whole-branch reviewer checks startup ordering, desired/applied association, immutable trusted facts, status error paths, stream/binary execution and restart scope. Real CI must execute the actual binary without Go-runtime dependency in the child, both families/protocols, valid/invalid reload, native time after process death, corrupt/foreign state refusal, namespace singleton, explicit withdrawal and drift refusal. Opted-in setup failures fail, never skip. Check all raw streams/artifact digests and final exact PR/main trees before reporting completion.
