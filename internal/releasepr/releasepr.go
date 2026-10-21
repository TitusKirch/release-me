// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package releasepr renders the branch name, title and body of the release
// pull request, and parses them back out of an existing one. It writes
// nothing: the command layer commits the branch and calls the forge.
package releasepr

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Branch is the name of the release pull request's branch. The single
// component this milestone supports hardcodes "main" the same way the tag
// prefix "v" does.
const Branch = "release-me/main"

// Title renders the release pull request title for version.
func Title(version string) string {
	return fmt.Sprintf("chore(main): release v%s", version)
}

// Body renders the release pull request body: the version line, the
// preserved release-me-notes block when notes is not empty, then the
// changelog for the version collapsed inside a <details> block.
func Body(version, notes, changelogSection string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Release v%s\n", version)

	if notes != "" {
		b.WriteString("\n```release-me-notes\n")
		b.WriteString(notes)
		b.WriteString("\n```\n")
	}

	b.WriteString("\n<details>\n<summary>Changelog</summary>\n\n")
	b.WriteString(strings.TrimRight(changelogSection, "\n"))
	b.WriteString("\n\n</details>\n")
	return b.String()
}

// notesBlock matches the "release-me-notes" fenced block of a release pull
// request body. Both fences are anchored to the start of their own line,
// so a nested fence inside the notes does not truncate the match early.
var notesBlock = regexp.MustCompile("(?sm)^```release-me-notes[ \t]*$\n(.*?)^```[ \t]*$")

// ExtractNotes returns the free text of the release-me-notes fenced block in
// body. ok is false when body carries no such block.
func ExtractNotes(body string) (notes string, ok bool) {
	m := notesBlock.FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	return strings.TrimRight(m[1], "\n"), true
}

// changelogDetails matches the collapsed changelog block of a release pull
// request body.
var changelogDetails = regexp.MustCompile(`(?s)<details>\s*<summary>Changelog</summary>\s*\n*(.*?)\n*</details>`)

// ExtractChangelog returns the changelog Markdown from inside the
// <details> block of a release pull request body, with the wrapper
// removed. It returns "" when body carries no such block.
func ExtractChangelog(body string) string {
	m := changelogDetails.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// headingDate matches the date in a changelog section's heading line,
// "## vX.Y.Z (YYYY-MM-DD)".
var headingDate = regexp.MustCompile(`^## v\d+\.\d+\.\d+ \((\d{4}-\d{2}-\d{2})\)`)

// ExtractDate returns the date rendered in the changelog heading inside the
// <details> block of a release pull request body. ok is false when body
// carries no such block, or its heading carries no parseable date.
func ExtractDate(body string) (date time.Time, ok bool) {
	m := headingDate.FindStringSubmatch(ExtractChangelog(body))
	if m == nil {
		return time.Time{}, false
	}
	d, err := time.Parse("2006-01-02", m[1])
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

// ReleaseBody renders the GitHub release body from a release pull request
// body: the free text of the release-me-notes block, when present, followed
// by the changelog, with the <details> wrapper removed.
func ReleaseBody(body string) string {
	var parts []string
	if notes, ok := ExtractNotes(body); ok && notes != "" {
		parts = append(parts, notes)
	}
	if changelog := ExtractChangelog(body); changelog != "" {
		parts = append(parts, changelog)
	}
	return strings.Join(parts, "\n\n")
}
