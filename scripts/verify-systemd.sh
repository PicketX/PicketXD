#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
set -euo pipefail
cd "$(dirname "$0")/.."
verify_dir=$(mktemp -d)
trap 'rm -rf "$verify_dir"' EXIT
# Offline syntax verification; the packaged binary is installed only on deployment.
sed 's|/usr/local/libexec/picketxd|/usr/bin/true|' deploy/systemd/picketxd.service > "$verify_dir/picketxd.service"
systemd-analyze verify --man=no --generators=no "$verify_dir/picketxd.service"
