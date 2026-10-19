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
node --test integration/console-evidence.test.mjs integration/human-requests.test.mjs
mkdir -p artifacts/console-kernel
CGO_ENABLED=0 go build -trimpath -o "$root/artifacts/picketxd-console" ./cmd/picketxd
(
  cd "$core"
  CGO_ENABLED=1 go build -tags 'netgo osusergo sqlite_omit_load_extension' -ldflags '-linkmode external -extldflags -static' -o "$root/artifacts/controller-console" ./cmd/picketx-controller
  CGO_ENABLED=1 go build -tags 'netgo osusergo sqlite_omit_load_extension' -ldflags '-linkmode external -extldflags -static' -o "$root/artifacts/picketx-console" ./cmd/picketx
)
go version > artifacts/console-kernel/go.txt
go version -m artifacts/picketxd-console > artifacts/console-kernel/agent-build.txt
go version -m artifacts/controller-console > artifacts/console-kernel/controller-build.txt
nginx -V 2> artifacts/console-kernel/nginx.txt
sudo --non-interactive env PATH="$PATH" PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:?set installed browser directory}" PICKETX_CONSOLE_KERNEL_TESTS=1 \
  unshare --net -- node "$root/integration/console-kernel.mjs" "$core"
node --input-type=module -e 'import fs from "node:fs"; import {validateEvidence} from "./integration/console-evidence.mjs"; const r=JSON.parse(fs.readFileSync("artifacts/console-kernel/report.json")); validateEvidence(r); if(r.result!=="pass") throw Error("native run incomplete"); console.log(JSON.stringify(r,null,2))'
