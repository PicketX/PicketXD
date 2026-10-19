# Source-backed standalone Agent distribution

## Intent and scope
Ship the existing Linux-amd64 GPL Agent as a relocatable, checksummed bundle with
its exact committed source, vendored build/test dependencies, upstream notices and
bilingual build/use instructions. A supported Go1.27.0 toolchain remains a build
requirement, not a runtime dependency. Keep Apache Controller a separate bundle.
No runtime/kernel/API/UI behavior change, root installer, service enablement, host
boot, actual backup/restore or legal-clearance claim in this increment.

## Decision and alternatives
Use the existing CGO_ENABLED=0 Go build from an isolated committed-source clone
and official go mod vendor, with module verification and normalized Go settings.
This is smaller than a container/runtime distribution and more useful offline than
a binary plus remote source URL. Vendoring omits upstream dependency tests by Go's
documented contract; retain complete Agent source and all dependencies required to
build/test the Agent. Toolchain source is obtained from upstream separately; retain
its runtime license/PATENTS. Do not claim bit-identical reproducible builds: source
exports lack the checkout's VCS metadata. Verify an actual offline rebuild instead.

## Bundle contract
scripts/build-agent-bundle.sh NEW_ABSOLUTE_DIRECTORY rejects existing/dangling
symlink/noncanonical output and tracked/untracked dirty checkout before building.
Never overwrite a previous bundle or recursively delete operator paths. Partial
build output remains diagnosable. Output contains bin/picketxd, source/PicketXD
(exact committed snapshot plus vendor; private clone Git metadata removed), deploy runbooks/systemd reference, LICENSE,
THIRD_PARTY_NOTICES.md, licenses/index.json/upstream texts, SOURCE.json and
SHA256SUMS covering every regular file except that root manifest. No symlinks,
special files, private keys, node_modules, .git or build credentials are bundled.
SOURCE.json fields commit/tree/platform/go identify actual clean source/toolchain.
Actual binary build metadata must match commit and vcs.modified=false; inspect ELF
for no interpreter/shared-library requirements. Default packaging is linux-amd64.

## Verification
scripts/agent-bundle.mjs exports verifyAgentBundle(directory), returning source,
manifestSHA256, binarySHA256 and files count. Reject unsafe/duplicate/unlisted/
missing/changed/symlinked entries, malformed source identity and missing required
binary/source/vendor/license/runbook content. This detects integrity errors, not
authenticity: obtain manifest/source identity through a trusted release channel.

scripts/test-agent-bundle.sh NEW_ABSOLUTE_DIRECTORY builds, tar/re-extracts and
relocates the bundle, verifies it, offline rebuilds exported source with empty Go
module/cache directories, GOPROXY=off/GOSUMDB=off/GOTOOLCHAIN=local/-mod=vendor,
and runs actual shipped/rebuilt preparation CLI with empty PATH/unrelated cwd.
Preparation is not enforcement; never label it applied. Full existing privileged
console workflow consumes the shipped binary after checksum verification, proving
31 prior/7 Gitea scenes and installed Controller/Agent restart on real Linux. Keep
112kernel/91required/9systemd and all Go/Web gates, plus a separately uploaded Agent
tar/SHA/public proof artifact. No mocked kernel or skipped opted-in scenario.

## Limits and self-review
No semantic policy/default/credential changes. No Core implementation changes.
No publishing a named release/tag or host installation. Follow-on host-service and
backup/restore acceptance remains separate. Autonomous mandate waives routine
design/plan/merge pauses; retain TDD, isolated exact-main branch and one fresh whole
branch review. Spec checked for placeholders, interface consistency and scope.

中文：交付携带源码/依赖及许可的独立Agent包，验证离线构建、空PATH运行及真实
Linux联合链路；不把准备结果当执行成功，不冒充宿主开机或法律合规认证。
