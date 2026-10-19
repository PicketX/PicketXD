# Installed Controller and native Agent conjunction

## Intent and boundary
Prove Core PR35's actual installation output drives existing real browser/Agent/
kernel/Nginx/Gitea workflows. Core93024b229c072ce1db8512323c3d7fa35e3df74e,
tree64ed0dcc4b6a3232e093150d7fc100cc9a852e43; Agent233f08112ce9fc17625a28d85e38c67e77eb6ef6.
No production protocol/kernel/UI change; preserve31 prior conjunction cases,
7 Gitea cases,112 kernel cases and9 systemd cases. No simulated/native-skipped proof.

## Selected design
Build pinned Core with its actual committed musl/static bundle script in CI,
tar/re-extract and relocate outside both checkouts. Verify SOURCE/manifest/regular
files/exec bits before running its CLI. Historic artifact downloads expire, so
build exact source instead. A formal GPL Agent distribution package is separate;
this gate deploys the existing real CGO0 executable, not a new Agent release.

Run actual controller-init in empty PATH/unrelated cwd. Preserve its generated
config, independent private credentials and randomized authority. Bootstrap audit
identity is principal:bootstrap. Explicit operator action expands ONLY the private
caller registry from route-v1 to also route-gitea-v1, never a wider default. Keep
source/users database modes and config unmodified. Start Controller/Agent using
absolute deployed binaries with empty PATH and unrelated cwd. Test driver/Nginx/
Gitea retain their own tools; no dependency-free claim for those fixtures. Agent
provider equals generated authority; scope remains dual-stack host INPUT/graceful.

After two source Grants are active, record full publication/accounts/Grants/audits,
stop both services cleanly and restart the identical DB/Agent ownership/LKG paths.
Old browser cookie must401; reauthenticate and require fresh post-restart Agent
status plus actual Controller stream report before probes. Full saved views remain
identical, with no init rerun, reimport, new revision or renewed deadline. Dual-stack
requests still allow; all later existing revoke/expiry/human/Gitea cases still run.
This is process restart, NOT host reboot or installed systemd acceptance.

## Evidence / acceptance
Required public proof: exact bundle source/tree/manifest/binary hashes, empty PATH,
generated authority, database modes, clean exits, before/after revision/hash/views
and fresh confirmation. Reject missing/substituted source, state changes, stale
status, nonzero exits, source runtime assets and credential-bearing extra fields.
Unit fixtures validate gates only; actual acceptance exclusively native Linux.
Full Go normal/race/vet/build + locked Web, one independent whole-branch review,
exact-head/source/raw/archive merge and main checks remain mandatory.

## Reviewed context map
| File | Responsibility/change |
| --- | --- |
| integration/controller-install{,.test}.mjs | Verify/initialize real bundle and strict public evidence gate |
| integration/console-kernel.mjs | Consume generated config/keys, relocated processes, real restart/probes |
| integration/console-evidence{,.test}.mjs | Require deployment evidence/pinned Core identity |
| integration/managed-humans.mjs | Generated bootstrap audit actor |
| scripts/test-console-kernel.sh | Actual bundle/tar/ELF and native invocation |
| .github/workflows/validation.yml | New Core pin/musl dependency, all existing gates retained |
| deploy/controller/README{,.zh-CN}.md | Narrow credential/provider pairing and process recovery runbook |
Dependencies unchanged: Agent hostagent/agentstate, Apache Core APIs/init/bundle;
no public API/database migration/production configuration change.

中文：实际安装包及生成配置进入既有Linux/Gitea验收，验证双服务重启及新鲜确认。
Core/Agent空PATH运行，测试工具不是产品运行时；正式Agent分发/宿主开机另做。
自主授权覆盖例行设计/计划等待，保留TDD、原生验证与一次独立评审。
