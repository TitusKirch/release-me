#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Publication runs serialize, and a queued run for an outdated SHA does not publish.
set -euo pipefail

commit="${NEXT_SHA:?NEXT_SHA is required}"
head="$(git ls-remote --exit-code origin refs/heads/next | cut -f1)"
if [ "${head}" = "${commit}" ]; then
  echo "current=true"
else
  echo "current=false"
  echo "Skipping outdated Next build ${commit}; next is now ${head}." >&2
fi
