#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Verifies the downloaded archive against the checksums file before
# anything extracts or runs it.
set -euo pipefail

cd "${RUNNER_TEMP}"

archive="release-me_${OS}_${ARCH}.tar.gz"
checksums="release-me_${VERSION}_checksums.txt"
# Escape the "." in the file name so it is not a regex metacharacter.
pattern="${archive//./\\.}"

if command -v sha256sum >/dev/null 2>&1; then
  grep -E " ${pattern}\$" "${checksums}" | sha256sum -c -
else
  grep -E " ${pattern}\$" "${checksums}" | shasum -a 256 -c -
fi
