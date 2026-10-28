// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package conventional_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/conventional"
)

func TestParseReadsTypeScopeAndDescription(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		want conventional.Message
	}{
		{
			name: "parses a type and description without a scope",
			text: "fix: stop the panic on empty history",
			want: conventional.Message{Type: "fix", Description: "stop the panic on empty history"},
		},
		{
			name: "parses a type, scope and description",
			text: "feat(cli): add the next command",
			want: conventional.Message{Type: "feat", Scope: "cli", Description: "add the next command"},
		},
		{
			name: "marks a bang as breaking",
			text: "feat(api)!: drop the v1 endpoints",
			want: conventional.Message{Type: "feat", Scope: "api", Description: "drop the v1 endpoints", Breaking: true},
		},
		{
			name: "marks a breaking change footer as breaking",
			text: "feat: rename the config key\n\nBREAKING CHANGE: the old key is gone",
			want: conventional.Message{Type: "feat", Description: "rename the config key", Breaking: true},
		},
		{
			name: "marks a breaking dash change footer as breaking",
			text: "fix: rename the config key\n\nBREAKING-CHANGE: the old key is gone",
			want: conventional.Message{Type: "fix", Description: "rename the config key", Breaking: true},
		},
		{
			name: "ignores a breaking footer that is not at the start of a line",
			text: "fix: mention breaking change in passing\n\nsee BREAKING CHANGE: nope",
			want: conventional.Message{Type: "fix", Description: "mention breaking change in passing"},
		},
		{
			name: "falls back when the text has no conventional header",
			text: "quick hack before the demo",
			want: conventional.Message{Type: "other", Description: "quick hack before the demo"},
		},
		{
			name: "falls back when the type has no colon",
			text: "feat add the next command",
			want: conventional.Message{Type: "other", Description: "feat add the next command"},
		},
		{
			name: "falls back when the type starts with an uppercase letter",
			text: "Feat: uppercase type",
			want: conventional.Message{Type: "other", Description: "Feat: uppercase type"},
		},
		{
			name: "falls back for an unconventional all caps prefix",
			text: "WIP: half a feature",
			want: conventional.Message{Type: "other", Description: "WIP: half a feature"},
		},
		{
			name: "keeps a multi word scope",
			text: "fix(release pr): keep the branch name stable",
			want: conventional.Message{Type: "fix", Scope: "release pr", Description: "keep the branch name stable"},
		},
		{
			name: "trims surrounding space from the first line",
			text: "  feat: add the next command  \nbody",
			want: conventional.Message{Type: "feat", Description: "add the next command"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := conventional.Parse(tc.text, conventional.FallbackType)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Parse(%q) = %+v, want %+v", tc.text, got, tc.want)
			}
		})
	}
}

func TestParseUsesTheGivenFallbackType(t *testing.T) {
	t.Parallel()
	got := conventional.Parse("quick hack before the demo", "misc")
	want := conventional.Message{Type: "misc", Description: "quick hack before the demo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse() = %+v, want %+v", got, want)
	}
}

func TestParseReadsAReleaseAsFooter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		want string
	}{
		{
			name: "reads the footer of a commit forcing a release",
			text: "chore: release 2.0.0\n\nRelease-As: 2.0.0",
			want: "2.0.0",
		},
		{
			name: "reads the key in any case",
			text: "chore: release 2.0.0\n\nrelease-as: 2.0.0",
			want: "2.0.0",
		},
		{
			name: "keeps a v prefix as written",
			text: "feat: add the next command\n\nRELEASE-AS: v1.4.0",
			want: "v1.4.0",
		},
		{
			name: "reads the footer below other trailers of a squash merge",
			text: "feat(cli): add the next command (#42)\n\n* feat: add the flag\n\n" +
				"Release-As: 1.0.0\nCo-authored-by: Jane Doe <jane@example.com>",
			want: "1.0.0",
		},
		{
			name: "takes the last of two footers",
			text: "chore: release\n\nRelease-As: 1.5.0\nRelease-As: 1.6.0",
			want: "1.6.0",
		},
		{
			name: "ignores the footer that is not at the start of a line",
			text: "docs: explain the footer\n\nwrite Release-As: 1.0.0 to force a version",
			want: "",
		},
		{
			name: "ignores the header line",
			text: "Release-As: 1.0.0",
			want: "",
		},
		{
			name: "ignores an empty value",
			text: "chore: release\n\nRelease-As:",
			want: "",
		},
		{
			name: "ignores the footer inside a fenced example in a pull request body",
			text: "docs(readme): document the Release-As footer\n" +
				"Add a footer example:\n\n```text\nchore: release 2.0.0\n\nRelease-As: 2.0.0\n```",
			want: "",
		},
		{
			name: "ignores the footer in a fenced example even when the fence closes the body",
			text: "docs: show the footer\n\n```text\nRelease-As: 2.0.0\n```",
			want: "",
		},
		{
			name: "ignores the footer inside an HTML comment",
			text: "feat: add the next command\n\n<!-- Release-As: 2.0.0 -->",
			want: "",
		},
		{
			name: "ignores the footer in a quote",
			text: "docs: quote the footer\n\n> Release-As: 2.0.0",
			want: "",
		},
		{
			name: "ignores the footer in a list item",
			text: "docs: list the footers\n\n- Release-As: 2.0.0 is the footer",
			want: "",
		},
		{
			name: "ignores the footer outside the last paragraph",
			text: "chore: release\n\nRelease-As: 2.0.0\n\nThis forces the next major.",
			want: "",
		},
		{
			name: "ignores the footer in a last paragraph that also holds prose",
			text: "chore: release\n\nForce the next major.\nRelease-As: 2.0.0",
			want: "",
		},
		{
			name: "reads the footer from a pull request body made of the trailer block alone",
			text: "chore: release 2.0.0\nRelease-As: 2.0.0\r\n",
			want: "2.0.0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := conventional.Parse(tc.text, conventional.FallbackType).ReleaseAs
			if got != tc.want {
				t.Fatalf("Parse(%q).ReleaseAs = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestParseReadsReleaseComponentFooters(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "reads the footer of a docs only commit",
			text: "feat(duxt): document the new layout prop\n\nRelease-Component: duxt",
			want: []string{"duxt"},
		},
		{
			name: "reads the key in any case",
			text: "fix: retag the broken build\n\nrelease-component: duxt",
			want: []string{"duxt"},
		},
		{
			name: "reads a repeated footer in the order written",
			text: "fix: share the date helper\n\nRelease-Component: duxt\nRelease-Component: api",
			want: []string{"duxt", "api"},
		},
		{
			name: "splits a comma separated value",
			text: "fix: share the date helper\n\nRelease-Component: duxt, api",
			want: []string{"duxt", "api"},
		},
		{
			name: "names a component once when two footers repeat it",
			text: "fix: share the date helper\n\nRelease-Component: duxt,api\nRelease-Component: duxt",
			want: []string{"duxt", "api"},
		},
		{
			name: "reads the footer below other trailers of a squash merge",
			text: "feat(cli): add the next command (#42)\n\n* feat: add the flag\n\n" +
				"Release-Component: cli\nCo-authored-by: Jane Doe <jane@example.com>",
			want: []string{"cli"},
		},
		{
			name: "ignores the footer that is not at the start of a line",
			text: "docs: explain the footer\n\nwrite Release-Component: duxt to attribute a commit",
			want: nil,
		},
		{
			name: "ignores the header line",
			text: "Release-Component: duxt",
			want: nil,
		},
		{
			name: "ignores an empty value and empty list items",
			text: "chore: retag\n\nRelease-Component:\nRelease-Component: , duxt,",
			want: []string{"duxt"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := conventional.Parse(tc.text, conventional.FallbackType).Components
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Parse(%q).Components = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestTrailersReadsTheTrailerBlock(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		want []conventional.Trailer
	}{
		{
			name: "reads every trailer of the last paragraph in order",
			text: "feat(cli): add the next command (#42)\n\nAdds the command.\n\n" +
				"Release-As: 1.0.0\nCo-authored-by: Jane Doe <jane@example.com>\n" +
				"BREAKING CHANGE: next replaces version\nRelease-As: 1.1.0\n",
			want: []conventional.Trailer{
				{Key: "Release-As", Value: "1.0.0"},
				{Key: "Co-authored-by", Value: "Jane Doe <jane@example.com>"},
				{Key: "BREAKING CHANGE", Value: "next replaces version"},
				{Key: "Release-As", Value: "1.1.0"},
			},
		},
		{
			name: "keeps the key as written",
			text: "chore: release\n\nrelease-AS: 2.0.0",
			want: []conventional.Trailer{{Key: "release-AS", Value: "2.0.0"}},
		},
		{
			name: "reads nothing from a subject alone",
			text: "Release-As: 1.0.0",
		},
		{
			name: "reads nothing when a fence line sits in the last paragraph",
			text: "docs: show it\n\n```\nRelease-As: 2.0.0\n```",
		},
		{
			name: "reads nothing from an indented line",
			text: "docs: show it\n\n    Release-As: 2.0.0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := conventional.Trailers(tc.text)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Trailers(%q) = %+v, want %+v", tc.text, got, tc.want)
			}
		})
	}
}

func TestParseOverride(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want conventional.Override
	}{
		{
			name: "returns not present without a block",
			body: "Just a regular pull request description.",
			want: conventional.Override{},
		},
		{
			name: "reads one entry per line",
			body: "Some context.\n\n```release-me\nfeat(cli): add the next command\n```\n",
			want: conventional.Override{Present: true, Entries: []string{"feat(cli): add the next command"}},
		},
		{
			name: "reads several entries",
			body: "```release-me\nfeat: add the next command\nfix: stop the panic on empty history\n```",
			want: conventional.Override{Present: true, Entries: []string{
				"feat: add the next command",
				"fix: stop the panic on empty history",
			}},
		},
		{
			name: "drops blank lines inside the block",
			body: "```release-me\nfeat: add the next command\n\nfix: stop the panic\n```",
			want: conventional.Override{Present: true, Entries: []string{
				"feat: add the next command",
				"fix: stop the panic",
			}},
		},
		{
			name: "an empty block is present with no entries",
			body: "```release-me\n```",
			want: conventional.Override{Present: true},
		},
		{
			name: "does not match the differently named notes block",
			body: "```release-me-notes\nfree text for the release\n```",
			want: conventional.Override{},
		},
		{
			name: "does not match a stray fence inside a sentence",
			body: "Wrap it in ```release-me for emphasis, not an actual block.",
			want: conventional.Override{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := conventional.ParseOverride(tc.body)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseOverride() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
