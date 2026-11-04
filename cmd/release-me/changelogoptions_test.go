// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
	"github.com/ostendorf-cloud/release-me/internal/config"
)

func TestChangelogOptionsTranslatesChangelogSections(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		hidden   []string
		sections []config.ChangelogSection
		want     changelog.Options
	}{
		{
			name:   "without changelog-sections the default sections apply",
			hidden: []string{"chore", "docs"},
			want:   changelog.Options{HiddenTypes: []string{"chore", "docs"}},
		},
		{
			name:   "a listed section shows a type hidden-types hides",
			hidden: []string{"chore", "docs"},
			sections: []config.ChangelogSection{
				{Type: "feat", Section: "Features"},
				{Type: "chore", Section: "Miscellaneous"},
			},
			want: changelog.Options{
				HiddenTypes: []string{"docs"},
				Sections: []changelog.Section{
					{Heading: "Features", Types: []string{"feat"}},
					{Heading: "Miscellaneous", Types: []string{"chore"}},
				},
			},
		},
		{
			name:   "a hidden entry hides a type hidden-types shows",
			hidden: []string{"chore"},
			sections: []config.ChangelogSection{
				{Type: "fix", Section: "Bug Fixes"},
				{Type: "perf", Section: "Performance", Hidden: true},
			},
			want: changelog.Options{
				HiddenTypes: []string{"chore", "perf"},
				Sections:    []changelog.Section{{Heading: "Bug Fixes", Types: []string{"fix"}}},
			},
		},
		{
			name: "types sharing a heading render together where it first appears",
			sections: []config.ChangelogSection{
				{Type: "feat", Section: "Changes"},
				{Type: "deps", Section: "Dependencies"},
				{Type: "fix", Section: "Changes"},
			},
			want: changelog.Options{
				Sections: []changelog.Section{
					{Heading: "Changes", Types: []string{"feat", "fix"}},
					{Heading: "Dependencies", Types: []string{"deps"}},
				},
			},
		},
		{
			name:     "an empty list keeps only Breaking Changes",
			hidden:   []string{"chore"},
			sections: []config.ChangelogSection{},
			want:     changelog.Options{HiddenTypes: []string{"chore"}, Sections: []changelog.Section{}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			comp := config.ResolvedComponent{HiddenTypes: tc.hidden, ChangelogSections: tc.sections}
			got := changelogOptions(comp)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("changelogOptions() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestChangelogOptionsLeavesTheConfiguredHiddenTypesUntouched(t *testing.T) {
	t.Parallel()
	hidden := []string{"chore", "docs"}
	comp := config.ResolvedComponent{
		HiddenTypes:       hidden,
		ChangelogSections: []config.ChangelogSection{{Type: "chore", Section: "Miscellaneous"}},
	}
	changelogOptions(comp)
	if !reflect.DeepEqual(hidden, []string{"chore", "docs"}) {
		t.Fatalf("hidden-types = %v after changelogOptions, want it unchanged", hidden)
	}
}

func TestDoPRRendersTheConfiguredChangelogSections(t *testing.T) {
	dir, origin := newPRRepo(t)
	writeConfig(t, dir, `changelog-sections:
  - type: fix
    section: Fixes
  - type: deps
    section: Dependencies
  - type: feat
    section: New Features
`)
	commitEntry(t, dir, "chore: initial commit")
	tagRepo(t, dir)
	commitEntry(t, dir, "feat: add the widget")
	commitEntry(t, dir, "deps: bump yaml.v3 to 3.0.1")
	commitEntry(t, dir, "fix: handle the empty case")

	cfg, err := config.Load(filepath.Join(dir, "release-me.yaml"))
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	f := newFake()
	if err := doPR(t.Context(), dir, cfg, "", f, discardLogger(), false, io.Discard); err != nil {
		t.Fatalf("doPR() error = %v", err)
	}

	got, ok := changelog.TopSection(showBareBranchFile(t, origin, "CHANGELOG.md"))
	if !ok {
		t.Fatal("CHANGELOG.md carries no version section")
	}
	want := "### Fixes\n\n- handle the empty case\n\n" +
		"### Dependencies\n\n- bump yaml.v3 to 3.0.1\n\n" +
		"### New Features\n\n- add the widget"
	if got != want {
		t.Fatalf("CHANGELOG.md section = %q, want %q", got, want)
	}
	if body := f.CreatedPullRequests[0].Body; !strings.Contains(body, want) {
		t.Fatalf("pull request body = %q, want it to carry %q", body, want)
	}
}
