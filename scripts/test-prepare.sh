#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail
cd "$(dirname "$0")/.."
prepare_tmp="$(mktemp -d)"
trap 'rm -rf "$prepare_tmp"' EXIT
CGO_ENABLED=0 go build -trimpath -o "$prepare_tmp/picketxd" ./cmd/picketxd
file "$prepare_tmp/picketxd"
headers="$(readelf -l "$prepare_tmp/picketxd")"
dynamic="$(readelf -d "$prepare_tmp/picketxd")"
if [[ "$headers" == *INTERP* || "$dynamic" == *NEEDED* ]]; then
  echo 'Preparation binary unexpectedly requires shared libraries' >&2
  exit 1
fi
cp examples/prepare/policy.yaml "$prepare_tmp/policy.yaml"
args=(prepare --once --policy "$prepare_tmp/policy.yaml" --state "$prepare_tmp/state/accepted.json" --provider smoke-static --agent agent-a --destination 192.0.2.100 --destination 2001:db8::100)
"$prepare_tmp/picketxd" "${args[@]}" > "$prepare_tmp/first.json"
"$prepare_tmp/picketxd" "${args[@]}" > "$prepare_tmp/restart.json"
printf 'broken publication\n' > "$prepare_tmp/policy.yaml"
if "$prepare_tmp/picketxd" "${args[@]}" > "$prepare_tmp/retained.json" 2> "$prepare_tmp/error.json"; then
  echo 'Rejected publication exited successfully' >&2
  exit 1
fi
python3 - "$prepare_tmp" <<'PY'
import json, pathlib, sys
root=pathlib.Path(sys.argv[1])
first=json.loads((root/'first.json').read_text())
restart=json.loads((root/'restart.json').read_text())
retained=json.loads((root/'retained.json').read_text())
assert first['prepared'] and not first['restored']
assert restart['restored'] and restart['sameBoot']
assert first['generation']==restart['generation']==retained['generation']==1
assert first['policyHash']==restart['policyHash']==retained['policyHash']
assert retained['lastError'] and retained['restored']
assert not any(x['enforcementConfirmed'] for x in [first,restart,retained])
print('Static preparation/restart/invalid-publication retention passed; no enforcement claimed')
PY
