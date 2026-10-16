# M2C4: durable host admission

> English (default) | [简体中文](2026-10-16-host-admission-design.zh-CN.md)

## Intent and scope

Continue the authorized production L4 work through a real, recoverable kernel admission engine. Preserve all 55 existing native gates. This increment exposes a package API, not a production daemon command. The command and standalone publication reload loop follow after this engine's crash contract is proven. Support only compiled host-local/original-destination/graceful TCP/UDP, equally for IPv4 and IPv6. No strict cleanup, controller transport, FORWARD, L7 or management API.

## Approach

Use a deterministic typed rule image, generated from complete `hostpolicy.Policy` inventory and durable marks. Do not adapt the private fixture backend. Replacing the whole owned table atomically is simpler than editing individual rules, but only after strict readback proves the entire previous table. Keeping marker-only ownership would prevent useful enforcement; adopting arbitrary tables based on marker alone would permit destructive replacement. Neither is acceptable.

Rules use original conntrack destination/source/protocol/port, with current-tuple INVALID/UNTRACKED protection. Denies precede established marked acceptance; new TCP admission requires SYN without ACK. Target-specific marks preserve graceful established flows while retirement prevents reincarnation reuse. Default drop remains when the last Grant disappears. Explicit empty target inventory replaces protection with the marker-only structure, preserving installation ownership.

Kernel `NFT_META_TIME_NS` plus host-to-network byte order conversion and unsigned comparisons enforce complete absolute validity intervals, including future permits and denies. No process timer or relative timeout extension is needed. Bound supported nonzero timestamps to Unix nanoseconds in [1, MaxInt64]; reject unsupported dates before effects. Clock correctness and trusted NET_ADMIN exclusivity remain operator prerequisites. Native tests must prove future deny activation, allow expiry and recovery without a live scheduler.

## Durable application contract

Journal v3 adds previous confirmed image and pending attempted image, with a monotonically increasing attempt number and SHA256 image identity. Store semantic targets, complete validity and assigned marks; regenerate exact kernel structure from validated values. Maximum 4096 targets, 8192 total generated rules and 8 MiB serialized image. Raise combined journal maximum to 64 MiB to hold the 32 MiB bounded mark history and two bounded images. No migration silently downgrades v3.

Under the Session mutex: validate policy and image, verify current full live structure, durably publish pending image while retaining previous confirmation, atomically replace owned table, obtain ACK, strictly read back exact complete structure, then durably publish confirmed image and clear pending. All failures after pending publication make the session faulted and return no enforcement receipt; close/reopen is required. A stored boolean is never a live receipt.

Open configures the native inspector using only validated journal images before touching the kernel. Same-boot recovery accepts exactly marker-only previous state, the recorded confirmed image, or the pending image. It resolves the attempt only from exact complete live structure. Unknown rules, changed expressions, hook/policy/priority, additional chains/sets/objects/flowtables, interrupted/malformed dumps, or an unrelated marker fail without replacement. A new boot preserves mark retirement but clears application proof; fresh ownership is marker-only until explicitly applied again. Shutdown never removes filtering.

Each rule's semantic expression attributes are compared strictly to generated expected wire attributes. Permit only counter value changes and kernel handles/use metadata. Validate nested attribute envelopes and duplicates; never rely on a decoder silently discarding unknown expressions. Read generation before and after all structure dumps and reject concurrent changes.

## Context map

| Files | Responsibility |
| --- | --- |
| `internal/hostrules/{image,rules}_linux.go` | Typed bounded image and deterministic native expressions; imports hostpolicy, markstate, shared Core and nftables. No sockets or persistence. |
| `internal/hostowner/{application,inspect}_linux.go` | Durable apply/recover and strict native full-image validation. |
| `internal/hostowner/{owner,nft,raw,marks}_linux.go` | v3 journal integration, namespace-bound transactions and session exclusion. |
| `internal/hostrules/*test.go`, `internal/hostowner/application_test.go` | Temporal/membership/validation, pending publication, ambiguous dispatch and recovery behavior. |
| `internal/hostowner/admission_integration_test.go` | Actual dual-stack packets, native timing, restart, drift, withdrawal and rejected transaction preservation. |
| `internal/validation/report/report.go` | Additional mandatory admission evidence without weakening existing 55 gates. |

API changes are internal; no dependencies or configuration schema changes. Journal v3 is a deliberate forward-only migration. Reference patterns are the existing descriptor-pinned journal, strict Netlink stream reader and ownership process tests. Fixture backends remain unchanged.

## Acceptance

Unit/race/vet/build and integration compilation must pass. Fresh review covers privilege/namespace scope, image tampering, pending/confirmed ambiguity, maximum sizes, and future boundaries. Real CI must pass every previous gate plus new engine packet/crash gates, with actual kernel/toolchain/module versions, no failed or skipped cases. Merge only the exact reviewed and tested tree. Do not describe this package increment as a deployed daemon.
