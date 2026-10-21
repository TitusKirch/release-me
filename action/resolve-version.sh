#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Resolves the release-me version to run: the "version" input, falling back
# to the ref this action itself was invoked at, or the literal "source" to
# build from the checkout instead of downloading a release.
set -euo pipefail

version="${INPUT_VERSION:-${ACTION_REF}}"
if [ -z "${version}" ]; then
  echo "::error::no version input and no action ref to fall back to" >&2
  exit 1
fi

if [ "${version}" = "source" ]; then
  echo "version=source" >>"${GITHUB_OUTPUT}"
  exit 0
fi

# A branch or other non-tag ref has no "vX.Y.Z" release to download: fail
# with a clear reason instead of turning, for example, "main" into the
# release tag "vmain".
if [[ ! "${version}" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]]; then
  echo "::error::'${version}' is not a release tag; set version explicitly, tag a release, or use version: source" >&2
  exit 1
fi

echo "version=${version#v}" >>"${GITHUB_OUTPUT}"
