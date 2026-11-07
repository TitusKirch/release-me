#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

commit="$(git rev-parse HEAD)"
image="release-me-runtime-test:${commit}"
docker build --build-arg "VERSION=next-${commit}" --build-arg "REVISION=${commit}" -t "${image}" .
test "$(docker run --rm --network=none "${image}" version)" = "next-${commit}"
test "$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "${image}")" = "${commit}"
docker run --rm --network=none --entrypoint /bin/sh "${image}" -ec '
  test "$(id -u)" != 0
  test -s /etc/ssl/certs/ca-certificates.crt
  command -v ssh >/dev/null
  test -s /usr/share/licenses/release-me/LICENSE
  test -s /usr/share/licenses/release-me/NOTICE
  work=$(mktemp -d)
  cd "${work}"
  git init -q -b main
  git config user.name "Next runtime test"
  git config user.email "runtime@example.invalid"
  git remote add origin https://github.com/example/project.git
  git commit -q --allow-empty -m "chore: initialize repository"
  git tag v1.0.0
  git commit -q --allow-empty -m "feat: add the export endpoint"
  actual=$(/release-me next)
  test "${actual}" = "1.1.0 (feat)" || {
    printf "unexpected next output: %s\n" "${actual}" >&2
    exit 1
  }
'
