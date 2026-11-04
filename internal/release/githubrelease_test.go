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
)

// releaseRun is one way release reads a merged release pull request: a
// component's own branch, a locked family or a shared release pull request.
type releaseRun func(f *forgetest.Fake, comps []release.Component, draftUntilAssetsAttach, dryRun bool) ([]release.Result, error)

func runOwn(f *forgetest.Fake, comps []release.Component, draftUntilAssetsAttach, dryRun bool) ([]release.Result, error) {
	return release.Run(context.Background(), f, "owner", "repo", comps[0], testLabels, draftUntilAssetsAttach, dryRun, discardLogger())
}

func runLocked(f *forgetest.Fake, comps []release.Component, draftUntilAssetsAttach, dryRun bool) ([]release.Result, error) {
	return release.RunLocked(
		context.Background(), f, "owner", "repo", "release-me/main", comps, nil, testLabels, draftUntilAssetsAttach, dryRun, discardLogger(),
	)
}

func runShared(f *forgetest.Fake, comps []release.Component, draftUntilAssetsAttach, dryRun bool) ([]release.Result, error) {
	return release.RunShared(
		context.Background(), f, "owner", "repo", "release-me/main", comps, nil, testLabels, draftUntilAssetsAttach, dryRun, discardLogger(),
	)
}

// mergedFake is a forge holding one merged release pull request, number
// 21, on the release branch "release-me/main", carrying label.
func mergedFake(title, label string) *forgetest.Fake {
	pr := forge.PullRequest{Number: 21, Title: title, Labels: []string{label}, Merged: true}
	return &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{{Head: "release-me/main", State: "closed", PullRequests: []forge.PullRequest{pr}}},
		MergeCommits:  map[int]string{21: "4f2a9c1"},
		PRLabels:      map[int][]string{21: {label}},
	}
}

// apiAndWeb are two configured components on the shared release branch,
// api skipping its GitHub release and web keeping its own.
var apiAndWeb = []release.Component{
	{Name: "api", Branch: "release-me/main", TagPrefix: "api/v", Multi: true, SkipGitHubRelease: true},
	{Name: "web", Branch: "release-me/main", TagPrefix: "web/v", Multi: true},
}

func TestReleaseSkipsTheGitHubReleaseButStillTagsAndSwapsTheLabel(t *testing.T) {
	t.Parallel()
	skipMain := release.Component{Name: "main", Branch: "release-me/main", TagPrefix: "v", SkipGitHubRelease: true}
	cases := []struct {
		name                   string
		run                    releaseRun
		fake                   *forgetest.Fake
		comps                  []release.Component
		draftUntilAssetsAttach bool
		dryRun                 bool
		wantTags               []string
		wantReleases           []string
		wantMessages           []string
		wantLabelSwap          bool
	}{
		{
			name: "tags the implicit main component without a release", run: runOwn,
			fake:  mergedFake("chore(main): release v0.2.0", "release: pending"),
			comps: []release.Component{skipMain}, wantTags: []string{"v0.2.0"},
			wantMessages: []string{"tagged v0.2.0, GitHub release skipped"}, wantLabelSwap: true,
		},
		{
			name: "drafts nothing with draft-until-assets-attach on", run: runOwn,
			fake:  mergedFake("chore(main): release v0.2.0", "release: pending"),
			comps: []release.Component{skipMain}, draftUntilAssetsAttach: true, wantTags: []string{"v0.2.0"},
			wantMessages: []string{"tagged v0.2.0, GitHub release skipped"}, wantLabelSwap: true,
		},
		{
			name: "still moves the major tag", run: runOwn,
			fake: mergedFake("chore(main): release v1.4.0", "release: pending"),
			comps: []release.Component{{
				Name: "main", Branch: "release-me/main", TagPrefix: "v", SkipGitHubRelease: true, MajorTag: true,
			}},
			wantTags:      []string{"v1.4.0", "v1"},
			wantMessages:  []string{"tagged v1.4.0, GitHub release skipped"},
			wantLabelSwap: true,
		},
		{
			name: "releases only the locked component that keeps its release", run: runLocked,
			fake:  mergedFake("chore: release v0.3.0 (api, web)", "release: pending"),
			comps: apiAndWeb, wantTags: []string{"api/v0.3.0", "web/v0.3.0"}, wantReleases: []string{"web/v0.3.0"},
			wantMessages:  []string{"tagged api/v0.3.0, GitHub release skipped", "tagged and released web/v0.3.0"},
			wantLabelSwap: true,
		},
		{
			name: "releases only the shared component that keeps its release", run: runShared,
			fake:  mergedFake("chore: release api v1.3.0, web v0.4.1", "release: pending"),
			comps: apiAndWeb, wantTags: []string{"api/v1.3.0", "web/v0.4.1"}, wantReleases: []string{"web/v0.4.1"},
			wantMessages:  []string{"tagged api/v1.3.0, GitHub release skipped", "tagged and released web/v0.4.1"},
			wantLabelSwap: true,
		},
		{
			name: "dry run reports the skipped release and writes nothing", run: runOwn,
			fake:  mergedFake("chore(main): release v0.2.0", "release: pending"),
			comps: []release.Component{skipMain}, dryRun: true,
			wantMessages: []string{"would tag v0.2.0, GitHub release skipped"},
		},
		{
			name: "resumes nothing for a tagged pull request without a release", run: runOwn,
			fake:  mergedFake("chore(main): release v0.2.0", "release: tagged"),
			comps: []release.Component{skipMain}, draftUntilAssetsAttach: true,
		},
		{
			name: "resumes nothing for a tagged shared pull request without a release", run: runShared,
			fake:  mergedFake("chore: release api v1.3.0", "release: tagged"),
			comps: apiAndWeb, draftUntilAssetsAttach: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			results, err := tc.run(tc.fake, tc.comps, tc.draftUntilAssetsAttach, tc.dryRun)
			if err != nil {
				t.Fatalf("release error = %v", err)
			}
			var messages []string
			for _, r := range results {
				messages = append(messages, r.Message)
			}
			if !reflect.DeepEqual(messages, tc.wantMessages) {
				t.Fatalf("messages = %q, want %q", messages, tc.wantMessages)
			}
			var tags []string
			for _, c := range tc.fake.CreatedTags {
				tags = append(tags, c.Tag)
			}
			if !reflect.DeepEqual(tags, tc.wantTags) {
				t.Fatalf("created tags = %q, want %q", tags, tc.wantTags)
			}
			var releases []string
			for _, c := range tc.fake.CreatedReleases {
				releases = append(releases, c.Tag)
			}
			if !reflect.DeepEqual(releases, tc.wantReleases) {
				t.Fatalf("created releases = %q, want %q", releases, tc.wantReleases)
			}
			swapped := len(tc.fake.SetLabelsCalls) == 1 && reflect.DeepEqual(tc.fake.SetLabelsCalls[0].Labels, []string{"release: tagged"})
			if swapped != tc.wantLabelSwap || (!tc.wantLabelSwap && len(tc.fake.SetLabelsCalls) != 0) {
				t.Fatalf("SetLabelsCalls = %+v, want a swap to release: tagged: %v", tc.fake.SetLabelsCalls, tc.wantLabelSwap)
			}
		})
	}
}

func TestTagOnlyReleaseReportsTheMergedHeadForBranchCleanup(t *testing.T) {
	t.Parallel()
	skipMain := release.Component{Name: "main", Branch: "release-me/main", TagPrefix: "v", SkipGitHubRelease: true}
	for _, dryRun := range []bool{false, true} {
		f := mergedFake("chore(main): release v1.4.0", "release: pending")
		f.ListResponses[0].PullRequests[0].HeadSHA = "9c1e4f2a9c1e4f2a9c1e4f2a9c1e4f2a9c1e4f2a"
		results, err := runOwn(f, []release.Component{skipMain}, false, dryRun)
		if err != nil {
			t.Fatalf("Run(dryRun %v) error = %v", dryRun, err)
		}
		if len(results) != 1 || results[0].HeadSHA != "9c1e4f2a9c1e4f2a9c1e4f2a9c1e4f2a9c1e4f2a" {
			t.Fatalf("Run(dryRun %v) = %+v, want the tag-only release to report the merged head", dryRun, results)
		}
		if len(f.CreatedReleases) != 0 {
			t.Fatalf("CreatedReleases = %+v, want none under skip-github-release", f.CreatedReleases)
		}
	}
}
