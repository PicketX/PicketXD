# Standalone source-backed Agent bundle

> English (default) | [简体中文](README.zh-CN.md)

The Agent is GPL-3.0-or-later; the Apache Controller is distributed separately.
Keep the whole bundle, LICENSE, upstream notices, source and SHA manifest together.
The bundle is engineering evidence, not automated legal clearance or an OEM grant.

## Build, verify and relocate

From a clean committed checkout with Go1.27.x, Node, Bash, Git, GNU tar/coreutils,
file and readelf, run `bash scripts/build-agent-bundle.sh /absolute/new/bundle`.
The builder compiles an isolated committed-source clone, verifies module-cache
integrity and normalizes Go input-selection settings/AMD64 v1. Ignored local Go
files, ambient overlays/workspaces or Go configuration cannot enter the binary.
The target must not exist, even as a dangling symlink; no overwrite or host install
is performed. A failed build leaves partial output for inspection. Confirm source
identity over your trusted release channel, then verify every `SHA256SUMS` entry
before executing the binary. SHA files alone do not authenticate their publisher.
Move/extract the entire bundle to a reviewed destination, preserving executable
modes. The shipped Linux-amd64 executable needs no Go, npm or shared libraries.

## Source and offline rebuild

`source/PicketXD` contains the exact committed Agent source (private clone Git
metadata removed), build scripts and
official Go-vendored dependencies needed to build/test its packages. Upstream
dependency tests are not included by `go mod vendor`. Actual upstream licenses/
notices and the Go runtime LICENSE/PATENTS are retained under `licenses/` with a
versioned index. Acquire the supported Go toolchain separately from its publisher.

With Go1.27.x installed, enter `source/PicketXD`, set `GOPROXY=off`, `GOSUMDB=off`,
`GOTOOLCHAIN=local`, `CGO_ENABLED=0`, `GOOS=linux`, `GOARCH=amd64`, and use
`go build -mod=vendor -trimpath -o /absolute/new/picketxd ./cmd/picketxd`.
For tests use `go test -mod=vendor ./...` in an appropriate Linux environment;
privileged opt-in scenarios need disposable network namespaces and explicit flags.
No network/module cache is required for compilation with the bundled vendor tree.
This is not a byte-identical reproducible-build claim: Git metadata is absent from
the source export. SOURCE.json records original commit/tree/platform/Go version.

## First use and boundaries

`bin/picketxd --help` works from an unrelated directory with an empty PATH.
`prepare --once` only validates/persists a policy; it never confirms enforcement.
Read the bundled systemd guide and source `deploy/controller/README.md` for actual
Controller enrollment, independent token files, authority/provider matching,
exclusive marks, dual-stack bounds and persistent state. Configure your real host;
do not install a sample unit, mark range or policy blindly. Retain private state
outside this public bundle. Nothing enables a host service automatically.

Native acceptance tar/extracts this bundle, offline rebuilds with empty module/cache
directories and runs the real binary with empty PATH. The existing actual Linux
browser→Controller→Agent→trusted Nginx→Gitea conjunction executes the shipped binary,
including restart, dual-stack traffic, denial, independent Grants and strict revoke.
Separate required native gates cover installed Controller service recovery and the
Agent's latest stopped original-path checkpoint; see source docs/validation/
AGENT_STATE_RECOVERY.md. Host boot, migration and old-snapshot rollback remain excluded.
