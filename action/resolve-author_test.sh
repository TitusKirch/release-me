#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Unit tests for resolve-author.sh. A fake gh on PATH stands in for the API
# call the app-slug lookup makes, so these run offline, and never touch a
# real GITHUB_ENV.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
resolver="${script_dir}/resolve-author.sh"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

fixed_id=12345
ok_bin="${work}/ok-bin"
fail_bin="${work}/fail-bin"
mkdir -p "${ok_bin}" "${fail_bin}"

# ok-bin/gh answers any "users/<login>" lookup with the fixed id. The
# literal $1 below is the fake script's own, not this test script's.
{
  echo '#!/usr/bin/env bash'
  echo 'set -euo pipefail'
  # shellcheck disable=SC2016
  echo 'if [ "$1" = "api" ]; then'
  echo "  echo ${fixed_id}"
  echo '  exit 0'
  echo 'fi'
  echo 'exit 1'
} >"${ok_bin}/gh"
chmod +x "${ok_bin}/gh"

# fail-bin/gh fails every lookup, as the API does for an app with no bot
# account, or a slug that does not exist.
{
  echo '#!/usr/bin/env bash'
  echo 'echo "gh: not found" >&2'
  echo 'exit 1'
} >"${fail_bin}/gh"
chmod +x "${fail_bin}/gh"

failures=0

# run_case name author_name author_email app_slug gh_bin want_status want_env
run_case() {
  local case_name="$1" author_name="$2" author_email="$3" app_slug="$4" gh_bin="$5"
  local want_status="$6" want_env="$7"
  local env_file status got_env

  env_file="${work}/github_env"
  : >"${env_file}"

  set +e
  INPUT_AUTHOR_NAME="${author_name}" \
    INPUT_AUTHOR_EMAIL="${author_email}" \
    INPUT_APP_SLUG="${app_slug}" \
    GH_TOKEN="test-token" \
    GITHUB_ENV="${env_file}" \
    PATH="${gh_bin}:${PATH}" \
    "${resolver}" 2>"${work}/stderr"
  status=$?
  set -e

  if [ "${status}" -ne "${want_status}" ]; then
    echo "FAIL ${case_name}: exit status = ${status}, want ${want_status}"
    failures=$((failures + 1))
    return
  fi

  got_env="$(cat "${env_file}")"
  if [ "${got_env}" != "${want_env}" ]; then
    echo "FAIL ${case_name}: GITHUB_ENV = ${got_env@Q}, want ${want_env@Q}"
    failures=$((failures + 1))
    return
  fi

  echo "PASS ${case_name}"
}

explicit_env="GIT_AUTHOR_NAME=Jane Doe
GIT_COMMITTER_NAME=Jane Doe
GIT_AUTHOR_EMAIL=jane@example.com
GIT_COMMITTER_EMAIL=jane@example.com"
run_case "explicit author-name and author-email win" "Jane Doe" "jane@example.com" "release-me" "${ok_bin}" 0 "${explicit_env}"

slug_env="GIT_AUTHOR_NAME=release-me[bot]
GIT_COMMITTER_NAME=release-me[bot]
GIT_AUTHOR_EMAIL=${fixed_id}+release-me[bot]@users.noreply.github.com
GIT_COMMITTER_EMAIL=${fixed_id}+release-me[bot]@users.noreply.github.com"
run_case "app-slug alone resolves the bot account" "" "" "release-me" "${ok_bin}" 0 "${slug_env}"

run_case "nothing set exports nothing" "" "" "" "${ok_bin}" 0 ""

run_case "a failed lookup fails the step" "" "" "release-me" "${fail_bin}" 1 ""
if ! grep -q "release-me" "${work}/stderr"; then
  echo "FAIL a failed lookup fails the step: stderr does not name the slug"
  failures=$((failures + 1))
fi

if [ "${failures}" -ne 0 ]; then
  echo "${failures} test(s) failed"
  exit 1
fi

echo "all tests passed"
