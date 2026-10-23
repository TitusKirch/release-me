---
name: migrate-release-please-to-release-me
description: >-
  Moves a repository from release-please to release-me without losing a tag or a changelog
  line: translate with release-me migrate, settle every warning, swap the workflow, and prove
  release-me continues from the last release. Use when the user wants to migrate, switch or move
  from release-please to release-me.
---

# migrate-release-please-to-release-me

A **lossless** switch: after it, release-me continues every component from the tag
release-please last cut, writes into the changelog release-please wrote, and nothing that
consumed release-please's results is left waiting for a signal that never comes.
`release-me migrate release-please` does the translation; this skill owns everything around it.
The [release-me README](https://github.com/ostendorf-cloud/release-me#readme) at the pinned
release is the single source of truth for every key and action input.

## When to use

The repository releases with release-please, a `release-please-config.json` at its root, and
the user wants release-me to take over. A repository with no release tool goes to the
`setup-release-me` skill instead.

## 1. Survey

Read the repository; every row below comes from a command or a file.

- **release-please files:** `release-please-config.json` and `.release-please-manifest.json`.
  Without the config the repository has nothing to migrate, and the run hands over to the
  `setup-release-me` skill, an optional call: where it is not installed, the run ends here and
  says the repository has no release-please setup.
- **Workflows:** every file under `.github/workflows/` running
  `googleapis/release-please-action`, or calling a reusable workflow that does.
- **Consumers of release-please's results:** every job or step reading its outputs
  (`releases_created`, `release_created`, `tag_name`, `paths_released`, `<path>--release_created`
  and the like), its labels, or its release pull request branch `release-please--*`. These are
  the parts release-me does not replace one for one, since its action has no outputs.
- **Open release pull request:** `gh pr list --label "autorelease: pending" --state open`. One
  that is open is merged or closed before the switch lands, so no release is half cut.
- **Version to pin:** `gh api repos/ostendorf-cloud/release-me/releases/latest --jq .tag_name`.
  Without a release, a 404, there is nothing to pin, and the run ends with that finding.
- **An earlier attempt:** a `release-me.yaml` already at the root is named in the plan; step 4
  replaces it only after the human confirms.

Done when the survey lists both files, every release-please workflow, every consumer with its
file and line, the open release pull request or its absence, the pinned version, and an
existing `release-me.yaml` or its absence.

## 2. Translate

Nothing is written into the repository yet: the translation and its warnings go to the
temporary directory, from any directory of the repository, since `migrate` finds the root
itself.

```sh
mise exec ubi:ostendorf-cloud/release-me@<version> -- release-me migrate release-please \
  > "${TMPDIR:-/tmp}/release-me-migrate.yaml" 2> "${TMPDIR:-/tmp}/release-me-migrate-warnings.txt"
```

Done when the command exited 0 and both files exist. On a non-zero exit, show the warnings file
to the human as it stands: it holds the warnings, the error naming the file or key that could
not be translated, and, for a translation release-me refuses, that translation to correct by
hand.

## 3. Settle every warning

Every `warning:` line in the warnings file gets a row in a table: the warning, the action
taken, and the reason. An action is one of: a change to the translated `release-me.yaml` in the
temporary directory (a file entry, a key), a change elsewhere in the repository, or an
acceptance the human confirms. Two kinds are always the human's:

- **A missing tag:** release-me would start that component over at its initial version. The
  choices are creating the tag at the commit that release shipped from, or accepting the
  restart.
- **Versioning:** the translation is `independent`, or `locked` for a `linked-versions` group
  over every component. Offer `linked` too, one shared version line where only changed
  components release, which release-please has no equivalent for.

Done when the table has exactly one row per warning line and every row has an action.

## 4. Plan, then write

Show the final `release-me.yaml`, the table, and the workflow diff, then change after
confirmation:

- **Configuration:** the settled translation copied from the temporary directory to
  `release-me.yaml` at the repository root.
- **Workflow:** replace the release-please step with `ostendorf-cloud/release-me/action` as the
  README's action section shows it: `push` to the default branch plus `pull_request` with types
  `labeled` and `unlabeled`, a job guard listing exactly the labels in `release-me.yaml`, a
  checkout of the base branch with `fetch-depth: 0`, the action pinned to the release's commit
  SHA with the version as a trailing comment.
- **Consumers:** each one from step 1 rewired to a signal release-me does produce: a pushed tag
  matching the component's `tag-prefix`, a published GitHub release, or a check against the
  package registry.
- **Labels:** release-please's labels carry over by name. Each one `release-me.yaml` names and
  `gh api repos/{owner}/{repo}/labels --paginate --jq '.[].name'` lacks is listed for the human
  to create; labels are repository settings, and the skill creates none.
- **Removal:** `release-please-config.json`, `.release-please-manifest.json` and the
  release-please workflow steps. Changelogs stay, since release-me prepends to them.

Done when `git grep -n release-please` returns only history, such as changelog entries and
release notes, and each remaining hit is named in the plan as history.

## 5. Prove

```sh
mise exec ubi:ostendorf-cloud/release-me@<version> -- release-me check
mise exec ubi:ostendorf-cloud/release-me@<version> -- release-me next
mise exec ubi:ostendorf-cloud/release-me@<version> -- release-me changelog
```

- `check` passes. It needs the token in the environment; without one locally, report the check
  as still open instead of calling the migration proven.
- `next` continues every component from its last release-please tag. A component at its
  initial version without a confirmed restart from step 3 sends the run back to step 3.
- `changelog` lists only changes after that tag.

Done when the three outputs are shown to the human.

## Guardrails

- Hand the change over uncommitted; the human commits and pushes it, ideally with no release
  pull request of either tool open.
- Tags come from release-me, or from the human's explicit choice in step 3.
- Labels and other repository settings are reported, never changed.
