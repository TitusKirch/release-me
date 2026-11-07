// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package changelog

import (
	"slices"
	"strings"
)

// sentenceCaseKeep are the words sentenceCase never upper cases although
// they start with a lower case letter, release-please's sentence-case
// plugin's default special words.
var sentenceCaseKeep = []string{"gRPC", "npm"}

// sentenceCase upper cases description's first letter, the way
// release-please's sentence-case plugin does: only an ASCII lower case
// letter at the very start of the description changes. A first word that
// is one of sentenceCaseKeep, or that already carries an upper case letter
// further in, such as "iOS" or "macOS", stays as written, and so does a
// description starting with anything else, a backtick or a digit among
// them.
func sentenceCase(description string) string {
	if description == "" || description[0] < 'a' || description[0] > 'z' {
		return description
	}
	first, _, _ := strings.Cut(description, " ")
	if slices.Contains(sentenceCaseKeep, first) || strings.ToLower(first) != first {
		return description
	}
	return strings.ToUpper(description[:1]) + description[1:]
}
