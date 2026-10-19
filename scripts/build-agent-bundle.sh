#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Build-time tools only; the shipped CGO-free executable needs none of them.
set -euo pipefail
if [[ $# != 1 || "$1" != /* || -e "$1" || -L "$1" || "$(realpath -m -- "$1")" != "$1" ]]; then
  echo 'Usage: build-agent-bundle.sh NEW_CANONICAL_ABSOLUTE_DIRECTORY (must not exist)' >&2
  exit 2
fi
output=$1
cd "$(dirname "$0")/.."
if [[ -n $(git status --porcelain --untracked-files=all) ]]; then
  echo 'Commit all source changes before making an attributable Agent bundle' >&2
  exit 2
fi
mkdir -m 755 "$output"
mkdir -p "$output/bin" "$output/source/PicketXD" "$output/deploy/systemd"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -trimpath -o "$output/bin/picketxd" ./cmd/picketxd
file "$output/bin/picketxd"
headers=$(readelf -l "$output/bin/picketxd"); dynamic=$(readelf -d "$output/bin/picketxd")
if [[ "$headers" == *INTERP* || "$dynamic" == *NEEDED* ]]; then
  echo 'Refusing dynamically linked Agent artifact' >&2
  exit 1
fi
git archive HEAD | tar -x -C "$output/source/PicketXD"
go mod vendor -o "$output/source/PicketXD/vendor"
cp LICENSE THIRD_PARTY_NOTICES.md "$output/"
cp deploy/agent/README.md deploy/agent/README.zh-CN.md "$output/deploy/"
cp deploy/systemd/picketxd.service deploy/systemd/README.md deploy/systemd/README.zh-CN.md "$output/deploy/systemd/"
printf '{"commit":"%s","tree":"%s","platform":"linux-amd64","go":"%s"}\n' \
  "$(git rev-parse HEAD)" "$(git rev-parse 'HEAD^{tree}')" "$(go env GOVERSION)" > "$output/SOURCE.json"
node scripts/agent-bundle.mjs collect "$output"
node scripts/agent-bundle.mjs seal "$output"
echo "Built source-backed standalone Agent bundle: $output"
