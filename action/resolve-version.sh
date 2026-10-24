#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Resolves the release-me version to run: the "version" input, falling back
# to the ref this action itself was invoked at when that ref is a release
# tag, then to the VERSION file shipped at that same commit, or the literal
# "source" to build from the checkout instead of downloading a release.
set -euo pipefail

tag_pattern='^v?[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$'
version_file="${GITHUB_ACTION_PATH}/../VERSION"

if [ -n "${INPUT_VERSION:-}" ]; then
  version="${INPUT_VERSION}"
elif [[ "${ACTION_REF}" =~ ${tag_pattern} ]]; then
  version="${ACTION_REF}"
elif [ -s "${version_file}" ]; then
  # A consumer pinned by commit digest, the convention this action's own
  # README recommends, invokes the action with github.action_ref set to
  # the 40 character commit sha rather than a release tag, so the regex
  # above never matches. Read the version release-me's own "files" marker
  # bumps into VERSION on every release instead: it is the exact version
  # released from this same commit.
  version="$(awk '{print $1; exit}' "${version_file}")"
else
  version="${ACTION_REF}"
fi

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
if [[ ! "${version}" =~ ${tag_pattern} ]]; then
  echo "::error::'${version}' is not a release tag; set version explicitly, tag a release, or use version: source" >&2
  exit 1
fi

echo "version=${version#v}" >>"${GITHUB_OUTPUT}"
