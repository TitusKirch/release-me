// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/history"
)

func TestAllEntries(t *testing.T) {
	t.Parallel()

	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit", "")

	runGit(t, dir, "checkout", "-q", "-b", "feat-branch")
	commitEntry(t, dir, "feat: start the command", "")
	commitEntry(t, dir, "fix(cli): a typo while at it", "")
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "merge", "--no-ff", "-q", "-m", "Merge pull request #5 from owner/feat-branch", "feat-branch")
	commitEntry(t, dir, "docs: update the readme", "")

	got, err := history.AllEntries(context.Background(), dir, "")
	if err != nil {
		t.Fatalf("AllEntries() error = %v", err)
	}

	want := []history.Entry{
		{Subject: "docs: update the readme"},
		{PullRequestNumber: 5, Subject: "fix(cli): a typo while at it"},
		{PullRequestNumber: 5, Subject: "feat: start the command"},
		{Subject: "chore: initial commit"},
	}
	if !reflect.DeepEqual(stripSHA(got), want) {
		t.Fatalf("AllEntries() = %+v, want %+v", stripSHA(got), want)
	}
}

func TestAllEntriesExcludesTheMergeCommitItself(t *testing.T) {
	t.Parallel()

	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit", "")
	runGit(t, dir, "checkout", "-q", "-b", "feat-branch")
	commitEntry(t, dir, "feat: start the command", "")
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "merge", "--no-ff", "-q", "-m", "Merge pull request #3 from owner/feat-branch", "feat-branch")

	got, err := history.AllEntries(context.Background(), dir, "")
	if err != nil {
		t.Fatalf("AllEntries() error = %v", err)
	}
	for _, e := range got {
		if e.Subject == "Merge pull request #3 from owner/feat-branch" {
			t.Fatalf("AllEntries() includes the merge commit: %+v", got)
		}
	}
}

func TestAllEntriesLeavesADirectCommitUnattributed(t *testing.T) {
	t.Parallel()

	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit", "")
	tag(t, dir, "v0.1.0")
	commitEntry(t, dir, "feat: a commit straight on the default branch", "")

	got, err := history.AllEntries(context.Background(), dir, "v0.1.0")
	if err != nil {
		t.Fatalf("AllEntries() error = %v", err)
	}
	if len(got) != 1 || got[0].PullRequestNumber != 0 {
		t.Fatalf("AllEntries() = %+v, want one unattributed entry", got)
	}
}
