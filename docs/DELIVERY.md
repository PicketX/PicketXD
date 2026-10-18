# Delivery route

> English (default) | [简体中文](DELIVERY.zh-CN.md)

The user approved continued, staged delivery on 2026-10-15 through usable L4 controls, L7 authorization and an approachable management application. Keep normal implementation, test, review and integration work moving within approved decisions; raise material product/security tradeoffs together. This is an execution mandate, not a claim that unfinished features exist or a background job.

| Increment | Deliverable | Acceptance boundary |
| --- | --- | --- |
| M1.3A | Shared planner to private M0 fixture compiler | Explicit-time conversion, provenance, replacement validation; no kernel effects |
| M1.3B | Confirmed isolated apply/revoke lifecycle | Atomic admission/DNAT, actual emission receipts/readback, cumulative history, scoped strict retry and empty ownership |
| M1.3C | Boundary driver and kernel conformance | Stale-generation/cancellation guards; all 17 M0 tests plus the approved bridge acceptance classes, actual dual-stack packet evidence |
| M2 | Deployable L4 workflow | Versioned authoritative config, Agent lifecycle and recovery, authenticated Controller synchronization, real apply status, independent expiry/revoke |
| M3 | HTTP/HTTPS L7 authorization | Trusted existing ingress and scoped local decision API; per-request routing/identity checks, fail-closed integration tests |
| M4 | Portal and administration | Source/resource/duration request, basic approval, Grant/revoke, Agent status and diagnostics; English/Chinese, accessible responsive flows |
| M5 | Usable basic release | Gitea browser/Git/SSH scenario, L4+L7 combined decisions, restart/outage tests, static binary/deployment guide and user-flow acceptance |

M2–M5 are delivery outcomes, not frozen interfaces. Refine each against the approved product/configuration baseline before implementation. L7 initially uses trusted ingress; native SSH/database proxies, NFQUEUE, transparent proxying and Agent TLS termination remain deferred. L4 admission and application authentication stay separate.

Each increment keeps its tests, actual CI evidence, known limits and next entry point in its PR. Merge only after the required checks/review. Do not describe a planned or mock-backed result as applied enforcement. The central [M1.3 design](https://github.com/PicketX/PicketX/blob/9d53b13e2d8a805b2a7b2f3a8ad957fcf84ea12d/docs/superpowers/specs/2026-10-15-core-m0-bridge-design.md) governs all three bridge slices; splitting delivery does not remove any acceptance requirement.

The static host slice now includes [M2C8 strict enforcement](validation/M2C8.md), alongside graceful admission and durable cleanup history. The next L4 gaps are durable completion across unobserved permission intervals, service/deployment packaging and authenticated Controller synchronization; no complete M2 release is claimed.

[M2C9](validation/M2C9.md) adds retained-image cleanup before strict replacement, including direct fresh-Grant restart after offline expiry. Full durable completion across unobserved gaps remains a separate L4 gap.

[M2C10](validation/M2C10.md) packages the existing static Agent as a reviewed systemd service, with bilingual installation/upgrade guidance and isolated real-service lifecycle acceptance. It adds no boot readiness guarantee; durable missed-interval revocation and authenticated Controller synchronization remain L4 gaps.

[M2C11](validation/M2C11.md) adds explicit service startup notification after clean current-policy confirmation and ordered-dependent acceptance. It closes the unit startup-readiness gap, not global boot protection or ongoing health. Durable missed-interval revocation and authenticated Controller synchronization remain L4 gaps.

[M2D1](validation/M2D1.md) connects authenticated Controller snapshots to the existing host executor, with exclusive providers, local scope mapping, remote LKG, native apply reports and real Core/SQLite-to-Agent packet acceptance. Management APIs/RBAC, HTTP authorization, live UI and missed-interval durable revocation remain pending.
