// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/history"
)

func TestOriginHEADReturnsTheRemotesDefaultBranch(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit", "")
	runGit(t, dir, "remote", "add", "origin", "git@github.com:owner/repo.git")
	runGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")

	got, err := history.OriginHEAD(t.Context(), dir)
	if err != nil {
		t.Fatalf("OriginHEAD() error = %v", err)
	}
	if got != "trunk" {
		t.Fatalf("OriginHEAD() = %q, want %q", got, "trunk")
	}
}

func TestOriginHEADFailsWithoutTheSymbolicRef(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit", "")

	if _, err := history.OriginHEAD(t.Context(), dir); err == nil {
		t.Fatal("OriginHEAD() error = nil, want an error")
	}
}
