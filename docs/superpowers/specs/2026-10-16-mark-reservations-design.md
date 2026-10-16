# M2C3: Durable conntrack mark reservations

> English (default) | [简体中文](2026-10-16-mark-reservations-design.zh-CN.md)

Continue the approved L4 readiness sequence after PR #6, main `201b79db07942f15bda8d473441d6aee248e75e7`. This architectural slice completes reserved mark lifecycle and its durable namespace boundary before admission code may consume marks. Use inline execution and one fresh final reviewer under the existing continuous-delivery authorization.

## Outcome and choices

An operator explicitly reserves a contiguous nonzero range of complete 32-bit conntrack mark values. No mask sharing, automatic default or claim of protection against another privileged manager is implied. Range capacity is 1..4096; allocation is monotonic for the installation lifetime, with permanent retirement tombstones and explicit exhaustion. This sacrifices recycling to prevent stale graceful connections regaining authority after target recreation. Garbage collection/reuse and larger ledgers are separate future work.

Use the existing locked, descriptor-relative ownership journal rather than a second file: losing or changing a mark file must not allow old marks to be silently reissued. Keep v1 journals readable; the first mark reservation publishes v2 with the complete ledger in the same atomic durable replacement. v2 requires a valid ledger, v1 forbids one. Old binaries reject v2. Keep the kernel owner marker v1 unchanged because the table still contains only that marker; versioning this journal does not introduce an applied-rule contract.

## Pure lifecycle contract

`internal/markstate` defines `Range{First,Last uint32}`, `Target{Key networkplan.TargetKey, Shape string}`, `Assignment{Target Target, Mark uint32, Retired bool}`, and `State{Range Range, Entries []Assignment}`. Shape is a canonical lowercase SHA-256 hex value. Every key identity is nonempty valid UTF-8, at most 256 bytes and contains no NUL.

`Reconcile(previous *State, reservation Range, targets []Target, occupied []uint32) (State,error)` validates the whole candidate and returns independent data without mutation. Reject duplicate desired target keys, invalid state, changed reservation, foreign occupied values anywhere inside the reservation, and exhaustion. Existing allocated marks, including retired entries, are recognized as potentially live owned marks; other occupied values in the reserved range are a collision. Marks outside the range do not interfere. Entire fresh range must be unused, even when only its first value would be allocated.

Sort desired targets by complete key before allocating. Entries are append-only, ordered, contiguous `First + index`; no gaps/reordering/duplicates. At most one nonretired assignment exists per target key. Same key and shape retains its active mark. Removing a target retires its assignment; reintroducing even the identical key/shape allocates a new mark. Changing physical shape retires the old assignment and allocates a new mark. No error may return partial state or retire the caller's previous entries.

`hostpolicy.Policy.MarkTargets() ([]markstate.Target,error)` derives requests from complete inventory, not just active Grants. Hash the full resolved context and physical matches; exclude membership, deadlines and Binding IDs so ordinary permission expiry/renewal and overlapping Bindings do not rotate the target's mark. Resource UID remains in the key. Shape or key changes cannot reuse stale authorization. Returned requests are independently mutable.

## Held-session and durability boundary

`hostowner.Session.ReconcileMarks(ctx, Range, []Target) (MarkStatus,error)` serializes on the held ownership session. Require each target's AgentRef to equal the owner. Freshly verify exact marker-only structure, complete both IPv4 and IPv6 conntrack dumps, reconcile the ledger, freshly verify ownership again, and durably publish the v2 ownership record before returning assignments. Check cancellation and pinned boot/namespace before phases. No-op desired state still checks both families and live ownership, but need not rewrite a durable ledger.

Retain the pinned journal writer on Session; no path reopen after setup. A save error, including post-rename fsync uncertainty, faults the session and returns no assignments; Close/reopen is required before more work. A restart validates scope, live marker and ledger; it never trusts disk confirmation alone. New boot keeps installation mark history/reservation while obtaining the normal fresh namespace epoch; no applied policy restoration is asserted. Missing/corrupt ownership journal with a present marker remains a refusal. Increase the ownership journal bound to 32 MiB to accommodate 4096 maximum-length entries; canonical JSON, private regular files and descriptor-relative publication remain required.

`MarkStatus` contains an independently owned State and fresh ObservedAt, not an apply receipt. Session status continues to report enforcement unconfirmed. No admission rules, conntrack mark writes/deletion, CLI filtering command, policy apply journal or recovery success is introduced. A future engine must require this ledger before writing marks and must preserve the table's versioned applied structure/history before extending marker-only ownership.

## Strict native collision observation

Reuse the namespace-bound raw Netlink inspector, extracting only the common multipart receive loop. NFT dumps retain the 4 MiB bound. Conntrack dumps use CT_GET/CT_NEW in NFNL_SUBSYS_CTNETLINK for AF_INET then AF_INET6, with a 64 MiB bound per family and streaming processing (no retained flow table). Validate sequence/PID, kernel sender, no truncation, completion, status, interrupted data/DONE, response type and nfgen family/version. Parse complete top-level attribute envelopes, reject duplicate/malformed mark attributes and read CTA_MARK as big-endian u32; absent mark means zero. Other flow fields are opaque because this operation proves only mark occupancy, not tuple ownership. No general deletion receipt can be inferred.

Complete both family scans before returning occupancy or publishing a reservation, including when the first scan already finds a collision. Bound/privilege/unsupported/canceled/interrupted/malformed errors are failures, never empty success. Five-second per-dump deadlines remain. This snapshot detects ordinary collisions; operator reservation and trusted NET_ADMIN exclusion remain necessary against concurrent privileged writers. Known numeric marks cannot authenticate who created a flow.

## Acceptance

Pure lifecycle tests cover deterministic allocation, high-u32 boundaries, duplicate/invalid input, whole-range collision, range mismatch, retirement/recreation, changed physical shape, exhaustion, malformed historical ledgers and mutation isolation. Host-policy tests cover stable marks across Grant/Binding changes and changed UID/physical shape. Real-filesystem session tests cover v1/v2 upgrade/reopen, failed publication before/after rename, no partial return, no-op scans, canceled/closed/drifted scope, new boot retaining tombstones, and large bounded canonical records. Raw wire tests cover both families, missing/zero/outside marks, duplicate/malformed attributes and interrupted completion.

Keep all 51 required kernel gates; add four mandatory `TestOwnerKernelMarkLifecycle`, `TestOwnerKernelMarkCollision`, `TestOwnerKernelMarkProcessRecovery`, `TestOwnerKernelMarkDrift`. Prove actual CT mark insertion/readback in both families, unrelated flows preserved, retirement/restart, standalone helper killed without Close, and drift refusal before publication in disposable namespaces. Native setup failures under opt-in fail. Real CI and downloaded exact-head artifact verification precede merge.
