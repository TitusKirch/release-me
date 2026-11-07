#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "${script_dir}/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

source_dir="${work}/parent/downloaded-action"
mkdir -p "${source_dir}/action" "${source_dir}/cmd/release-me" "${work}/caller" "${work}/bin"
cp "${script_dir}/build-source.sh" "${source_dir}/action/"
# The real compiler exercises the same entry point and version ldflag contract
# without requiring a module cache or downloading third-party dependencies.
go_version="$(awk '$1 == "go" { print $2; exit }' "${repo_dir}/go.mod")"
printf 'module example.com/release-me-action-source\n\ngo %s\n' "${go_version}" >"${source_dir}/go.mod"
cat >"${source_dir}/cmd/release-me/main.go" <<'GO'
// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
    "fmt"
    "os"
)

var Version = "dev"

func main() {
    if len(os.Args) != 2 || os.Args[1] != "version" {
        os.Exit(2)
    }
    fmt.Println(Version)
}
GO
# A remote action is an archive without .git. Go must not inspect the caller's
# or any other ancestor's metadata while building that downloaded source.
printf 'gitdir: missing-repository\n' >"${work}/parent/.git"

(
  cd "${work}/caller"
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off \
    GITHUB_ACTION_PATH="${source_dir}/action" RUNNER_TEMP="${work}/bin" VERSION=source \
    bash "${source_dir}/action/build-source.sh"
)
actual="$("${work}/bin/release-me" version)"
if [ "${actual}" != source ]; then
  echo "FAIL downloaded source version = ${actual}, want source" >&2
  exit 1
fi
echo 'PASS downloaded action builds offline from a foreign working directory despite invalid parent Git metadata'
