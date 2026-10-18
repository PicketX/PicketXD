# Autonomous delivery checkpoint

Updated: 2026-10-18T09:42:00Z.

## Current lease
Owner: automated Controller Provider increment. Branch: `codex/controller-provider`.
Expires: 2026-10-18T12:30:00Z. Release on merged PR with green main CI.
No competing PR existed at start. Core main is 5e2254efec90f2a5448450a2974c587c37f1dd18;
Agent baseline main is 1949d6352da6afb0bf6796d97e45eabe6d197ee8.

## Scope
Implement a selectable Controller provider in the existing host enforcement command,
provider/endpoint-bound remote LKG, local scope mapping and actual execution reports.
Keep static mode exclusive, independent absolute deadlines and all existing native gates.
Use existing typed protocol, optional verified server TLS or explicit plaintext, no mTLS.

## Remaining delivery
Management persistence/API/authentication/RBAC, HTTP ingress authorization, live React
management pages and actual browser/proxy/application end-to-end acceptance are pending.
Full durable revocation across unobserved gaps remains pending. Do not claim completion.
Core deferred stale UPDATE CAS regression is tracked in the Core checkpoint.
