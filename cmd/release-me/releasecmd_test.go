// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/forge/forgetest"
)

func TestDoReleaseTagsAndReleasesAMergedPendingPullRequest(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "chore: initial commit")

	pr := forge.PullRequest{
		Number: 9, Title: "chore(main): release v0.1.0",
		Body: "Release v0.1.0\n\n<details>\n<summary>Changelog</summary>\n\n## v0.1.0 (2026-01-02)\n\n### Features\n\n" +
			"- add the thing\n\n</details>\n",
		Labels: []string{"release: pending"}, Merged: true,
	}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: mainBranch, State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
		MergeCommits: map[int]string{9: "deadbeef"},
		PRLabels:     map[int][]string{9: {"release: pending"}},
	}
	var stdout bytes.Buffer

	if err := doRelease(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doRelease() error = %v", err)
	}

	if len(f.CreatedTags) != 1 || f.CreatedTags[0].Tag != "v0.1.0" {
		t.Fatalf("CreatedTags = %+v", f.CreatedTags)
	}
	if len(f.CreatedReleases) != 1 {
		t.Fatalf("CreatedReleases = %+v", f.CreatedReleases)
	}
	if len(f.SetLabelsCalls) != 1 {
		t.Fatalf("SetLabelsCalls = %+v", f.SetLabelsCalls)
	}
	if !strings.Contains(stdout.String(), "tagged and released v0.1.0") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestDoReleaseStillFinishesAnAlreadyTaggedPullRequest(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "chore: initial commit")

	pr := forge.PullRequest{Number: 9, Title: "chore(main): release v0.1.0", Labels: []string{"release: pending"}, Merged: true}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: mainBranch, State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
		MergeCommits: map[int]string{9: "deadbeef"},
		ExistingTags: map[string]bool{"v0.1.0": true},
		PRLabels:     map[int][]string{9: {"release: pending"}},
	}
	var stdout bytes.Buffer

	if err := doRelease(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doRelease() error = %v", err)
	}

	// The tag existing already means an earlier run got partway through;
	// this run still creates the release and swaps the label, instead of
	// leaving the pull request pending forever.
	if !strings.Contains(stdout.String(), "already tagged v0.1.0") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if len(f.CreatedReleases) != 1 {
		t.Fatalf("CreatedReleases = %+v, want the release created", f.CreatedReleases)
	}
	if len(f.SetLabelsCalls) != 1 {
		t.Fatalf("SetLabelsCalls = %+v, want the label swapped", f.SetLabelsCalls)
	}
}

func TestDoReleasePrintsNothingToReleaseWithoutAMergedPendingPullRequest(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "chore: initial commit")

	f := &forgetest.Fake{}
	var stdout bytes.Buffer

	if err := doRelease(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doRelease() error = %v", err)
	}
	if stdout.String() != "nothing to release\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "nothing to release\n")
	}
}

func TestDoReleaseDryRunWritesNothing(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "chore: initial commit")

	pr := forge.PullRequest{Number: 9, Title: "chore(main): release v0.1.0", Labels: []string{"release: pending"}, Merged: true}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: mainBranch, State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
	}
	var stdout bytes.Buffer

	if err := doRelease(t.Context(), dir, config.Default(), "", f, discardLogger(), true, &stdout); err != nil {
		t.Fatalf("doRelease() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "would tag and release v0.1.0") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if len(f.CreatedTags) != 0 || len(f.CreatedReleases) != 0 || len(f.SetLabelsCalls) != 0 {
		t.Fatalf("dry run wrote something: tags=%+v releases=%+v labels=%+v", f.CreatedTags, f.CreatedReleases, f.SetLabelsCalls)
	}
}
