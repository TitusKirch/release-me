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
	"github.com/ostendorf-cloud/release-me/internal/version"
)

// titlePattern matches the release pull request title releasepr.Title
// renders, "chore(<name>): release vX.Y.Z" or "chore(<name>): release
// vX.Y.Z-<pre>" for a pre-release.
var titlePattern = regexp.MustCompile(`^chore\(([^)]+)\): release v(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)$`)

// VersionFromTitle extracts "X.Y.Z", or "X.Y.Z-<pre>" for a pre-release,
// from a release pull request title whose component name matches name. ok
// is false when title does not match that shape, or names a different
// component.
func VersionFromTitle(title, name string) (versionStr string, ok bool) {
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

// Labels names the labels Run reads and writes: Pending and Tagged on
// every release, plus the three pre-release type labels, so a pre-release
// pull request's type label swaps to Tagged alongside Pending, the same as
// Pending does.
type Labels struct {
	Pending string
	Tagged  string
	Alpha   string
	Beta    string
	Rc      string
}

// typeLabel returns the label naming preType, "" when preType is "" or
// names none of the three pre-release types.
func (l Labels) typeLabel(preType string) string {
	switch preType {
	case "alpha":
		return l.Alpha
	case "beta":
		return l.Beta
	case "rc":
		return l.Rc
	default:
		return ""
	}
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
	ctx context.Context, f forge.Forge, owner, repoName string, comp Component, labels Labels, dryRun bool, logger *slog.Logger,
) ([]Result, error) {
	prs, err := f.PullRequests(ctx, owner, repoName, comp.Branch, "closed")
	if err != nil {
		return nil, fmt.Errorf("release: component %s: list pull requests on %s: %w", comp.Name, comp.Branch, err)
	}

	var results []Result
	for _, pr := range prs {
		if !pr.Merged || !slices.Contains(pr.Labels, labels.Pending) {
			continue
		}
		logger.Debug("release pull request merged", "component", comp.Name, "number", pr.Number, "title", pr.Title)

		versionStr, ok := VersionFromTitle(pr.Title, comp.Name)
		if !ok {
			return nil, fmt.Errorf("release: component %s: pull request #%d: title %q does not name a version", comp.Name, pr.Number, pr.Title)
		}
		ver, ok := version.ParseTag("v" + versionStr)
		if !ok {
			return nil, fmt.Errorf("release: component %s: pull request #%d: version %q does not parse", comp.Name, pr.Number, versionStr)
		}
		tag := comp.TagPrefix + versionStr
		name := tag
		if comp.Multi {
			name = fmt.Sprintf("%s v%s", comp.Name, versionStr)
		}

		result, err := releaseOne(ctx, f, owner, repoName, comp.Name, pr, tag, name, ver, labels, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func releaseOne(
	ctx context.Context, f forge.Forge, owner, repoName, componentName string,
	pr forge.PullRequest, tag, releaseName string, ver version.Semver, labels Labels, dryRun bool,
) (Result, error) {
	versionStr := ver.String()
	if dryRun {
		return Result{PullRequestNumber: pr.Number, Version: versionStr, Tag: tag, Message: fmt.Sprintf("would tag and release %s", tag)}, nil
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
	if err := f.CreateRelease(ctx, owner, repoName, tag, releaseName, body, ver.IsPreRelease()); err != nil && !errors.Is(err, forge.ErrReleaseExists) {
		return Result{}, fmt.Errorf("release: component %s: pull request #%d: create release %s: %w", componentName, pr.Number, tag, err)
	}

	prLabels, err := f.Labels(ctx, owner, repoName, pr.Number)
	if err != nil {
		return Result{}, fmt.Errorf("release: component %s: pull request #%d: labels: %w", componentName, pr.Number, err)
	}
	remove := []string{labels.Pending}
	if preType, _, ok := ver.PreReleaseType(); ok {
		if typeLabel := labels.typeLabel(preType); typeLabel != "" {
			remove = append(remove, typeLabel)
		}
	}
	if err := f.SetLabels(ctx, owner, repoName, pr.Number, swapLabels(prLabels, remove, labels.Tagged)); err != nil {
		return Result{}, fmt.Errorf("release: component %s: pull request #%d: set labels: %w", componentName, pr.Number, err)
	}

	message := fmt.Sprintf("tagged and released %s", tag)
	if alreadyTagged {
		message = fmt.Sprintf("already tagged %s", tag)
	}
	return Result{
		PullRequestNumber: pr.Number, Version: versionStr, Tag: tag, Skipped: alreadyTagged,
		Message: message,
	}, nil
}

// swapLabels removes every label in remove from labels, when present, and
// appends add.
func swapLabels(labels, remove []string, add string) []string {
	out := make([]string, 0, len(labels)+1)
	for _, l := range labels {
		if slices.Contains(remove, l) {
			continue
		}
		out = append(out, l)
	}
	return append(out, add)
}
