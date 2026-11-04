// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

func TestFromReleasePleaseTranslatesChangelogSections(t *testing.T) {
	t.Parallel()
	shared := []config.ChangelogSection{
		{Type: "feat", Section: "Features"},
		{Type: "deps", Section: "Dependencies"},
		{Type: "chore", Section: "Miscellaneous Chores", Hidden: true},
	}
	cases := []struct {
		name       string
		configJSON string
		// want is each resolved component's sections, in package order.
		want [][]config.ChangelogSection
		// wantTopLevel is true when the list belongs at the top level.
		wantTopLevel bool
	}{
		{
			name: "a top-level default every package inherits is written once",
			configJSON: `{
			  "changelog-sections": [
			    {"type": "feat", "section": "Features"},
			    {"type": "deps", "section": "Dependencies"},
			    {"type": "chore", "section": "Miscellaneous Chores", "hidden": true}
			  ],
			  "packages": {"api": {"component": "api", "release-type": "go"}, "web": {"component": "web", "release-type": "go"}}
			}`,
			want:         [][]config.ChangelogSection{shared, shared},
			wantTopLevel: true,
		},
		{
			name: "a package's own list stays on its component",
			configJSON: `{
			  "packages": {
			    "api": {"component": "api", "release-type": "go", "changelog-sections": [{"type": "fix", "section": "Fixes"}]},
			    "web": {"component": "web", "release-type": "go"}
			  }
			}`,
			want: [][]config.ChangelogSection{{{Type: "fix", Section: "Fixes"}}, nil},
		},
		{
			name: "the root package carries it at the top level",
			configJSON: `{
			  "packages": {".": {"release-type": "go", "changelog-sections": [{"type": "fix", "section": "Fixes"}]}}
			}`,
			want:         [][]config.ChangelogSection{{{Type: "fix", Section: "Fixes"}}},
			wantTopLevel: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := migrate.FromReleasePlease([]byte(tc.configJSON), nil, files(nil))
			if err != nil {
				t.Fatalf("FromReleasePlease() error = %v", err)
			}
			for _, w := range got.Warnings {
				if strings.Contains(w, "changelog-sections") {
					t.Fatalf("Warnings = %q, want none about changelog-sections", got.Warnings)
				}
			}
			cfg := load(t, got.YAML)
			if (cfg.ChangelogSections != nil) != tc.wantTopLevel {
				t.Fatalf("top-level ChangelogSections = %+v, want top level %v:\n%s", cfg.ChangelogSections, tc.wantTopLevel, got.YAML)
			}
			resolved := cfg.Resolve()
			for i, want := range tc.want {
				if !reflect.DeepEqual(resolved[i].ChangelogSections, want) {
					t.Fatalf("component %s ChangelogSections = %+v, want %+v:\n%s", resolved[i].Name, resolved[i].ChangelogSections, want, got.YAML)
				}
			}
		})
	}
}

func TestFromReleasePleaseTranslatesTheSentenceCasePlugin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		plugins     string
		wantWarning string
	}{
		{name: "named by its type alone", plugins: `["sentence-case"]`},
		{name: "as an object", plugins: `[{"type": "sentence-case"}]`},
		{
			name:    "with special words release-me cannot carry",
			plugins: `[{"type": "sentence-case", "specialWords": ["gRPC", "npm", "macOS"]}]`,
			wantWarning: "plugins: sentence-case: specialWords: release-me keeps gRPC, npm and any first word " +
				"already holding an upper case letter as written, gRPC, npm, macOS left out",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configJSON := `{"plugins": ` + tc.plugins + `, "packages": {".": {"release-type": "go"}}}`
			got, err := migrate.FromReleasePlease([]byte(configJSON), nil, files(nil))
			if err != nil {
				t.Fatalf("FromReleasePlease() error = %v", err)
			}
			if !load(t, got.YAML).ChangelogStyle.SentenceCase {
				t.Fatalf("ChangelogStyle.SentenceCase = false, want true:\n%s", got.YAML)
			}
			var pluginWarnings []string
			for _, w := range got.Warnings {
				if strings.HasPrefix(w, "plugins:") {
					pluginWarnings = append(pluginWarnings, w)
				}
			}
			var want []string
			if tc.wantWarning != "" {
				want = []string{tc.wantWarning}
			}
			if !reflect.DeepEqual(pluginWarnings, want) {
				t.Fatalf("plugin warnings = %q, want %q", pluginWarnings, want)
			}
		})
	}
}

func keyWarnings(warnings []string, key string) []string {
	var out []string
	for _, w := range warnings {
		if strings.HasPrefix(w, key) {
			out = append(out, w)
		}
	}
	return out
}

func TestFromReleasePleaseTranslatesIncludeCommitAuthors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		configJSON  string
		want        bool
		wantWarning []string
	}{
		{
			name:       "a top-level default every package inherits",
			configJSON: `{"include-commit-authors": true, "packages": {"api": {"component": "api"}, "web": {"component": "web"}}}`,
			want:       true,
		},
		{
			name:       "the root package's own",
			configJSON: `{"packages": {".": {"release-type": "go", "include-commit-authors": true}}}`,
			want:       true,
		},
		{
			name: "only some packages",
			configJSON: `{"packages": {"api": {"component": "api", "include-commit-authors": true},
			  "web": {"component": "web"}}}`,
			wantWarning: []string{"include-commit-authors: release-me credits authors for every component or none, left out"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := migrate.FromReleasePlease([]byte(tc.configJSON), nil, files(nil))
			if err != nil {
				t.Fatalf("FromReleasePlease() error = %v", err)
			}
			if cfg := load(t, got.YAML); cfg.ChangelogStyle.Authors != tc.want {
				t.Fatalf("ChangelogStyle.Authors = %v, want %v:\n%s", cfg.ChangelogStyle.Authors, tc.want, got.YAML)
			}
			if w := keyWarnings(got.Warnings, "include-commit-authors"); !reflect.DeepEqual(w, tc.wantWarning) {
				t.Fatalf("warnings = %q, want %q", w, tc.wantWarning)
			}
		})
	}
}

func TestFromReleasePleaseKeepsReleasePleasesCommitAndIssueLinks(t *testing.T) {
	t.Parallel()
	got, err := migrate.FromReleasePlease([]byte(`{"packages": {".": {"release-type": "go"}}}`), nil, files(nil))
	if err != nil {
		t.Fatalf("FromReleasePlease() error = %v", err)
	}
	style := load(t, got.YAML).ChangelogStyle
	if !style.CommitLinks || !style.IssueReferences {
		t.Fatalf("ChangelogStyle = %+v, want commit links and issue references on, "+
			"the shape release-please's default changelog has:\n%s", style, got.YAML)
	}
}
