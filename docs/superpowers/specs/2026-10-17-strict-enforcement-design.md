# M2C8: static host strict enforcement

> English (default) | [简体中文](2026-10-17-strict-enforcement-design.zh-CN.md)

## Intent and scope

Continue the approved staged L4/L7/management delivery with a runnable strict host-local command. The user has authorized implementation, push and suitable passing merges; execute inline, with one fresh whole-branch review. Preserve graceful defaults, existing hashes and all80 native gates. No Controller, L7, FORWARD/container, automatic handling-mode migration, durable cleanup receipt, or fixes to inherited deferred minors.

Strict means every protected packet must satisfy current source/endpoint/absolute-time permission and deny precedence. Removing a Grant or reaching its deadline blocks existing traffic even if the process disappears. Exact conntrack cleanup is a separate, observed reconcile operation; it does not send TCP RST or promise immediate peer notification. Dead processes do not independently delete conntrack entries. UDP may resume on a later valid grant because it is connectionless; successfully cleaned TCP must not be admitted solely by re-created unmarked conntrack state. Retained marked TCP may resume if permission returns before cleanup; this increment does not prove completed revocation across unobserved permission gaps.

## Rule image

Accept explicit host-local/original-destination strict as well as graceful scope. Keep graceful image v1 and exact JSON/hash unchanged. Strict uses image v2 with optional sorted unique KnownMarks, bounded by4096, copied from durable assignments in the same Agent/context (active and retired). Current target marks must be included. Owner and cleanup constructors additionally validate every alias against the exact durable ledger; unknown aliases refuse startup/selection. V2 cannot be silently read by older binaries. Journal remains v4; strict images and history are validated before any effects.

Strict target chains evaluate deny first, then current members with kernel absolute time predicates. UDP allowed members mark and accept as before. Each TCP allowed member first jumps to a shared owned-established chain: only known marks and ESTABLISHED/RELATED accept. The chain returns otherwise; a separate matching fresh SYN plus conntrack NEW rule writes the current target mark and accepts. Everything else drops. There is no unconditional established bypass. A shared chain costs one rule per historical mark, rather than per member per mark; MaxRules8192 and MaxImageBytes8MiB include the new rules and aliases and reject before effects.

Known retired marks preserve independently allowed established connections across physical target changes. They are never reclaimed; current member matching still gates acceptance. Empty target images withdraw protection and emit owner-only structure; cleanup still uses retained history. Existing original-destination/DNAT, invalid/untracked and exact readback behavior remain.

## Reverse reconstruction and endpoint reservation

Native run39004655614 reproduced new IPv6 TCP payload passing after exact deletion: the kernel recreated mark0 conntrack with the service port as original source and the client port as original destination. Original-destination dispatch alone could not classify subsequent client packets. Strict therefore adds one final raw-destination/reply-direction DROP per protected endpoint, after every original-destination dispatch. Original-direction DNAT aliases outside scope retain their behavior; normal currently permitted owned flows dispatch first. Unmarked/unrelated conntrack records are never deleted by this guard.

Strict protected local address/port pairs must be reserved for services. Do not use them as source endpoints for locally initiated client sessions: their incoming replies can be denied. This packet-scope cost is explicit; graceful rules/encoding remain unchanged. No namespace-wide conntrack sysctl or output policy is changed. Count each added guard in MaxRules. Regression includes both-family reversed unmarked UDP records (packet denial plus record preservation), five isolated physical-retarget repeats and strict protected/outside-scope original DNAT.

## Runtime and status

Config/CLI adds handling=graceful|strict, default graceful. Desired trusted facts advertise the selected handling. Runtime key serialization stays unchanged, preserving old state identity. Same-boot handling/context migration remains unsupported: reject a recorded application of the other handling before desired-store changes. Existing history also rejects incompatible scope before mark/nft effects. Fresh strict deployment, same-mode reload/restart and verified new boot are supported; no deletion/reset of old state is offered as a migration shortcut.

For strict, every accepted driver callback applies or verifies the image, then calls held Session.RevokeRecorded, then re-reads coherent desired status. This includes initial start, valid replacement, poll, invalid-publication retained cache and absolute-time boundaries. Incomplete history, pending ambiguity, delete/dump/drift failure or a crossed membership boundary is fatal and cannot report strict cleanup confirmation. Rules/journal are retained; restart reobserves and retries cleanup. Existing Store boundary scheduling is reused. Full both-family scan/readback runs each callback: predictable recovery rather than a persisted/cached receipt; polling is operator-configurable.

JSON adds optional strict metadata: enabled, cleanupConfirmed, observedAt, validUntil, imageDigest and scoped Result. Confirmation requires successful current cleanup, remaining0, matching latest desired generation/hash, live application/owner proof and no crossed earlier NextChange. It describes that observation only, is not persisted, and expires at validUntil or a later desired change. Existing top-level enforcement confirmation in strict additionally requires cleanup success. Graceful does not call cleanup and keeps existing semantics. Once performs one reconcile and exits with rules retained; future packet deadlines remain in kernel, future conntrack deletion requires a later running process/restart.

## Context map (self-reviewed)

| Files to modify/create | Purpose/change |
| --- | --- |
| hostpolicy/policy.go/tests | Allow explicit strict scope; preserve matching context/capability checks. |
| hostrules/image_linux.go; strict_linux.go/tests; rules_linux.go | V2 aliases, exact ledger check, bounded strict layout; v1 unchanged. |
| hostowner/application_linux.go/tests; hostrevoke/plan.go/tests | Validate image aliases against journal ledger; expose current handling. |
| hostagent/config_linux.go; run_linux.go; strict_linux.go/tests | Selected facts, apply/verify→cleanup→coherent status; no persisted receipt. |
| agentcli/enforce_linux.go/tests | Explicit handling flag; default unchanged, unknown values fail before effects. |
| hostowner/strict_integration_test.go; hostagent/strict_process_integration_test.go | Actual rules, preservation, pending/restart, process loss/expiry and regrant. |
| validation/report/report.go; bilingual examples/README/validation | Seven required native gates and exact evidence. |

Dependencies: existing pinned Core/planner/configuration, markstate, hostpolicy/hostrules, hostowner and hostrevoke; no new module/version. Reference patterns: hostrules absolute time, M2C6 cleanup, M2C7 complete history, hostagent driver/process harness. Tests: existing unit/layout/readback/agent fake effects plus new semantic/native gates. Risk: internal image v2/Owner interface/status extension; no database/config document schema migration, optional CLI flag only. Context map reviewed against the exact accepted main tree before changes.

## Acceptance

Observe strict-scope/image/ledger and driver-order/status regression RED before implementation. Test graceful serialization unchanged, strict no bypass, alias corruption, limits, source union and failure/crossed-boundary/latest-generation status. Full unit/race/vet/build/static smoke and integration compile. Seven new required native gates: owner strict membership and unrelated/independent preservation; physical replacement with retired mark; rejected/applied pending recovery; actual command revoke/reload; killed process expiry plus restart cleanup/regrant; reverse reconstruction and original DNAT scope. All87 required and actual raw counts, zero skip/fail, downloaded artifact hash/report/raw/pins/binary checkout/exact PR/main tree checks. One fresh whole-branch review; Important/Critical fixes one RED→GREEN pass, Minor deferred.
