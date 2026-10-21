// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package changelog renders the Markdown changelog for one version from
// classified conventional entries.
package changelog

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Entry is one changelog line.
type Entry struct {
	Type              string
	Scope             string
	Description       string
	Breaking          bool
	PullRequestNumber int
	URL               string
}

type section struct {
	heading string
	match   func(Entry) bool
}

var sections = []section{
	{heading: "Breaking Changes", match: func(e Entry) bool { return e.Breaking }},
	{heading: "Features", match: func(e Entry) bool { return !e.Breaking && e.Type == "feat" }},
	{heading: "Bug Fixes", match: func(e Entry) bool { return !e.Breaking && e.Type == "fix" }},
	{heading: "Performance", match: func(e Entry) bool { return !e.Breaking && e.Type == "perf" }},
	{heading: "Reverts", match: func(e Entry) bool { return !e.Breaking && e.Type == "revert" }},
}

// Sections renders the section bodies for one version's entries, with no
// heading, so a caller can compare them independently of the version and
// the date. It applies the same hiddenTypes and section rules as Render:
// entries whose type is in hiddenTypes produce no line, and an entry that
// belongs to none of the five sections, breaking already counted as one,
// also produces no line. Sections returns "" when no entry is visible.
func Sections(entries []Entry, hiddenTypes []string) string {
	visible := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if slices.Contains(hiddenTypes, e.Type) {
			continue
		}
		visible = append(visible, e)
	}

	var body strings.Builder
	for _, s := range sections {
		var lines []string
		for _, e := range visible {
			if !s.match(e) {
				continue
			}
			lines = append(lines, renderLine(e))
		}
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintf(&body, "### %s\n\n", s.heading)
		for _, line := range lines {
			fmt.Fprintf(&body, "%s\n", line)
		}
		body.WriteString("\n")
	}
	return strings.TrimRight(body.String(), "\n")
}

// Render renders the Markdown changelog for one version: the heading, then
// Sections. Render returns "" when Sections does, no entry visible.
func Render(version string, date time.Time, entries []Entry, hiddenTypes []string) string {
	body := Sections(entries, hiddenTypes)
	if body == "" {
		return ""
	}

	var out strings.Builder
	fmt.Fprintf(&out, "## v%s (%s)\n\n", version, date.Format("2006-01-02"))
	out.WriteString(body)
	out.WriteString("\n")
	return out.String()
}

func renderLine(e Entry) string {
	var b strings.Builder
	b.WriteString("- ")
	if e.Scope != "" {
		fmt.Fprintf(&b, "**%s:** ", e.Scope)
	}
	b.WriteString(e.Description)
	if e.PullRequestNumber > 0 && e.URL != "" {
		fmt.Fprintf(&b, " ([#%d](%s))", e.PullRequestNumber, e.URL)
	}
	return b.String()
}
