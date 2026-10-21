// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package changelog_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
)

func TestTopSection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		wantOK  bool
		want    string
	}{
		{name: "no heading at all", content: "# Changelog\n", wantOK: false},
		{name: "empty file", content: "", wantOK: false},
		{
			name:    "a single section",
			content: "# Changelog\n\n## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the thing\n",
			wantOK:  true,
			want:    "### Features\n\n- add the thing",
		},
		{
			name: "stops before the next version heading",
			content: "# Changelog\n\n## v0.2.0 (2026-02-03)\n\n### Bug Fixes\n\n- stop the panic\n\n" +
				"## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the thing\n",
			wantOK: true,
			want:   "### Bug Fixes\n\n- stop the panic",
		},
		{
			name:    "a non-section line under the heading",
			content: "# Changelog\n\n## v0.1.0 (2026-01-02)\n\n- Version bump without user facing changes.\n",
			wantOK:  true,
			want:    "- Version bump without user facing changes.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := changelog.TopSection(tc.content)
			if ok != tc.wantOK {
				t.Fatalf("TopSection() ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("TopSection() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTopSectionMatchesSectionsForARoundTrippedEntry(t *testing.T) {
	t.Parallel()
	entries := []changelog.Entry{{Type: "feat", Description: "add the next command"}}
	body := changelog.Sections(entries, nil)

	rendered := changelog.Render("0.1.0", fixtureDate, entries, nil)
	changelogFile := changelog.Prepend("", rendered)

	got, ok := changelog.TopSection(changelogFile)
	if !ok || got != body {
		t.Fatalf("TopSection() = (%q, %v), want (%q, true)", got, ok, body)
	}
}
