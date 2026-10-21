// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history

import (
	"context"
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
