// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo creates a git repository in a temporary directory, with an origin
// remote and a local identity so commits never depend on the host's git
// configuration, and switches the test's working directory into it.
//
// It uses t.Chdir, so the calling test must not run in parallel.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.name", "release-me tests")
	runGit(t, dir, "config", "user.email", "release-me-tests@example.com")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	runGit(t, dir, "remote", "add", "origin", "git@github.com:owner/repo.git")
	t.Chdir(dir)
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func commitEntry(t *testing.T, dir, subject string) {
	t.Helper()
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", subject)
}

func tagRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "tag", "v0.1.0")
}

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "release-me.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write release-me.yaml: %v", err)
	}
}

func TestRunNextPrintsTheInitialVersionWithoutATag(t *testing.T) {
	dir := newRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	var stdout, stderr bytes.Buffer
	code := run([]string{"next"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr: %s", code, stderr.String())
	}
	want := "0.1.0 (initial version)\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunNextPrintsAMinorBumpForAFeature(t *testing.T) {
	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	tagRepo(t, dir)
	commitEntry(t, dir, "feat: add the changelog command")

	var stdout, stderr bytes.Buffer
	code := run([]string{"next"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr: %s", code, stderr.String())
	}
	want := "0.2.0 (feat)\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunNextPrintsNothingWhenThereIsNothingToRelease(t *testing.T) {
	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	tagRepo(t, dir)
	commitEntry(t, dir, "chore: tidy up")

	var stdout, stderr bytes.Buffer
	code := run([]string{"next"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr: %s", code, stderr.String())
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunNextLogsDebugOnStderrWhenVerbose(t *testing.T) {
	dir := newRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	var stdout, stderr bytes.Buffer
	code := run([]string{"next", "--verbose"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "resolved repository") {
		t.Fatalf("stderr = %q, want debug logging", stderr.String())
	}
}

func TestRunNextFailsWithoutAnOriginRemote(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.name", "release-me tests")
	runGit(t, dir, "config", "user.email", "release-me-tests@example.com")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	commitEntry(t, dir, "feat: add the next command")
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := run([]string{"next"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr is empty, want an error")
	}
}

func TestRunChangelogPrintsTheRenderedEntries(t *testing.T) {
	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	tagRepo(t, dir)
	commitEntry(t, dir, "feat(cli): add the changelog command")
	commitEntry(t, dir, "fix: stop the panic on empty history")

	var stdout, stderr bytes.Buffer
	code := run([]string{"changelog"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "### Features") || !strings.Contains(stdout.String(), "### Bug Fixes") {
		t.Fatalf("stdout = %q, want Features and Bug Fixes sections", stdout.String())
	}
	if !strings.Contains(stdout.String(), "add the changelog command") {
		t.Fatalf("stdout = %q, want the feature description", stdout.String())
	}
}

func TestRunChangelogPrintsNothingWhenThereIsNothingToRelease(t *testing.T) {
	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	tagRepo(t, dir)
	commitEntry(t, dir, "chore: tidy up")

	var stdout, stderr bytes.Buffer
	code := run([]string{"changelog"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr: %s", code, stderr.String())
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunChangelogHidesAConfiguredHiddenType(t *testing.T) {
	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	tagRepo(t, dir)
	writeConfig(t, dir, "hidden-types: [fix]\n")
	commitEntry(t, dir, "feat: add the changelog command")
	commitEntry(t, dir, "fix: stop the panic on empty history")

	var stdout, stderr bytes.Buffer
	code := run([]string{"changelog"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "Bug Fixes") {
		t.Fatalf("stdout = %q, want no Bug Fixes section", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Features") {
		t.Fatalf("stdout = %q, want a Features section", stdout.String())
	}
}

func TestRunChangelogReadsConfigFromTheRepositoryRootNotTheWorkingDirectory(t *testing.T) {
	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	tagRepo(t, dir)
	writeConfig(t, dir, "hidden-types: [feat]\n")
	commitEntry(t, dir, "feat: add the changelog command")

	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Chdir(sub)

	var stdout, stderr bytes.Buffer
	code := run([]string{"changelog"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr: %s", code, stderr.String())
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty, the root config hides feat", stdout.String())
	}
}
