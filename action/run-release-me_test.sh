#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Unit tests for run-release-me.sh. A fake release-me in RUNNER_TEMP stands
# in for the real binary: it records its arguments, prints the
# "released-tag: <tag>" lines the "tags" output is read from and, like the
# real one, writes its own outputs to the file RELEASE_ME_GITHUB_OUTPUT
# names, so these run offline and never touch a real GITHUB_OUTPUT.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
runner="${script_dir}/run-release-me.sh"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

current="${work}/current"
older="${work}/older"
mkdir -p "${current}" "${older}"

# current/release-me is a release-me that writes its own outputs. The
# literal $@ and variables below are the fake script's own, not this test
# script's.
{
  echo '#!/usr/bin/env bash'
  echo 'set -euo pipefail'
  # shellcheck disable=SC2016
  echo 'echo "$@" >"${RUNNER_TEMP}/args"'
  echo 'echo "created chore: release duxt v0.9.0, duxt-ui v0.4.1"'
  echo 'echo "released-tag: duxt@v0.9.0"'
  echo 'echo "released-tag: duxt-ui@v0.4.1"'
  # shellcheck disable=SC2016
  echo 'if [ -n "${RELEASE_ME_GITHUB_OUTPUT:-}" ]; then'
  # shellcheck disable=SC2016
  echo '  echo "released=true" >>"${RELEASE_ME_GITHUB_OUTPUT}"'
  echo 'fi'
} >"${current}/release-me"
chmod +x "${current}/release-me"

# older/release-me predates the outputs: it ignores
# RELEASE_ME_GITHUB_OUTPUT and only prints the released-tag lines.
{
  echo '#!/usr/bin/env bash'
  # shellcheck disable=SC2016
  echo 'echo "$@" >"${RUNNER_TEMP}/args"'
  echo 'echo "released-tag: v0.2.1"'
} >"${older}/release-me"
chmod +x "${older}/release-me"

failures=0

# run_case name runner_temp component dry_run verbose want_args want_output
run_case() {
  local case_name="$1" runner_temp="$2" component="$3" dry_run="$4" verbose="$5"
  local want_args="$6" want_output="$7"
  local output_file status got_args got_output

  output_file="${work}/github_output"
  : >"${output_file}"

  set +e
  RUNNER_TEMP="${runner_temp}" \
    COMPONENT="${component}" \
    DRY_RUN="${dry_run}" \
    VERBOSE="${verbose}" \
    GITHUB_OUTPUT="${output_file}" \
    "${runner}" >"${work}/stdout" 2>"${work}/stderr"
  status=$?
  set -e

  if [ "${status}" -ne 0 ]; then
    echo "FAIL ${case_name}: exit status = ${status}, want 0"
    echo "  stderr: $(cat "${work}/stderr")"
    failures=$((failures + 1))
    return
  fi

  got_args="$(cat "${runner_temp}/args")"
  if [ "${got_args}" != "${want_args}" ]; then
    echo "FAIL ${case_name}: args = ${got_args@Q}, want ${want_args@Q}"
    failures=$((failures + 1))
    return
  fi

  got_output="$(cat "${output_file}")"
  if [ "${got_output}" != "${want_output}" ]; then
    echo "FAIL ${case_name}: GITHUB_OUTPUT = ${got_output@Q}, want ${want_output@Q}"
    failures=$((failures + 1))
    return
  fi

  echo "PASS ${case_name}"
}

run_case "release-me writes its own outputs next to the tags read from its output" \
  "${current}" "" "false" "false" \
  "run" \
  "released=true
tags<<RELEASE_ME_TAGS_EOF
duxt@v0.9.0
duxt-ui@v0.4.1
RELEASE_ME_TAGS_EOF"

run_case "the inputs map to flags" \
  "${current}" "duxt" "true" "true" \
  "run --component duxt --dry-run --verbose" \
  "released=true
tags<<RELEASE_ME_TAGS_EOF
duxt@v0.9.0
duxt-ui@v0.4.1
RELEASE_ME_TAGS_EOF"

run_case "a release-me older than the outputs still fills tags" \
  "${older}" "" "false" "false" \
  "run" \
  "tags<<RELEASE_ME_TAGS_EOF
v0.2.1
RELEASE_ME_TAGS_EOF"

if [ "${failures}" -ne 0 ]; then
  echo "${failures} test(s) failed"
  exit 1
fi

echo "all tests passed"
