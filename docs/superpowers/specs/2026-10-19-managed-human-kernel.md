# Managed human account / native enforcement conjunction

The user authorizes autonomous focused design, inline implementation, push and
exact-head integration after real verification and one independent review.

## Intent and scope
Prove that accounts created through Core's real React console become the actual
requester and independent approver in the existing Linux enforcement conjunction.
This is a bounded extension of the accepted PR19 harness, not new runtime behavior.
Use Core main31c7ac762ca0ea7d2d06ee2b30cb511c161cdad1. Preserve all29 existing
native cases and original112 kernel/9systemd cases. No mock routes or reports in
native acceptance; unit evidence fixtures are explicitly not native proof.

## Design
Use explicit management.users_mode: database and a private admin recovery key
document with no users. Create requester and approver through Accounts using real
CSRF/session/API/SQLite writes. Read actual directory and account audit; bind each
password session's /me identity and roles to its random principal:account- ID.
Request, approval, full Grant and publication audit must retain those exact IDs,
separate from the dual-stack IP subjects. Keep narrowing, independent approval,
absolute expiry, sibling isolation, deny and revocation tests unchanged.

After dual-stack approval, disable then re-enable requester via Accounts. Assert
old cookie401 both while disabled and after re-enable, fresh login same durable ID,
generation3, directory revisions2→3→4 and exact persisted account audit. Neither
account edit may change publication revision, renew deadlines or revoke independent
network Grants: contemporaneous IPv4/IPv6 traffic still reaches the real app.
Then retain explicit Grant revoke/absolute-expiry/rejection and actual Agent
generation/hash/report checks. Add two named traffic cases, no skipped scenarios.

## Evidence boundaries
Keep initial/final public directory, login identities, account audit and lifecycle
receipts in sanitized report, never passwords/hashes/tokens/cookies. Evidence gate
rejects missing/mismatched/disabled accounts, duplicate identities, bad role/audit
bindings, revived cookies and accidental publication changes. Screenshots at375
and1440 show actual account and request pages, not kernel proof by themselves.

Existing React visual system is unchanged. UX skills apply to real label-driven
form operations, success feedback and responsive screenshots; no new UI styling.
Local Unixgram EPERM is known and must remain reported; native CI supplies required
unrestricted execution. No new OIDC/HA/PostgreSQL/Gitea/TLS/release claim.

## Context map (self-reviewed under autonomous authority)
| File | Responsibility / change |
|---|---|
| integration/managed-humans.mjs | Real Accounts provisioning/lifecycle and evidence checks |
| integration/human-requests.mjs | Actual password identities; lifecycle between approval/revoke |
| integration/console-kernel.mjs | Database authority, recovery-key-only fixture; pass admin page |
| integration/console-evidence.mjs | Pin Core, require account provenance and2 traffic cases |
| integration/console-evidence.test.mjs | Independent literal unit fixture; adversarial mutations |
| .github/workflows/validation.yml | Immutable Core checkout matching evidence pin |
| docs/validation / AUTONOMOUS_DELIVERY | Honest verification, review, lease and remaining work |

Dependencies: Core Accounts.tsx/e2e/accounts.mjs and account/model.go define actual
labels/public contracts. Existing process-health and test-console-kernel gates
remain. No public API change, database migration or product configuration change.
Only the native test runtime opts into the already delivered database user mode.

Alternatives: API-only provisioning would miss the first-use console workflow;
repeating fixed file users would not prove PR33; creating a parallel fixture would
duplicate native setup. Extend the existing real fixture instead.
