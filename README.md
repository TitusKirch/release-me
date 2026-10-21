# release-me

Release automation for repositories that merge with merge commits. One changelog entry per
pull request, versions from Conventional Commits, a release pull request, a tag and a release.

## What it does

- Reads the first parent history since the last tag. A merge commit is one entry, its pull
  request title is the text. A commit directly on the default branch is its own entry.
- Computes the next version from the entry types. Pre-releases through a label on the release
  pull request.
- Opens and updates one release pull request per component with the changelog, bumps the
  version in the files you list, and on merge creates the tag and the release.
- GitHub first. GitLab follows through the same forge interface.

## Install

Not available yet. The first release is cut by hand, from then on the tool releases itself.

```sh
mise use ubi:ostendorf-cloud/release-me
```

## Use

```sh
release-me next        # next version, nothing written
release-me changelog   # entries for the next version, nothing written
release-me pr          # create or update the release pull requests
release-me release     # tag and release merged release pull requests
release-me run         # pr then release, the CI entry point
release-me check       # config, labels and token
```

Every writing command accepts `--dry-run`.

## Configuration

`release-me.yaml` at the repository root. A repository without `components` is one component.

```yaml
initial-version: 0.1.0
bump-minor-pre-major: true
major-tag: false
hidden-types: [chore, docs, ci, style, refactor, test, build]
components:
  cli:
    path: cmd/cli
    major-tag: true
    files:
      - path: cmd/cli/version.go
        updater: go
        key: Version
```

- A pull request overrides its entries with a fenced block `release-me` in its body, one entry
  per line. An empty block hides the pull request.
- The release pull request takes free text for the release from a fenced block
  `release-me-notes` in its body.
- Labels the tool expects: `release: pending`, `release: tagged`, `release: alpha`,
  `release: beta`, `release: rc`. It reports a missing label, it never creates one.

## Development

```sh
task setup
task --list
task check
```

One target per check, one CI job per target. `check` only calls the small targets.

## License

Apache-2.0. See `LICENSE`.
