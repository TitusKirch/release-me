// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

const (
	packageJSON = `{
  "name": "@acme/web",
  "version": "2.0.0",
  "private": true
}
`
	packageLockJSON = `{
  "name": "@acme/web",
  "version": "2.0.0",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": {
      "name": "@acme/web",
      "version": "2.0.0"
    }
  }
}
`
	cargoTOML = `[package]
name = "release-core"
version = "0.4.2"
edition = "2021"

[dependencies]
anyhow = "1.0"
`
	cargoLockFile = `version = 4

[[package]]
name = "anyhow"
version = "1.0.86"
source = "registry+https://github.com/rust-lang/crates.io-index"

[[package]]
name = "release-core"
version = "0.4.2"
`
	gemfileLockFile = `PATH
  remote: .
  specs:
    release-notes (1.4.0)

GEM
  remote: https://rubygems.org/
  specs:

DEPENDENCIES
  release-notes!
`
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
		{
			name:     "node bumps package.json and the npm lockfiles that exist",
			settings: `"release-type": "node"`,
			repo: map[string]string{
				"web/package.json":      packageJSON,
				"web/package-lock.json": packageLockJSON,
			},
			wantFiles: []config.File{
				{Path: "web/package.json", Updater: "json", Key: "version"},
				{Path: "web/package-lock.json", Updater: "npm-lock"},
			},
		},
		{
			name:     "node leaves out an npm-shrinkwrap.json without a top-level version",
			settings: `"release-type": "node"`,
			repo: map[string]string{
				"web/package.json":        packageJSON,
				"web/npm-shrinkwrap.json": "{\n  \"lockfileVersion\": 3\n}\n",
				"web/package-lock.json":   packageLockJSON,
			},
			wantFiles: []config.File{
				{Path: "web/package.json", Updater: "json", Key: "version"},
				{Path: "web/package-lock.json", Updater: "npm-lock"},
			},
		},
		{
			name:     "rust bumps Cargo.toml and the crate's Cargo.lock entry",
			settings: `"release-type": "rust"`,
			repo: map[string]string{
				"web/Cargo.toml": cargoTOML,
				"web/Cargo.lock": cargoLockFile,
			},
			wantFiles: []config.File{
				{Path: "web/Cargo.toml", Updater: "toml", Key: "package.version"},
				{Path: "web/Cargo.lock", Updater: "cargo-lock", Key: "release-core"},
			},
		},
		{
			name:     "rust warns about a workspace manifest with no package of its own",
			settings: `"release-type": "rust"`,
			repo: map[string]string{
				"web/Cargo.toml": "[workspace]\nmembers = [\"crates/*\"]\n",
			},
			wantWarnings: []string{
				"package web: release-type rust: web/Cargo.toml declares no [package] name and version, " +
					"add the crates it releases under files",
			},
		},
		{
			name:     "ruby bumps the gem's Gemfile.lock entry",
			settings: `"release-type": "ruby", "component": "release-notes"`,
			repo: map[string]string{
				"web/Gemfile.lock": gemfileLockFile,
			},
			wantFiles: []config.File{
				{Path: "web/Gemfile.lock", Updater: "gemfile-lock", Key: "release-notes"},
			},
			wantWarnings: []string{
				"package web: release-type ruby: its version.rb is not carried over, add it under files",
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
