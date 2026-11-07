// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

func TestFromReleasePleaseCarriesOverMarkedExtraFiles(t *testing.T) {
	t.Parallel()
	configJSON := `{"packages": {"web": {"release-type": "go", "extra-files": [
	  "internal/version/version.go",
	  "/README.md",
	  "deploy/values.yaml",
	  "manifest.json",
	  {"type": "generic", "path": "install.sh"},
	  {"type": "generic", "path": "CITATION.cff"},
	  "notes.txt",
	  "missing.go",
	  {"type": "json", "path": "packages/*/package.json", "jsonpath": "$.version", "glob": true}
	]}}}`
	read := files(map[string]string{
		"web/internal/version/version.go": "package version\n\nconst Version = \"1.4.0\" // x-release-please-version\n",
		"README.md":                       "<!-- x-release-please-start-version -->\nnotes@1.4.0\n<!-- x-release-please-end -->\n",
		"web/deploy/values.yaml":          "image:\n  tag: 1.4.0 # x-release-please-version\nversion: 1.4.0\n",
		"web/manifest.json":               "{\n  \"name\": \"notes\",\n  \"version\": \"1.4.0\"\n}\n",
		"web/install.sh":                  "VERSION=1.4.0 # x-release-please-version\n",
		"web/CITATION.cff":                "date-released: 2026-01-05 # x-release-please-date\n",
		"web/notes.txt":                   "no markers here\n",
	})

	got, err := migrate.FromReleasePlease([]byte(configJSON), nil, read)
	if err != nil {
		t.Fatalf("FromReleasePlease() error = %v", err)
	}
	want := []config.File{
		{Path: "web/internal/version/version.go", Updater: "generic"},
		{Path: "README.md", Updater: "generic"},
		{Path: "web/deploy/values.yaml", Updater: "yaml", Key: "version"},
		{Path: "web/deploy/values.yaml", Updater: "generic"},
		{Path: "web/manifest.json", Updater: "json", Key: "version"},
		{Path: "web/install.sh", Updater: "generic"},
	}
	if got := load(t, got.YAML).Resolve()[0].Files; !reflect.DeepEqual(got, want) {
		t.Fatalf("Files = %+v, want %+v", got, want)
	}
	wantWarnings := []string{
		"package web: extra-files: web/CITATION.cff: updater: generic: line 1: x-release-please-date writes the " +
			"release date, which release-me's generic updater does not support, left out of files",
		"package web: extra-files: web/notes.txt: updater: generic: no line carries an x-release-please version marker, " +
			"left out of files",
		"package web: extra-files: web/missing.go does not exist, left out of files",
		"package web: extra-files: packages/*/package.json: a glob, release-me lists its files, " +
			"add each file it matches under files",
	}
	if !reflect.DeepEqual(got.Warnings, wantWarnings) {
		t.Fatalf("Warnings = %q, want %q", got.Warnings, wantWarnings)
	}
}
