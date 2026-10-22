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

// Title renders the release pull request title for one component's
// version: "chore(<name>): release vX.Y.Z". The implicit "main" component
// a repository without "components" carries renders the same title it
// always has.
func Title(name, version string) string {
	return fmt.Sprintf("chore(%s): release v%s", name, version)
}

// GuideLabels is the label names rendered in the release pull request's
// "How this release works" guide: the tagged label and the three
// pre-release type labels. The pending label never appears there.
type GuideLabels struct {
	Tagged string
	Alpha  string
	Beta   string
	Rc     string
}

// Body renders the release pull request body: the version line, the "How
// this release works" guide, the preserved release-me-notes block when
// notes is not empty, then the changelog for the version collapsed inside
// a <details> block. tag is the tag merging this pull request creates,
// and labels names the repository's configured label values, so the
// guide always matches what merging and labelling actually do.
func Body(version, tag string, labels GuideLabels, notes, changelogSection string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Release v%s\n\n", version)
	b.WriteString(guideBlock(tag, labels))

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

// guideBlock renders the collapsed "How this release works" section: one
// line each for the merge, the notes block, the pre-release labels, the
// override block and the push behaviour.
func guideBlock(tag string, labels GuideLabels) string {
	var b strings.Builder
	b.WriteString("<details>\n<summary>How this release works</summary>\n\n")
	fmt.Fprintf(&b, "- Merging this pull request tags `%s`, creates the release and moves the label to `%s`. "+
		"With `major-tag` enabled, the moving major tag follows a stable release.\n", tag, labels.Tagged)
	b.WriteString("- Release notes: add a fenced block with the info string `release-me-notes` anywhere in this body. " +
		"It is rendered above the entries in the changelog file and in the release and survives updates.\n")
	fmt.Fprintf(&b, "- Pre-release: add the label `%s`, `%s` or `%s`. The next run turns the version into `X.Y.Z-<type>.N`. "+
		"Remove the label to go back to the stable version.\n", labels.Alpha, labels.Beta, labels.Rc)
	b.WriteString("- Hide or rewrite a change: a fenced block `release-me` in the changed pull request's body, " +
		"one conventional line per entry, an empty block hides it.\n")
	b.WriteString("- Every push to the default branch updates this pull request. " +
		"Edits to the body outside the notes block are overwritten.\n")
	b.WriteString("\n</details>\n")
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
