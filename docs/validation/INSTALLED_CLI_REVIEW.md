# Installed CLI whole-branch review

ONE fresh independent read-only review of b14b54d40b2e1c20af0d12735722202d494580dd
through353c178663548f329ac5332c3e5d5c7cd2485112:0Critical,0Important,1Minor.
Reviewer read the entire branch/spec/plan/ledger and relevant pinned Core CLI,
independently ran236 targeted tests with no skip/cancel, and checked clean diff.
Verdict: code ready, conditional on mandatory native/delivery evidence. No nested
reviewer or subsequent review was requested. No code changed after this review.

## Root ruling on the minor

Accepted and deferred: cli-diagnostics.test.mjs unit fixture uses the same baseline
and grant6 revision2 and policy hash, so it would not catch all cross-phase receipt/
status substitutions. Current validator selects baseline and grant6 independently,
and the mandatory real native pass publishes different revisions. Cost retained:
a future validator regression has weaker isolated unit coverage until distinct
fixture revisions/hashes and cross-phase substitution cases are added. This is not
a claim that the unit fixture proves native execution; no gate is waived.

## Every declined-to-judge item and executor ruling

| Reviewer exclusion | Root ruling and retained cost |
| --- | --- |
| Actual password/OIDC native execution, kernel/systemd and remote CI were not executed/inspected | Not outside delivery scope: root must inspect exact-head three-job raw logs and actual packet/native receipts before merge; reviewer verification is not that evidence. |
| Published archives, source exports/reproducibility/notices and post-merge main were future evidence not supplied | Not waived: root must download all four actual archives, validate checksums/source/GPL export/static/notices/images, exact-head merge and repeat on main; no acceptance until then. |
| Full Go/race/Python/Web relied on coordinator results, not rerun | Root owns full execution: local Go ordinary/race five Unixgram EPERM failures remain explicitly not green; native Go must pass. Python12, vet/build, locked Web and full304Node passed locally. |
| Unchanged product/UI/identity/kernel internals were not a fresh whole-product audit | Boundary accepted for this test-only increment; no product/topology compatibility expansion inferred. Root still must audit overall basic-completion requirements separately before pausing ongoing delivery. |

Review receipt persisted before final CI/main acceptance. Exact final remote head,
source tree, raw CI, actual archives and immediate lease release are recorded in
PR29's immutable final receipt, superseding source text marked pending.
