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

// releaseRun is one way release reads a merged release pull request: a
// component's own branch, a locked family or a shared release pull request.
// mergedFake is a forge holding one merged release pull request, number
// 21, on the release branch "release-me/main", carrying label.
// releaseDraft is one created release's tag and whether it was a draft.
type releaseDraft struct {
	Tag   string
	Draft bool
}

func TestReleaseDraftsTheReleaseUnderDraftAndLeavesItForAHuman(t *testing.T) {
	t.Parallel()
	draftMain := release.Component{Name: "main", Branch: "release-me/main", TagPrefix: "v", Draft: true}
	apiDraft := []release.Component{
		{Name: "api", Branch: "release-me/main", TagPrefix: "api/v", Multi: true, Draft: true},
		{Name: "web", Branch: "release-me/main", TagPrefix: "web/v", Multi: true},
	}
	withDraft := func(f *forgetest.Fake, tag string) *forgetest.Fake {
		f.ReleaseDrafts = map[string]bool{tag: true}
		return f
	}
	cases := []struct {
		name                   string
		run                    releaseRun
		fake                   *forgetest.Fake
		comps                  []release.Component
		draftUntilAssetsAttach bool
		dryRun                 bool
		wantReleases           []releaseDraft
		wantMessages           []string
		wantLabelSwap          bool
	}{
		{
			name: "drafts the release of the implicit main component", run: runOwn,
			fake:         mergedFake("chore(main): release v0.2.0", "release: pending"),
			comps:        []release.Component{draftMain},
			wantReleases: []releaseDraft{{Tag: "v0.2.0", Draft: true}},
			wantMessages: []string{"tagged and drafted v0.2.0"}, wantLabelSwap: true,
		},
		{
			name: "drafts no second release when a rerun finds the first draft", run: runOwn,
			fake:         withDraft(mergedFake("chore(main): release v0.2.0", "release: pending"), "v0.2.0"),
			comps:        []release.Component{draftMain},
			wantMessages: []string{"tagged and drafted v0.2.0"}, wantLabelSwap: true,
		},
		{
			name: "resumes no draft left on purpose", run: runOwn,
			fake:  withDraft(mergedFake("chore(main): release v0.2.0", "release: tagged"), "v0.2.0"),
			comps: []release.Component{draftMain},
		},
		{
			name: "resumes the draft when draft-until-assets-attach is on too", run: runOwn,
			fake:  withDraft(mergedFake("chore(main): release v0.2.0", "release: tagged"), "v0.2.0"),
			comps: []release.Component{draftMain}, draftUntilAssetsAttach: true,
			wantMessages: []string{"resuming unpublished release v0.2.0"},
		},
		{
			name: "drafts only the locked component that asks for it", run: runLocked,
			fake:         mergedFake("chore: release v0.3.0 (api, web)", "release: pending"),
			comps:        apiDraft,
			wantReleases: []releaseDraft{{Tag: "api/v0.3.0", Draft: true}, {Tag: "web/v0.3.0"}},
			wantMessages: []string{"tagged and drafted api/v0.3.0", "tagged and released web/v0.3.0"}, wantLabelSwap: true,
		},
		{
			name: "drafts only the shared component that asks for it", run: runShared,
			fake:         mergedFake("chore: release api v1.3.0, web v0.4.1", "release: pending"),
			comps:        apiDraft,
			wantReleases: []releaseDraft{{Tag: "api/v1.3.0", Draft: true}, {Tag: "web/v0.4.1"}},
			wantMessages: []string{"tagged and drafted api/v1.3.0", "tagged and released web/v0.4.1"}, wantLabelSwap: true,
		},
		{
			name: "dry run reports the draft and writes nothing", run: runOwn,
			fake:  mergedFake("chore(main): release v0.2.0", "release: pending"),
			comps: []release.Component{draftMain}, dryRun: true,
			wantMessages: []string{"would tag and draft v0.2.0"},
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
			var releases []releaseDraft
			for _, c := range tc.fake.CreatedReleases {
				releases = append(releases, releaseDraft{Tag: c.Tag, Draft: c.Draft})
			}
			if !reflect.DeepEqual(releases, tc.wantReleases) {
				t.Fatalf("created releases = %+v, want %+v", releases, tc.wantReleases)
			}
			if swapped := len(tc.fake.SetLabelsCalls) == 1; swapped != tc.wantLabelSwap {
				t.Fatalf("SetLabelsCalls = %+v, want a swap: %v", tc.fake.SetLabelsCalls, tc.wantLabelSwap)
			}
		})
	}
}

func TestSkipGitHubReleaseWinsOverDraft(t *testing.T) {
	t.Parallel()
	both := release.Component{Name: "main", Branch: "release-me/main", TagPrefix: "v", SkipGitHubRelease: true, Draft: true}
	for _, dryRun := range []bool{false, true} {
		f := mergedFake("chore(main): release v1.4.0", "release: pending")
		results, err := runOwn(f, []release.Component{both}, false, dryRun)
		if err != nil {
			t.Fatalf("Run(dryRun %v) error = %v", dryRun, err)
		}
		want := "tagged v1.4.0, GitHub release skipped"
		if dryRun {
			want = "would tag v1.4.0, GitHub release skipped"
		}
		if len(results) != 1 || results[0].Message != want {
			t.Fatalf("Run(dryRun %v) = %+v, want one result reporting %q", dryRun, results, want)
		}
		if len(f.CreatedReleases) != 0 {
			t.Fatalf("CreatedReleases = %+v, want none: skip-github-release leaves no release to draft", f.CreatedReleases)
		}
		if !dryRun && (len(f.CreatedTags) != 1 || len(f.SetLabelsCalls) != 1) {
			t.Fatalf("CreatedTags = %+v, SetLabelsCalls = %+v, want the tag and the label swap", f.CreatedTags, f.SetLabelsCalls)
		}
	}
}

// releaseRun is one way release reads a merged release pull request: a
// component's own branch, a locked family or a shared release pull request.
// mergedTagOnPublishFake is a forge holding one merged, pending release pull request,
// number 21, on the release branch "release-me/main".
func mergedTagOnPublishFake(title string) *forgetest.Fake {
	const label = "release: pending"
	pr := forge.PullRequest{Number: 21, Title: title, Labels: []string{label}, Merged: true}
	return &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{{Head: "release-me/main", State: "closed", PullRequests: []forge.PullRequest{pr}}},
		MergeCommits:  map[int]string{21: "4f2a9c1"},
		PRLabels:      map[int][]string{21: {label}},
	}
}

// releaseTarget is one created release's tag and the commit it hands the
// forge to create a missing tag on.
type releaseTarget struct {
	Tag    string
	Target string
}

func TestReleaseLeavesTheTagToThePublishWithoutForceTagCreation(t *testing.T) {
	t.Parallel()
	onPublish := release.Component{Name: "main", Branch: "release-me/main", TagPrefix: "v", TagOnPublish: true}
	apiOnPublish := []release.Component{
		{Name: "api", Branch: "release-me/main", TagPrefix: "api/v", Multi: true, TagOnPublish: true},
		{Name: "web", Branch: "release-me/main", TagPrefix: "web/v", Multi: true},
	}
	cases := []struct {
		name                   string
		run                    releaseRun
		fake                   *forgetest.Fake
		comps                  []release.Component
		draftUntilAssetsAttach bool
		dryRun                 bool
		wantTags               []string
		wantReleases           []releaseTarget
		wantMessages           []string
	}{
		{
			name: "hands a published release the merge commit to tag", run: runOwn,
			fake:         mergedTagOnPublishFake("chore(main): release v0.2.0"),
			comps:        []release.Component{onPublish},
			wantReleases: []releaseTarget{{Tag: "v0.2.0", Target: "4f2a9c1"}},
			wantMessages: []string{"tagged and released v0.2.0"},
		},
		{
			name: "leaves the tag of a draft to its publish", run: runOwn,
			fake:  mergedTagOnPublishFake("chore(main): release v0.2.0"),
			comps: []release.Component{onPublish}, draftUntilAssetsAttach: true,
			wantReleases: []releaseTarget{{Tag: "v0.2.0", Target: "4f2a9c1"}},
			wantMessages: []string{"drafted v0.2.0, tagged on publish"},
		},
		{
			name: "still moves the major tag", run: runOwn,
			fake: mergedTagOnPublishFake("chore(main): release v1.4.0"),
			comps: []release.Component{{
				Name: "main", Branch: "release-me/main", TagPrefix: "v", TagOnPublish: true, MajorTag: true,
			}},
			wantTags:     []string{"v1"},
			wantReleases: []releaseTarget{{Tag: "v1.4.0", Target: "4f2a9c1"}},
			wantMessages: []string{"tagged and released v1.4.0"},
		},
		{
			name: "tags only the shared component that forces its tag", run: runShared,
			fake:  mergedTagOnPublishFake("chore: release api v1.3.0, web v0.4.1"),
			comps: apiOnPublish, draftUntilAssetsAttach: true,
			wantTags:     []string{"web/v0.4.1"},
			wantReleases: []releaseTarget{{Tag: "api/v1.3.0", Target: "4f2a9c1"}, {Tag: "web/v0.4.1"}},
			wantMessages: []string{"drafted api/v1.3.0, tagged on publish", "tagged and released web/v0.4.1"},
		},
		{
			name: "tags only the locked component that forces its tag", run: runLocked,
			fake:  mergedTagOnPublishFake("chore: release v0.3.0 (api, web)"),
			comps: apiOnPublish, draftUntilAssetsAttach: true,
			wantTags:     []string{"web/v0.3.0"},
			wantReleases: []releaseTarget{{Tag: "api/v0.3.0", Target: "4f2a9c1"}, {Tag: "web/v0.3.0"}},
			wantMessages: []string{"drafted api/v0.3.0, tagged on publish", "tagged and released web/v0.3.0"},
		},
		{
			name: "dry run reports the tag left to the publish", run: runOwn,
			fake:  mergedTagOnPublishFake("chore(main): release v0.2.0"),
			comps: []release.Component{onPublish}, draftUntilAssetsAttach: true, dryRun: true,
			wantMessages: []string{"would draft v0.2.0, tagged on publish"},
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
			var releases []releaseTarget
			for _, c := range tc.fake.CreatedReleases {
				releases = append(releases, releaseTarget{Tag: c.Tag, Target: c.Target})
			}
			if !reflect.DeepEqual(releases, tc.wantReleases) {
				t.Fatalf("created releases = %+v, want %+v", releases, tc.wantReleases)
			}
		})
	}
}

func TestSkipGitHubReleaseTagsEvenWithoutForceTagCreation(t *testing.T) {
	t.Parallel()
	comp := release.Component{Name: "main", Branch: "release-me/main", TagPrefix: "v", SkipGitHubRelease: true, TagOnPublish: true}
	f := mergedFake("chore(main): release v1.4.0", "release: pending")
	results, err := runOwn(f, []release.Component{comp}, true, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(f.CreatedTags) != 1 || f.CreatedTags[0].Tag != "v1.4.0" {
		t.Fatalf("CreatedTags = %+v, want v1.4.0: no release publish would create it", f.CreatedTags)
	}
	if len(f.CreatedReleases) != 0 {
		t.Fatalf("CreatedReleases = %+v, want none", f.CreatedReleases)
	}
	if len(results) != 1 || results[0].Message != "tagged v1.4.0, GitHub release skipped" {
		t.Fatalf("Run() = %+v, want the skipped release reported", results)
	}
}

// releaseRun is one way release reads a merged release pull request: a
// component's own branch, a locked family or a shared release pull request.
// mergedPendingFake is a forge holding one merged, pending release pull request,
// number 21, on the release branch "release-me/main".
func mergedPendingFake(title string) *forgetest.Fake {
	const label = "release: pending"
	pr := forge.PullRequest{Number: 21, Title: title, Labels: []string{label}, Merged: true}
	return &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{{Head: "release-me/main", State: "closed", PullRequests: []forge.PullRequest{pr}}},
		MergeCommits:  map[int]string{21: "4f2a9c1"},
		PRLabels:      map[int][]string{21: {label}},
	}
}

func TestReleaseNamesTheReleaseWithOrWithoutTheV(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		run       releaseRun
		fake      *forgetest.Fake
		comps     []release.Component
		wantNames []string
	}{
		{
			name: "keeps the v for the implicit main component by default", run: runOwn,
			fake:      mergedPendingFake("chore(main): release v0.2.0"),
			comps:     []release.Component{mainComponent},
			wantNames: []string{"v0.2.0"},
		},
		{
			name: "drops the v for the implicit main component", run: runOwn,
			fake:      mergedPendingFake("chore(main): release v0.2.0"),
			comps:     []release.Component{{Name: "main", Branch: "release-me/main", TagPrefix: "v", OmitVInReleaseName: true}},
			wantNames: []string{"0.2.0"},
		},
		{
			name: "drops the v after a configured component's name", run: runOwn,
			fake: mergedPendingFake("chore(api): release v1.3.0-rc.1"),
			comps: []release.Component{{
				Name: "api", Branch: "release-me/main", TagPrefix: "api@v", Multi: true, OmitVInReleaseName: true,
			}},
			wantNames: []string{"api 1.3.0-rc.1"},
		},
		{
			name: "drops the v only for the shared component that asks", run: runShared,
			fake: mergedPendingFake("chore: release api v1.3.0, web v0.4.1"),
			comps: []release.Component{
				{Name: "api", Branch: "release-me/main", TagPrefix: "api/v", Multi: true, OmitVInReleaseName: true},
				{Name: "web", Branch: "release-me/main", TagPrefix: "web/v", Multi: true},
			},
			wantNames: []string{"api 1.3.0", "web v0.4.1"},
		},
		{
			name: "drops the v for every locked component that asks", run: runLocked,
			fake: mergedPendingFake("chore: release v0.3.0 (api, web)"),
			comps: []release.Component{
				{Name: "api", Branch: "release-me/main", TagPrefix: "api/v", Multi: true, OmitVInReleaseName: true},
				{Name: "web", Branch: "release-me/main", TagPrefix: "web/v", Multi: true, OmitVInReleaseName: true},
			},
			wantNames: []string{"api 0.3.0", "web 0.3.0"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := tc.run(tc.fake, tc.comps, false, false); err != nil {
				t.Fatalf("release error = %v", err)
			}
			var names []string
			for _, c := range tc.fake.CreatedReleases {
				names = append(names, c.Name)
			}
			if !reflect.DeepEqual(names, tc.wantNames) {
				t.Fatalf("release names = %q, want %q", names, tc.wantNames)
			}
		})
	}
}
