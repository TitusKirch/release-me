// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package conventional_test

import (
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
			if got != tc.want {
				t.Fatalf("Parse(%q) = %+v, want %+v", tc.text, got, tc.want)
			}
		})
	}
}

func TestParseUsesTheGivenFallbackType(t *testing.T) {
	t.Parallel()
	got := conventional.Parse("quick hack before the demo", "misc")
	want := conventional.Message{Type: "misc", Description: "quick hack before the demo"}
	if got != want {
		t.Fatalf("Parse() = %+v, want %+v", got, want)
	}
}
