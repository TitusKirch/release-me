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
			name:     "ruby bumps the gem's version.rb and its Gemfile.lock entry",
			settings: `"release-type": "ruby", "component": "release-notes"`,
			repo: map[string]string{
				"web/lib/release/notes/version.rb": "module ReleaseNotes\n  VERSION = \"1.4.0\"\nend\n",
				"web/Gemfile.lock":                 gemfileLockFile,
			},
			wantFiles: []config.File{
				{Path: "web/lib/release/notes/version.rb", Updater: "ruby"},
				{Path: "web/Gemfile.lock", Updater: "gemfile-lock", Key: "release-notes"},
			},
		},
		{
			name:     "ruby bumps the gem's version.rb",
			settings: `"release-type": "ruby", "component": "release-notes"`,
			repo: map[string]string{
				"web/lib/release/notes/version.rb": "module ReleaseNotes\n  VERSION = \"1.4.0\"\nend\n",
			},
			wantFiles: []config.File{
				{Path: "web/lib/release/notes/version.rb", Updater: "ruby"},
			},
		},
		{
			name:     "ruby bumps the version-file it names",
			settings: `"release-type": "ruby", "component": "release-notes", "version-file": "lib/notes/version.rb"`,
			repo: map[string]string{
				"web/lib/notes/version.rb": "module Notes\n  VERSION = \"1.4.0\"\nend\n",
			},
			wantFiles: []config.File{{Path: "web/lib/notes/version.rb", Updater: "ruby"}},
		},
		{
			name:     "python bumps setup.py, pyproject.toml and the package's __init__.py",
			settings: `"release-type": "python"`,
			repo: map[string]string{
				"web/setup.py":                      "from setuptools import setup\n\nsetup(\n    name=\"release-notes\",\n    version=\"1.4.0\",\n)\n",
				"web/pyproject.toml":                "[project]\nname = \"release-notes\"\nversion = \"1.4.0\"\n",
				"web/src/release_notes/__init__.py": "__version__ = \"1.4.0\"\n",
				"web/release_notes/version.py":      "VERSION_INFO = (1, 4, 0)\n",
			},
			wantFiles: []config.File{
				{Path: "web/setup.py", Updater: "python"},
				{Path: "web/pyproject.toml", Updater: "toml", Key: "project.version"},
				{Path: "web/src/release_notes/__init__.py", Updater: "python"},
			},
		},
		{
			name:     "python bumps a poetry project's version and setup.cfg",
			settings: `"release-type": "python"`,
			repo: map[string]string{
				"web/setup.cfg":         "[metadata]\nname = release-notes\nversion = 1.4.0\n",
				"web/pyproject.toml":    "[tool.poetry]\nname = \"notes\"\nversion = \"1.4.0\"\n",
				"web/notes/__init__.py": "__version__ = '1.4.0'\n",
			},
			wantFiles: []config.File{
				{Path: "web/setup.cfg", Updater: "python"},
				{Path: "web/pyproject.toml", Updater: "toml", Key: "tool.poetry.version"},
				{Path: "web/notes/__init__.py", Updater: "python"},
			},
		},
		{
			name:     "python warns when it finds no version file",
			settings: `"release-type": "python"`,
			wantWarnings: []string{
				"package web: release-type python: no setup.cfg, setup.py, pyproject.toml, __init__.py or version.py " +
					"with a version found, add the version file under files",
			},
		},
		{
			name:      "elixir bumps mix.exs",
			settings:  `"release-type": "elixir"`,
			repo:      map[string]string{"web/mix.exs": "  @version \"1.4.0\"\n"},
			wantFiles: []config.File{{Path: "web/mix.exs", Updater: "elixir"}},
		},
		{
			name:     "ocaml bumps the opam file named after the package and dune-project",
			settings: `"release-type": "ocaml", "component": "notes"`,
			repo: map[string]string{
				"web/notes.opam":   "opam-version: \"2.0\"\nversion: \"1.4.0\"\n",
				"web/dune-project": "(lang dune 3.0)\n(version 1.4.0)\n",
			},
			wantFiles: []config.File{
				{Path: "web/notes.opam", Updater: "ocaml"},
				{Path: "web/dune-project", Updater: "ocaml"},
			},
			wantWarnings: []string{
				"package web: release-type ocaml: release-please bumped every .opam and esy JSON file of the package, " +
					"only those named after the package are carried over, add any other under files",
			},
		},
		{
			name:      "r bumps DESCRIPTION",
			settings:  `"release-type": "r"`,
			repo:      map[string]string{"web/DESCRIPTION": "Package: notes\nVersion: 1.4.0\n"},
			wantFiles: []config.File{{Path: "web/DESCRIPTION", Updater: "r"}},
		},
		{
			name:      "bazel bumps MODULE.bazel",
			settings:  `"release-type": "bazel"`,
			repo:      map[string]string{"web/MODULE.bazel": "module(name = \"rules_notes\", version = \"1.4.0\")\n"},
			wantFiles: []config.File{{Path: "web/MODULE.bazel", Updater: "bazel"}},
		},
		{
			name:     "php bumps composer.json's version and a VERSION file",
			settings: `"release-type": "php"`,
			repo: map[string]string{
				"web/composer.json": "{\n  \"name\": \"acme/notes\",\n  \"version\": \"1.4.0\"\n}\n",
				"web/VERSION":       "1.4.0\n",
			},
			wantFiles: []config.File{
				{Path: "web/composer.json", Updater: "json", Key: "version"},
				{Path: "web/VERSION", Updater: "plain"},
			},
		},
		{
			name:     "php leaves out a VERSION file holding more than the version",
			settings: `"release-type": "php"`,
			repo: map[string]string{
				"web/composer.json": "{\n  \"name\": \"acme/notes\",\n  \"version\": \"1.4.0\"\n}\n",
				"web/VERSION":       "notes 1.4.0, built 2026-10-30\n",
			},
			wantFiles: []config.File{
				{Path: "web/composer.json", Updater: "json", Key: "version"},
			},
		},
		{
			name:     "php leaves out a composer.json without a version",
			settings: `"release-type": "php"`,
			repo:     map[string]string{"web/composer.json": "{\n  \"name\": \"acme/notes\"\n}\n"},
		},
		{
			name:      "dart bumps pubspec.yaml",
			settings:  `"release-type": "dart"`,
			repo:      map[string]string{"web/pubspec.yaml": "name: notes\nversion: 1.4.0+3\n"},
			wantFiles: []config.File{{Path: "web/pubspec.yaml", Updater: "dart"}},
		},
		{
			name:     "expo bumps package.json, its npm lockfiles and app.json",
			settings: `"release-type": "expo"`,
			repo: map[string]string{
				"web/package.json":      packageJSON,
				"web/package-lock.json": packageLockJSON,
				"web/app.json":          "{\n  \"expo\": {\n    \"version\": \"2.0.0\"\n  }\n}\n",
			},
			wantFiles: []config.File{
				{Path: "web/package.json", Updater: "json", Key: "version"},
				{Path: "web/package-lock.json", Updater: "npm-lock"},
				{Path: "web/app.json", Updater: "json", Key: "expo.version"},
			},
		},
		{
			name:     "expo bumps package.json and app.json, and warns about versionCode",
			settings: `"release-type": "expo"`,
			repo: map[string]string{
				"web/package.json": packageJSON,
				"web/app.json": "{\n  \"expo\": {\n    \"version\": \"2.0.0\",\n    \"ios\": {\n      \"buildNumber\": \"2.0.0\"\n" +
					"    },\n    \"android\": {\n      \"versionCode\": 510020000\n    }\n  }\n}\n",
			},
			wantFiles: []config.File{
				{Path: "web/package.json", Updater: "json", Key: "version"},
				{Path: "web/app.json", Updater: "json", Key: "expo.version"},
				{Path: "web/app.json", Updater: "json", Key: "expo.ios.buildNumber"},
			},
			wantWarnings: []string{
				"package web: release-type expo: app.json's expo.android.versionCode is computed from the Expo SDK " +
					"and the version, release-me does not bump it",
			},
		},
		{
			name:      "sfdx bumps sfdx-project.json",
			settings:  `"release-type": "sfdx"`,
			repo:      map[string]string{"web/sfdx-project.json": "{}\n"},
			wantFiles: []config.File{{Path: "web/sfdx-project.json", Updater: "sfdx"}},
		},
		{
			name:     "terraform-module warns that its files are searched for",
			settings: `"release-type": "terraform-module"`,
			wantWarnings: []string{
				"package web: release-type terraform-module: release-please searched the package for the files it bumps, " +
					"release-me lists them: mark each version line with \"release-me: version\" and add it under files",
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

func TestFromReleasePleaseKeepsAnRPackagesChangelogInNEWS(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		settings string
		want     string
	}{
		{name: "defaults to NEWS.md, as release-please's r type does", settings: `"release-type": "r"`, want: "web/NEWS.md"},
		{name: "keeps a changelog-path it is given", settings: `"release-type": "r", "changelog-path": "CHANGES.md"`, want: "web/CHANGES.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configJSON := `{"packages": {"web": {` + tc.settings + `}}}`
			read := files(map[string]string{"web/DESCRIPTION": "Package: notes\nVersion: 1.0.0\n"})
			got, err := migrate.FromReleasePlease([]byte(configJSON), nil, read)
			if err != nil {
				t.Fatalf("FromReleasePlease() error = %v", err)
			}
			if path := load(t, got.YAML).Resolve()[0].ChangelogPath; path != tc.want {
				t.Fatalf("ChangelogPath = %q, want %q", path, tc.want)
			}
		})
	}
}

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
