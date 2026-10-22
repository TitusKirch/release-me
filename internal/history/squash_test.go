// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/history"
)

func TestWalkSquash(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  []history.Entry
	}{
		{
			name: "parses the pull request number from a trailing (#N)",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "feat(cli): add the next command (#5)", "")
			},
			want: []history.Entry{
				{PullRequestNumber: 5, Subject: "feat(cli): add the next command", Squashed: true},
			},
		},
		{
			name: "treats a commit without a trailing number as a direct commit",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: tidy up", "")
			},
			want: []history.Entry{
				{Subject: "chore: tidy up"},
			},
		},
		{
			name: "keeps the commit body as the override fallback",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "fix: stop the panic (#7)", "the forge has no record of this one")
			},
			want: []history.Entry{
				{PullRequestNumber: 7, Subject: "fix: stop the panic", Body: "the forge has no record of this one", Squashed: true},
			},
		},
		{
			name: "still treats a genuine merge commit as one entry",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
				commitEntry(t, dir, "Merge pull request #9 from owner/feat-branch", "feat: add a command")
			},
			want: []history.Entry{
				{PullRequestNumber: 9, Subject: "Merge pull request #9 from owner/feat-branch", Body: "feat: add a command"},
				{Subject: "chore: initial commit"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := newRepo(t)
			tc.setup(t, dir)

			got, err := history.WalkSquash(context.Background(), dir, "")
			if err != nil {
				t.Fatalf("WalkSquash() error = %v", err)
			}
			if !reflect.DeepEqual(stripSHA(got), tc.want) {
				t.Fatalf("WalkSquash() = %+v, want %+v", stripSHA(got), tc.want)
			}
		})
	}
}
