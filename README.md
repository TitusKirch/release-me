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

Every writing command accepts `--dry-run`: it prints what would change and writes nothing, not
even a branch. `--verbose` enables debug logging on stderr.

Every command loops over the configured components, in `release-me.yaml`'s order, or the
single implicit `main` component a repository without `components` carries. `--component
<name>` limits a command to one. Output lines are prefixed with the component's name, `"<name>:
<line>"`, only when a command runs more than one component; a run limited to one component, by
`--component` or because the repository has only one, prints unprefixed lines, the same shape a
single-component repository always has.

`next` prints the next version and the bump reason per component, for example `0.2.0 (feat)`.
`changelog` prints the rendered entries for that version. Both print nothing for a component
with nothing to release.

### pr

Creates or updates one release pull request per component: branch `release-me/<name>` off the
repository's default branch, with that component's changelog update and its configured files
bumped to its next version. The default branch is read from the forge;
`git symbolic-ref refs/remotes/origin/HEAD` is the fallback when that call fails. The branch
itself is still forked from whatever commit is checked out, which must be the default branch's
tip, same as `next` and `changelog`. The branch is written in a temporary checkout, never in
your working copy.

`pr` checks that the `pending`, `tagged`, `alpha`, `beta` and `rc` labels exist before it writes
anything, the same check `check` runs. The labels are shared across every component; the
default branch is resolved once and shared too.

Rerunning `pr` is judged on content, not on the rendered pull request body: the changelog
entries for the version, without the heading's date, and every configured file's bumped
content are compared against the release branch's current tip. Equal content changes nothing,
not even the pull request body, and nothing is pushed. A change pushes the branch and sets the
heading to today's date; the push carries a `--force-with-lease` whose expected value is the
existing pull request's head SHA from the forge, not a locally fetched ref, so nothing can
quietly defeat it. A fresh branch, with no existing pull request to lease against, pushes
without force. Editing only the notes block of an open pull request updates its body, with no
push: the content that would land on the branch did not change.

Updating an existing pull request never replaces its labels: a label added by hand stays, and
the `pending` label is added back if it is ever missing, never set by replacing the full list.

The pull request title is `chore(<name>): release vX.Y.Z`, `chore(main): release vX.Y.Z` for
the implicit `main` component. Its body starts with `Release vX.Y.Z`, then the notes block
(below) when one is present, then the full changelog for the version inside a collapsed
`<details>` section. When every entry is a hidden type, the changelog carries one explicit
line, `- Version bump without user facing changes.`, instead of an empty heading.

### Pre-releases

A pre-release type label on the open release pull request, `release: alpha`, `release: beta`
or `release: rc` by default, turns its next run into a pre-release: version `X.Y.Z-alpha.N`,
`X.Y.Z-beta.N` or `X.Y.Z-rc.N`. `X.Y.Z` is the version a stable release would get from the
entries since the last stable tag; pre-release tags never move that base. Exactly one of the
three labels may be present at a time; two at once is an error naming both.

`N` starts at `1` for a type and counts up from the highest `N` an earlier pre-release tag of
that same type and base version already carries. Switching the label to a different type does
not inherit the previous type's counter: it starts at `1` again, even when tags for the other
type already exist.

The changelog a pre-release renders depends on what came before it: the first pre-release since
the last stable tag collects the same entries a stable release would, since that stable tag; a
pre-release that follows another one collects only the entries since that previous pre-release
tag. Removing the label from the open pull request returns it to the stable version on the next
run, branch content, title and body updated accordingly.

### release

Tags and releases every merged pull request on a component's release branch that still carries
the `pending` label: a tag on the merge commit, a GitHub release with the pull request's notes
and changelog, draft always false and prerelease true for a pre-release version. The label
swap removes `pending`, and the matching pre-release type label when the version carries one,
replacing both with `tagged`. The implicit `main` component tags `vX.Y.Z` and names the release
after the tag, today's shape; a configured component tags `<name>/vX.Y.Z` and names the release
`<name> vX.Y.Z`. A tag that already exists, left over from an earlier, interrupted run, is not
an error: the release and the label swap still happen, so the pull request does not stay
pending forever, and the run reports it as already tagged.

With `major-tag: true`, every stable release additionally creates or force-moves the major
tag, `vX` for the implicit `main` component or `<name>/vX` for a configured one, onto the same
commit. A tag that does not exist yet is created; one that does is moved, never left behind. A
pre-release, once that lands, never moves it. `--dry-run` reports whether the major tag would
be created or moved, writing nothing.

### run

`pr`, then `release`, for every component. The CI entry point: one command covers opening or
updating every release pull request and releasing whatever merged since the last run.

### check

Validates `release-me.yaml`, including an unknown `--component` name, that the `pending`,
`tagged`, `alpha`, `beta` and `rc` labels exist on the repository, and that the token
environment variable is both set and accepted by the forge: a rejected token is reported as
such, not as a missing label, since the symptom from the forge is the same 401 either way.
Prints every missing or invalid item and exits 1 when anything is missing, 0 otherwise.

### Notes block

A fenced block `release-me-notes` in the release pull request body is free text for the
release:

````markdown
```release-me-notes
Run the migration before upgrading.
```
````

Add it by hand once the pull request is open. `pr` keeps it verbatim on every later update, and
`release` carries it into the GitHub release, above the changelog.

### Override block

A fenced block `release-me` in an ordinary pull request body overrides its changelog entries,
one Conventional Commits line per entry:

````markdown
```release-me
feat(cli): add the next command
fix: stop the panic on empty history
```
````

An empty block hides the pull request from the changelog and the version computation
entirely.

## Configuration

`release-me.yaml` at the repository root. A repository without `components` is one component.

```yaml
initial-version: 0.1.0
bump-minor-pre-major: true
hidden-types: [chore, docs, ci, style, refactor, test, build]
changelog: CHANGELOG.md
labels:
  pending: "release: pending"
  tagged: "release: tagged"
  alpha: "release: alpha"
  beta: "release: beta"
  rc: "release: rc"
files:
  - path: cmd/cli/version.go
    updater: marker
```

- `initial-version`: the version when no tag exists yet, default `0.1.0`.
- `bump-minor-pre-major`: turns a major bump into a minor one below `1.0.0`, default `false`.
- `hidden-types`: hides entries of these types from the changelog, default `chore`, `docs`,
  `ci`, `style`, `refactor`, `test`, `build`, `other`. Only `feat`, `fix`, `perf`, `revert` and
  a breaking change ever render a section; a type outside those five never renders, hidden or
  not.
- `fallback-type`: the type assigned to text that does not parse as a conventional commit,
  default `other`. `feat`, `fix`, `perf` and `revert` are refused, unparseable text never
  bumps the version.
- `token-env`: the environment variable holding the GitHub token, default `GITHUB_TOKEN`. It
  governs API calls only; `pr`'s push to the release branch authenticates through the git
  credentials the checkout's `origin` remote already carries, the same as any other push.
- `changelog`: the changelog file path, default `CHANGELOG.md`. `pr` prepends the new version's
  section to it, creating the file when it does not exist yet.
- `labels.pending` and `labels.tagged`: the label names `pr`, `release` and `check` use,
  default `release: pending` and `release: tagged`.
- `labels.alpha`, `labels.beta` and `labels.rc`: the pre-release type labels `pr`, `release`
  and `check` use, default `release: alpha`, `release: beta` and `release: rc`. See
  [Pre-releases](#pre-releases). `check` reports any of the five missing labels by name, it
  never creates one.
- `files`: files `pr` bumps to the next version, each a `path`, an optional `updater`, default
  `marker`, and a `key` where the updater needs one. Every updater keeps the rest of the file
  byte for byte: order, comments, indentation. A file that does not exist, or a `key` not found
  in it, fails `pr`, naming the path and the key.

  - `marker` replaces the version on a line ending in `# release-me: version` or
    `// release-me: version`, keeping the rest of the line, including the comment, as it is. No
    `key`:

    ```go
    const Version = "0.1.0" // release-me: version
    ```

  - `helm` replaces a `Chart.yaml`'s top-level `version` field. No `key`:

    ```yaml
    version: 0.1.0
    ```

  - `go` replaces the string literal assigned to the constant or variable `key` names:

    ```yaml
    files:
      - path: internal/version/version.go
        updater: go
        key: Version
    ```

  - `json`, `yaml` and `toml` replace the value at `key`, a dotted path of object keys,
    mapping keys or table-and-key, such as `tool.poetry.version`.

  Every file entry is validated when release-me.yaml loads: an unknown `updater`, a `key`
  missing where the updater requires one, and a `key` given to an updater that takes none are
  all errors.

- `major-tag`: moves the major tag on every stable release, default `false`. See `release`
  above. A component overrides it through its own `major-tag` key.
- `components`: maps a name to a `path` and optional overrides of `initial-version`,
  `bump-minor-pre-major`, `hidden-types`, `files` and `major-tag`. A key left out of a
  component falls back to the top-level setting above. A repository without `components` is
  one component named `main` at path `.`, tag prefix `v`, branch `release-me/main`, the shape
  every example above already is. A configured component gets tag prefix `<name>/v`, branch
  `release-me/<name>`, changelog `<path>/CHANGELOG.md`, and a release title `<name> vX.Y.Z`:

  ```yaml
  components:
    frontend:
      path: web/frontend
      initial-version: 0.2.0
      files:
        - path: web/frontend/package.json
    backend:
      path: services/backend
      hidden-types: [chore, docs, test]
  ```

  A first parent entry belongs to every component whose path it touches, through the files a
  merge commit's branch changed; a pull request touching several components lands in every one
  of their changelogs. The component at path `.`, explicit or implicit, matches every entry.
  Two components with overlapping paths, or an unknown name on `--component`, are configuration
  errors.

An unknown key is an error, inside a component too. The file is read from the repository
root, found through `git rev-parse --show-toplevel`, whatever directory the command runs
from.

## Development

```sh
task setup
task --list
task check
```

One target per check, one CI job per target. `check` only calls the small targets.

## License

Apache-2.0. See `LICENSE`.
