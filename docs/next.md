# Next channel

`TitusKirch/release-me` is an unofficial fork of
[ostendorf-cloud/release-me](https://github.com/ostendorf-cloud/release-me). Its `next` branch
integrates upstream pull requests before their official release, including their review fixes.
The [NOTICE](../NOTICE) records the upstream attribution and included changes. Next builds are
prereleases, and their version is `next-<full commit SHA>`.

## GitHub Action

```yaml
permissions:
  contents: write
  pull-requests: write

steps:
  - uses: actions/checkout@fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09  # v5.1.0
    with:
      fetch-depth: 0
  - uses: TitusKirch/release-me/action@next
    with:
      token: ${{ secrets.RELEASE_TOKEN }}
```

Set `RELEASE_TOKEN` to the token your release workflow uses; a GitHub App installation token
also signs release commits and can trigger the release pull request's checks. See the
[Action reference](action.md) for configuration and outputs.

With `action@next`, leaving `version` out builds the downloaded action's Next source with Go.
It does not download the older upstream version in `VERSION`, and it does not build the
calling repository. Go is installed automatically from the action's `go.mod`. An explicit
`version: source` selects the same source build. An explicit release such as `version: 0.3.0`
continues to download that upstream release. `version: next` is deliberately not accepted;
select the Next action ref and leave `version` out.

For a fixed source revision, pin the action to that Next commit and set `version: source`.
Commit pins without that input retain the established `VERSION` file lookup.

## Downloads

Each successful publication creates a
[Next prerelease](https://github.com/TitusKirch/release-me/releases) tagged
`next-<full commit SHA>`, with these archives:

| Platform | Archive |
| --- | --- |
| Linux amd64 | `release-me_linux_amd64.tar.gz` |
| Linux arm64 | `release-me_linux_arm64.tar.gz` |
| macOS amd64 | `release-me_darwin_amd64.tar.gz` |
| macOS arm64 | `release-me_darwin_arm64.tar.gz` |

The release also includes `release-me_next-<SHA>_checksums.txt` and `next.json` with its source
commit. The successful [Next workflow run](https://github.com/TitusKirch/release-me/actions/workflows/next.yaml)
summary links directly to that commit's release and image. Download a known build with:

```sh
commit=<full published Next commit SHA>
tag="next-${commit}"
gh release download "${tag}" --repo TitusKirch/release-me \
  --pattern release-me_linux_amd64.tar.gz \
  --pattern "release-me_${tag}_checksums.txt"
grep ' release-me_linux_amd64.tar.gz$' "release-me_${tag}_checksums.txt" | sha256sum -c -
tar -xzf release-me_linux_amd64.tar.gz release-me
./release-me version
```

On macOS, use its archive and `shasum -a 256 -c -` to verify the matching checksum line.
Every archive contains `LICENSE` and `NOTICE`. Once published, the release's assets stay
unchanged on workflow reruns. A failed upload is retried while its release is still a draft.

## Container

```sh
docker run --rm --user "$(id -u):$(id -g)" -e GITHUB_TOKEN \
  -v "$PWD:/repo" -w /repo \
  ghcr.io/tituskirch/release-me:next next
```

The image supports Linux amd64 and arm64, includes Git, CA certificates and the SSH client,
and defaults to uid/gid `65532`. The example selects your local uid/gid so the mounted
repository keeps the ownership Git expects. Its filesystem permissions and credentials still
need to allow the operation you invoke.

Pin a published build with `ghcr.io/tituskirch/release-me:sha-<full commit SHA>` or its registry
digest. OCI labels record the source repository, revision and version. License and notice
are under `/usr/share/licenses/release-me/`.

## Publication

A push to `next` runs the repository's individual check jobs, including Go race tests, Action
and publication tests, and an offline container test that runs the real CLI against Git
history. After those pass, GoReleaser builds all four archives and checksums. The workflow
publishes the commit's prerelease and the container's `sha-<SHA>` and `next` tags. It never
updates stable releases or the container's `latest` tag. The ordinary release image workflow
ignores `next-*` prereleases.

The workflow serializes runs. Before publication it checks that the queued commit is still
the branch tip, and checks again before updating the floating container tag; older queued
builds are skipped. The Action source follows the branch immediately; binary and image
downloads become available after that commit's checks and publication finish. A failed build
leaves the last successful image in place.

Maintainers can rerun a failed job or dispatch the `next` workflow on the `next` branch. It
needs the repository's Actions token with `contents: write` and `packages: write`. The first
GHCR publication may need its package visibility set to public in GitHub's package settings.
Publication is restricted to this fork; ordinary upstream release workflows keep their own
release flow.
