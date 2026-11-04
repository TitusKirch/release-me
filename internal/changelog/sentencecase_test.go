// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package changelog_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
)

func TestSectionsSentenceCasesTheDescriptionWhenAskedTo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		description string
		want        string
	}{
		{name: "a lower case first letter is upper cased", description: "add the next command", want: "Add the next command"},
		{name: "an upper case first letter stays", description: "Add the next command", want: "Add the next command"},
		{name: "a word with an upper case letter further in stays", description: "iOS builds sign again", want: "iOS builds sign again"},
		{name: "release-please's special word npm stays", description: "npm publish runs once", want: "npm publish runs once"},
		{name: "release-please's special word gRPC stays", description: "gRPC client retries", want: "gRPC client retries"},
		{name: "a backtick start stays", description: "`next` prints the version", want: "`next` prints the version"},
		{name: "a digit start stays", description: "2fa codes expire", want: "2fa codes expire"},
		{name: "a one word description", description: "typo", want: "Typo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entries := []changelog.Entry{{Type: "fix", Scope: "cli", Description: tc.description}}
			got := changelog.Sections(entries, changelog.Options{SentenceCase: true})
			want := "### Bug Fixes\n\n- **cli:** " + tc.want
			if got != want {
				t.Fatalf("Sections() = %q, want %q", got, want)
			}
		})
	}
}

func TestSectionsSentenceCasesSubEntriesToo(t *testing.T) {
	t.Parallel()
	entries := []changelog.Entry{{
		Type: "feat", Description: "add the widget",
		SubEntries: []changelog.Entry{{Type: "feat", Description: "wire up the flag"}},
	}}

	got := changelog.Sections(entries, changelog.Options{SentenceCase: true})
	want := "### Features\n\n- Add the widget\n  - Wire up the flag"
	if got != want {
		t.Fatalf("Sections() = %q, want %q", got, want)
	}
}

func TestSectionsKeepsTheDescriptionAsWrittenByDefault(t *testing.T) {
	t.Parallel()
	entries := []changelog.Entry{{Type: "feat", Description: "add the widget"}}

	got := changelog.Sections(entries, changelog.Options{})
	if want := "### Features\n\n- add the widget"; got != want {
		t.Fatalf("Sections() = %q, want %q", got, want)
	}
}
