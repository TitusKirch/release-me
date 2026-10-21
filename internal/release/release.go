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
	"strconv"

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
	// MajorTag is comp's resolved "major-tag" key: when true, a stable
	// release moves the major tag, TagPrefix plus the released major
	// number, to the release commit. A pre-release never moves it.
	MajorTag bool
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
	// MajorTag is the major tag name major-tag moved or created, empty
	// when major-tag is off for the component or the version released is
	// a pre-release.
	MajorTag string
	// MajorTagMessage reports what happened to the major tag, including
	// why it was skipped; empty when major-tag is off for the component.
	MajorTagMessage string
}

// MajorTagName returns the major tag name for tagPrefix and releasedVersion,
// an "X.Y.Z" version string without its "v" prefix, and whether major-tag
// applies to it: false for a pre-release version, such as "1.2.3-rc.1",
// which never moves the major tag.
func MajorTagName(tagPrefix, releasedVersion string) (tag string, applies bool) {
	v, ok := version.ParseTag("v" + releasedVersion)
	if !ok || v.IsPreRelease() {
		return "", false
	}
	return tagPrefix + strconv.Itoa(v.Major), true
}

// applyMajorTag creates or force-moves the major tag for comp, named by
// tagPrefix, onto sha, when majorTagEnabled and releasedVersion is not a
// pre-release. dryRun reads whether the tag exists, to report accurately
// what would happen, but writes nothing.
func applyMajorTag(
	ctx context.Context, f forge.Forge, owner, repoName, componentName, tagPrefix, releasedVersion, sha string,
	majorTagEnabled, dryRun bool,
) (tag, message string, err error) {
	if !majorTagEnabled {
		return "", "", nil
	}
	tag, applies := MajorTagName(tagPrefix, releasedVersion)
	if !applies {
		return "", "pre-release, major tag not moved", nil
	}

	exists, err := f.TagExists(ctx, owner, repoName, tag)
	if err != nil {
		return "", "", fmt.Errorf("release: component %s: major tag %s: %w", componentName, tag, err)
	}

	verb := "create"
	if exists {
		verb = "move"
	}
	if dryRun {
		return tag, fmt.Sprintf("would %s major tag %s", verb, tag), nil
	}

	if exists {
		if err := f.MoveTag(ctx, owner, repoName, tag, sha); err != nil {
			return "", "", fmt.Errorf("release: component %s: move major tag %s: %w", componentName, tag, err)
		}
		return tag, fmt.Sprintf("moved major tag %s", tag), nil
	}
	if err := f.CreateTag(ctx, owner, repoName, tag, sha); err != nil {
		return "", "", fmt.Errorf("release: component %s: create major tag %s: %w", componentName, tag, err)
	}
	return tag, fmt.Sprintf("created major tag %s", tag), nil
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

		result, err := releaseOne(ctx, f, owner, repoName, comp.Name, comp.TagPrefix, pr, tag, name, ver, labels, dryRun, comp.MajorTag)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func releaseOne(
	ctx context.Context, f forge.Forge, owner, repoName, componentName, tagPrefix string,
	pr forge.PullRequest, tag, releaseName string, ver version.Semver, labels Labels, dryRun, majorTagEnabled bool,
) (Result, error) {
	versionStr := ver.String()
	if dryRun {
		result := Result{PullRequestNumber: pr.Number, Version: versionStr, Tag: tag, Message: fmt.Sprintf("would tag and release %s", tag)}
		mtag, mmsg, err := applyMajorTag(ctx, f, owner, repoName, componentName, tagPrefix, versionStr, "", majorTagEnabled, dryRun)
		if err != nil {
			return Result{}, err
		}
		result.MajorTag, result.MajorTagMessage = mtag, mmsg
		return result, nil
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

	mtag, mmsg, err := applyMajorTag(ctx, f, owner, repoName, componentName, tagPrefix, versionStr, sha, majorTagEnabled, dryRun)
	if err != nil {
		return Result{}, err
	}

	return Result{
		PullRequestNumber: pr.Number, Version: versionStr, Tag: tag, Skipped: alreadyTagged,
		Message: message, MajorTag: mtag, MajorTagMessage: mmsg,
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
