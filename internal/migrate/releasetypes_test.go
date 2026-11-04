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
