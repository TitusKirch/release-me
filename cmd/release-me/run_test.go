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
	"github.com/ostendorf-cloud/release-me/internal/releasepr"
)

// TestRunIsPRThenRelease exercises doPR followed by doRelease the same way
// runRun wires them, since runRun itself goes through setup, which builds a
// real GitHub forge and would reach the network.
func TestRunIsPRThenRelease(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	var stdout bytes.Buffer

	if err := doPR(t.Context(), dir, config.Default(), f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doPR() error = %v", err)
	}
	if len(f.CreatedPullRequests) != 1 {
		t.Fatalf("CreatedPullRequests = %+v, want one", f.CreatedPullRequests)
	}
	created := f.CreatedPullRequests[0]

	// The release pull request merges; the forge reports it closed, merged
	// and still carrying the pending label.
	f.ListResponses = []forgetest.ListResponse{
		{Head: releasepr.Branch, State: "closed", PullRequests: []forge.PullRequest{
			{Number: 1, Title: created.Title, Body: created.Body, Labels: []string{"release: pending"}, Merged: true},
		}},
	}
	f.MergeCommits = map[int]string{1: "deadbeef"}
	f.PRLabels = map[int][]string{1: {"release: pending"}}

	if err := doRelease(t.Context(), dir, config.Default(), f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doRelease() error = %v", err)
	}

	if len(f.CreatedTags) != 1 || f.CreatedTags[0].Tag != "v0.1.0" {
		t.Fatalf("CreatedTags = %+v", f.CreatedTags)
	}
	if len(f.CreatedReleases) != 1 {
		t.Fatalf("CreatedReleases = %+v", f.CreatedReleases)
	}
	if !strings.Contains(stdout.String(), "created chore(main): release v0.1.0") {
		t.Fatalf("stdout = %q, want the pr step reported", stdout.String())
	}
	if !strings.Contains(stdout.String(), "tagged and released v0.1.0") {
		t.Fatalf("stdout = %q, want the release step reported", stdout.String())
	}
}
