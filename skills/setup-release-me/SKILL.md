---
name: setup-release-me
description: >-
  Sets up release-me in a repository with no release tool yet: release-me.yaml, the labels it
  expects and the release workflow, proven by release-me check and next. Use when the user wants
  to set up, add or adopt release-me, or start releasing a repository with it.
---

# setup-release-me

Give a repository a working **release-me** setup: one `release-me.yaml`, the labels it names,
one release workflow, and a dry run proving the three agree. The
[release-me README](https://github.com/ostendorf-cloud/release-me#readme) at the pinned
release is the single source of truth for every key and action input; this skill decides which
of them the repository needs.

## 1. Survey

Read the repository; every row below comes from a command or a file, never from a guess.

- **Forge and merge methods:**
  `gh repo view --json nameWithOwner,defaultBranchRef,mergeCommitAllowed,squashMergeAllowed,rebaseMergeAllowed`.
  A repository off GitHub ends the run here: release-me releases through the GitHub API.
- **Release tooling already present:** a `release-please-config.json` hands the run to the
  `migrate-release-please-to-release-me` skill, an optional call: where it is not installed,
  the run ends here and names the release-please setup as the reason. Any other tool (`.releaserc*`, `.changeset/`, a
  workflow running semantic-release) is named in the plan and removed in the same change,
  because two tools tagging one history race each other.
- **Existing tags:** `git tag --list --sort=-v:refname | head -20`. Their scheme becomes the tag
  prefix, so release-me continues from the last one: `vX.Y.Z` is the implicit main component,
  `name@vX.Y.Z` a component with `tag-prefix: name@v`.
- **Components:** each separately versioned package and the file carrying its version
  (`package.json`, `Chart.yaml`, `Cargo.toml`, `pyproject.toml`, a Go constant). One package at
  the root is the implicit main component and needs no `components` key.
- **Version to pin:** `gh release list --repo ostendorf-cloud/release-me --limit 1`. Without a
  release there is nothing to pin, and the run ends with that finding.

Done when a survey table lists forge, merge methods, every component with its path, version
file and tag scheme, and the pinned version, each row filled or marked absent.

## 2. Decide

Three choices belong to the human; ask each once, recommendation first:

- **Versioning**, only with more than one component: `independent` (each its own line),
  `linked` (one shared line, only changed components release) or `locked` (one shared line,
  every component releases together).
- **Pre-releases:** wanted, or turned off with `alpha`, `beta` and `rc` set to `""` under
  `labels`, so the repository needs only the pending and tagged labels.
- **Token:** a GitHub App (recommended: its release pull requests trigger checks and the
  release commit is attributed to its bot) or a repository token.

Everything else is derived:

- **`changelog-source`** from the merge methods: squash only is `squash`, merge commits allowed
  is `first-parent`, rebase only is `all`.
- **`files`** per component from its version file: `package.json` with updater `json` and key
  `version`, `Chart.yaml` with `helm`, `Cargo.toml` with `toml` and key `package.version`,
  `pyproject.toml` with `toml` and key `project.version`, a Go constant with `go` and the
  constant's name as key.

Done when every key the plan writes is either answered by the human or derived above.

## 3. Plan, then write

Show the complete `release-me.yaml`, the labels to create and the workflow, then write after
confirmation:

- `release-me.yaml` at the repository root, carrying only keys that differ from the defaults.
- Each label `release-me.yaml` names and the repository lacks:
  `gh label create "<name>" --color ededed`.
- `.github/workflows/release.yaml` from the README's action section: `push` to the default
  branch plus `pull_request` with types `labeled` and `unlabeled`, the job guard listing
  exactly the configured label names, a checkout of the base branch with `fetch-depth: 0`, and
  `ostendorf-cloud/release-me/action` pinned to the release's commit SHA with the version as a
  trailing comment.

Done when the three artefacts exist and the guard's label list equals the labels in
`release-me.yaml`, name for name.

## 4. Prove

Run the pinned version locally, through mise:

```sh
mise exec ubi:ostendorf-cloud/release-me@<version> -- release-me check
mise exec ubi:ostendorf-cloud/release-me@<version> -- release-me next
```

- `check` passes. It needs the token in the environment; without one locally, report that the
  check is still open instead of calling the setup proven.
- `next` prints one line per component. A component with existing tags continues from its last
  one; one that reports its initial version despite such tags has the wrong prefix, and the run
  goes back to step 1.

Done when both outputs are shown to the human.

## Guardrails

- Hand the change over uncommitted; the human commits and pushes it.
- Tags are release-me's from the first run on: the setup creates none.
