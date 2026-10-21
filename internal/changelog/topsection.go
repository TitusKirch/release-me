// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package changelog

import "strings"

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
