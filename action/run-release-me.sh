#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Runs release-me with the inputs mapped to flags.
set -euo pipefail

args=(run)
if [ -n "${COMPONENT}" ]; then
  args+=(--component "${COMPONENT}")
fi
if [ "${DRY_RUN}" = "true" ]; then
  args+=(--dry-run)
fi
if [ "${VERBOSE}" = "true" ]; then
  args+=(--verbose)
fi

"${RUNNER_TEMP}/release-me" "${args[@]}"
