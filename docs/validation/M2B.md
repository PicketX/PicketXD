# M2B static preparation and host-input conformance

> English (default) | [简体中文](M2B.zh-CN.md)

[PR #4](https://github.com/PicketX/PicketXD/pull/4), stacked on bridge PR #3, adds a runnable static preparation service and seven required real host INPUT scenarios. `picketxd prepare` loads complete YAML through the shared M2A compiler, retains private provider-bound last-valid desired state, recovers on restart, polls for replacement and projects absolute boundaries independently of reload errors. See the [runnable example](../../examples/prepare/README.md).

The command reports `prepared` and `enforcementConfirmed: false`. It does not change the kernel. Production table/mark ownership, strict recovery, future deny guarantees across process loss, container paths, authenticated Controller synchronization, L7 and management UI remain subsequent increments. Host enforcement below is deliberately validation-only; neither cached desired state nor an approval is an apply receipt.

## Observed kernel evidence

The [kernel job in run 38759897095](https://github.com/PicketX/PicketXD/actions/runs/38759897095) passed all **17 M0 + 19 bridge + 7 host required scenarios** on source `04335f0d48555c9ceb5f38d1e795a0c0c9447986`: **63 passing top-level tests, zero failures and zero skips**. This is one measured Linux namespace envelope, not distribution or production deployment coverage. That run's unit job failed the former FIFO test's 150ms whole-startup timing guard; the corrected test isolates nonblocking journal reads and separately verifies startup rejection. Final-head CI must also pass before readiness.

Measured Linux **6.17.0-1022-azure**, Go **1.27.0**, shared configuration **v0.0.0-20261015145159-81daf5abb831**, nftables **v0.3.1-0.20250606091825-8a8ad2be8103**. [Machine-readable evidence](M2B-evidence.json) records exact module versions and scenario statuses. Downloaded artifact `11984658856` contains eight evidence files; ZIP SHA-256 was checked against GitHub metadata: `af035508311fa8282c005b3b1a2615eefb2c225d32dfa3bc3ba8e5e7b6c282c9`.

| Required host case | Observed proof |
| --- | --- |
| ConfigurationDefaultDeny | Actual local INPUT TCP/UDP deny in both families; unrelated control remains reachable |
| ConfigurationSourcesAndDeny | Exact and overlapping CIDR union, exact revocation and explicit deny precedence |
| ConfigurationExpiryWithInvalidReload | Rejected replacement retains desired state without renewing native absolute expiry; independent IPv6 contribution remains |
| ConfigurationRecoveryDeadline | Reopened cache and replay retain original end; restore does not report enforcement |
| OriginalDestinationDNAT | Original TCP/8001 authorization still applies after translation to local TCP/8000; direct unmanaged port remains reachable |
| UntrackedProtectedTraffic | Proven unfiltered NOTRACK TCP path is dropped for protected current destinations |
| InputDoesNotClaimTransit | INPUT rules in the router namespace do not claim or block forwarded service traffic |

All host case names use `TestHost`. Both-family unfiltered TCP/UDP/control probes precede interpretation of policy denies. Actual apply receipts are read back before packet assertions. Existing gates are retained; missing, failed, duplicate or skipped required scenarios reject acceptance. The policy fixture is embedded for prebuilt test binaries; runner `cmp` prevents divergence from the user example.

## Recovery and review

One independent review found missing parent-directory durability at first creation. Startup now synchronizes the state directory and every ancestor, including retries after uncertain creation. Deterministic failure tests cover newly created components and real rename followed by injected directory-sync failure: in-memory generation does not advance, the store faults, further acceptance is blocked, and reopen validates the actually published canonical cache. These tests establish operation/error behavior, not physical power-loss testing.

Local Go tests/race/vet/build and static ELF inspection pass, including separate-process acceptance, restart and rejected-publication retention. Removing `O_NONBLOCK` with a local overlay makes the corrected FIFO regression fail; the actual source passes. The nftables upgrade fixes upstream CT direction decoding from kernel u8 attributes; it also supplies family-specific original address keys. All 43 required kernel cases passed with the pinned versions rather than weakening readback.
