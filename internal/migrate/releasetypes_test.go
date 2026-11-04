// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

func TestFromReleasePleaseCarriesOverTheFilesAReleaseTypeBumps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		settings     string
		repo         map[string]string
		wantFiles    []config.File
		wantWarnings []string
	}{
		{
			name:      "simple bumps version.txt",
			settings:  `"release-type": "simple"`,
			repo:      map[string]string{"web/version.txt": "2.0.0\n"},
			wantFiles: []config.File{{Path: "web/version.txt", Updater: "plain"}},
		},
		{
			name:      "simple bumps the version-file it names",
			settings:  `"release-type": "simple", "version-file": "VERSION"`,
			repo:      map[string]string{"web/VERSION": "2.0.0\n"},
			wantFiles: []config.File{{Path: "web/VERSION", Updater: "plain"}},
		},
		{
			name:     "simple warns about a version file that does not exist",
			settings: `"release-type": "simple"`,
			wantWarnings: []string{
				"package web: release-type simple: web/version.txt does not exist, left out of files",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configJSON := `{"packages": {"web": {` + tc.settings + `}}}`
			got, err := migrate.FromReleasePlease([]byte(configJSON), nil, files(tc.repo))
			if err != nil {
				t.Fatalf("FromReleasePlease() error = %v", err)
			}
			resolved := load(t, got.YAML).Resolve()
			if !reflect.DeepEqual(resolved[0].Files, tc.wantFiles) {
				t.Fatalf("Files = %+v, want %+v", resolved[0].Files, tc.wantFiles)
			}
			if !reflect.DeepEqual(got.Warnings, tc.wantWarnings) {
				t.Fatalf("Warnings = %q, want %q", got.Warnings, tc.wantWarnings)
			}
		})
	}
}
