# M2C2: Production host policy compilation

> English (default) | [简体中文](2026-10-16-host-policy-design.zh-CN.md)

Continue the approved production L4 sequence after ownership PR #5. The outcome is a production input contract for the upcoming kernel engine, independent of private validation fixtures. It does not install rules or establish enforcement readiness. Implement inline under the continuous-delivery mandate, with one fresh whole-branch review.

## Scope and alternatives

Introduce `internal/hostpolicy` to consume immutable shared `networkplan.Plan` values, including `configuration.Candidate.Plan()`. Reusing the fixture compiler would retain its hard-coded catalog and test-only types; implementing an enforcement command now would skip mark/history/process-loss prerequisites. A separate pure compiler provides a usable boundary while preserving those prerequisites. Existing dependencies and public configuration schemas stay unchanged.

`NewScope(authorityID string, context networkplan.ExecutionContext) (*Scope,error)` validates and copies trusted runtime facts. Require nonempty valid UTF-8 identities (at most 256 bytes, no NUL), explicit host-local/original-destination/graceful context, and canonical normalized destination bounds. The selected context's identity/profile/namespace must match trusted facts; policy may narrow destinations, never widen them. The scope is immutable, with no exported mutable fields.

`Compile(plan *networkplan.Plan, scope *Scope) (*Policy,error)` rejects nil/uncompiled plans, unknown Agent, authority mismatch, and every selected unsupported target, including targets with no active Grants. Strict, transit and published-ingress targets are unsupported. Other Agents are outside this local compiler. Binding removal may produce an explicitly empty desired policy for a still-known Agent; absence of the Agent is an error. Empty desired output is never an installation or deletion receipt.

`Policy.Evaluate(at time.Time) (Snapshot,error)` requires explicit nonzero time, reads shared active projection and emits contract version 1: authority, trusted context, UTC evaluation time, next desired boundary, and per-target key/binding IDs/protected endpoints/allow and deny members. Endpoint fields are canonical destination prefix, IP protocol 6 or 17, and one port 1..65535. Preserve Resource UID through TargetKey.Entry.ResourceName, contribution UID, source prefix, and complete absolute Validity. Do not collapse permission unions, deadlines or ID domains. Protection remains after the last Grant expires. Arbitrary valid destinations, TCP/UDP and ports are supported equally for IPv4/IPv6; no fixture address/port catalog is consulted.

`Policy.Inventory() ([]networkplan.TargetInventory,error)` returns complete desired provenance, including future/expired/revoked contributions. Returned inventory and snapshots must be independently mutable; concurrent reads must not mutate internal state. The plan remains the source of truth for evaluation; clocks and schedulers stay outside this package.

Future denies are represented as desired inventory and evaluation boundaries, not accepted as a process-loss-safe installation capability. Version 1 is an internal compiler contract, not the future nftables structural ownership contract. This increment allocates no marks and adds no production apply/confirm command. `prepare` behavior and marker-only `hostowner` stay unchanged.

## Acceptance

Test arbitrary dual-stack TCP/UDP targets, separate same-ID Grant/Deny, independent deadlines, NotBefore/exclusive expiry, permanent members, revocation, last-Grant protection, empty Binding withdrawal, unknown Agent, selected dormant unsupported contexts, authority/namespace/profile/bounds mismatch, returned-data mutation and concurrent evaluation. Exercise real `configuration.Decode`/`Compile` input with UID preservation. Adapt the existing host kernel fixture only at its compiler input boundary; production code must never import validation packages. Keep all 50 existing required gates and add `TestHostPolicyCompilerPorts` as a required packet case using TCP/8001 and UDP/8002 on both families. Kernel fixture application is not a production engine acceptance claim.
