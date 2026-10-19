#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail
cd "$(dirname "$0")/.."
verify_dir=$(mktemp -d)
trap 'rm -rf "$verify_dir"' EXIT
# Offline syntax verification; the packaged binary is installed only on deployment.
for unit in picketxd.service picketxd-controller.service; do
  sed 's|/usr/local/libexec/picketxd|/usr/bin/true|' "deploy/systemd/$unit" > "$verify_dir/$unit"
  systemd-analyze verify --man=no --generators=no "$verify_dir/$unit"
done
