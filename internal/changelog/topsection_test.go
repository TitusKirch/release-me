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

func TestHeadingVersion(t *testing.T) {
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
			name:    "a stable version",
			content: "# Changelog\n\n## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the thing\n",
			wantOK:  true,
			want:    "0.1.0",
		},
		{
			name:    "a pre-release version",
			content: "# Changelog\n\n## v0.1.0-rc.2 (2026-01-02)\n\n### Features\n\n- add the thing\n",
			wantOK:  true,
			want:    "0.1.0-rc.2",
		},
		{
			name:    "a version with build metadata",
			content: "# Changelog\n\n## v1.2.3+build.5 (2026-01-02)\n\n### Features\n\n- add the thing\n",
			wantOK:  true,
			want:    "1.2.3+build.5",
		},
		{
			name:    "a pre-release version with build metadata",
			content: "# Changelog\n\n## v1.2.3-rc.1+build.5 (2026-01-02)\n\n### Features\n\n- add the thing\n",
			wantOK:  true,
			want:    "1.2.3-rc.1+build.5",
		},
		{
			name: "reads the first heading, not a later one",
			content: "# Changelog\n\n## v0.2.0 (2026-02-03)\n\n### Bug Fixes\n\n- stop the panic\n\n" +
				"## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the thing\n",
			wantOK: true,
			want:   "0.2.0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := changelog.HeadingVersion(tc.content)
			if ok != tc.wantOK {
				t.Fatalf("HeadingVersion() ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("HeadingVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHeadingVersionMatchesTheVersionRenderPrepended(t *testing.T) {
	t.Parallel()
	entries := []changelog.Entry{{Type: "feat", Description: "add the next command"}}

	rendered := changelog.Render("0.1.0-alpha.1", fixtureDate, entries, nil)
	changelogFile := changelog.Prepend("", rendered)

	got, ok := changelog.HeadingVersion(changelogFile)
	if !ok || got != "0.1.0-alpha.1" {
		t.Fatalf("HeadingVersion() = (%q, %v), want (%q, true)", got, ok, "0.1.0-alpha.1")
	}
}
