# Installed HTTP audit conjunction

Goal: correlate actual installed Controller/Agent/Nginx/application decisions with
the real PR44 audit UI, in both password and real verified-HTTPS OIDC workflows.
Fresh entry Corec67da10767d5cb92012366b6328baa31be8afa61/treeb2d6855d8406d70f98cc0d4c17fae9e31803a249/
CI39421146281 and Agent665c9a4c2c7134543942ed769e6ba12814c5d825/tree5907b051/
CI39412700638 green, no open PR. PR44 release6290761489 and PR29 release6289674170
read back. Owner codex/installed-http-audit, lease13:40Z or earlier explicit verified
main release. User explicitly waives routine design/plan/execution/push/merge pauses,
not TDD, native tests or one independent whole-branch review.

Extend the existing conjunction, not another product subsystem. Pin every active
Core consumer/test fixture to the exact Core main above. After actual deployed
Controller/Agent restart with both Grants applied (existing grant6/restart revision),
make four serial real ingress probes: allow4,deny4,allow6,deny6, /allowed versus
/denied on existing route-v1. Require200/appDelta1 or403/appDelta0, respectively.
Read the real authenticated audit keyset before and after EACH probe; exactly one
new row, ID strictly above previous cursor, same applied revision, caller
initial-ingress, family source127.0.0.1/::1, route-v1, reason allowed/route-mismatch.
Allow rows contain the exact issued family's Grant UID, denial rows no Grant/deny
IDs. These probes occur only after real native confirmation, not an approval alone.

Use a separate fresh browser context for managed approver password or actual
Keycloak OIDC sign-in (no bootstrap fallback in the audit view). Read /me and match
the provisioned approver ID/role/audit permission. Open Audit → HTTP decisions and
page actual keyset results as needed, bounded to20pages. For each selected actual
decision, read visible DOM ID/outcome/source/reason/route/caller/revision and both
contributing-ID lists; compare the full selected public row to the actual API row.
Capture375/1440 screenshots, assert no overflow/no browser credential storage, and
close the owned context. Do not stub routes or seed decision evidence. Return main
page to existing Grants workflow unchanged; keep every old required case/gate.

New report.httpAudit exact fields: mode(password|oidc),revision,principal,grants
([IPv4UID,IPv6UID]),probes(four ordered{name,family,before,event,status,appDelta}),
rows(four DOM-derived event projections),viewports([1440,375]). Event projection:
id,caller,route,source,allowed,reason,revision,grant_ids,deny_ids. No URL, URI, query,
headers, cookies, token, password, external subject or arbitrary extra fields.
Gate requires this evidence; correlate revision to restored deployment and actual
confirmations, principal to provisioned approver, mode to actual OIDC presence,
IDs/order/cursor/probe outcomes/UIDs and exact DOM rows. Reject booleans-as-string,
missing/duplicate/unsafe/cross-family/cross-revision/inconsistent/private evidence.
The gate doesn't turn UI data into kernel evidence; existing raw Agent receipt
and packet gates independently prove L4. This is source-backed L7, not request-user
identity or TLS termination. No Go/API/schema/runtime/kernel/dependency/UI changes.

Review correction: retain independent report.grantIssuances, two ordered public
{family,source,uid,revision} records from the actual original Grant-create responses
and known source inputs. Exact sequential creation revisions must match existing
grant4/grant6 cases and restored revision; all receipt UID copies must match these
independent originals. Coordinated substitution must fail, not just single-field
mutations. This strengthens the binding requirement without changing product APIs.

## Context map (self-reviewed)
| File | Responsibility/change |
| --- | --- |
| integration/http-audit.mjs (new) | Native probes/browser row capture + strict public evidence validator |
| integration/http-audit.test.mjs (new), fixtures/http-audit.mjs (new) | Hand-derived unit proof/negative mutations, never native evidence |
| integration/console-evidence.mjs, .test.mjs | Require new receipt and preserve every old case |
| integration/console-kernel.mjs | Invoke helper after restored applied state, fresh human login |
| integration/controller-install.mjs/.test.mjs, oidc-identity.mjs/.test.mjs | Exact Core main pins |
| scripts/test-controller-service.py, .github/workflows/validation.yml | Same exact pins, retain all native gates |
| docs/AUTONOMOUS_DELIVERY*.md, validation/INSTALLED_HTTP_AUDIT*.md | Lease/evidence/boundaries/review/next |

Dependencies: existing request(family,options), api(path), real browser, existing
OIDC signIn, installed init/caller/Grant UIDs/restart/raw confirmation. Reference
patterns: installed-cli receipt validation and managed-human browser sign-in.
No public API break/migration/new runtime config. UI skills retain existing PR44
layout/labels; no re-design or sample UI. Local Go baseline five Unixgram EPERM;
304Node pass with configured Go1.27 path. Full Go/race/vet/build, locked Core Web,
Python gates/nativeLinux/raw/four actual archives/source/static/notices/screenshots,
expected-head merge and fresh main mandatory. Spec self-review: no placeholders,
one integrated deliverable, all failure domains and report fields explicit.
