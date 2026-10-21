// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package release tags, releases and swaps the label of merged release pull
// requests. It calls the forge directly: tagging and releasing need no
// local git write, only the command layer's git worktree for the pr command
// does.
package release

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"

	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/releasepr"
)

// titlePattern matches the release pull request title releasepr.Title
// renders, "chore(main): release vX.Y.Z".
var titlePattern = regexp.MustCompile(`^chore\(main\): release v(\d+\.\d+\.\d+)$`)

// VersionFromTitle extracts "X.Y.Z" from a release pull request title. ok
// is false when title does not match that shape.
func VersionFromTitle(title string) (version string, ok bool) {
	m := titlePattern.FindStringSubmatch(title)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// Result is the outcome for one merged release pull request.
type Result struct {
	PullRequestNumber int
	Version           string
	Tag               string
	Message           string
	Skipped           bool
}

// Run tags, releases and swaps the label of every merged, pending-labeled
// pull request on branch. dryRun reports what would happen and calls
// neither CreateTag, CreateRelease nor SetLabels.
func Run(
	ctx context.Context, f forge.Forge, owner, repoName, branch, pendingLabel, taggedLabel string, dryRun bool, logger *slog.Logger,
) ([]Result, error) {
	prs, err := f.PullRequests(ctx, owner, repoName, branch, "closed")
	if err != nil {
		return nil, fmt.Errorf("release: list pull requests on %s: %w", branch, err)
	}

	var results []Result
	for _, pr := range prs {
		if !pr.Merged || !slices.Contains(pr.Labels, pendingLabel) {
			continue
		}
		logger.Debug("release pull request merged", "number", pr.Number, "title", pr.Title)

		version, ok := VersionFromTitle(pr.Title)
		if !ok {
			return nil, fmt.Errorf("release: pull request #%d: title %q does not name a version", pr.Number, pr.Title)
		}
		tag := "v" + version

		result, err := releaseOne(ctx, f, owner, repoName, pr, tag, version, pendingLabel, taggedLabel, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func releaseOne(
	ctx context.Context, f forge.Forge, owner, repoName string, pr forge.PullRequest, tag, version, pendingLabel, taggedLabel string, dryRun bool,
) (Result, error) {
	if dryRun {
		return Result{PullRequestNumber: pr.Number, Version: version, Tag: tag, Message: fmt.Sprintf("would tag and release %s", tag)}, nil
	}

	sha, err := f.MergeCommit(ctx, owner, repoName, pr.Number)
	if err != nil {
		return Result{}, fmt.Errorf("release: pull request #%d: merge commit: %w", pr.Number, err)
	}

	alreadyTagged := false
	if err := f.CreateTag(ctx, owner, repoName, tag, sha); err != nil {
		if !errors.Is(err, forge.ErrTagExists) {
			return Result{}, fmt.Errorf("release: pull request #%d: create tag %s: %w", pr.Number, tag, err)
		}
		// The tag exists, so an earlier run got this far before failing or
		// being interrupted. Carry on: make sure the release exists and
		// the label still swaps, so the pull request does not stay
		// pending forever.
		alreadyTagged = true
	}

	body := releasepr.ReleaseBody(pr.Body)
	if err := f.CreateRelease(ctx, owner, repoName, tag, tag, body); err != nil && !errors.Is(err, forge.ErrReleaseExists) {
		return Result{}, fmt.Errorf("release: pull request #%d: create release %s: %w", pr.Number, tag, err)
	}

	labels, err := f.Labels(ctx, owner, repoName, pr.Number)
	if err != nil {
		return Result{}, fmt.Errorf("release: pull request #%d: labels: %w", pr.Number, err)
	}
	if err := f.SetLabels(ctx, owner, repoName, pr.Number, swapLabel(labels, pendingLabel, taggedLabel)); err != nil {
		return Result{}, fmt.Errorf("release: pull request #%d: set labels: %w", pr.Number, err)
	}

	message := fmt.Sprintf("tagged and released %s", tag)
	if alreadyTagged {
		message = fmt.Sprintf("already tagged %s", tag)
	}
	return Result{
		PullRequestNumber: pr.Number, Version: version, Tag: tag, Skipped: alreadyTagged,
		Message: message,
	}, nil
}

// swapLabel removes remove from labels, when present, and appends add.
func swapLabel(labels []string, remove, add string) []string {
	out := make([]string, 0, len(labels)+1)
	for _, l := range labels {
		if l == remove {
			continue
		}
		out = append(out, l)
	}
	return append(out, add)
}
