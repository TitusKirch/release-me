// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"slices"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/forge/forgetest"
	"github.com/ostendorf-cloud/release-me/internal/releasepr"
)

func TestRunRetriesAnUnpublishedDeferredTagWithoutAnotherReleasePR(t *testing.T) {
	for _, tc := range []struct {
		name, config    string
		legacy, deleted bool
	}{
		{name: "single"},
		{name: "shared", config: "release-pull-request: shared\ncomponents:\n  api:\n    path: api\n  web:\n    path: web\n"},
		{name: "locked", config: "versioning: locked\ncomponents:\n  api:\n    path: api\n  web:\n    path: web\n"},
		{name: "legacy per-component draft after switching to shared", config: "components:\n  api:\n    path: api\n  web:\n    path: web\n", legacy: true},
		{
			name:   "legacy per-component draft whose branch is gone after switching to shared",
			config: "components:\n  api:\n    path: api\n  web:\n    path: web\n", legacy: true, deleted: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, remote := newPRRepo(t)
			writeConfig(t, dir, tc.config)
			writeFiles(t, dir, "chore: scaffold packages", map[string]string{"api/README.md": "api\n", "web/README.md": "web\n"})
			cfg := loadConfig(t, dir)
			if tc.config == "" {
				runGit(t, dir, "tag", "v1.0.0")
			} else {
				runGit(t, dir, "tag", "api/v1.0.0")
				runGit(t, dir, "tag", "web/v1.0.0")
			}
			writeFiles(t, dir, "feat: export reports", map[string]string{"api/export.go": "package api\n"})
			cfg.ForceTagCreation = false
			cfg.DraftUntilAssetsAttach = true
			f := newPRFake(remote)
			var out bytes.Buffer
			if err := doRun(t.Context(), dir, cfg, "", f, discardLogger(), false, &out); err != nil {
				t.Fatal(err)
			}
			if len(f.CreatedPullRequests) != 1 {
				t.Fatalf("first PRs = %+v", f.CreatedPullRequests)
			}
			created := f.CreatedPullRequests[0]
			runGit(t, dir, "fetch", "-q", remote, "refs/heads/"+created.Head)
			runGit(t, dir, "merge", "-q", "--no-ff", "FETCH_HEAD", "-m", "Merge release pull request")
			merged := headSHA(t, dir)
			f.ListResponses = []forgetest.ListResponse{{Head: created.Head, State: "closed", PullRequests: []forge.PullRequest{{Number: 9, Title: created.Title, Body: created.Body, Merged: true, HeadSHA: merged, Labels: []string{"release: pending"}}}}}
			f.MergeCommits = map[int]string{9: merged}
			f.PRLabels = map[int][]string{9: {"release: pending"}}
			if err := doRun(t.Context(), dir, cfg, "", f, discardLogger(), false, &out); err != nil {
				t.Fatal(err)
			}
			releases := len(f.CreatedReleases)
			if releases == 0 || len(f.CreatedTags) != 0 {
				t.Fatalf("draft releases=%+v, tags=%+v", f.CreatedReleases, f.CreatedTags)
			}
			f.ListResponses[0].PullRequests[0].Labels = f.PRLabels[9]
			if tc.legacy {
				cfg.ReleasePullRequest = config.ReleasePullRequestShared
			}
			if tc.deleted {
				// GitHub's own "Automatically delete head branches" may have
				// removed the legacy branch: its closed pull request still
				// keeps pr from opening another one for the same version.
				if out, err := exec.CommandContext(t.Context(), "git", "--git-dir", remote, "update-ref", "-d", "refs/heads/"+created.Head).CombinedOutput(); err != nil {
					t.Fatalf("delete %s: %v: %s", created.Head, err, out)
				}
			}
			for retry := 0; retry < 2; retry++ {
				out.Reset()
				if err := doRun(t.Context(), dir, cfg, "", f, discardLogger(), false, &out); err != nil {
					t.Fatal(err)
				}
				if len(f.CreatedPullRequests) != 1 || len(f.CreatedReleases) != releases || len(f.CreatedTags) != 0 {
					t.Fatalf("retry duplicated work: PRs=%+v releases=%+v tags=%+v", f.CreatedPullRequests, f.CreatedReleases, f.CreatedTags)
				}
				if !bytes.Contains(out.Bytes(), []byte("waiting for unpublished release")) || !bytes.Contains(out.Bytes(), []byte("publish or delete its draft")) {
					t.Fatalf("missing draft deferral: %s", out.String())
				}
				if !bytes.Contains(out.Bytes(), []byte("released-tag:")) {
					t.Fatalf("draft not resumed: %s", out.String())
				}
			}
		})
	}
}

func TestDeferredDraftDeferralRequiresAnAbsentTagAndAnUnpublishedRelease(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		tagExists, draft, force bool
		title                   string
		want                    bool
	}{
		{name: "unpublished draft", draft: true, want: true},
		{name: "tag already exists", tagExists: true, draft: true},
		{name: "published release"},
		{name: "forced tag creation", draft: true, force: true},
		{name: "unrelated title", draft: true, title: "feat: add a command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			f.ExistingTags = map[string]bool{"v1.1.0": tc.tagExists}
			f.ReleaseDrafts = map[string]bool{"v1.1.0": tc.draft}
			cfg := config.Default()
			cfg.ForceTagCreation = tc.force
			title := tc.title
			if title == "" {
				title = "chore(main): release v1.1.0"
			}
			got, err := unpublishedDeferredRelease(t.Context(), f, "owner", "repo", cfg, forge.PullRequest{Number: 9, Title: title})
			if err != nil || got != tc.want {
				t.Fatalf("deferred=%v,%v;want %v", got, err, tc.want)
			}
		})
	}
}

// TestDeferredDraftGuardReadsTheRecordedTitlePattern holds back pr for an
// unpublished deferred-tag draft whose pull request was titled with a
// custom pattern its body records, after the configured pattern changed:
// the recorded pattern reads the title first, the legacy default shape
// next, so the current setting never reinterprets it.
func TestDeferredDraftGuardReadsTheRecordedTitlePattern(t *testing.T) {
	components := "components:\n  api:\n    path: api\n  web:\n    path: web\n"
	for _, tc := range []struct {
		name, config, pattern, title, current string
		kind                                  releasepr.TitleKind
		tag                                   string
	}{
		{
			name: "single", pattern: "ship ${component} ${version}", title: "ship main 1.1.0",
			current: "release(${component}): ${version}", kind: releasepr.TitlePerComponent, tag: "v1.1.0",
		},
		{
			name: "shared", config: "release-pull-request: shared\n" + components,
			pattern: "ship ${releases}", title: "ship api v1.1.0, web v0.3.0",
			current: "release ${releases} now", kind: releasepr.TitleShared, tag: "web/v0.3.0",
		},
		{
			name: "locked", config: "versioning: locked\n" + components,
			pattern: "ship ${version} of ${components}", title: "ship 1.1.0 of api, web",
			current: "locked ${version}: ${components}", kind: releasepr.TitleLocked, tag: "web/v1.1.0",
		},
		{
			name: "legacy per-component after switching to shared", config: "release-pull-request: shared\n" + components,
			pattern: "ship ${component} ${version}", title: "ship web 0.3.0",
			kind: releasepr.TitlePerComponent, tag: "web/v0.3.0",
		},
		{
			name: "legacy default title after switching to shared", config: "release-pull-request: shared\n" + components,
			title: "chore(web): release v0.3.0", tag: "web/v0.3.0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := newRepo(t)
			writeConfig(t, dir, tc.config)
			cfg := loadConfig(t, dir)
			cfg.ForceTagCreation = false
			for i := range cfg.Components {
				off := false
				cfg.Components[i].ForceTagCreation = &off
			}
			if tc.current != "" && tc.kind == releasepr.TitlePerComponent {
				cfg.ReleasePullRequestTitle = tc.current
			} else if tc.current != "" {
				cfg.ReleasePullRequestSharedTitle = tc.current
			}
			body := "Release notes\n"
			if tc.pattern != "" {
				p, err := releasepr.NewTitlePattern(tc.pattern, tc.kind)
				if err != nil {
					t.Fatal(err)
				}
				body += releasepr.TitlePatternMarker(p, tc.kind)
			}
			f := newFake()
			f.ReleaseDrafts = map[string]bool{tc.tag: true}
			got, err := unpublishedDeferredRelease(t.Context(), f, "owner", "repo", cfg, forge.PullRequest{Number: 9, Title: tc.title, Body: body})
			if err != nil || !got {
				t.Fatalf("deferred = %v, %v, want the draft %s held back", got, err, tc.tag)
			}
		})
	}
}

// deferredLookupForge records remote probes made while deciding whether
// an unpublished release should hold back a new release pull request.
type deferredLookupForge struct {
	*forgetest.Fake
	tags, drafts []string
}

func (f *deferredLookupForge) TagExists(ctx context.Context, owner, repo, tag string) (bool, error) {
	f.tags = append(f.tags, tag)
	return f.Fake.TagExists(ctx, owner, repo, tag)
}

func (f *deferredLookupForge) Release(ctx context.Context, owner, repo, tag string) (forge.Release, error) {
	f.drafts = append(f.drafts, tag)
	return f.Fake.Release(ctx, owner, repo, tag)
}

func TestReleaseDeferralOnlyChecksTheNewestMergedTaggedPullRequest(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse listing %t", reverse), func(t *testing.T) {
			prs := []forge.PullRequest{
				{Number: 10, Title: "chore(main): release v1.0.0", Merged: true, Labels: []string{"release: tagged"}},
				{Number: 20, Title: "chore(main): release v1.1.0", Merged: true, Labels: []string{"release: tagged"}},
				{Number: 30, Title: "chore(main): release v1.2.0", Labels: []string{"release: tagged"}},
			}
			if reverse {
				slices.Reverse(prs)
			}
			f := &deferredLookupForge{Fake: &forgetest.Fake{
				ListResponses: []forgetest.ListResponse{{Head: mainBranch, State: "closed", PullRequests: prs}},
				ExistingTags:  map[string]bool{"v1.1.0": true},
			}}
			cfg := config.Default()
			cfg.ForceTagCreation, cfg.DraftUntilAssetsAttach = false, true
			deferral, err := mergedReleaseDeferral(t.Context(), f, "owner", "repo", mainBranch, cfg, discardLogger())
			if err != nil || deferral != "" {
				t.Fatalf("deferral = %q, %v; want empty, nil", deferral, err)
			}
			if !reflect.DeepEqual(f.tags, []string{"v1.1.0"}) || len(f.drafts) != 0 {
				t.Fatalf("remote lookups: tags = %q, drafts = %q; want only newest merged tag", f.tags, f.drafts)
			}
		})
	}
}
