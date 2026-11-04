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

func TestFromReleasePleaseTranslatesChangelogHost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		configJSON  string
		want        string
		wantWarning []string
	}{
		{
			name:       "a top-level default every package inherits",
			configJSON: `{"changelog-host": "https://github.example.com/", "packages": {"api": {"component": "api"}, "web": {"component": "web"}}}`,
			want:       "https://github.example.com",
		},
		{
			name: "packages on different hosts",
			configJSON: `{"packages": {"api": {"component": "api", "changelog-host": "https://github.example.com"},
			  "web": {"component": "web"}}}`,
			wantWarning: []string{"changelog-host: release-me links every component on one host, left out"},
		},
		{
			name:        "a host release-me refuses",
			configJSON:  `{"packages": {".": {"release-type": "go", "changelog-host": "github.example.com"}}}`,
			wantWarning: []string{"changelog-host: is not an absolute http or https address, left out"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := migrate.FromReleasePlease([]byte(tc.configJSON), nil, files(nil))
			if err != nil {
				t.Fatalf("FromReleasePlease() error = %v", err)
			}
			if cfg := load(t, got.YAML); cfg.ChangelogStyle.Host != tc.want {
				t.Fatalf("ChangelogStyle.Host = %q, want %q:\n%s", cfg.ChangelogStyle.Host, tc.want, got.YAML)
			}
			if w := keyWarnings(got.Warnings, "changelog-host"); !reflect.DeepEqual(w, tc.wantWarning) {
				t.Fatalf("warnings = %q, want %q", w, tc.wantWarning)
			}
		})
	}
}

func TestFromReleasePleaseExplainsWhyDateFormatIsLeftOut(t *testing.T) {
	t.Parallel()
	configJSON := `{"date-format": "%d.%m.%Y", "packages": {".": {"release-type": "go", "date-format": "%Y"}}}`
	got, err := migrate.FromReleasePlease([]byte(configJSON), nil, files(nil))
	if err != nil {
		t.Fatalf("FromReleasePlease() error = %v", err)
	}
	reason := "release-please formats the date its generic updater writes with it, not the changelog's, " +
		"which release-me's changelog-date-format sets, left out"
	want := []string{"date-format: " + reason, "package .: date-format: " + reason}
	var w []string
	for _, warning := range got.Warnings {
		if strings.Contains(warning, "date-format") {
			w = append(w, warning)
		}
	}
	if !reflect.DeepEqual(w, want) {
		t.Fatalf("warnings = %q, want %q", w, want)
	}
	if load(t, got.YAML).ChangelogStyle.DateFormat != "" {
		t.Fatalf("YAML = %s, want no changelog-date-format", got.YAML)
	}
}

func TestFromReleasePleaseTranslatesSkipChangelog(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		configJSON string
		// wantTop is the top-level key, want each component's resolved one.
		wantTop bool
		want    []bool
	}{
		{
			name:       "a top-level default every package inherits is written once",
			configJSON: `{"skip-changelog": true, "packages": {"api": {"component": "api"}, "web": {"component": "web"}}}`,
			wantTop:    true,
			want:       []bool{true, true},
		},
		{
			name:       "a package's own stays on its component",
			configJSON: `{"packages": {"api": {"component": "api", "skip-changelog": true}, "web": {"component": "web"}}}`,
			want:       []bool{true, false},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := migrate.FromReleasePlease([]byte(tc.configJSON), nil, files(nil))
			if err != nil {
				t.Fatalf("FromReleasePlease() error = %v", err)
			}
			if w := keyWarnings(got.Warnings, "skip-changelog"); len(w) > 0 {
				t.Fatalf("warnings = %q, want none", w)
			}
			cfg := load(t, got.YAML)
			if cfg.SkipChangelog != tc.wantTop {
				t.Fatalf("top-level SkipChangelog = %v, want %v:\n%s", cfg.SkipChangelog, tc.wantTop, got.YAML)
			}
			for i, comp := range cfg.Resolve() {
				if comp.SkipChangelog != tc.want[i] {
					t.Fatalf("component %s SkipChangelog = %v, want %v:\n%s", comp.Name, comp.SkipChangelog, tc.want[i], got.YAML)
				}
			}
		})
	}
}

func TestFromReleasePleaseTranslatesChangelogType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		configJSON  string
		wantTop     string
		want        []string
		wantWarning []string
	}{
		{
			name:       "github for every package is written once",
			configJSON: `{"changelog-type": "github", "packages": {"api": {"component": "api"}, "web": {"component": "web"}}}`,
			wantTop:    "github",
			want:       []string{"github", "github"},
		},
		{
			name: "one package's github stays on its component",
			configJSON: `{"packages": {"api": {"component": "api", "changelog-type": "github"},
			  "web": {"component": "web", "changelog-type": "default"}}}`,
			want: []string{"github", ""},
		},
		{
			name:        "an unknown type",
			configJSON:  `{"packages": {".": {"release-type": "go", "changelog-type": "gitlab"}}}`,
			want:        []string{""},
			wantWarning: []string{"package .: changelog-type gitlab: release-me knows default and github, left out"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := migrate.FromReleasePlease([]byte(tc.configJSON), nil, files(nil))
			if err != nil {
				t.Fatalf("FromReleasePlease() error = %v", err)
			}
			cfg := load(t, got.YAML)
			if cfg.ChangelogType != tc.wantTop {
				t.Fatalf("top-level ChangelogType = %q, want %q:\n%s", cfg.ChangelogType, tc.wantTop, got.YAML)
			}
			for i, comp := range cfg.Resolve() {
				if comp.ChangelogType != tc.want[i] {
					t.Fatalf("component %s ChangelogType = %q, want %q:\n%s", comp.Name, comp.ChangelogType, tc.want[i], got.YAML)
				}
			}
			if w := keyWarnings(got.Warnings, "package .: changelog-type"); !reflect.DeepEqual(w, tc.wantWarning) {
				t.Fatalf("warnings = %q, want %q", w, tc.wantWarning)
			}
		})
	}
}
