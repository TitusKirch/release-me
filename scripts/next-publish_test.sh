#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
mkdir -p "${work}/bin" "${work}/repo/dist"
git init -q --bare "${work}/origin.git"
git -C "${work}/repo" init -q -b next
git -C "${work}/repo" -c user.name=Test -c user.email=test@example.invalid \
  commit -q --allow-empty -m "feat: add the next release"
commit="$(git -C "${work}/repo" rev-parse HEAD)"
git -C "${work}/repo" remote add origin "${work}/origin.git"
git -C "${work}/repo" push -q origin HEAD:refs/heads/next
export NEXT_SHA="${commit}" GITHUB_REPOSITORY=TitusKirch/release-me
export GH_CALLS="${work}/calls" TEST_COMMIT="${commit}"
export PATH="${work}/bin:${PATH}"
cat >"${work}/bin/gh" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${GH_CALLS}"
case "$1 $2" in
  'api --paginate')
    if [ "${TEST_MODE:-}" = failure ]; then exit 1; fi
    if [[ "${TEST_MODE:-}" = existing-* ]]; then echo 17; fi
    ;;
  'release view')
    if [[ "$*" = *targetCommitish* ]]; then
      echo "${TEST_COMMIT}"
    elif [ "${TEST_MODE:-}" = existing-draft ]; then
      echo true
    else
      echo false
    fi
    ;;
esac
FAKE
chmod +x "${work}/bin/gh"
cd "${work}/repo"

make_assets() {
  for os in linux darwin; do
    for arch in amd64 arm64; do
      printf 'archive for %s %s\n' "${os}" "${arch}" >"dist/release-me_${os}_${arch}.tar.gz"
    done
  done
  printf '{"commit":"%s","version":"next-%s"}\n' "${commit}" "${commit}" >dist/next.json
  (cd dist && sha256sum ./*.tar.gz next.json >"release-me_next-${commit}_checksums.txt")
  : >"${GH_CALLS}"
}

make_assets
"${script_dir}/next-publish.sh" >/dev/null
grep -q "release create next-${commit}.*--target ${commit}.*--draft.*--prerelease.*--latest=false" "${GH_CALLS}"
grep -q "release edit next-${commit}.*--draft=false.*--prerelease.*--latest=false" "${GH_CALLS}"
echo 'PASS current Next builds publish a prerelease for their exact commit'

make_assets
TEST_MODE=existing-published "${script_dir}/next-publish.sh" >/dev/null
if grep -Eq 'release (create|upload|edit)' "${GH_CALLS}"; then
  echo 'FAIL a published commit release was modified' >&2
  exit 1
fi
echo 'PASS published commit releases and assets remain unchanged on a rerun'

make_assets
TEST_MODE=existing-draft "${script_dir}/next-publish.sh" >/dev/null
grep -q 'release upload .*--clobber' "${GH_CALLS}"
grep -q 'release edit .*--draft=false' "${GH_CALLS}"
if grep -q 'release create' "${GH_CALLS}"; then exit 1; fi
echo 'PASS interrupted draft uploads resume before publishing'

make_assets
if TEST_MODE=failure "${script_dir}/next-publish.sh" >/dev/null 2>&1; then
  echo 'FAIL release lookup failures must fail the run' >&2
  exit 1
fi
if grep -Eq 'release (create|upload|edit)' "${GH_CALLS}"; then exit 1; fi
echo 'PASS lookup failures never create a new release'

make_assets
echo corrupt >>dist/release-me_linux_amd64.tar.gz
if "${script_dir}/next-publish.sh" >/dev/null 2>&1; then
  echo 'FAIL a corrupt archive was accepted' >&2
  exit 1
fi
test ! -s "${GH_CALLS}"
echo 'PASS corrupt archives fail before any release API call'

make_assets
printf '{"commit":"other","version":"next-other"}\n' >dist/next.json
(cd dist && sha256sum ./*.tar.gz next.json >"release-me_next-${commit}_checksums.txt")
if "${script_dir}/next-publish.sh" >/dev/null 2>&1; then
  echo 'FAIL artifacts from a different commit were accepted' >&2
  exit 1
fi
test ! -s "${GH_CALLS}"
echo 'PASS artifacts cannot be relabeled as another commit'

make_assets
NEXT_SHA=0000000000000000000000000000000000000000 "${script_dir}/next-publish.sh" >/dev/null 2>&1
test ! -s "${GH_CALLS}"
echo 'PASS stale queued builds never overwrite a newer Next publication'

make_assets
if TEST_MODE=existing-published TEST_COMMIT=0000000000000000000000000000000000000000 \
  "${script_dir}/next-publish.sh" >/dev/null 2>&1; then
  echo 'FAIL an existing release targeting another commit was accepted' >&2
  exit 1
fi
if grep -Eq 'release (create|upload|edit)' "${GH_CALLS}"; then exit 1; fi
echo 'PASS an existing tag cannot be reused for another source commit'

make_assets
git remote set-url origin "${work}/missing-origin.git"
if "${script_dir}/next-publish.sh" >/dev/null 2>&1; then
  echo 'FAIL a branch lookup error was treated as an outdated build' >&2
  exit 1
fi
test ! -s "${GH_CALLS}"
echo 'PASS branch lookup failures stop publication'

echo 'all tests passed'
