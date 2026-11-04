// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

const pomXML = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>cloud.ostendorf</groupId>
  <artifactId>notes-web</artifactId>
  <version>1.4.0</version>
</project>
`

func TestFromReleasePleaseCarriesOverTypedXMLExtraFiles(t *testing.T) {
	t.Parallel()
	configJSON := `{"packages": {"web": {"release-type": "go", "extra-files": [
	  {"type": "xml", "path": "src/Notes/Notes.csproj", "xpath": "/Project/PropertyGroup/Version"},
	  {"type": "xml", "path": "/pom.xml", "xpath": "/*[local-name()='project']/*[local-name()=\"version\"]"},
	  {"type": "pom", "path": "notes-core/pom.xml"},
	  {"type": "xml", "path": "build.xml", "xpath": "//version"},
	  {"type": "xml", "path": "plugin.xml", "xpath": "/plugin/@version"}
	]}}}`

	got, err := migrate.FromReleasePlease([]byte(configJSON), nil, files(nil))
	if err != nil {
		t.Fatalf("FromReleasePlease() error = %v", err)
	}
	want := []config.File{
		{Path: "web/src/Notes/Notes.csproj", Updater: "xml", Key: "Project.PropertyGroup.Version"},
		{Path: "pom.xml", Updater: "xml", Key: "project.version"},
		{Path: "web/notes-core/pom.xml", Updater: "xml", Key: "project.version"},
	}
	if files := load(t, got.YAML).Resolve()[0].Files; !reflect.DeepEqual(files, want) {
		t.Fatalf("Files = %+v, want %+v", files, want)
	}
	wantWarnings := []string{
		`package web: extra-files: build.xml: type "xml" at "//version" has no typed updater, add it under files`,
		`package web: extra-files: plugin.xml: type "xml" at "/plugin/@version" has no typed updater, add it under files`,
	}
	if !reflect.DeepEqual(got.Warnings, wantWarnings) {
		t.Fatalf("Warnings = %q, want %q", got.Warnings, wantWarnings)
	}
}

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
			name:     "maven bumps the project version of pom.xml",
			settings: `"release-type": "maven"`,
			repo:     map[string]string{"web/pom.xml": pomXML},
			wantFiles: []config.File{
				{Path: "web/pom.xml", Updater: "xml", Key: "project.version"},
			},
		},
		{
			name:     "maven bumps the parent version of a pom.xml inheriting its version",
			settings: `"release-type": "maven"`,
			repo: map[string]string{"web/pom.xml": `<project>
  <parent>
    <groupId>cloud.ostendorf</groupId>
    <artifactId>notes-parent</artifactId>
    <version>1.4.0</version>
  </parent>
  <artifactId>notes-web</artifactId>
</project>
`},
			wantFiles: []config.File{
				{Path: "web/pom.xml", Updater: "xml", Key: "project.parent.version"},
			},
		},
		{
			name:     "maven warns about the modules of a multi-module project",
			settings: `"release-type": "maven"`,
			repo: map[string]string{"web/pom.xml": `<project>
  <artifactId>notes-parent</artifactId>
  <version>1.4.0</version>
  <packaging>pom</packaging>
  <modules>
    <module>notes-core</module>
  </modules>
</project>
`},
			wantFiles: []config.File{
				{Path: "web/pom.xml", Updater: "xml", Key: "project.version"},
			},
			wantWarnings: []string{
				"package web: release-type maven: release-please bumped every pom.xml below the package, " +
					"only web/pom.xml is carried over, add each module's pom.xml under files",
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
