# Installed ingress failure privacy

Bounded design/context map self-reviewed, 2026-10-20T00:12Z. Purpose: close PR26's
known transient token-file-path disclosure without changing runtime authorization.
Existing flow: prepareInstalledIngress -> execFileSync -> Node command error ->
outer redactFixtureFailure; the outer redactor strips values, not private paths.
Synchronous subprocess stderr can also be written before that outer catch.

## Context map

| File | Role / change |
| --- | --- |
| integration/installed-ingress.mjs | Catch the whole preparation boundary; safe phase and bounded numeric exit only, no raw error/cause/arguments/output; explicit captured stdio |
| integration/installed-ingress.test.mjs | Actual Node subprocess/filesystem failure tests, capture both caller streams and all error properties |
| integration/console-kernel.mjs | Existing consumer, must still fail closed; unchanged native success acceptance |
| integration/gitea-application.mjs | Existing outer sanitizer reference, not a new general redaction framework |
| docs/AUTONOMOUS_DELIVERY*.md | Short lease and durable next step; final exact-source receipts in PR metadata |

No API, schema, migration, Go/kernel or UI change. Existing static/build/Web/
browser/Gitea/Agent/kernel/backup gates unchanged. Separate Apache Core executable
and GPL fixture boundary unchanged. No frontend skills needed for this non-UI fix.

## Focused plan and tests

1. From exact fetched Agent524a62c/main39344752833 and Coreadfa1b1/main39339061250,
   confirm no open PR and PR26 release00:05:45Z. Reuse clean linked worktree on new
   branch; preserve original checkouts and prior feature branches.
2. Four real process/filesystem failures: Node rejects a missing script; invalid
   cwd prevents spawn; absent CLI read; wrong CLI checksum. Capture raw caller
   stdout/stderr and inspect all Error properties. These unit failures are NOT a
   fake successful Core CLI or native release proof. Watch privacy regression RED.
3. Rebuild a safe Error at preparation boundary, suppress subprocess stdio/cause,
   retain only fixed phase/route IDs and bounded numeric exit. Full suite GREEN.
4. Audit basic-goal evidence and actual remaining gaps without inventing new scope.
   Full Go normal/race/vet/build (record local five inherited Unixgram EPERM),
   one independent whole-branch review, root ONE blocker RED→GREEN pass, exact-head
   native three-job CI/four actual archives/expected-head merge/main verification.

Ruling: user's explicit autonomous mandate waives routine approval pauses; this
bounded fix needs no new subsystem or formal architecture plan. Cost if wrong:
less diagnostic detail or configuration drift; retain safe phase/exit and native
gates, preserve private files for local diagnosis. No production success claim yet.

Observed RED: all4 cases fail against original adapter (path disclosure for real
exit/cwd/read, unbounded AssertionError fields for identity mismatch). GREEN:
fresh safe Error and explicit captured stdio, full233Node pass/no skip. Full
native/four-archive/source/main acceptance still pending; final receipt in PR.

## Basic-goal audit at the accepted entry mains

| Area | Existing actual evidence | Remaining boundary |
| --- | --- | --- |
| L4 / Agent synchronization | Agent main524a62c CI39344752833:112 kernel scenarios,23 confirmations,9 static systemd scenarios,4 Controller-mode PIDs/36 observations, immutable deadlines and LKG/restore | Disposable Linux fixtures and explicitly scoped topology; not a general host-reboot/migration guarantee |
| Basic L7 | Same main:31 prior +7 Gitea cases; real browser/API/Agent/Nginx/HTTPS/SSH application, header forgery, reuse/revoke/expiry,4 private-route denies; actual generated installed includes | Source Grants and native application credentials are separate; do not claim PicketX request-identity policy enforcement |
| Management | Real account provisioning/lifecycle, request/approval/rejection, Grant/revoke and audit in conjunction; Core Web/API acceptance on adfa1b1 CI39339061250 | OIDC session workflow remains missing, not replaced by local password login |
| Single-node operations | Separate verified Core239-file and GPL Agent1415-file entry bundles, static ELF, source manifests/export/offline rebuild, private init, service and stopped recovery | This is inspected CI distribution and documented topology, not every production topology or a host rollback migration |

Concrete next functional gap: Core `docs/ARCHITECTURE.md` M1 and
`docs/BASIC_FUNCTION_ROADMAP.md` order3 explicitly include browser/OIDC sessions.
`docs/HUMAN_LOGIN.md` explicitly excludes OIDC; entry source in internal/, cmd/ and
apps/web/src has no OIDC issuer/callback integration. Preserve SCS/Casbin and
existing local/bootstrap access; next increment needs actual OIDC protocol/browser
validation, not another packaging-only pass. This audit does not silently delete
OIDC from the accepted roadmap or declare the overall goal complete.

Do not repeat delivered account lifecycle or the old PR12 update-CAS item:
Core internal/publication/store_test.go has existing-row stale UPDATE coverage;
requests_test.go TestAccessRequestStaleRevisionOneUpdateCAS also covers revision1.
Other historical review Minors remain in their original records, not silently
upgraded to blockers or used to invent new first-release functionality.
