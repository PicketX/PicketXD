# Installed OIDC acceptance

The original local-password native workflow remains mandatory and unchanged in
meaning. A second disposable network namespace runs the same full installed
Controller/Agent/browser/Nginx/application/Gitea suite with real Keycloak26.8.1
and normal HTTPS console/provider certificate verification. It uses exact Core
64c8cb8e6c92669873031ca45cff47dbb38d0224, tree4637c029a34c71ec04b66fcc205c288f1280dfb9.

## Operator boundaries
This is CI acceptance, not a production IdP installer or a requirement to deploy
Keycloak/JDK/our test CA in PicketX. Retain selected runtime/protocol/licensing
boundaries. For production onboarding use Core OIDC_LOGIN.md: register the exact
redirect, explicitly map issuer/subject to existing local human IDs, keep private
files and recovery credentials, and use normal trusted ingress. Claims do not
grant roles or network permissions. Local sign-out is not IdP-wide sign-out.

The fixture retains independent recovery-key onboarding, then uses real OIDC
requester and approver logins for the existing independent approval workflow,
including local disable/re-enable while provider SSO can remain alive. Before the
Gitea publication sequence it ends the recovery session and performs fresh OIDC
approver login. Every login requires an observed real S256/nonce authorization
request, exact current local ID/roles, new Secure/HttpOnly/Strict SCS and no browser
token storage; there is no local-password fallback. Network source remains an
independently reviewed IP/CIDR, never an IdP claim.

Approval records are correlated with actual Agent applied generations/status hashes
and Controller received APPLIED reports, dual-stack packets and zero denied
forwarding. All four Gitea Grant audit actors must be that OIDC approver. Existing
absolute-expiry, independent union, established-connection revoke, application
authentication and health controls stay. Existing systemd/outage/LKG/stopped state
recovery/112 kernel cases remain separate mandatory gates; no new claim of IdP
outage handling, MFA/HA, arbitrary providers or production topology review.

## Private lifecycle and evidence
Wrapper netns owns a private0700 tree separate from Gitea's0711 deployment root.
It copies the pristine pinned provider there, imports private realm/client values,
sets Node CA trust before the native child starts and explicit Controller CA trust,
adds/removes only a unique owned NSS label, and stops owned process groups/provider
before removing state. It never runs/mutates/deletes the input distribution. TLS
verification is never disabled; unknown CA and wrong SAN are actually rejected.
Only after successful child execution and checked cleanup is oidc-report.json
written. Failures print fixed phases, not IdP/child exception bodies or private
paths; no raw provider logs. New mode omits raw Controller/Nginx diagnostics and
failure process logs, retaining successful original Agent logs for restart fencing.

The four artifact names remain; original evidence stays at its paths. New complete
native report/status/Agent logs and375/1440 screenshots are nested under
console-kernel/oidc/. oidc-report.json allows only public source/provider descriptors,
local identity/role observations, correlated approval/Gitea revisions and TLS/cleanup
proofs. Strict unit data is labeled synthetic, never execution evidence.

## Validation contract
Local25new regressions were observed missing-helper RED→GREEN; actual private
filesystem tests protect modes/bindings/DB authority/symlink rejection. Combined258
Node and12Python state tests, vet/build and locked Web checks pass. Actual installed
Core archive/manifest/static CLI initialized a controlled instance with explicit
HTTPS browser origin, DB authority and empty PATH; not a kernel/runtime claim.
Full local normal/race retain five inherited TestNotifyEmitter Unixgram EPERM.
Only exact-head native CI can prove the real provider/browser/kernel conjunction.
Native setup/privilege failures must fail, not skip. One independent whole-branch
review/root regression-first fix pass, original raw logs, all four actual downloaded
archives/source/static/notices/images, expected-head merge and fresh-main verification
are required. Final immutable receipt and immediate lease release in PR28 override
pending source notes. Overall completion is conditional on actual observed evidence
and the agreed basic-function checklist; no completion claim is made by this document.
