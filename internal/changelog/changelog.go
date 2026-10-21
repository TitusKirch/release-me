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

// Render renders the Markdown changelog for one version.
//
// Entries whose type is in hiddenTypes produce no line. An entry that
// belongs to none of the five sections, breaking already counted as one,
// also produces no line. Render returns "" when no entry is visible.
func Render(version string, date time.Time, entries []Entry, hiddenTypes []string) string {
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
	if body.Len() == 0 {
		return ""
	}

	var out strings.Builder
	fmt.Fprintf(&out, "## v%s (%s)\n\n", version, date.Format("2006-01-02"))
	out.WriteString(strings.TrimRight(body.String(), "\n"))
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
