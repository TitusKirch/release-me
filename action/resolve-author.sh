#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Resolves the git author and committer identity for the release commit and
# exports it to GITHUB_ENV. An explicit author-name or author-email input
# wins, each on its own; with only app-slug set, the missing one is built
# from the GitHub App's bot account, "<slug>[bot]", whose id GH_TOKEN reads
# through the API. Nothing set leaves the tool's own configured or default
# identity in place: GITHUB_ENV is left untouched.
set -euo pipefail

name="${INPUT_AUTHOR_NAME:-}"
email="${INPUT_AUTHOR_EMAIL:-}"
slug="${INPUT_APP_SLUG:-}"

if [ -n "${slug}" ] && { [ -z "${name}" ] || [ -z "${email}" ]; }; then
  login="${slug}[bot]"
  if ! id="$(gh api "users/${login}" --jq .id)"; then
    echo "::error::no bot user for app slug '${slug}'" >&2
    exit 1
  fi
  [ -n "${name}" ] || name="${login}"
  [ -n "${email}" ] || email="${id}+${login}@users.noreply.github.com"
fi

if [ -n "${name}" ]; then
  {
    echo "GIT_AUTHOR_NAME=${name}"
    echo "GIT_COMMITTER_NAME=${name}"
  } >>"${GITHUB_ENV}"
fi

if [ -n "${email}" ]; then
  {
    echo "GIT_AUTHOR_EMAIL=${email}"
    echo "GIT_COMMITTER_EMAIL=${email}"
  } >>"${GITHUB_ENV}"
fi
