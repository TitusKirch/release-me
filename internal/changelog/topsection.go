// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package changelog

import (
	"regexp"
	"strings"
)

// TopSection returns the body of the first version section in rendered
// CHANGELOG.md content, the text under its heading line up to the next
// version heading or the end of the file, excluding the heading itself so
// a caller can compare it without the date the heading carries. ok is
// false when content carries no version heading at all.
func TopSection(content string) (body string, ok bool) {
	rest := strings.TrimLeft(strings.TrimPrefix(content, header), "\n")
	if !strings.HasPrefix(rest, "## ") {
		return "", false
	}

	_, afterHeading, found := strings.Cut(rest, "\n")
	if !found {
		return "", true
	}
	afterHeading = strings.TrimLeft(afterHeading, "\n")
	if idx := strings.Index(afterHeading, "\n## "); idx >= 0 {
		afterHeading = afterHeading[:idx+1]
	}
	return strings.TrimRight(afterHeading, "\n"), true
}

// HeadingVersionPattern is the regexp fragment matching the version in a
// changelog section's heading line, the full semantic version shape
// Render can write: major.minor.patch, with an optional pre-release
// suffix and an optional build metadata suffix. releasepr.ExtractDate
// shares it, so the version it skips over to reach the date is read the
// same way HeadingVersion reads it.
const HeadingVersionPattern = `\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?`

// headingVersion matches the version in a changelog section's heading
// line, "## vX.Y.Z (YYYY-MM-DD)", with its optional pre-release and build
// metadata suffixes, the same shape Render writes.
var headingVersion = regexp.MustCompile(`^## v(` + HeadingVersionPattern + `) \(\d{4}-\d{2}-\d{2}\)`)

// HeadingVersion returns the version rendered in the first version
// heading of rendered CHANGELOG.md content, the same heading TopSection
// strips. ok is false when content carries no version heading, or that
// heading's version is not shaped like a semantic version, with an
// optional pre-release and build metadata suffix.
func HeadingVersion(content string) (version string, ok bool) {
	rest := strings.TrimLeft(strings.TrimPrefix(content, header), "\n")
	m := headingVersion.FindStringSubmatch(rest)
	if m == nil {
		return "", false
	}
	return m[1], true
}
