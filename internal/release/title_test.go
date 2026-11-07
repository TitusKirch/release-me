// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package release_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/forge/forgetest"
	"github.com/ostendorf-cloud/release-me/internal/release"
	"github.com/ostendorf-cloud/release-me/internal/releasepr"
)

// mustTitlePatterns returns the patterns release reads a title of kind back
// with when configured is set.
func mustTitlePatterns(t *testing.T, configured string, kind releasepr.TitleKind) []releasepr.TitlePattern {
	t.Helper()
	patterns, err := releasepr.TitlePatterns(configured, kind)
	if err != nil {
		t.Fatalf("TitlePatterns(%q) error = %v", configured, err)
	}
	return patterns
}

func TestRunReadsTheVersionWithTheConfiguredTitlePatternOrTheDefault(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		title string
	}{
		{name: "the configured pattern", title: "chore(main): release frontend 0.4.0"},
		{name: "a pull request opened before the pattern", title: "chore(frontend): release v0.4.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pr := forge.PullRequest{Number: 9, Title: tc.title, Labels: []string{"release: pending"}, Merged: true}
			f := &forgetest.Fake{
				ListResponses: []forgetest.ListResponse{
					{Head: "release-me/frontend", State: "closed", PullRequests: []forge.PullRequest{pr}},
				},
				MergeCommits: map[int]string{9: "deadbeef"},
				PRLabels:     map[int][]string{9: {"release: pending"}},
			}
			comp := release.Component{
				Name: "frontend", Branch: "release-me/frontend", TagPrefix: "frontend/v", Multi: true,
				Titles: mustTitlePatterns(t, "chore(${branch}): release ${component} ${version}", releasepr.TitlePerComponent),
			}

			results, err := release.Run(context.Background(), f, "owner", "repo", comp, testLabels, false, false, discardLogger())
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if len(results) != 1 || results[0].Tag != "frontend/v0.4.0" {
				t.Fatalf("Run() = %+v, want one release tagged frontend/v0.4.0", results)
			}
		})
	}
}

func TestRunRefusesACustomTitleNamingADifferentComponent(t *testing.T) {
	t.Parallel()
	pr := forge.PullRequest{Number: 9, Title: "chore(main): release backend 0.4.0", Labels: []string{"release: pending"}, Merged: true}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: "release-me/frontend", State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
	}
	comp := release.Component{
		Name: "frontend", Branch: "release-me/frontend", TagPrefix: "frontend/v", Multi: true,
		Titles: mustTitlePatterns(t, "chore(${branch}): release ${component} ${version}", releasepr.TitlePerComponent),
	}
	if _, err := release.Run(context.Background(), f, "owner", "repo", comp, testLabels, false, false, discardLogger()); err == nil {
		t.Fatal("Run() error = nil, want an error naming the mismatched title")
	}
}

func TestRunSharedAndRunLockedReadTheConfiguredTitlePatternOrTheDefault(t *testing.T) {
	t.Parallel()
	comps := []release.Component{
		{Name: "api", Branch: "release-me/main", TagPrefix: "api/v", Multi: true},
		{Name: "web", Branch: "release-me/main", TagPrefix: "web/v", Multi: true},
	}
	cases := []struct {
		name     string
		title    string
		kind     releasepr.TitleKind
		pattern  string
		wantTags []string
	}{
		{
			name: "shared", title: "build: ship api v1.3.0, web v0.4.1", kind: releasepr.TitleShared,
			pattern: "build: ship ${releases}", wantTags: []string{"api/v1.3.0", "web/v0.4.1"},
		},
		{
			name: "shared opened before the pattern", title: "chore: release api v1.3.0, web v0.4.1", kind: releasepr.TitleShared,
			pattern: "build: ship ${releases}", wantTags: []string{"api/v1.3.0", "web/v0.4.1"},
		},
		{
			name: "locked", title: "release 2.0.0 of api, web", kind: releasepr.TitleLocked,
			pattern: "release ${version} of ${components}", wantTags: []string{"api/v2.0.0", "web/v2.0.0"},
		},
		{
			name: "locked opened before the pattern", title: "chore: release v2.0.0 (api, web)", kind: releasepr.TitleLocked,
			pattern: "release ${version} of ${components}", wantTags: []string{"api/v2.0.0", "web/v2.0.0"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pr := forge.PullRequest{Number: 12, Title: tc.title, Body: sharedBody, Labels: []string{"release: pending"}, Merged: true}
			f := &forgetest.Fake{
				ListResponses: []forgetest.ListResponse{
					{Head: "release-me/main", State: "closed", PullRequests: []forge.PullRequest{pr}},
				},
				MergeCommits: map[int]string{12: "cafe1234"},
				PRLabels:     map[int][]string{12: {"release: pending"}},
			}
			run := release.RunShared
			if tc.kind == releasepr.TitleLocked {
				run = release.RunLocked
			}
			patterns := mustTitlePatterns(t, tc.pattern, tc.kind)
			if _, err := run(context.Background(), f, "owner", "repo", "release-me/main", comps, patterns, testLabels, false, false, discardLogger()); err != nil {
				t.Fatalf("run() error = %v", err)
			}
			var tags []string
			for _, tag := range f.CreatedTags {
				tags = append(tags, tag.Tag)
			}
			if !reflect.DeepEqual(tags, tc.wantTags) {
				t.Fatalf("CreatedTags = %v, want %v", tags, tc.wantTags)
			}
		})
	}
}

func TestRunReadsAPullRequestTitledWithAPreviousPattern(t *testing.T) {
	t.Parallel()
	previous := mustTitlePatterns(t, "release ${component} ${version}", releasepr.TitlePerComponent)[0]
	pr := forge.PullRequest{
		Number: 9, Title: "release frontend 0.4.0", Labels: []string{"release: pending"}, Merged: true,
		Body: "Release v0.4.0\n" + releasepr.TitlePatternMarker(previous, releasepr.TitlePerComponent),
	}
	f := &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{
			{Head: "release-me/frontend", State: "closed", PullRequests: []forge.PullRequest{pr}},
		},
		MergeCommits: map[int]string{9: "deadbeef"},
		PRLabels:     map[int][]string{9: {"release: pending"}},
	}
	comp := release.Component{
		Name: "frontend", Branch: "release-me/frontend", TagPrefix: "frontend/v", Multi: true,
		Titles: mustTitlePatterns(t, "ship(${component}): ${version}", releasepr.TitlePerComponent),
	}
	results, err := release.Run(context.Background(), f, "owner", "repo", comp, testLabels, false, false, discardLogger())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(results) != 1 || results[0].Tag != "frontend/v0.4.0" {
		t.Fatalf("Run() = %+v, want one release tagged frontend/v0.4.0", results)
	}
}
