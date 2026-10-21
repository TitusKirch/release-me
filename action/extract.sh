#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Extracts the binary from the verified archive, in place in RUNNER_TEMP.
set -euo pipefail

cd "${RUNNER_TEMP}"

tar -xzf "release-me_${OS}_${ARCH}.tar.gz" release-me
chmod +x release-me
