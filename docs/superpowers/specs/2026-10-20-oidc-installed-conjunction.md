# Installed real-OIDC conjunction

## Intent and boundary
Close the remaining identity-to-execution acceptance gap: actual Keycloak HTTPS
login maps two existing local humans; requester and independent approver operate
real APIs, installed Controller/Agent apply grants, and Nginx/application traffic
is actually observed on IPv4 and IPv6. Publication/approval/login are not execution.
No product Go/kernel/UI/API/schema or selected stack change. This is disposable
CI acceptance, not production Keycloak/MFA/HA or arbitrary provider compatibility.

Entry: Core64c8cb8e6c92669873031ca45cff47dbb38d0224 /
tree4637c029a34c71ec04b66fcc205c288f1280dfb9 / CI39379765387;
Agent0255d4dc8574f102e8a2d016d837292aa304d68f /
treeced537dafb0ca557528cf38b37ed5812273fd6f0 / CI39347548020.
Both all jobs/steps green, no open PR; PR41 released06:30:15Z and PR27 released
00:34:27Z. Existing clean linked worktree, new branch from exact Agent main;
host checkouts/old branches preserved. Local baseline five inherited Unixgram EPERM.

## Alternatives and chosen shape
1. Recommended: retain old password conjunction, add a second wrapper-owned real
   provider/HTTPS pass through the same complete native runner and assertions.
   Keeps old evidence and tests drift once; explicit hooks only for identity/setup.
2. Replace password flow: cheaper runtime but deletes prior acceptance; rejected.
3. Duplicate the entire runner: isolated but doubles ownership and can diverge;
   rejected. No GPL code moves to Apache Core; consume its pinned TLS helper read-only.

## Topology and lifecycle
Pinned real Keycloak26.8.1 archive SHA256
4e2740b181faa2ea623a1c2ecf00855935311e546046e2f857615a47ed24cbe8 and JDK21,
same as Core PR41, CI-only. Wrapper inside disposable netns owns a separate 0700
mkdtemp tree, stages the pristine distribution privately using Core's tested helper,
imports two distinct subjects with a separate IdP password, runs normal verified
HTTPS on127.0.0.2:19460, and removes processes/trust/state before its pass receipt.
Never mutate/delete the supplied provider distribution. This tree must not be the
Gitea fixture tree which intentionally becomes0711 for its unprivileged process.

Generate private CA/leaf, add only a unique NSS label, remove it on both paths.
Launch the existing native Node child with NODE_EXTRA_CA_CERTS before Node starts;
normal Node/Playwright API TLS and hostname verification stay enabled. Installed
Controller keeps empty PATH and uses explicit SSL_CERT_FILE for provider trust.
Console public origin https://127.0.0.1:18081 through owned Nginx; private backend
127.0.0.1:18080, gRPC19090/authz19091 unchanged. Provider and console are distinct
sites, no ignoreHTTPSErrors/rejectUnauthorized:false/global trust bypass.

Explicit private subject bindings select the two accounts created by actual Accounts
UI; no claim/email/role auto-provisioning. Keep bootstrap recovery onboarding and
old audit expectations, use OIDC for every requester/approver login including
disable/re-enable recovery, and switch the Gitea publishing page to OIDC approver.
No password fallback in OIDC mode. Assert exact local IDs/current roles, S256/nonce,
Secure/HttpOnly/Strict session and no browser token storage. IdP SSO may stay live;
local account invalidation must still deny both probed and untouched old sessions.

## Execution and evidence
Retain all original cases and real raw Agent status/report/restart boundaries,
independent absolute expiries, denied forwarding checks, Gitea HTTPS/SSH auth and
positive controls. Correlate OIDC humans with two actual approved revisions and
all four Gitea Grant audit actors; login alone never passes this gate. Existing
separate systemd/outage/LKG/recovery and112 kernel scenarios remain mandatory.
Do not claim new real-IdP outage/backup/production topology coverage beyond observed.

Keep four artifact names; old evidence remains at its current paths, new full
report/status/log/screenshots under console-kernel/oidc/. Public OIDC receipt has
only exact source/provider descriptors, bounded IDs/roles, correlated revisions
and proven booleans; no issuer query/code/state/token/client secret/password,
private import/config/data paths or raw provider logs. New mode's errors are fixed
phases, never browser exceptions. Native setup/privilege failures fail, never skip.

## Context Map
| File | Purpose / change |
| --- | --- |
| integration/oidc-identity.mjs (new) | Private installed config/bindings, actual OIDC browser sign-in, strict public identity evidence |
| integration/oidc-conjunction.mjs (new) | Owned real-provider/CA/NSS/child lifecycle and final public receipt |
| integration/oidc-identity.test.mjs (new) | Missing/malformed/private/stale/uncorrelated evidence and actual private filesystem regressions |
| integration/console-kernel.mjs | Optional explicit OIDC mode, HTTPS ingress, separate evidence path, real-human Gitea actor correlation |
| integration/human-requests.mjs | Optional sign-in hook; shared actual ID/role/API assertions retained |
| integration/controller-install.mjs | Current exact Core pin; optional browser origin only, old defaults retained |
| integration/controller-install.test.mjs, console-evidence.test.mjs | Exact updated source pin fixtures, no validation loosening |
| scripts/test-controller-service.py | Same current Core source/tree pin, unchanged service requirements |
| .github/workflows/validation.yml | Exact Core checkout, pinned Keycloak/JDK, second mandatory netns pass, same artifacts |
| docs/AUTONOMOUS_DELIVERY*.md, validation/OIDC_CONJUNCTION*.md | Short lease, evidence boundaries, final receipts/next steps |

Dependencies: pinned external Core scripts/real-idp-https.mjs exports
createTestCertificate, requestHTTPS, stagePrivateProvider; existing installed
ingress/Gitea/process health/managed humans/console evidence/distribution helpers.
Reference: Core apps/web/e2e/real-idp.mjs and existing console-kernel.mjs.
Risk assessment: no public API break/database migration/product config schema;
only disposable test configuration, pin and optional native adapter arguments.

Self-review: one integrated acceptance deliverable, no placeholders/interface
conflicts; identity, execution and lifetime boundaries deliberately separate.
Explicit autonomy waives routine design/plan/execution pauses, not TDD, one fresh
whole-branch review, full CI/raw/four actual archives/source/expected-head/main gates.
