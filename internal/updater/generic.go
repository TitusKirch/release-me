// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"fmt"
	"regexp"
	"strings"
)

// The markers release-please's generic updater reads: an inline one bumps
// its own line, a block start bumps every line up to and including the
// line carrying the block end. The scope names which part of the version a
// line receives.
var (
	genericInline     = regexp.MustCompile(`x-release-please-(major|minor|patch|version-date|version|date)`)
	genericBlockStart = regexp.MustCompile(`x-release-please-start-(major|minor|patch|version-date|version|date)`)
	genericBlockEnd   = regexp.MustCompile(`x-release-please-end`)
)

// The values a scope replaces, the same expressions release-please uses:
// the first full version on the line for "version", the first number for
// "major", "minor" and "patch".
var (
	genericVersion = regexp.MustCompile(`\d+\.\d+\.\d+(?:-[\w.]+)?(?:\+[-\w.]+)?`)
	genericMajor   = regexp.MustCompile(`\d+\b`)
	genericSingle  = regexp.MustCompile(`\b\d+\b`)
	genericCore    = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)
)

// UpdateGeneric bumps the lines release-please's generic version markers
// mark: a line carrying "x-release-please-version", "-major", "-minor" or
// "-patch", and every line of a block opened by
// "x-release-please-start-version" (or "-major", "-minor", "-patch") and
// closed by the line carrying "x-release-please-end", that closing line
// included, as release-please reads a block. A "version" scope replaces the
// first full version on the line, the others the first number with the
// version's major, minor or patch number. The markers sit in whatever
// comment the file's language has, so files release-please bumped through
// them need no change. Everything else is kept byte for byte.
//
// The "date" and "version-date" scopes are refused: they write the release
// date, which an updater, given only the version, does not know. An error
// is also returned when content carries no marker at all, or a block that
// is never closed.
func UpdateGeneric(content, version string) (string, error) {
	core := genericCore.FindStringSubmatch(version)
	if core == nil {
		return "", fmt.Errorf("updater: generic: %q is not a version", version)
	}
	parts := map[string]string{"major": core[1], "minor": core[2], "patch": core[3]}

	replace := func(line, scope string, n int) (string, error) {
		switch scope {
		case "version":
			if loc := genericVersion.FindStringIndex(line); loc != nil {
				return line[:loc[0]] + version + line[loc[1]:], nil
			}
		case "major":
			if loc := genericMajor.FindStringIndex(line); loc != nil {
				return line[:loc[0]] + parts[scope] + line[loc[1]:], nil
			}
		case "minor", "patch":
			if loc := genericSingle.FindStringIndex(line); loc != nil {
				return line[:loc[0]] + parts[scope] + line[loc[1]:], nil
			}
		default:
			return "", fmt.Errorf("updater: generic: line %d: x-release-please-%s writes the release date, "+
				"which release-me's generic updater does not support", n, scope)
		}
		return line, nil
	}

	lines := strings.Split(content, "\n")
	found := false
	block, blockLine := "", 0
	for i, line := range lines {
		var err error
		switch {
		case genericInline.MatchString(line):
			found = true
			lines[i], err = replace(line, genericInline.FindStringSubmatch(line)[1], i+1)
		case block != "":
			lines[i], err = replace(line, block, i+1)
			if genericBlockEnd.MatchString(line) {
				block = ""
			}
		case genericBlockStart.MatchString(line):
			found = true
			block, blockLine = genericBlockStart.FindStringSubmatch(line)[1], i+1
			if block == "date" || block == "version-date" {
				_, err = replace(line, block, i+1)
			}
		}
		if err != nil {
			return "", err
		}
	}
	if block != "" {
		return "", fmt.Errorf("updater: generic: line %d: the x-release-please-start-%s block is never closed "+
			"by x-release-please-end", blockLine, block)
	}
	if !found {
		return "", fmt.Errorf("updater: generic: no line carries an x-release-please version marker")
	}
	return strings.Join(lines, "\n"), nil
}
