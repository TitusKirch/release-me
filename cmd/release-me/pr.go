// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/history"
	"github.com/ostendorf-cloud/release-me/internal/releasepr"
	"github.com/ostendorf-cloud/release-me/internal/updater"
)

// noUserFacingChangesLine is rendered under the changelog heading when a
// release has no visible entry, every one of them a hidden type, so the
// version bump stays visible instead of leaving a bare heading.
const noUserFacingChangesLine = "- Version bump without user facing changes."

// compareRef is the local ref the release branch's current remote tip is
// fetched into, so its content can be read with "git show" without
// disturbing any real remote-tracking ref.
const compareRef = "refs/release-me/compare"

// runPR implements the pr command: it creates or updates the release pull
// request.
func runPR(args []string, stdout, stderr io.Writer) int {
	verbose, dryRun, ok := parseWriteFlags("pr", args, stderr)
	if !ok {
		return 2
	}

	ctx := context.Background()
	dir, cfg, f, logger, err := setup(ctx, stderr, verbose)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	if err := doPR(ctx, dir, cfg, f, logger, dryRun, stdout); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

// doPR creates or updates the release pull request for the repository at
// dir. Idempotency is judged on content, not on the rendered title and
// body: the changelog entries (without the heading's date) and the bumped
// files are compared against the release branch's current tip. Equal
// content changes nothing, not even the pull request body, unless the
// notes block itself changed, which updates the body alone, no push.
func doPR(ctx context.Context, dir string, cfg config.Config, f forge.Forge, logger *slog.Logger, dryRun bool, stdout io.Writer) error {
	owner, repoName, err := history.Remote(ctx, dir)
	if err != nil {
		return fmt.Errorf("release-me: origin remote: %w", err)
	}

	rel, released, err := computeRelease(ctx, dir, cfg, f, logger)
	if err != nil {
		return err
	}
	if !released {
		fmt.Fprintln(stdout, "nothing to release")
		return nil
	}

	// The pending and tagged labels must exist before anything is written,
	// the same check doCheck runs.
	missing, unauthorized, err := missingLabels(ctx, f, owner, repoName, cfg)
	if err != nil {
		if unauthorized {
			return fmt.Errorf("release-me: token rejected by the forge: %w", err)
		}
		return fmt.Errorf("release-me: check labels: %w", err)
	}
	if len(missing) > 0 {
		return fmt.Errorf("release-me: label(s) do not exist on the repository: %s", strings.Join(missing, ", "))
	}

	existingList, err := f.PullRequests(ctx, owner, repoName, releasepr.Branch, "open")
	if err != nil {
		return fmt.Errorf("release-me: list release pull requests: %w", err)
	}
	var existing *forge.PullRequest
	if len(existingList) > 0 {
		existing = &existingList[0]
		if len(existingList) > 1 {
			logger.Warn("more than one open release pull request, using the first",
				"head", releasepr.Branch, "count", len(existingList), "number", existing.Number)
		}
	}

	base, err := resolveBaseBranch(ctx, f, dir, owner, repoName)
	if err != nil {
		return err
	}

	entriesBody := changelog.Sections(rel.entries, rel.hiddenTypes)
	if entriesBody == "" {
		entriesBody = noUserFacingChangesLine
	}

	plan, err := planReleaseBranch(ctx, dir, rel.version.Version, entriesBody, cfg)
	if err != nil {
		return err
	}
	defer plan.cleanup()

	notes := ""
	if existing != nil {
		notes, _ = releasepr.ExtractNotes(existing.Body)
	}

	date := time.Now()
	if existing != nil && !plan.changed {
		if d, ok := releasepr.ExtractDate(existing.Body); ok {
			date = d
		}
	}
	section := fmt.Sprintf("## v%s (%s)\n\n%s\n", rel.version.Version, date.Format("2006-01-02"), entriesBody)

	title := releasepr.Title(rel.version.Version)
	body := releasepr.Body(rel.version.Version, notes, section)

	switch {
	case existing != nil && !plan.changed && body == existing.Body:
		fmt.Fprintln(stdout, "release pull request up to date")
		return nil

	case existing != nil && !plan.changed:
		// The content that would land on the branch did not change, only
		// the notes block or another free-text part of the body did: there
		// is nothing to push.
		if dryRun {
			fmt.Fprintf(stdout, "would update %s\n", title)
			return nil
		}
		if _, err := f.UpdatePullRequest(ctx, owner, repoName, existing.Number, title, body); err != nil {
			return fmt.Errorf("release-me: update release pull request #%d: %w", existing.Number, err)
		}
		if err := ensurePendingLabel(ctx, f, owner, repoName, existing.Number, existing.Labels, cfg.Labels.Pending); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "updated %s\n", title)
		return nil

	default:
		action := "create"
		if existing != nil {
			action = "update"
		}
		if dryRun {
			fmt.Fprintf(stdout, "would %s %s\n", action, title)
			return nil
		}

		if err := plan.write(ctx, existing, cfg, section, title); err != nil {
			return err
		}

		if existing == nil {
			if _, err := f.CreatePullRequest(ctx, owner, repoName, base, releasepr.Branch, title, body, []string{cfg.Labels.Pending}); err != nil {
				return fmt.Errorf("release-me: create release pull request: pushed branch %s but the pull request failed: %w", releasepr.Branch, err)
			}
			fmt.Fprintf(stdout, "created %s\n", title)
			return nil
		}
		if _, err := f.UpdatePullRequest(ctx, owner, repoName, existing.Number, title, body); err != nil {
			return fmt.Errorf(
				"release-me: update release pull request #%d: pushed branch %s but the update failed: %w",
				existing.Number, releasepr.Branch, err,
			)
		}
		if err := ensurePendingLabel(ctx, f, owner, repoName, existing.Number, existing.Labels, cfg.Labels.Pending); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "updated %s\n", title)
		return nil
	}
}

// resolveBaseBranch reads the repository's default branch from the forge,
// falling back to the "origin" remote's reported HEAD when the forge call
// fails. It never uses the branch currently checked out: on a detached
// HEAD, that would be the literal string "HEAD", not a branch name.
func resolveBaseBranch(ctx context.Context, f forge.Forge, dir, owner, repoName string) (string, error) {
	base, err := f.DefaultBranch(ctx, owner, repoName)
	if err == nil && base != "" {
		return base, nil
	}

	fallback, fallbackErr := history.OriginHEAD(ctx, dir)
	if fallbackErr != nil {
		if err == nil {
			err = fmt.Errorf("the forge returned an empty name")
		}
		return "", fmt.Errorf("release-me: default branch: %w, and the origin HEAD fallback failed: %w", err, fallbackErr)
	}
	return fallback, nil
}

// ensurePendingLabel adds the pending label to a pull request when it is
// not already carrying it, never replacing the labels it has.
func ensurePendingLabel(ctx context.Context, f forge.Forge, owner, repoName string, number int, currentLabels []string, pending string) error {
	if slices.Contains(currentLabels, pending) {
		return nil
	}
	if err := f.AddLabels(ctx, owner, repoName, number, []string{pending}); err != nil {
		return fmt.Errorf("release-me: add label %q to release pull request #%d: %w", pending, number, err)
	}
	return nil
}

// releasePlan is the release branch content doPR would write, computed
// against a temporary worktree checked out at dir's HEAD, together with
// whether it differs from what the release branch currently carries.
type releasePlan struct {
	dir               string
	tmp               string
	baselineChangelog string
	desiredFiles      map[string]string
	changed           bool
	cleanedUp         bool
}

// planReleaseBranch computes the release branch content doPR would write
// for version and entriesBody, and compares it against the release
// branch's current tip, fetched into compareRef. A branch that does not
// exist on the remote yet is always "changed": there is nothing to compare
// it against.
func planReleaseBranch(ctx context.Context, dir, version, entriesBody string, cfg config.Config) (*releasePlan, error) {
	tmp, err := os.MkdirTemp("", "release-me-pr-")
	if err != nil {
		return nil, fmt.Errorf("release-me: temporary worktree: %w", err)
	}
	plan := &releasePlan{dir: dir, tmp: tmp}

	if err := gitRun(ctx, dir, "worktree", "add", "--detach", "--quiet", tmp, "HEAD"); err != nil {
		_ = os.RemoveAll(tmp)
		return nil, fmt.Errorf("release-me: add worktree: %w", err)
	}

	baseline, err := os.ReadFile(filepath.Join(tmp, cfg.ChangelogPath))
	if err != nil && !os.IsNotExist(err) {
		plan.cleanup()
		return nil, fmt.Errorf("release-me: read %s: %w", cfg.ChangelogPath, err)
	}
	plan.baselineChangelog = string(baseline)

	plan.desiredFiles = make(map[string]string, len(cfg.Files))
	for _, file := range cfg.Files {
		content, err := os.ReadFile(filepath.Join(tmp, file.Path))
		if err != nil {
			plan.cleanup()
			return nil, fmt.Errorf("release-me: read %s: %w", file.Path, err)
		}
		up, ok := updater.Updaters[file.Updater]
		if !ok {
			plan.cleanup()
			return nil, fmt.Errorf("release-me: file %s: unknown updater %q", file.Path, file.Updater)
		}
		newContent, err := up(string(content), version)
		if err != nil {
			plan.cleanup()
			return nil, fmt.Errorf("release-me: file %s: %w", file.Path, err)
		}
		plan.desiredFiles[file.Path] = newContent
	}

	plan.changed = !sameAsReleaseBranch(ctx, tmp, entriesBody, plan.desiredFiles, cfg.ChangelogPath)
	return plan, nil
}

// sameAsReleaseBranch reports whether entriesBody and desiredFiles already
// match the release branch's current tip. It fetches the branch into
// compareRef to read it; a branch that does not exist remotely yet, or any
// read failure, is reported as not matching.
func sameAsReleaseBranch(ctx context.Context, tmp, entriesBody string, desiredFiles map[string]string, changelogPath string) bool {
	pushURL, err := gitOutput(ctx, tmp, "remote", "get-url", "--push", "origin")
	if err != nil {
		return false
	}
	if err := gitRun(ctx, tmp, "fetch", "--quiet", pushURL, "+refs/heads/"+releasepr.Branch+":"+compareRef); err != nil {
		return false
	}

	changelogContent, ok := gitShowFile(ctx, tmp, compareRef, changelogPath)
	if !ok {
		return false
	}
	existingBody, ok := changelog.TopSection(changelogContent)
	if !ok || existingBody != entriesBody {
		return false
	}

	for path, want := range desiredFiles {
		got, ok := gitShowFile(ctx, tmp, compareRef, path)
		if !ok || got != want {
			return false
		}
	}
	return true
}

// write commits the changelog update and the bumped files onto
// releasepr.Branch and pushes it, through the temporary worktree planned
// in p. A fresh branch, with no existing pull request to lease against, is
// pushed without force; otherwise the lease is the existing pull request's
// head SHA from the forge, not a locally fetched ref, so a concurrent
// fetch can never defeat it.
func (p *releasePlan) write(ctx context.Context, existing *forge.PullRequest, cfg config.Config, section, commitMessage string) error {
	if err := gitRun(ctx, p.tmp, "checkout", "--quiet", "-B", releasepr.Branch); err != nil {
		return fmt.Errorf("release-me: checkout %s: %w", releasepr.Branch, err)
	}

	changelogPath := filepath.Join(p.tmp, cfg.ChangelogPath)
	if err := os.WriteFile(changelogPath, []byte(changelog.Prepend(p.baselineChangelog, section)), 0o644); err != nil {
		return fmt.Errorf("release-me: write %s: %w", cfg.ChangelogPath, err)
	}
	for path, content := range p.desiredFiles {
		if err := os.WriteFile(filepath.Join(p.tmp, path), []byte(content), 0o644); err != nil {
			return fmt.Errorf("release-me: write %s: %w", path, err)
		}
	}

	if err := gitRun(ctx, p.tmp, "add", "-A"); err != nil {
		return fmt.Errorf("release-me: stage release branch: %w", err)
	}
	if err := gitRun(ctx, p.tmp, "commit", "--quiet", "-m", commitMessage); err != nil {
		return fmt.Errorf("release-me: commit release branch: %w", err)
	}

	pushArgs := []string{"push", "--quiet", "origin", "HEAD:refs/heads/" + releasepr.Branch}
	if existing != nil && existing.HeadSHA != "" {
		pushArgs = []string{
			"push", "--force-with-lease=refs/heads/" + releasepr.Branch + ":" + existing.HeadSHA,
			"--quiet", "origin", "HEAD:refs/heads/" + releasepr.Branch,
		}
	}
	if err := gitRun(ctx, p.tmp, pushArgs...); err != nil {
		return fmt.Errorf("release-me: push %s: %w", releasepr.Branch, err)
	}
	return nil
}

// cleanup removes the temporary worktree. It is safe to call more than
// once.
func (p *releasePlan) cleanup() {
	if p.cleanedUp {
		return
	}
	p.cleanedUp = true
	_ = gitRun(context.Background(), p.dir, "worktree", "remove", "--force", p.tmp)
	_ = os.RemoveAll(p.tmp)
}

// gitRun runs git in dir, surfacing stderr on failure.
func gitRun(ctx context.Context, dir string, args ...string) error {
	_, err := gitOutput(ctx, dir, args...)
	return err
}

// gitOutput runs git in dir and returns its trimmed stdout, surfacing
// stderr on failure. Use gitShowFile, not this, to read file content: a
// trimmed read would silently drop a real trailing newline.
func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// gitShowFile returns the exact, untrimmed content of path at ref in dir.
// ok is false for any failure, a missing ref or a missing path alike: both
// are the ordinary shape of "nothing to compare against yet".
func gitShowFile(ctx context.Context, dir, ref, path string) (content string, ok bool) {
	cmd := exec.CommandContext(ctx, "git", "show", ref+":"+path)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return stdout.String(), true
}
