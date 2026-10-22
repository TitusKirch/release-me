// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/history"
)

func TestSubCommits(t *testing.T) {
	t.Parallel()

	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit", "")
	runGit(t, dir, "checkout", "-q", "-b", "feat-branch")
	commitEntry(t, dir, "feat: start the command", "")
	commitEntry(t, dir, "fix(cli): a typo while at it", "")
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "merge", "--no-ff", "-q", "-m", "Merge pull request #5 from owner/feat-branch", "feat-branch")

	head, err := history.Walk(context.Background(), dir, "")
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if len(head) == 0 {
		t.Fatal("Walk() returned no entries")
	}
	merge := head[0]

	got, err := history.SubCommits(context.Background(), dir, merge.SHA)
	if err != nil {
		t.Fatalf("SubCommits() error = %v", err)
	}

	want := []history.Entry{
		{Subject: "feat: start the command"},
		{Subject: "fix(cli): a typo while at it"},
	}
	if !reflect.DeepEqual(stripSHA(got), want) {
		t.Fatalf("SubCommits() = %+v, want %+v", stripSHA(got), want)
	}
}

func TestSubCommitsReturnsNoneForADirectCommit(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	commitEntry(t, dir, "feat: add a command", "")

	head, err := history.Walk(context.Background(), dir, "")
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	got, err := history.SubCommits(context.Background(), dir, head[0].SHA)
	if err != nil {
		t.Fatalf("SubCommits() error = %v", err)
	}
	if got != nil {
		t.Fatalf("SubCommits() = %+v, want none", got)
	}
}
