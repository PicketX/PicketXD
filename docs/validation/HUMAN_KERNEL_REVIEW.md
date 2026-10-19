# Human/kernel whole-branch review

One fresh-context independent read-only review of local
f9e656c40802ce24a7b059ffe824eac1c18ed53a..be932c2bea0e988d8c54de40c94ad2ca0f8d31bc,
covering the complete12-file branch, spec/plan/ledger and adjacent Agent/Core
contracts. Reviewer independently ran46 Node tests and diff checks. No Critical
or Minor findings; three Important findings enter one root-authored fix pass.
No GitHub approval is fabricated and no second review is substituted for tests.

## Important findings and fixes
1. Human records/cases independently accepted any confirmed revision. Negative
   mutations using other *existing* confirmations reproduced acceptance (RED).
   A strict baseline→approval4→approval6→revoke4 timeline now binds every named
   human case, with unchanged expiry/rejection revisions and exact advancement.
2. First-scope/actor/UID-substring projection lost complete Grant/audit provenance.
   Extra scopes, wrong resource/selection/mode/UID, audit revision and unrelated
   operation mutations reproduced acceptance (RED). The native helper now retains
   and validates the entire public Grant view, exact bounded single named-resource
   scope, request identity/source/resource retention and exact audit revision/reason.
3. Signal-terminated required services retain null exitCode. A real SIGTERM Node
   subprocess reproduced false healthy acceptance (RED). Health checks now require
   null exitCode AND signalCode around observations, with contemporaneous dual-stack
   Nginx-owned unprotected positive controls before/after every traffic scenario.

All14 new review regressions pass after the fix (full Node suite60/60, no skips).
The fixture's existing missing-confirmation mutation moved from3 to99 because3 is
now a deliberately present confirmation in the realistic multi-revision fixture.
That test adjustment is not a production guard relaxation. Complete final native
CI/raw-artifact/tree proof remains mandatory and is recorded in PR19/checkpoint.

## Root rulings on every declined-to-judge boundary
- Final CI/artifact/screenshots/immutable trees: root verifies actual raw logs,
  downloaded bytes/digests and375/1440 images, not schema fixtures. Cost if wrong:
  false acceptance of a different tree; exact-head merge remains gated.
- Privileged kernel/systemd: only disposable native CI can establish compatibility;
  root inspects original91 required gates/112 cases and9 service cases. Cost:
  compatibility remains limited to recorded kernel/toolchain.
- Local five Unixgram EPERM/full Go/race counts: root ran complete suites and
  retains all five failures honestly; native complete unit/race/vet/build required.
  Cost: local environment cannot independently replace native socket evidence.
- OIDC/TLS/account lifecycle/Gitea/release: explicitly remain subsequent work;
  this actual loopback HTTP app is not Gitea HTTPS/SSH or product completion. Cost:
  those workflows require separately implemented/verified increments.
- Strict TCP resets/conntrack: existing native suite owns strict semantics; this
  fixture uses graceful revocation plus per-request L7 denial. Cost: no immediate
  reset promise follows from this increment.
- Controller crash/restart: prior accepted durability gates remain; this increment
  validates actual database-backed API readback, not a new restart experiment.
  Cost: no additional human-workflow crash scenario is claimed here.
- Sub-millisecond precision: browser generates millisecond deadlines, so comparisons
  use valid absolute millisecond instants; Core retains its higher precision. Cost:
  this fixture does not distinguish nanosecond-only lifetime changes.
- Malicious log/exfiltration fault injection: no broad new fault-injection project;
  private random credentials, secret-free report fields, redaction and fixture-only
  cleanup remain inspected. Cost: not a proof against arbitrary malicious binaries.
- General external Core security: retain accepted pinned Core and inspect contracts
  used by this conjunction; not a whole-Core re-review. Cost: independent Core
  correctness remains supported by its own accepted full test/review evidence.

## Other execution rulings
Preserve host-owned linked worktree/branch (already isolated); exact local/remote
trees, not identical commit IDs, prove source equivalence. Cost: manual tree checks.
Extend rather than replace original18 cases or reuse standalone Core UI evidence.
Cost: additional CI time/pin upkeep. Explicit catalog rebase retains draft/CAS
safety; no production permission or source semantics are changed.

## Remaining scope
Account lifecycle/OIDC, request-bound verified protocol identities where needed,
actual Gitea HTTPS/SSH walkthrough and deployment/backup/restore/doctor/release
hardening remain. Prior Core PR32 Chinese UTF8 byte feedback Minor is still deferred;
this Agent test-only increment neither fixes nor adds a new UI Minor.
