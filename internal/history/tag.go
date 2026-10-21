// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history

import (
	"context"
	"slices"
	"strings"

	"github.com/ostendorf-cloud/release-me/internal/version"
)

// LastTag finds the highest semantic version tag reachable from HEAD whose
// name starts with prefix, "v" for the single component this milestone
// supports. Pre-release tags are ignored. ok is false when no matching tag
// exists.
func LastTag(ctx context.Context, dir, prefix string) (tag string, v version.Semver, ok bool, err error) {
	out, err := run(ctx, dir, "tag", "--list", prefix+"*", "--merged", "HEAD")
	if err != nil {
		return "", version.Semver{}, false, err
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, prefix) {
			continue
		}
		// version.ParseTag only understands a "v" prefix, so the configured
		// prefix is normalized to "v" before parsing the semantic version
		// that follows it.
		candidate, candidateOK := version.ParseTag("v" + strings.TrimPrefix(line, prefix))
		if !candidateOK || candidate.IsPreRelease() {
			continue
		}
		if !ok || v.Less(candidate) {
			tag, v, ok = line, candidate, true
		}
	}
	return tag, v, ok, nil
}

// PreReleaseTag is one pre-release tag reachable from HEAD, together with
// the semantic version it names.
type PreReleaseTag struct {
	Tag     string
	Version version.Semver
}

// PreReleaseTags lists every pre-release tag reachable from HEAD whose name
// is prefix followed by base's "X.Y.Z-<identifier>" shape, for the given
// stable base version. Tags are ordered from lowest to highest semver
// precedence, so the last element is always the pre-release tagged most
// recently for base; a type change does not move it back.
func PreReleaseTags(ctx context.Context, dir, prefix string, base version.Semver) ([]PreReleaseTag, error) {
	out, err := run(ctx, dir, "tag", "--list", prefix+base.String()+"-*", "--merged", "HEAD")
	if err != nil {
		return nil, err
	}

	var tags []PreReleaseTag
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, prefix) {
			continue
		}
		candidate, ok := version.ParseTag("v" + strings.TrimPrefix(line, prefix))
		if !ok || !candidate.IsPreRelease() {
			continue
		}
		if candidate.Major != base.Major || candidate.Minor != base.Minor || candidate.Patch != base.Patch {
			continue
		}
		tags = append(tags, PreReleaseTag{Tag: line, Version: candidate})
	}

	slices.SortFunc(tags, func(a, b PreReleaseTag) int {
		switch {
		case a.Version.LessPrecedence(b.Version):
			return -1
		case b.Version.LessPrecedence(a.Version):
			return 1
		default:
			return 0
		}
	})
	return tags, nil
}
