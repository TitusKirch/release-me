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
- **Version to pin:** `gh release list --repo ostendorf-cloud/release-me --limit 1`.

Done when the survey lists both files, every release-please workflow, every consumer with its
file and line, the open release pull request or its absence, and the pinned version.

## 2. Translate

```sh
mise exec ubi:ostendorf-cloud/release-me@<version> -- release-me migrate release-please \
  > release-me.yaml 2> "${TMPDIR:-/tmp}/release-me-migrate-warnings.txt"
```

Done when the command exited 0 and both files exist. A non-zero exit names the file or key it
could not translate; that finding goes to the human as it stands.

## 3. Settle every warning

Every `warning:` line in the warnings file gets a row in a table: the warning, the action
taken, and the reason. An action is one of: a change to `release-me.yaml` (a file entry, a
key), a change elsewhere in the repository, or an acceptance the human confirms. Two kinds are
always the human's:

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

- **Workflow:** replace the release-please step with `ostendorf-cloud/release-me/action` as the
  README's action section shows it: `push` to the default branch plus `pull_request` with types
  `labeled` and `unlabeled`, a job guard listing exactly the labels in `release-me.yaml`, a
  checkout of the base branch with `fetch-depth: 0`, the action pinned to the release's commit
  SHA with the version as a trailing comment.
- **Consumers:** each one from step 1 rewired to a signal release-me does produce: a pushed tag
  matching the component's `tag-prefix`, a published GitHub release, or a check against the
  package registry.
- **Labels:** release-please's labels carry over by name; create only those `release-me.yaml`
  names and the repository lacks.
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
