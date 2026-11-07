#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
commit="${NEXT_SHA:?NEXT_SHA is required}"
repo="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
if [[ ! "${commit}" =~ ^[0-9a-f]{40}$ ]]; then
  echo "Next commit must be a full Git SHA." >&2
  exit 1
fi
current="$("${script_dir}/next-current.sh")"
if [ "${current}" != "current=true" ]; then
  exit 0
fi

jq -e --arg commit "${commit}" '
  .commit == $commit and .version == ("next-" + $commit)
' dist/next.json >/dev/null

tag="next-${commit}"
checksums="release-me_${tag}_checksums.txt"
(
  cd dist
  sha256sum -c "${checksums}"
)
assets=(dist/release-me_{linux,darwin}_{amd64,arm64}.tar.gz "dist/${checksums}" dist/next.json)
for asset in "${assets[@]}"; do
  test -s "${asset}"
done

# List errors fail the run; they must not be mistaken for a missing release.
existing="$(gh api --paginate "repos/${repo}/releases?per_page=100" \
  --jq ".[] | select(.tag_name == \"${tag}\") | .id")"
if [ -n "${existing}" ]; then
  target="$(gh release view "${tag}" --repo "${repo}" --json targetCommitish --jq .targetCommitish)"
  if [ "${target}" != "${commit}" ]; then
    echo "Existing Next release ${tag} targets another commit." >&2
    exit 1
  fi
  draft="$(gh release view "${tag}" --repo "${repo}" --json isDraft --jq .isDraft)"
  if [ "${draft}" != "true" ]; then
    echo "Next release ${tag} is already published; its assets remain unchanged."
    exit 0
  fi
  gh release upload "${tag}" --repo "${repo}" --clobber "${assets[@]}"
else
  notes="$(mktemp)"
  trap 'rm -f "${notes}"' EXIT
  cat >"${notes}" <<EOF
Unofficial Next build of the TitusKirch/release-me fork. This integrates upstream
pull requests before their official release; see NOTICE in every archive.

Source: https://github.com/${repo}/commit/${commit}
Version: ${tag}
Container: ghcr.io/${repo,,}:sha-${commit}

The archives cover Linux and macOS, each on amd64 and arm64. Verify your archive
against ${checksums}. This prerelease does not update stable or latest.
EOF
  gh release create "${tag}" --repo "${repo}" --target "${commit}" --draft \
    --prerelease --latest=false --title "Next (${commit:0:12})" --notes-file "${notes}" "${assets[@]}"
fi
gh release edit "${tag}" --repo "${repo}" --draft=false --prerelease --latest=false
