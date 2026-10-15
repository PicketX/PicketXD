# M2B implementation plan and ledger

> Use executing-plans inline; user authorized continuous implementation/pushing. One final fresh reviewer.

Spec: [Static lifecycle](../specs/2026-10-15-static-lifecycle.md). Go 1.27; pinned M2A shared configuration and existing Netlink backends. No production enforcement claim. Cache/status never contains applied success. No Controller/Web/migration changes.

1. agentstate: New(Config), Restore, Accept, Status(at), Current, Close; private atomic cache/lock, complete shared replacement. RED→GREEN missing/corrupt/wrong identity/boot cache, copies, locking, write failure, generation and absolute replay; full suite and commit.
2. Driver/Cobra: Run(ctx,path,poll,emit), bounded publication reads, latest-generation projections, retained-error status. RED→GREEN startup/fallback/deadlines despite errors/cancel/Close/flags/stdout, static build and commit.
3. HostInputFixture and hostbridge: expression tests first, original CT fields, invalid/untracked guard, mixed options rejected. Add required host scenarios to report/runner, retaining old 36. Commit, one review, regression fixes, publish and inspect actual kernel evidence.

Review focus: cache identity cannot switch authority; uncertain writes cannot advance; failed reload cannot renew; old timers cannot revive; INPUT must use original NAT target and reject untracked protected traffic.

Ledger:
- M1.3 local tree equals previously passed remote tree. Shared module update targets tested M2A PR #10.
- Ruling: M0 does not establish production ownership/mark or crash/strict/future-deny semantics. Deliver preparation daemon plus test-only real INPUT conformance, not a rushed production promotion.

- Task 1: complete, ebb0c7c. Missing API RED→GREEN; private state/identity/corruption/locking/write-failure/replay tests and full suite pass. Deep-copy regression caught reuse of caller slice storage during JSON decode; fixed by decoding into a fresh Facts value. New restores automatically before any acceptance, keeping invalid-cache bootstrap impossible.
- Task 2: complete, dc7aaa4. Driver and Cobra RED→GREEN; independent expiry under failed reload, latest generation, cancellation and real command restarts pass.
- Task 3: host expression codec reproduced upstream failure: kernel u8 direction was decoded as u32. Official upstream fix 8a8ad2be810324e22fbfc7db3259d3d76e5d811d also supplies family-specific CT address keys; pin v0.3.1-0.20250606091825-8a8ad2be8103. Its required x/sys v0.31.0 and x/net v0.38.0 upgrades are explicit. Preserve all 36 old gates and run all 7 host gates. Cost if wrong: backend compatibility changes; actual full kernel CI required, not just unit approval.
- New upstream rule creation expects echoed handles before ACKs. Old unit transports treated EOF as success; now model actual echoed handles and ACKs. This is test transport correction, not weakened production confirmation.
- FIFO cache startup hang reproduced RED and fixed with nonblocking open before regular-file validation. No production enforcement API introduced.
- Independent review: one Important first-directory durability finding. Fix syncs state directory and every ancestor before lock/startup, including retries after a failed mkdir sync. Deterministic directory-operation failure and real post-rename/injected-fsync failure tests assert no in-memory advancement, fault-until-reopen, and validation of whichever canonical cache was actually published.
- First real CI 38759041126: original 17 M0 + 19 bridge all pass under the updated modules. All 7 host cases reached successful unfiltered IPv4/IPv6 TCP/UDP/control probes, then failed because prebuilt test binaries run from repository root and used a package-relative example path. Embed the example into the host test binary; runner cmp prevents example/embedded fixture drift. No kernel gate removed or relaxed.
- Second CI kernel job: 43 required cases and 63 top-level tests passed, no failures/skips; verified artifact recorded in docs/validation/M2B*. Unit race run exposed the FIFO test timing guard including unrelated durable ancestor sync. Isolate journal-read blocking guard, separately assert startup rejection; removing O_NONBLOCK with a local overlay fails, actual source passes 10 race repetitions and full race/vet/build/static smoke.
