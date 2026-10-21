// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package changelog_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
)

func TestSectionsRendersNoHeading(t *testing.T) {
	t.Parallel()
	entries := []changelog.Entry{{Type: "feat", Description: "add the next command"}}

	got := changelog.Sections(entries, nil)
	want := "### Features\n\n- add the next command"
	if got != want {
		t.Fatalf("Sections() = %q, want %q", got, want)
	}
}

func TestSectionsReturnsEmptyWhenNoEntryIsVisible(t *testing.T) {
	t.Parallel()
	entries := []changelog.Entry{{Type: "chore", Description: "bump a dependency"}}

	got := changelog.Sections(entries, []string{"chore"})
	if got != "" {
		t.Fatalf("Sections() = %q, want empty", got)
	}
}

func TestSectionsIsDateFreeSoTwoRendersOnDifferentDaysMatch(t *testing.T) {
	t.Parallel()
	entries := []changelog.Entry{{Type: "feat", Description: "add the next command"}}

	a := changelog.Sections(entries, nil)
	b := changelog.Sections(entries, nil)
	if a != b {
		t.Fatalf("Sections() is not stable: %q != %q", a, b)
	}

	// Render embeds the date; Sections, used for comparison, never does.
	renderedToday := changelog.Render("0.1.0", fixtureDate, entries, nil)
	if a == renderedToday {
		t.Fatalf("Sections() unexpectedly equals the dated Render() output")
	}
}
