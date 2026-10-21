// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package release_test

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/forge/forgetest"
	"github.com/ostendorf-cloud/release-me/internal/release"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestVersionFromTitle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		title  string
		want   string
		wantOK bool
	}{
		{name: "reads the version", title: "chore(main): release v1.2.3", want: "1.2.3", wantOK: true},
		{name: "rejects an unrelated title", title: "feat: add the next command"},
		{name: "rejects a title without the v prefix", title: "chore(main): release 1.2.3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := release.VersionFromTitle(tc.title)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("VersionFromTitle(%q) = (%q, %v), want (%q, %v)", tc.title, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestRunTagsReleasesAndSwapsTheLabel(t *testing.T) {
	t.Parallel()
	pr := forge.PullRequest{
		Number: 9, Title: "chore(main): release v0.2.0",
		Body:   "Release v0.2.0\n\n<details>\n<summary>Changelog</summary>\n\n## v0.2.0 (2026-01-02)\n\n### Features\n\n- add the thing\n\n</details>\n",
		Labels: []string{"release: pending"}, Merged: true,
	}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: "release-me/main", State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
		MergeCommits: map[int]string{9: "deadbeef"},
		PRLabels:     map[int][]string{9: {"release: pending"}},
	}

	results, err := release.Run(context.Background(), f, "owner", "repo", "release-me/main", "release: pending", "release: tagged", false, discardLogger())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 1 || results[0].Tag != "v0.2.0" || results[0].Skipped {
		t.Fatalf("Run() = %+v", results)
	}

	if len(f.CreatedTags) != 1 || f.CreatedTags[0] != (forgetest.CreateTagCall{Owner: "owner", Repo: "repo", Tag: "v0.2.0", SHA: "deadbeef"}) {
		t.Fatalf("CreatedTags = %+v", f.CreatedTags)
	}
	if len(f.CreatedReleases) != 1 {
		t.Fatalf("CreatedReleases = %+v", f.CreatedReleases)
	}
	wantBody := "## v0.2.0 (2026-01-02)\n\n### Features\n\n- add the thing"
	if f.CreatedReleases[0].Body != wantBody {
		t.Fatalf("release body = %q, want %q", f.CreatedReleases[0].Body, wantBody)
	}
	if len(f.SetLabelsCalls) != 1 || !reflect.DeepEqual(f.SetLabelsCalls[0].Labels, []string{"release: tagged"}) {
		t.Fatalf("SetLabelsCalls = %+v", f.SetLabelsCalls)
	}
}

func TestRunStillReleasesAndSwapsTheLabelWhenTheTagAlreadyExists(t *testing.T) {
	t.Parallel()
	pr := forge.PullRequest{Number: 9, Title: "chore(main): release v0.2.0", Labels: []string{"release: pending"}, Merged: true}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: "release-me/main", State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
		MergeCommits: map[int]string{9: "deadbeef"},
		ExistingTags: map[string]bool{"v0.2.0": true},
		PRLabels:     map[int][]string{9: {"release: pending"}},
	}

	results, err := release.Run(context.Background(), f, "owner", "repo", "release-me/main", "release: pending", "release: tagged", false, discardLogger())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 1 || !results[0].Skipped || results[0].Message != "already tagged v0.2.0" {
		t.Fatalf("Run() = %+v, want an already-tagged result", results)
	}
	// The tag already existing means an earlier run got partway through;
	// this run still finishes the job instead of leaving it stuck pending.
	if len(f.CreatedReleases) != 1 {
		t.Fatalf("CreatedReleases = %+v, want the release created", f.CreatedReleases)
	}
	if len(f.SetLabelsCalls) != 1 || !reflect.DeepEqual(f.SetLabelsCalls[0].Labels, []string{"release: tagged"}) {
		t.Fatalf("SetLabelsCalls = %+v, want the label swapped", f.SetLabelsCalls)
	}
}

func TestRunToleratesAReleaseThatAlreadyExistsTooWhenTheTagDoes(t *testing.T) {
	t.Parallel()
	pr := forge.PullRequest{Number: 9, Title: "chore(main): release v0.2.0", Labels: []string{"release: pending"}, Merged: true}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: "release-me/main", State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
		MergeCommits:     map[int]string{9: "deadbeef"},
		ExistingTags:     map[string]bool{"v0.2.0": true},
		ExistingReleases: map[string]bool{"v0.2.0": true},
		PRLabels:         map[int][]string{9: {"release: pending"}},
	}

	results, err := release.Run(context.Background(), f, "owner", "repo", "release-me/main", "release: pending", "release: tagged", false, discardLogger())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 1 || !results[0].Skipped {
		t.Fatalf("Run() = %+v, want an already-tagged result", results)
	}
	if len(f.SetLabelsCalls) != 1 {
		t.Fatalf("SetLabelsCalls = %+v, want the label swapped", f.SetLabelsCalls)
	}
}

func TestRunIgnoresAPullRequestWithoutThePendingLabel(t *testing.T) {
	t.Parallel()
	pr := forge.PullRequest{Number: 9, Title: "chore(main): release v0.2.0", Labels: []string{"release: tagged"}, Merged: true}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: "release-me/main", State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
	}

	results, err := release.Run(context.Background(), f, "owner", "repo", "release-me/main", "release: pending", "release: tagged", false, discardLogger())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Run() = %+v, want no results", results)
	}
}

func TestRunIgnoresAnUnmergedPullRequest(t *testing.T) {
	t.Parallel()
	pr := forge.PullRequest{Number: 9, Title: "chore(main): release v0.2.0", Labels: []string{"release: pending"}, Merged: false}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: "release-me/main", State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
	}

	results, err := release.Run(context.Background(), f, "owner", "repo", "release-me/main", "release: pending", "release: tagged", false, discardLogger())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Run() = %+v, want no results", results)
	}
}

func TestRunDryRunChangesNothing(t *testing.T) {
	t.Parallel()
	pr := forge.PullRequest{Number: 9, Title: "chore(main): release v0.2.0", Labels: []string{"release: pending"}, Merged: true}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: "release-me/main", State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
		MergeCommits: map[int]string{9: "deadbeef"},
	}

	results, err := release.Run(context.Background(), f, "owner", "repo", "release-me/main", "release: pending", "release: tagged", true, discardLogger())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 1 || results[0].Message == "" {
		t.Fatalf("Run() = %+v", results)
	}
	if len(f.CreatedTags) != 0 || len(f.CreatedReleases) != 0 || len(f.SetLabelsCalls) != 0 {
		t.Fatalf("dry run wrote something: tags=%+v releases=%+v labels=%+v", f.CreatedTags, f.CreatedReleases, f.SetLabelsCalls)
	}
}
