// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"unicode/utf8"

	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/releasepr"
)

// renderTitle renders a release pull request title with the first of
// patterns, the configured one, and refuses it, before anything is pushed or
// written, when it does not read back as v or runs past
// forge.MaxTitleLength: release reads the version back from the title after
// the merge, so a title the forge rejects or truncates, or one the pattern
// parses differently, would leave the merged release untagged. who names the
// component, locked family or shared release pull request in an error.
func renderTitle(patterns []releasepr.TitlePattern, v releasepr.TitleValues, who string) (string, error) {
	title, err := patterns[0].Render(v)
	if err != nil {
		return "", fmt.Errorf("release-me: %s: %w", who, err)
	}
	if n := utf8.RuneCountInString(title); n > forge.MaxTitleLength {
		return "", fmt.Errorf(
			"release-me: %s: title %q is %d characters, past the forge's limit of %d: "+
				"release reads the version back from the title, so shorten the title pattern",
			who, title, n, forge.MaxTitleLength,
		)
	}
	return title, nil
}
