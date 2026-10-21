// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/history"
)

// writeFile writes content to a path inside dir, creating parent
// directories as needed.
func writeFile(t *testing.T, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// mergeBranch commits onto a side branch created from base, then merges it
// back with --no-ff, the shape a pull request merge takes in the first
// parent history. It returns the merge commit's SHA.
func mergeBranch(t *testing.T, dir, branch string, commits func()) string {
	t.Helper()
	runGit(t, dir, "checkout", "-q", "-b", branch)
	commits()
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "merge", "--no-ff", "-q", "-m", "merge "+branch, branch)
	return strings.TrimSpace(runGitOutput(t, dir, "rev-parse", "HEAD"))
}

func TestTouchesAndBelongsAttributeAMergeCommitToItsComponents(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	writeFile(t, dir, "frontend/README.md", "frontend\n")
	writeFile(t, dir, "backend/README.md", "backend\n")
	writeFile(t, dir, "README.md", "root\n")
	runGit(t, dir, "add", ".")
	commitEntry(t, dir, "chore: scaffold the two components", "")

	cases := []struct {
		name         string
		sha          string
		wantFrontend bool
		wantBackend  bool
	}{
		{
			name: "a pull request touching both components",
			sha: mergeBranch(t, dir, "both", func() {
				writeFile(t, dir, "frontend/app.js", "console.log(1)\n")
				writeFile(t, dir, "backend/app.go", "package backend\n")
				runGit(t, dir, "add", ".")
				commitEntry(t, dir, "feat: wire the frontend to the backend", "")
			}),
			wantFrontend: true, wantBackend: true,
		},
		{
			name: "a pull request touching neither component",
			sha: mergeBranch(t, dir, "none", func() {
				writeFile(t, dir, "README.md", "root, updated\n")
				runGit(t, dir, "add", ".")
				commitEntry(t, dir, "docs: update the root readme", "")
			}),
			wantFrontend: false, wantBackend: false,
		},
		{
			name: "a pull request touching only one component",
			sha: mergeBranch(t, dir, "frontend-only", func() {
				writeFile(t, dir, "frontend/app.js", "console.log(2)\n")
				runGit(t, dir, "add", ".")
				commitEntry(t, dir, "fix: stop a frontend crash", "")
			}),
			wantFrontend: true, wantBackend: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paths, err := history.Touches(context.Background(), dir, tc.sha)
			if err != nil {
				t.Fatalf("Touches() error = %v", err)
			}
			if got := history.Belongs(paths, "frontend"); got != tc.wantFrontend {
				t.Fatalf("Belongs(%v, %q) = %v, want %v", paths, "frontend", got, tc.wantFrontend)
			}
			if got := history.Belongs(paths, "backend"); got != tc.wantBackend {
				t.Fatalf("Belongs(%v, %q) = %v, want %v", paths, "backend", got, tc.wantBackend)
			}
			if !history.Belongs(paths, ".") {
				t.Fatalf("Belongs(%v, %q) = false, want true, the root component matches everything", paths, ".")
			}
		})
	}
}

func TestTouchesHandlesTheRootCommitWithNoParent(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	writeFile(t, dir, "frontend/README.md", "frontend\n")
	runGit(t, dir, "add", ".")
	commitEntry(t, dir, "chore: initial commit", "")
	sha := strings.TrimSpace(runGitOutput(t, dir, "rev-parse", "HEAD"))

	paths, err := history.Touches(context.Background(), dir, sha)
	if err != nil {
		t.Fatalf("Touches() error = %v", err)
	}
	if !history.Belongs(paths, "frontend") {
		t.Fatalf("Belongs(%v, %q) = false, want true", paths, "frontend")
	}
}

func TestBelongsTreatsAFileNamedLikeAComponentPathAsASeparateFile(t *testing.T) {
	t.Parallel()
	if history.Belongs([]string{"frontend-tools/README.md"}, "frontend") {
		t.Fatal("Belongs() = true, want false: frontend-tools is not nested under frontend")
	}
}
