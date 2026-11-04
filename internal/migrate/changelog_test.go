// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

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

func keyWarnings(warnings []string, key string) []string {
	var out []string
	for _, w := range warnings {
		if strings.HasPrefix(w, key) {
			out = append(out, w)
		}
	}
	return out
}
