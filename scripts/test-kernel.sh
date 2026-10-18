#!/usr/bin/env bash
# Build unprivileged; execute only prebuilt test binaries in disposable namespaces.
set -euo pipefail
if [[ ${PICKETX_KERNEL_TESTS:-} != 1 ]]; then
  echo 'Set PICKETX_KERNEL_TESTS=1 to run real isolated Linux tests.' >&2
  exit 2
fi
if [[ $(uname -s) != Linux ]]; then
  echo 'Kernel validation requires Linux.' >&2
  exit 2
fi
repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
artifacts_dir=${PICKETX_TEST_ARTIFACTS:-$repo_dir/artifacts}
mkdir -p "$artifacts_dir"
artifacts_dir=$(CDPATH= cd -- "$artifacts_dir" && pwd)
# Keep repeat runs from reusing stale successes.
rm -f "$artifacts_dir/nft.json" "$artifacts_dir/ct.json" "$artifacts_dir/bridge.json" "$artifacts_dir/host.json" "$artifacts_dir/owner.json" "$artifacts_dir/agent.json" "$artifacts_dir/report.json"
cmp examples/prepare/policy.yaml internal/validation/hostbridge/testdata/policy.yaml
cmp examples/enforce/policy.yaml internal/hostagent/testdata/policy.yaml
go mod download
CGO_ENABLED=0 go test -c -tags=integration -o "$artifacts_dir/nft.test" ./internal/validation/nftbackend
CGO_ENABLED=0 go test -c -tags=integration -o "$artifacts_dir/ct.test" ./internal/validation/ctbackend
CGO_ENABLED=0 go test -c -tags=integration -o "$artifacts_dir/bridge.test" ./internal/validation/planbridge
CGO_ENABLED=0 go test -c -tags=integration -o "$artifacts_dir/host.test" ./internal/validation/hostbridge
CGO_ENABLED=0 go test -c -tags=integration -o "$artifacts_dir/owner.test" ./internal/hostowner
CGO_ENABLED=0 go test -c -tags=integration -o "$artifacts_dir/agent.test" ./internal/hostagent
CGO_ENABLED=0 go build -o "$artifacts_dir/picketxd" ./cmd/picketxd
sha256sum "$artifacts_dir/picketxd" > "$artifacts_dir/agent-binary.txt"
go version -m "$artifacts_dir/picketxd" >> "$artifacts_dir/agent-binary.txt"
# Build the actual Apache Core executable from the pinned module, without copying
# Agent implementation across repository/license boundaries.
core_module_dir=$(go list -m -f '{{.Dir}}' github.com/PicketX/PicketX)
(cd "$core_module_dir" && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags 'netgo osusergo sqlite_omit_load_extension' -ldflags '-linkmode external -extldflags -static' -o "$artifacts_dir/picketx-controller" ./cmd/picketx-controller)
sha256sum "$artifacts_dir/picketx-controller" > "$artifacts_dir/controller-binary.txt"
go version -m "$artifacts_dir/picketx-controller" >> "$artifacts_dir/controller-binary.txt"
if readelf -l "$artifacts_dir/picketx-controller" | grep -q INTERP; then
  echo 'Controller artifact unexpectedly requires an interpreter' >&2
  exit 1
fi
CGO_ENABLED=0 go build -o "$artifacts_dir/m0-report" ./cmd/m0-report
uname -r > "$artifacts_dir/kernel.txt"
go version > "$artifacts_dir/go.txt"
go list -m all > "$artifacts_dir/modules.txt"
# unshare must succeed in opted-in runs; no privilege failure is a skip.
privilege=()
if [[ $(id -u) != 0 ]]; then privilege=(sudo --non-interactive); fi
result=0
for pair in nft:nftbackend ct:ctbackend bridge:planbridge host:hostbridge owner:hostowner agent:hostagent; do
  binary=${pair%%:*}
  package=${pair#*:}
  extra=()
  if [[ $binary == bridge ]]; then extra=(-test.run '^TestBridge'); fi
  package_path="github.com/PicketX/PicketXD/internal/validation/$package"
  if [[ $binary == owner ]]; then
    extra=(-test.run '^TestOwnerKernel')
    package_path="github.com/PicketX/PicketXD/internal/hostowner"
  fi
  if [[ $binary == agent ]]; then
    extra=(-test.run '^TestEnforceProcess')
    package_path="github.com/PicketX/PicketXD/internal/hostagent"
  fi
  go tool test2json -t -p "$package_path" \
    "${privilege[@]}" env PICKETX_KERNEL_TESTS=1 PICKETX_AGENT_BINARY="$artifacts_dir/picketxd" PICKETX_CONTROLLER_BINARY="$artifacts_dir/picketx-controller" unshare --net \
    "$artifacts_dir/$binary.test" -test.v -test.timeout=3m "${extra[@]}" > "$artifacts_dir/$binary.json" 2>&1 || result=1
  cat "$artifacts_dir/$binary.json"
done
"$artifacts_dir/m0-report" "$artifacts_dir" || result=1
exit "$result"
