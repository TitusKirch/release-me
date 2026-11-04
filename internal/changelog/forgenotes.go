// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package changelog

import (
	"regexp"
	"strings"
)

// forgeHeading matches an ATX heading line of forge-generated notes: its
// hashes and its text.
var forgeHeading = regexp.MustCompile(`^(#{1,6})[ \t]+(.*?)[ \t#]*$`)

// ForgeNotes fits release notes a forge generated, such as GitHub's
// "## What's Changed" list, under a version heading in place of Sections:
// every heading of level one to three becomes a level three one, the level
// Sections renders, and every deeper heading a bold line. A "## " heading
// left in would read as the next version's heading, ending the section
// TopSection and ParseHeading read back, and a "#### " heading as a
// component of a shared release pull request. Trailing whitespace is
// trimmed, so the result compares the way Sections' does. Inside a code
// fence only a leading "#" changes, indented by one space. "" stays "".
func ForgeNotes(notes string) string {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(notes), "\r\n", "\n"), "\n")
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			// Code keeps its text, but a line starting with "#" would
			// still read as a heading to TopSection and a shared release
			// pull request's component parser: one space keeps it inert.
			if strings.HasPrefix(line, "#") {
				lines[i] = " " + line
			}
			continue
		}
		m := forgeHeading.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if len(m[1]) <= 3 {
			lines[i] = "### " + m[2]
			continue
		}
		lines[i] = "**" + m[2] + "**"
	}
	return strings.Join(lines, "\n")
}
