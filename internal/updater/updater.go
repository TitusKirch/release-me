// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package updater returns the updated content of a release-managed file. It
// never writes to disk; the command layer does, in a temporary checkout.
package updater

import (
	"fmt"
	"regexp"
	"strings"
)

// Marker is the name of the default updater: it replaces the version on
// every line of a file that ends in a release-me marker comment.
const Marker = "marker"

// Updaters maps a configured updater name to its implementation.
var Updaters = map[string]func(content, version string) (string, error){
	Marker: UpdateMarker,
}

// markerLine splits a line into the text before a trailing release-me
// marker comment and the comment itself, kept verbatim.
var markerLine = regexp.MustCompile(`^(.*?)((?:#|//)\s*release-me:\s*version\s*)$`)

// versionLiteral matches a dotted-number version such as "1.2.3".
var versionLiteral = regexp.MustCompile(`\d+\.\d+\.\d+`)

// UpdateMarker replaces the version on every line of content that ends in
// "# release-me: version" or "// release-me: version" with version. The
// version replaced is the first dotted-number literal found before the
// marker comment on that line; everything else on the line, including the
// comment itself, is kept unchanged.
//
// An error is returned when content has no line ending in a marker comment,
// or when a marked line carries no version literal to replace.
func UpdateMarker(content, version string) (string, error) {
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		m := markerLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		prefix, suffix := m[1], m[2]
		loc := versionLiteral.FindStringIndex(prefix)
		if loc == nil {
			return "", fmt.Errorf("updater: marker: line %d has no version before the marker: %q", i+1, line)
		}
		lines[i] = prefix[:loc[0]] + version + prefix[loc[1]:] + suffix
		found = true
	}
	if !found {
		return "", fmt.Errorf("updater: marker: no line ends in a release-me version marker")
	}
	return strings.Join(lines, "\n"), nil
}
