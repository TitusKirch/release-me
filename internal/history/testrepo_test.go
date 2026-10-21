// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history_test

import (
	"os/exec"
	"strings"
	"testing"
)

// newRepo creates an empty git repository in a temporary directory, with a
// local identity so commits never depend on the host's git configuration.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.name", "release-me tests")
	runGit(t, dir, "config", "user.email", "release-me-tests@example.com")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

// runGit runs git in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// runGitOutput runs git in dir and returns its standard output, failing the
// test on error.
func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

// commitEntry creates an empty commit with subject and, when body is not
// empty, a blank line followed by body.
func commitEntry(t *testing.T, dir, subject, body string) {
	t.Helper()
	message := subject
	if body != "" {
		message += "\n\n" + body
	}
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", message)
}

// tag creates a lightweight tag at HEAD.
func tag(t *testing.T, dir, name string) {
	t.Helper()
	runGit(t, dir, "tag", name)
}
