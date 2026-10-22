// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRunMigrateTranslatesTheRepositorysReleasePleaseConfig exercises
// migrate end to end through run(): it reads both release-please files from
// the repository root, prints a release-me.yaml that loads, and warns about
// the one package whose last release tag is missing.
func TestRunMigrateTranslatesTheRepositorysReleasePleaseConfig(t *testing.T) {
	dir := newRepo(t)
	writeFiles(t, dir, "chore: configure release-please", map[string]string{
		"release-please-config.json": `{"tag-separator": "@", "separate-pull-requests": true, "packages": {
		  "packages/layer": {"package-name": "@acme/layer", "changelog-path": "/CHANGELOG.md"},
		  "packages/search": {"package-name": "@acme/search"}}}`,
		".release-please-manifest.json": `{"packages/layer": "0.8.1", "packages/search": "0.1.0"}`,
		"packages/layer/package.json":   `{"name": "@acme/layer", "version": "0.8.1"}`,
		"packages/search/package.json":  `{"name": "@acme/search", "version": "0.1.0"}`,
	})
	runGit(t, dir, "tag", "layer@v0.8.1")

	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", "release-please"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr: %s", code, stderr.String())
	}
	for _, want := range []string{"  layer:\n", "    tag-prefix: layer@v\n", "    changelog: CHANGELOG.md\n", "  search:\n"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %s, want it to contain %q", stdout.String(), want)
		}
	}
	wantWarning := "warning: search: tag search@v0.1.0 does not exist, release-me would start over at the initial version\n"
	if stderr.String() != wantWarning {
		t.Fatalf("stderr = %q, want only %q", stderr.String(), wantWarning)
	}
}

func TestRunMigrateRejectsAMissingOrUnknownSource(t *testing.T) {
	for _, args := range [][]string{{"migrate"}, {"migrate", "semantic-release"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("run(%q) exit code = %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "release-me migrate release-please") {
			t.Fatalf("run(%q) stderr = %q, want the migrate usage", args, stderr.String())
		}
	}
}

func TestRunMigrateReportsAMissingReleasePleaseConfig(t *testing.T) {
	newRepo(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", "release-please"}, &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "release-please-config.json") {
		t.Fatalf("exit code = %d, stderr = %q, want 1 naming the missing file", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want nothing printed", stdout.String())
	}
}
