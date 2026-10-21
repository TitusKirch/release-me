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
// renders, "chore(<name>): release vX.Y.Z".
var titlePattern = regexp.MustCompile(`^chore\(([^)]+)\): release v(\d+\.\d+\.\d+)$`)

// VersionFromTitle extracts "X.Y.Z" from a release pull request title whose
// component name matches name. ok is false when title does not match that
// shape, or names a different component.
func VersionFromTitle(title, name string) (version string, ok bool) {
	m := titlePattern.FindStringSubmatch(title)
	if m == nil || m[1] != name {
		return "", false
	}
	return m[2], true
}

// Component names the release branch, tag prefix and display name release
// Run tags, releases and swaps the label for, and whether release-me.yaml
// declares more than one component. Multi false, the implicit "main"
// component a repository without "components" carries, keeps today's bare
// tag prefix and release name instead of the generic "<name>/v" prefix and
// "<name> vX.Y.Z" release name a configured component carries.
type Component struct {
	Name      string
	Branch    string
	TagPrefix string
	Multi     bool
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
// pull request on comp's branch, for the component comp names. dryRun
// reports what would happen and calls neither CreateTag, CreateRelease nor
// SetLabels.
func Run(
	ctx context.Context, f forge.Forge, owner, repoName string, comp Component, pendingLabel, taggedLabel string, dryRun bool, logger *slog.Logger,
) ([]Result, error) {
	prs, err := f.PullRequests(ctx, owner, repoName, comp.Branch, "closed")
	if err != nil {
		return nil, fmt.Errorf("release: component %s: list pull requests on %s: %w", comp.Name, comp.Branch, err)
	}

	var results []Result
	for _, pr := range prs {
		if !pr.Merged || !slices.Contains(pr.Labels, pendingLabel) {
			continue
		}
		logger.Debug("release pull request merged", "component", comp.Name, "number", pr.Number, "title", pr.Title)

		version, ok := VersionFromTitle(pr.Title, comp.Name)
		if !ok {
			return nil, fmt.Errorf("release: component %s: pull request #%d: title %q does not name a version", comp.Name, pr.Number, pr.Title)
		}
		tag := comp.TagPrefix + version
		name := tag
		if comp.Multi {
			name = fmt.Sprintf("%s v%s", comp.Name, version)
		}

		result, err := releaseOne(ctx, f, owner, repoName, comp.Name, pr, tag, name, version, pendingLabel, taggedLabel, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func releaseOne(
	ctx context.Context, f forge.Forge, owner, repoName, componentName string,
	pr forge.PullRequest, tag, releaseName, version, pendingLabel, taggedLabel string, dryRun bool,
) (Result, error) {
	if dryRun {
		return Result{PullRequestNumber: pr.Number, Version: version, Tag: tag, Message: fmt.Sprintf("would tag and release %s", tag)}, nil
	}

	sha, err := f.MergeCommit(ctx, owner, repoName, pr.Number)
	if err != nil {
		return Result{}, fmt.Errorf("release: component %s: pull request #%d: merge commit: %w", componentName, pr.Number, err)
	}

	alreadyTagged := false
	if err := f.CreateTag(ctx, owner, repoName, tag, sha); err != nil {
		if !errors.Is(err, forge.ErrTagExists) {
			return Result{}, fmt.Errorf("release: component %s: pull request #%d: create tag %s: %w", componentName, pr.Number, tag, err)
		}
		// The tag exists, so an earlier run got this far before failing or
		// being interrupted. Carry on: make sure the release exists and
		// the label still swaps, so the pull request does not stay
		// pending forever.
		alreadyTagged = true
	}

	body := releasepr.ReleaseBody(pr.Body)
	if err := f.CreateRelease(ctx, owner, repoName, tag, releaseName, body); err != nil && !errors.Is(err, forge.ErrReleaseExists) {
		return Result{}, fmt.Errorf("release: component %s: pull request #%d: create release %s: %w", componentName, pr.Number, tag, err)
	}

	labels, err := f.Labels(ctx, owner, repoName, pr.Number)
	if err != nil {
		return Result{}, fmt.Errorf("release: component %s: pull request #%d: labels: %w", componentName, pr.Number, err)
	}
	if err := f.SetLabels(ctx, owner, repoName, pr.Number, swapLabel(labels, pendingLabel, taggedLabel)); err != nil {
		return Result{}, fmt.Errorf("release: component %s: pull request #%d: set labels: %w", componentName, pr.Number, err)
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
