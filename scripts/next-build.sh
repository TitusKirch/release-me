#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

commit="$(git rev-parse HEAD)"
RELEASE_ME_NEXT=1 goreleaser release --snapshot --clean --skip=publish

version="next-${commit}"
checksums="release-me_${version}_checksums.txt"
for os in linux darwin; do
  for arch in amd64 arm64; do
    test -s "dist/release-me_${os}_${arch}.tar.gz"
  done
done

printf '{"commit":"%s","version":"%s","repository":"%s"}\n' \
  "${commit}" "${version}" "${GITHUB_REPOSITORY:-TitusKirch/release-me}" >dist/next.json
(
  cd dist
  sha256sum next.json >>"${checksums}"
  sha256sum -c "${checksums}"
)
