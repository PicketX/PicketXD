# Installed generated ingress evidence

Scope: Agent acceptance now consumes the shipped separate Core CLI, not Core's
source Nginx template. Existing ingress still owns TLS/listeners/proxy/health.
Core pin adfa1b111e719b606222f6643c7343afa98d3f03 / tree
7a5cbb6c764c5e2b4a98e956a3e23fb1b3169d71. No production Go/kernel/API/UI change.

## Local evidence, 2026-10-19T23:44Z
- New `installed gate rejects an old report without generated ingress` initially
  FAIL: Missing expected exception. Mandatory strict receipt plus13 malformed/
  private-field mutations pass. Full217Node pass, no skip.
- Actual old accepted cdee CLI: `unknown command "nginx-init" for "picketx"`.
  Actual downloaded/verified Core main39339061250 CLI SHA256
  53b506b078566339a4f35edfb26ea50cada504e3e284395614f14a9709e6624b:
  controller-init then nginx-init with empty PATH produce three distinct callbacks,
  all0700 directories/0600 regular includes; strict receipt passes. This local
  filesystem/CLI probe is not native application/kernel execution proof.
- Full Go normal/race fail only TestNotifyEmitterFirstCleanConfirmationOnly,
  TestNotifyEmitterDoesNotSignalWhenJSONWriteFails,
  TestNotifyEmitterFailureDoesNotConsumeReadiness,
  TestNotifyEmitterOffIgnoresEnvironment, TestNotifyEmitterAbstractSocket:
  socket operation not permitted. Go vet/build pass. Initial Node run lacked Go
  in PATH (three builder tests failed); configured existing Go1.27, full217 pass.
- Exact native jobs/raw logs, four downloaded archives/source identities,
  whole-branch review, final head and main verification pending; no merge claim.

Ruling: explicit autonomous mandate waives routine approvals; this slice changes
an installed integration boundary, not product authorization. Cost if wrong:
ingress configuration drift; real native conjunction and all previous controls
are mandatory. One independent review and one root regression-first blocker pass.
Final exact receipts and immediate lease release persist in PR metadata to avoid
an evidence-only commit invalidating its own accepted source identity.
