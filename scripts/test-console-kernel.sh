#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
set -eu
if [ "${PICKETX_CONSOLE_KERNEL_TESTS:-}" != 1 ]; then
  echo 'PICKETX_CONSOLE_KERNEL_TESTS=1 is required; no native test was run' >&2
  exit 1
fi
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
core=$(CDPATH= cd -- "${1:?provide the pinned Core checkout}" && pwd)
cd "$root"
expected=$(node --input-type=module -e 'import {coreCommit} from "./integration/console-evidence.mjs"; console.log(coreCommit)')
test "$(git -C "$core" rev-parse HEAD)" = "$expected"
test -z "$(git -C "$core" status --porcelain --untracked-files=no)"
test -f "$core/apps/web/dist/index.html"
test -z "$(git status --porcelain --untracked-files=all)"
node --test integration/*.test.mjs
mkdir -p artifacts/console-kernel
CGO_ENABLED=0 go build -trimpath -o "$root/artifacts/picketxd-console" ./cmd/picketxd
file "$root/artifacts/picketxd-console"
headers=$(readelf -l "$root/artifacts/picketxd-console")
dynamic=$(readelf -d "$root/artifacts/picketxd-console")
case "$headers$dynamic" in *INTERP*|*NEEDED*) echo 'Agent must be fully static' >&2; exit 1;; esac
(
  cd "$core"
  bash scripts/build-controller-bundle.sh "$root/artifacts/controller-bundle-build"
)
tar -C "$root/artifacts/controller-bundle-build" -czf "$root/artifacts/installed-controller.tar.gz" .
(cd "$root/artifacts" && sha256sum installed-controller.tar.gz > installed-controller.tar.gz.sha256)
mkdir "$root/artifacts/controller-bundle"
tar -xzf "$root/artifacts/installed-controller.tar.gz" -C "$root/artifacts/controller-bundle"
go version > artifacts/console-kernel/go.txt
go version -m artifacts/picketxd-console > artifacts/console-kernel/agent-build.txt
go version -m artifacts/controller-bundle/bin/picketx-controller > artifacts/console-kernel/controller-build.txt
cp artifacts/controller-bundle/SOURCE.json artifacts/console-kernel/controller-source.json
cp artifacts/controller-bundle/SHA256SUMS artifacts/console-kernel/controller-manifest.txt
nginx -V 2> artifacts/console-kernel/nginx.txt
sudo --non-interactive env PATH="$PATH" PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:?set installed browser directory}" PICKETX_CONSOLE_KERNEL_TESTS=1 \
  unshare --net -- node "$root/integration/console-kernel.mjs" "$core"
node --input-type=module -e 'import fs from "node:fs"; import {validateEvidence} from "./integration/console-evidence.mjs"; import {parseEnforceStatusLog} from "./integration/controller-install.mjs"; import {validateGiteaEvidence} from "./integration/gitea-application.mjs"; const dir="artifacts/console-kernel/"; const r=JSON.parse(fs.readFileSync(dir+"report.json")); validateEvidence(r,{statuses:JSON.parse(fs.readFileSync(dir+"agent-status.json")),restartedStatuses:parseEnforceStatusLog(fs.readFileSync(dir+"agent-restarted.log","utf8"))}); validateGiteaEvidence(r.gitea,r.confirmations); if(r.result!=="pass") throw Error("native run incomplete"); console.log(JSON.stringify(r,null,2))'
