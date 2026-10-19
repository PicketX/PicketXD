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
# A clean working tree can still contain ignored Go/embedded input, and ambient
# overlays/workspaces can replace committed code without setting vcs.modified.
repo=$(pwd -P)
source_commit=$(git rev-parse HEAD)
source_tree=$(git rev-parse 'HEAD^{tree}')
export GOENV=off GOWORK=off GOFLAGS='' GOTOOLCHAIN=local GO111MODULE=on
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 GOEXPERIMENT=''
if [[ ! $(go env GOVERSION) =~ ^go1\.27\.[0-9]+$ ]]; then
  echo 'Use a supported local Go1.27.x toolchain' >&2
  exit 2
fi
mkdir -m 755 "$output"
mkdir -p "$output/bin" "$output/source" "$output/deploy/systemd"
snapshot="$output/source/PicketXD"
git -c core.hooksPath=/dev/null clone --quiet --no-hardlinks --no-checkout -- "$repo" "$snapshot"
git -c core.hooksPath=/dev/null -C "$snapshot" checkout --quiet --detach "$source_commit"
# Only the dependency snapshot intentionally added by this builder is ignored.
printf '/vendor/\n' >> "$snapshot/.git/info/exclude"
cd "$snapshot"
go mod verify
go build -mod=readonly -trimpath -o "$output/bin/picketxd" ./cmd/picketxd
metadata=$(go version -m "$output/bin/picketxd")
if [[ "$metadata" != *"vcs.revision=$source_commit"* || "$metadata" != *'vcs.modified=false'* || "$metadata" != *'GOAMD64=v1'* || "$metadata" != *'CGO_ENABLED=0'* ]]; then
  echo 'Refusing Agent binary with inconsistent source/target build metadata' >&2
  exit 1
fi
file "$output/bin/picketxd"
headers=$(readelf -l "$output/bin/picketxd"); dynamic=$(readelf -d "$output/bin/picketxd")
if [[ "$headers" == *INTERP* || "$dynamic" == *NEEDED* ]]; then
  echo 'Refusing dynamically linked Agent artifact' >&2
  exit 1
fi
go mod vendor
cp LICENSE THIRD_PARTY_NOTICES.md "$output/"
cp deploy/agent/README.md deploy/agent/README.zh-CN.md "$output/deploy/"
cp deploy/systemd/picketxd.service deploy/systemd/README.md deploy/systemd/README.zh-CN.md "$output/deploy/systemd/"
printf '{"commit":"%s","tree":"%s","platform":"linux-amd64","go":"%s"}\n' \
  "$source_commit" "$source_tree" "$(go env GOVERSION)" > "$output/SOURCE.json"
node scripts/agent-bundle.mjs collect "$output"
test -z "$(git status --porcelain --untracked-files=all)"
# Remove only this freshly created clone's private Git metadata. Original source
# repository/worktree/branches are untouched; the public snapshot keeps all files.
test -d "$snapshot/.git" && test ! -L "$snapshot/.git"
rm -rf -- "$snapshot/.git"
node scripts/agent-bundle.mjs seal "$output"
echo "Built source-backed standalone Agent bundle: $output"
