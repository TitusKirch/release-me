// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package releasepr_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/releasepr"
)

func TestTitle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, version, want string
	}{
		{name: "main", version: "1.2.3", want: "chore(main): release v1.2.3"},
		{name: "frontend", version: "0.4.0", want: "chore(frontend): release v0.4.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := releasepr.Title(tc.name, tc.version); got != tc.want {
				t.Fatalf("Title() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBodyRendersTheVersionNotesAndChangelog(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		notes   string
		section string
		want    string
	}{
		{
			name:    "without notes",
			section: "## v1.2.3 (2026-01-02)\n\n### Features\n\n- add the thing\n",
			want: "Release v1.2.3\n\n" +
				"<details>\n<summary>Changelog</summary>\n\n" +
				"## v1.2.3 (2026-01-02)\n\n### Features\n\n- add the thing\n\n</details>\n",
		},
		{
			name:    "with notes",
			notes:   "Upgrade note: run the migration first.",
			section: "## v1.2.3 (2026-01-02)\n\n### Features\n\n- add the thing\n",
			want: "Release v1.2.3\n\n" +
				"```release-me-notes\nUpgrade note: run the migration first.\n```\n\n" +
				"<details>\n<summary>Changelog</summary>\n\n" +
				"## v1.2.3 (2026-01-02)\n\n### Features\n\n- add the thing\n\n</details>\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := releasepr.Body("1.2.3", tc.notes, tc.section)
			if got != tc.want {
				t.Fatalf("Body() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractNotes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		body     string
		wantOK   bool
		wantText string
	}{
		{name: "no block", body: "Release v1.2.3\n", wantOK: false},
		{
			name:     "a notes block",
			body:     "Release v1.2.3\n\n```release-me-notes\nUpgrade note.\n```\n\n<details>...",
			wantOK:   true,
			wantText: "Upgrade note.",
		},
		{
			name:     "an empty notes block",
			body:     "Release v1.2.3\n\n```release-me-notes\n```\n",
			wantOK:   true,
			wantText: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			text, ok := releasepr.ExtractNotes(tc.body)
			if ok != tc.wantOK {
				t.Fatalf("ExtractNotes() ok = %v, want %v", ok, tc.wantOK)
			}
			if text != tc.wantText {
				t.Fatalf("ExtractNotes() = %q, want %q", text, tc.wantText)
			}
		})
	}
}

func TestBodyAndExtractionsRoundTrip(t *testing.T) {
	t.Parallel()
	section := "## v1.2.3 (2026-01-02)\n\n### Features\n\n- add the thing\n"
	notes := "Upgrade note: run the migration first."

	body := releasepr.Body("1.2.3", notes, section)

	gotNotes, ok := releasepr.ExtractNotes(body)
	if !ok || gotNotes != notes {
		t.Fatalf("ExtractNotes() = (%q, %v), want (%q, true)", gotNotes, ok, notes)
	}
	if got := releasepr.ExtractChangelog(body); got != "## v1.2.3 (2026-01-02)\n\n### Features\n\n- add the thing" {
		t.Fatalf("ExtractChangelog() = %q", got)
	}

	want := notes + "\n\n## v1.2.3 (2026-01-02)\n\n### Features\n\n- add the thing"
	if got := releasepr.ReleaseBody(body); got != want {
		t.Fatalf("ReleaseBody() = %q, want %q", got, want)
	}
}

func TestReleaseBodyWithoutNotes(t *testing.T) {
	t.Parallel()
	section := "## v1.2.3 (2026-01-02)\n\n### Features\n\n- add the thing\n"
	body := releasepr.Body("1.2.3", "", section)

	want := "## v1.2.3 (2026-01-02)\n\n### Features\n\n- add the thing"
	if got := releasepr.ReleaseBody(body); got != want {
		t.Fatalf("ReleaseBody() = %q, want %q", got, want)
	}
}

func TestExtractNotesDoesNotStopAtTripleBackticksMidLine(t *testing.T) {
	t.Parallel()
	// The closing fence must sit alone on its own line, so a mid-sentence
	// "```" no longer ends the match early the way an unanchored pattern
	// would. A standalone nested fence is still ambiguous with the real
	// closing fence; that is a harder problem than this anchoring fixes.
	body := "Release v1.2.3\n\n```release-me-notes\n" +
		"Wrap code in ``` for emphasis, like this.\n" +
		"```\n\n<details>...</details>\n"

	got, ok := releasepr.ExtractNotes(body)
	if !ok {
		t.Fatal("ExtractNotes() ok = false, want true")
	}
	want := "Wrap code in ``` for emphasis, like this."
	if got != want {
		t.Fatalf("ExtractNotes() = %q, want %q", got, want)
	}
}

func TestExtractDate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		body   string
		wantOK bool
		want   string
	}{
		{name: "no details block", body: "Release v1.2.3\n", wantOK: false},
		{
			name:   "reads the heading date",
			body:   releasepr.Body("1.2.3", "", "## v1.2.3 (2026-03-14)\n\n### Features\n\n- add the thing\n"),
			wantOK: true,
			want:   "2026-03-14",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			date, ok := releasepr.ExtractDate(tc.body)
			if ok != tc.wantOK {
				t.Fatalf("ExtractDate() ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && date.Format("2006-01-02") != tc.want {
				t.Fatalf("ExtractDate() = %v, want %v", date.Format("2006-01-02"), tc.want)
			}
		})
	}
}
