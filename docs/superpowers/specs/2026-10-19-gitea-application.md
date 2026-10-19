# Real Gitea HTTPS / SSH conjunction

## Intent and design
Prove the M5 first application workflow with real Gitea, native Git and existing
PicketX management/Agent/ingress, not a substitute HTTP server. The user explicitly
authorizes autonomous design, inline execution, commit/push/PR and exact-head merge
after verification and one independent whole-branch review. No routine approval pause.

Extend the accepted console-kernel fixture. Preserve its 31 cases, DB-created human
request/independent approval and original 112 kernel / 9 systemd cases. Pin Core
298863f4e0cb1a72ef18e8abe77f8b8fb28464de and upstream Gitea v28.2.0 Linux amd64
(published 2026-10-18; asset SHA256
15681fc615d56611c274f52803fef4d6d5316b620f93b677dd4b831a74e50b9f).

Use the existing authenticated Desired configuration and Grants pages. Publish a
Resource with named https TCP/19445 and ssh TCP/19222, exact host-local Binding,
explicit root path prefix and GET/POST HTTPRoute. Keep caller scope explicit. Run
Gitea under UID/GID65534 in a private directory; Nginx owns TLS and auth_request,
Gitea owns its normal private repository login/token/key checks. No native SSH proxy.
TLS uses a temporary CA and verified DNS/IP SAN leaf; Chromium trusts that exact CA,
Git uses sslCAInfo, Node uses ca. SSH known_hosts is pinned from the generated actual
host public key; StrictHostKeyChecking=yes, no acceptance bypasses.

Check IPv4/IPv6 transport denial before Grants; HTTPS-only Grants leave SSH denied;
independent SSH Grants enable native git ls-remote/clone with exact commit IDs.
While admitted, missing HTTPS credentials and a real outsider account/key cannot
read the private repository. Actual Gitea browser password login and repository
navigation succeed. Revoke each HTTPS/SSH source Grant independently: new flows
fail, other family/entry stays usable; an already-established HTTPS connection gets
fresh L7 denial. Graceful L4 does not promise resetting existing SSH connections.
Require live Gitea/ingress controls around negative probes and actual Agent applied
revision/hash plus Controller gRPC report for every publication.

Evidence contains only allowlisted public outcomes, actual binary hashes/version,
repository OID, source family/entry, exact revision and persisted Grant views.
Never archive app passwords, tokens, SSH/CA private keys, cookies or authorization
headers. In-memory evidence tests are not native proof. Root namespace ownership,
ephemeral CA trust and cleanup are confined to the disposable CI fixture.

## Context map (self-reviewed)
| File | Responsibility |
|---|---|
| integration/gitea-application.mjs | Real app lifecycle, browser/native Git and evidence gate |
| integration/gitea-application.test.mjs | Literal evidence and adversarial validator regressions |
| integration/console-kernel.mjs | Invoke new real workflow; existing lifecycle and redaction |
| integration/console-evidence.mjs | Immutable Core pin and required Gitea gate |
| integration/console-evidence.test.mjs | Existing literal report gains independent Gitea fixture |
| .github/workflows/validation.yml | Pinned Core/Gitea download/hash, NSS dependency, full native gate |
| docs/validation, AUTONOMOUS_DELIVERY | Evidence, limits, lease and next deployment entry |

Dependencies: existing process-health, static build/unshare scripts and real React
labels. No product API/schema/database migration, kernel or runtime dependency change.
UI skills preserve existing design and real feedback/responsive screenshots; no new
page/style. Alternatives: API-only or fake app misses Gitea auth; a separate harness
duplicates proven setup. Extending the existing fixture is the smallest useful slice.
Deployment/release acceptance remains separate; no full-product completion claim.

中文：真实Gitea私有仓库通过独立HTTPS与SSH入口验收，网络Grant不替代应用登录、
Token或SSH密钥权限。保留既有人员审批与真实内核门禁，不将模拟报告当作原生证据。
