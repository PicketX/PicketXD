#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Actual artifacts/offline compilation; no mocked success or privileged execution.
set -euo pipefail
if [[ $# != 1 || "$1" != /* || -e "$1" || -L "$1" || "$(realpath -m -- "$1")" != "$1" ]]; then
  echo 'Usage: test-agent-bundle.sh NEW_CANONICAL_ABSOLUTE_DIRECTORY' >&2
  exit 2
fi
work=$1
cd "$(dirname "$0")/.."
mkdir -m 700 "$work"
bash scripts/build-agent-bundle.sh "$work/build"
tar -C "$work/build" -czf "$work/installed-agent.tar.gz" .
(cd "$work" && sha256sum installed-agent.tar.gz > installed-agent.tar.gz.sha256 && sha256sum --check --strict installed-agent.tar.gz.sha256)
mkdir "$work/payload"
tar -xzf "$work/installed-agent.tar.gz" -C "$work/payload" --no-same-owner
node scripts/agent-bundle.mjs verify "$work/payload"
git init --bare -q "$work/source-check.git"
git --git-dir="$work/source-check.git" --work-tree="$work/payload/source/PicketXD" add --force --all -- . ':!vendor'
exported_tree=$(git --git-dir="$work/source-check.git" write-tree)
test "$exported_tree" = "$(git rev-parse 'HEAD^{tree}')"
echo "PASS Agent exported source tree: $exported_tree"
mkdir "$work/mod-cache" "$work/go-cache" "$work/unrelated-cwd"
(
  cd "$work/payload/source/PicketXD"
  export CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOWORK=off GOENV=off GOFLAGS=''
  export GOAMD64=v1 GOEXPERIMENT='' GO111MODULE=on
  export GOMODCACHE="$work/mod-cache" GOCACHE="$work/go-cache"
  # Do not accidentally discover the outer checkout while rebuilding an export.
  go build -buildvcs=false -mod=vendor -trimpath -o "$work/rebuilt-picketxd" ./cmd/picketxd
  go test -buildvcs=false -mod=vendor -count=1 -v ./... > "$work/offline-go.log" 2>&1
)
cat "$work/offline-go.log"
go version -m "$work/payload/bin/picketxd" > "$work/agent-build.txt"
go version -m "$work/rebuilt-picketxd" > "$work/rebuilt-build.txt"
node --input-type=module - "$work" <<'JS'
import assert from 'node:assert/strict'
import { readFile, writeFile, chmod, copyFile } from 'node:fs/promises'
import { join } from 'node:path'
import { execFileSync } from 'node:child_process'
import { verifyAgentBundle, validateAgentDistributionEvidence } from './scripts/agent-bundle.mjs'
const work = process.argv[2], bundle = join(work, 'payload'), p = await verifyAgentBundle(bundle)
const build = await readFile(join(work, 'agent-build.txt'), 'utf8')
assert(build.includes(`vcs.revision=${p.source.commit}`) && build.includes('vcs.modified=false'), 'actual clean shipped binary source')
const offline = await readFile(join(work, 'offline-go.log'), 'utf8')
assert(!/--- FAIL:|--- SKIP:|DATA RACE/.test(offline) && offline.includes('--- PASS:'), 'complete real offline Go tests')
for (const binary of [join(bundle, 'bin/picketxd'), join(work, 'rebuilt-picketxd')]) {
  assert(!execFileSync('readelf', ['-l', binary], { encoding: 'utf8' }).includes('INTERP'))
  assert(!execFileSync('readelf', ['-d', binary], { encoding: 'utf8' }).includes('NEEDED'))
}
await copyFile(join(bundle, 'source/PicketXD/examples/prepare/policy.yaml'), join(work, 'policy.yaml'))
await chmod(join(work, 'policy.yaml'), 0o600)
function prepare(binary, state) {
  const stdout = execFileSync(binary, ['prepare', '--once', '--policy', join(work, 'policy.yaml'), '--state', join(work, state),
    '--provider', 'bundle-smoke', '--agent', 'agent-a', '--destination', '192.0.2.100', '--destination', '2001:db8::100'],
  { cwd: join(work, 'unrelated-cwd'), env: { PATH: '' }, encoding: 'utf8' })
  const s = JSON.parse(stdout); assert.equal(s.prepared, true); assert.equal(s.enforcementConfirmed, false)
  return s
}
const original = prepare(join(bundle, 'bin/picketxd'), 'original-state/accepted.json')
const rebuilt = prepare(join(work, 'rebuilt-picketxd'), 'rebuilt-state/accepted.json')
const proof = { ...p, offlineBuild: true, offlineTests: true, relocated: true, runtimePATH: '', preparationOnly: true,
  originalPrepared: original.prepared, rebuiltPrepared: rebuilt.prepared, enforcementConfirmed: false,
  originalPolicyHash: original.policyHash, rebuiltPolicyHash: rebuilt.policyHash }
validateAgentDistributionEvidence(proof, { agentCommit: p.source.commit, agentTree: p.source.tree, agentBinarySHA256: p.binarySHA256 })
await writeFile(join(work, 'proof.json'), JSON.stringify(proof, null, 2) + '\n')
console.log('PASS source-backed Agent bundle: clean source, offline build/tests, relocated empty-PATH preparation; no enforcement claimed')
JS
