// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/forge/forgetest"
	"github.com/ostendorf-cloud/release-me/internal/history"
	"github.com/ostendorf-cloud/release-me/internal/version"
)

func TestAssembleEntries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		raw         []history.Entry
		responses   map[int]forgetest.Response
		wantChanges []version.Change
		wantEntries []changelog.Entry
	}{
		{
			name: "uses the pull request title when the forge finds it",
			raw: []history.Entry{
				{PullRequestNumber: 5, Subject: "Merge pull request #5 from owner/feat-branch", Body: "feat(cli): add the next command"},
			},
			responses: map[int]forgetest.Response{
				5: {PullRequest: forge.PullRequest{
					Title: "feat(cli): add the real next command",
					URL:   "https://github.com/owner/repo/pull/5",
				}},
			},
			wantChanges: []version.Change{{Type: "feat"}},
			wantEntries: []changelog.Entry{{
				Type:              "feat",
				Scope:             "cli",
				Description:       "add the real next command",
				PullRequestNumber: 5,
				URL:               "https://github.com/owner/repo/pull/5",
			}},
		},
		{
			name: "falls back to the commit body when the forge has no pull request",
			raw: []history.Entry{
				{PullRequestNumber: 7, Subject: "Merge pull request #7 from owner/fix-branch", Body: "fix: stop the panic on empty history"},
			},
			wantChanges: []version.Change{{Type: "fix"}},
			wantEntries: []changelog.Entry{{
				Type:              "fix",
				Description:       "stop the panic on empty history",
				PullRequestNumber: 7,
			}},
		},
		{
			name: "parses a direct commit subject without a link",
			raw: []history.Entry{
				{Subject: "fix: stop the panic on empty history"},
			},
			wantChanges: []version.Change{{Type: "fix"}},
			wantEntries: []changelog.Entry{{Type: "fix", Description: "stop the panic on empty history"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &forgetest.Fake{Responses: tc.responses}

			changes, entries, err := assembleEntries(context.Background(), f, "owner", "repo", tc.raw, "other")
			if err != nil {
				t.Fatalf("assembleEntries() error = %v", err)
			}
			if !reflect.DeepEqual(changes, tc.wantChanges) {
				t.Fatalf("changes = %+v, want %+v", changes, tc.wantChanges)
			}
			if !reflect.DeepEqual(entries, tc.wantEntries) {
				t.Fatalf("entries = %+v, want %+v", entries, tc.wantEntries)
			}
		})
	}
}

func TestAssembleEntriesUsesTheGivenFallbackType(t *testing.T) {
	t.Parallel()
	raw := []history.Entry{{Subject: "quick hack before the demo"}}
	f := &forgetest.Fake{}

	_, entries, err := assembleEntries(context.Background(), f, "owner", "repo", raw, "misc")
	if err != nil {
		t.Fatalf("assembleEntries() error = %v", err)
	}

	want := []changelog.Entry{{Type: "misc", Description: "quick hack before the demo"}}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %+v, want %+v", entries, want)
	}
}

func TestAssembleEntriesPropagatesAForgeError(t *testing.T) {
	t.Parallel()
	raw := []history.Entry{
		{PullRequestNumber: 1, Subject: "Merge pull request #1 from owner/branch", Body: "feat: x"},
	}
	f := &forgetest.Fake{Responses: map[int]forgetest.Response{
		1: {Err: context.DeadlineExceeded},
	}}

	_, _, err := assembleEntries(context.Background(), f, "owner", "repo", raw, "other")
	if err == nil {
		t.Fatal("assembleEntries() error = nil, want an error")
	}
}
