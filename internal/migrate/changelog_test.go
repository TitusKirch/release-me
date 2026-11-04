// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

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

func keyWarnings(warnings []string, key string) []string {
	var out []string
	for _, w := range warnings {
		if strings.HasPrefix(w, key) {
			out = append(out, w)
		}
	}
	return out
}
