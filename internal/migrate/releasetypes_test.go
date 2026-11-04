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
			name:     "rust warns about a crate inheriting its workspace version",
			settings: `"release-type": "rust"`,
			repo: map[string]string{
				"web/Cargo.toml": "[package]\nname = \"release-core\"\nversion.workspace = true\n",
				"web/Cargo.lock": cargoLockFile,
			},
			wantWarnings: []string{
				"package web: release-type rust: web/Cargo.toml declares no [package] name and version, " +
					"add the crates it releases under files",
			},
		},
		{
			name:     "ruby warns when an existing lockfile has no configured gem name",
			settings: `"release-type": "ruby"`,
			repo: map[string]string{
				"web/Gemfile.lock": gemfileLockFile,
			},
			wantWarnings: []string{
				"package web: release-type ruby: its version.rb is not carried over, add it under files",
				"package web: release-type ruby: web/Gemfile.lock is not carried over without component or package-name, " +
					"add it under files with a gem name as key",
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
			name:     "maven ignores modules mentioned only in a comment",
			settings: `"release-type": "maven"`,
			repo: map[string]string{"web/pom.xml": `<project>
  <version>1.4.0</version>
  <!-- <modules><module>notes-core</module></modules> -->
</project>
`},
			wantFiles: []config.File{
				{Path: "web/pom.xml", Updater: "xml", Key: "project.version"},
			},
		},
		{
			name:     "maven recognizes prefixed modules with a declared encoding",
			settings: `"release-type": "maven"`,
			repo: map[string]string{"web/pom.xml": `<?xml version="1.0" encoding="ISO-8859-1"?>
<pom:project xmlns:pom="http://maven.apache.org/POM/4.0.0">
  <pom:version>1.4.0</pom:version>
  <pom:modules><pom:module>notes-core</pom:module></pom:modules>
</pom:project>
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
