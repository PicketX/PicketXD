# Installed HTTP decision audit conjunction

Core pin: `c67da10767d5cb92012366b6328baa31be8afa61`, tree
`b2d6855d8406d70f98cc0d4c17fae9e31803a249`. Agent PR30 extends the existing
password and real verified-HTTPS Keycloak26.8.1 workflows, not product semantics.

After clean installed Controller/Agent restart and fresh native confirmation,
four serial real Nginx requests (`allow4`, `deny4`, `allow6`, `deny6`) must return
200 with one application hit, or403 with zero hits. Each actual authenticated
HTTP-audit keyset before/after the request must identify exactly one new decision.
Source family, caller, route, reason, current/applied revision and the original
issued Grant UID must match. Existing raw Agent/Controller fences independently
prove execution; audit rows are not kernel receipts or request-user identities.

A separate owned browser context signs in as the actual managed approver using
the selected password or real OIDC method, without bootstrap fallback. `/me`
must match that account and audit permission. The actual HTTP decisions page
is read through its bounded keyset controls; nine public fields, including both
contribution lists, must exactly equal the four real API decisions.1440/375 PNGs,
no horizontal overflow, no browser credential storage and no page errors are
required. Context cleanup is in `finally`; existing main workflow is untouched.

`report.httpAudit` is a strict public receipt. Missing, old, duplicate, unsafe,
cross-family/revision/Grant, mismatched DOM, transport-as403 or extra private fields
fail with a fixed nonprivate message. Hand-derived fixtures are unit data only,
never emitted by native execution. All previous31cases, Gitea7cases, raw statuses,
CLI six phases, systemd/LKG/stopped backup, static/source/offline/notices and four
artifact gates remain required. New proof is additive.

TDD: old complete-report test first failed `Missing expected exception`; new
model tests first failed missing implementation. Then192targeted and339fullNode
tests passed without fail/skip. Python12, Go vet/build and locked CoreWeb
ci/typecheck/lint/build passed. Full local ordinary/race Go both encounter only
the five inherited Unixgram `operation not permitted` tests; this is not green
native evidence. Exact-head real Linux CI, one independent whole-branch review,
raw logs, actual four archives and fresh main acceptance remain mandatory.

Owner `codex/installed-http-audit`, lease13:40Z or earlier explicit verified-main
release. Final immutable PR receipt supersedes pending evidence here. No complete
basic-goal or merge claim before actual final proof.
