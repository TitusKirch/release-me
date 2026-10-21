#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Downloads the release archive and checksums file for VERSION, OS and ARCH
# from the public release download URL. DOWNLOAD_TOKEN is sent only to
# github.com, never to the signed storage URL it redirects to, and a public
# release does not need it at all; it only matters for a private repository.
set -euo pipefail

archive="release-me_${OS}_${ARCH}.tar.gz"
checksums="release-me_${VERSION}_checksums.txt"
base="https://github.com/ostendorf-cloud/release-me/releases/download/v${VERSION}"

for asset in "${archive}" "${checksums}"; do
  curl -fsSL \
    -H "Authorization: Bearer ${DOWNLOAD_TOKEN}" \
    -o "${RUNNER_TEMP}/${asset}" \
    "${base}/${asset}"
done
