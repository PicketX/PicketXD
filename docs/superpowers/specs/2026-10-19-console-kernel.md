# M2D2: real console and kernel conjunction / 管理端与内核联合验收

Pin Apache Core a99d9fc479bdf1490f78e669291ec67ca84876e3 as a separate
checkout; do not change the GPL Agent module pin or import Agent into Core.
Run built React/Chromium, database-authoritative Controller, official gRPC,
actual Agent nftables, Nginx auth_request and an actual HTTP application inside
a disposable Linux network namespace. No simulated execution or API interception.

Acceptance: browser publication and structured Grant issue/revoke; exact Agent
generation/hash confirmation; IPv4/IPv6 independently blocked then allowed;
sibling endpoint isolation; request path/method/host denial on a live connection;
overlapping Grant union and last-Grant revocation; independent absolute expiry;
unmanaged traffic preservation. Denial must distinguish transport failure from
HTTP 403 and verify no application hit. Positive controls establish healthy servers.
Preserve raw sanitized execution reports, source/binary/kernel identity and screenshots.
An opted-in privilege/tool/browser failure is fatal, never skipped.

Use graceful handling: new connections are blocked at L4 after revocation while
an established connection must still be denied per-request by L7. Do not claim
connection resets, production TLS topology, human OIDC or approval workflow.
Existing native strict recovery and systemd gates remain unchanged and mandatory.

中文：本段验证真实浏览器写入、持久化、下发、内核规则和逐请求授权的交集。
IPv4/IPv6、授权并集、撤销和绝对期限必须有实际流量证据；管理写入不等于应用。
只在隔离网络命名空间执行，不触碰宿主规则；失败不能转成跳过或模拟成功。
不更换技术栈，不新增生产依赖。身份登录/申请/审批及生产部署仍为后续范围。
