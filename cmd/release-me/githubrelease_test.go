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

// mergedReleaseFake is a forge holding one merged, pending release pull
// request for the implicit main component, number 9, at v0.1.0.
func mergedReleaseFake() *forgetest.Fake {
	pr := forge.PullRequest{
		Number: 9, Title: "chore(main): release v0.1.0",
		Body: "Release v0.1.0\n\n<details>\n<summary>Changelog</summary>\n\n## v0.1.0 (2026-01-02)\n\n### Features\n\n" +
			"- add the thing\n\n</details>\n",
		Labels: []string{"release: pending"}, Merged: true,
	}
	return &forgetest.Fake{
		ListResponses: []forgetest.ListResponse{{Head: mainBranch, State: "closed", PullRequests: []forge.PullRequest{pr}}},
		MergeCommits:  map[int]string{9: "deadbeef"},
		PRLabels:      map[int][]string{9: {"release: pending"}},
	}
}

func TestDoReleaseWithSkipGitHubReleaseTagsAndReportsTheTagWithoutARelease(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	f := mergedReleaseFake()
	cfg := config.Default()
	cfg.SkipGitHubRelease = true
	var stdout bytes.Buffer

	out := &runOutputs{}
	if err := doRelease(withOutputs(t.Context(), out), dir, cfg, "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doRelease() error = %v", err)
	}

	if len(f.CreatedTags) != 1 || f.CreatedTags[0].Tag != "v0.1.0" {
		t.Fatalf("CreatedTags = %+v, want v0.1.0", f.CreatedTags)
	}
	if len(f.CreatedReleases) != 0 {
		t.Fatalf("CreatedReleases = %+v, want none", f.CreatedReleases)
	}
	if len(f.SetLabelsCalls) != 1 {
		t.Fatalf("SetLabelsCalls = %+v, want the label swapped", f.SetLabelsCalls)
	}
	for _, want := range []string{"tagged v0.1.0, GitHub release skipped", "released-tag: v0.1.0"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), want)
		}
	}
	got, err := out.render([]string{"main"})
	if err != nil {
		t.Fatalf("render() error = %v", err)
	}
	if !strings.Contains(got, "released=true\n") || !strings.Contains(got, `"tag":"v0.1.0"`) {
		t.Fatalf("outputs = %q, want the tag reported as released", got)
	}
}

// mergedReleaseFake is a forge holding one merged, pending release pull
// request for the implicit main component, number 9, at v0.1.0.
func TestDoReleaseWithDraftLeavesTheReleaseADraftAndNeverResumesIt(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	f := mergedReleaseFake()
	cfg := config.Default()
	cfg.Draft = true
	var stdout bytes.Buffer

	if err := doRelease(t.Context(), dir, cfg, "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doRelease() error = %v", err)
	}
	if len(f.CreatedReleases) != 1 || !f.CreatedReleases[0].Draft {
		t.Fatalf("CreatedReleases = %+v, want one draft", f.CreatedReleases)
	}
	for _, want := range []string{"tagged and drafted v0.1.0", "released-tag: v0.1.0"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), want)
		}
	}

	// The next run finds the pull request tagged and its release still a
	// draft: left for a human on purpose, so nothing is resumed.
	f.ListResponses[0].PullRequests[0].Labels = f.PRLabels[9]
	stdout.Reset()
	if err := doRelease(t.Context(), dir, cfg, "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("second doRelease() error = %v", err)
	}
	if got := stdout.String(); got != "nothing to release\n" {
		t.Fatalf("second stdout = %q, want nothing to release", got)
	}
	if len(f.CreatedReleases) != 1 {
		t.Fatalf("CreatedReleases = %+v, want still one", f.CreatedReleases)
	}
}

// mergedReleaseFake is a forge holding one merged, pending release pull
// request for the implicit main component, number 9, at v0.1.0.
func TestDoReleaseWithoutForceTagCreationLeavesTheTagToThePublish(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	f := mergedReleaseFake()
	cfg := config.Default()
	cfg.ForceTagCreation = false
	cfg.DraftUntilAssetsAttach = true
	var stdout bytes.Buffer

	if err := doRelease(t.Context(), dir, cfg, "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doRelease() error = %v", err)
	}
	if len(f.CreatedTags) != 0 {
		t.Fatalf("CreatedTags = %+v, want none, the publish creates the tag", f.CreatedTags)
	}
	if len(f.CreatedReleases) != 1 || f.CreatedReleases[0].Target != "deadbeef" || !f.CreatedReleases[0].Draft {
		t.Fatalf("CreatedReleases = %+v, want one draft targeting the merge commit", f.CreatedReleases)
	}
	for _, want := range []string{"drafted v0.1.0, tagged on publish", "released-tag: v0.1.0"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), want)
		}
	}
}

// mergedReleaseFake is a forge holding one merged, pending release pull
// request for the implicit main component, number 9, at v0.1.0.
func TestDoReleaseWithoutIncludeVInReleaseNameNamesTheReleaseAfterTheBareVersion(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	f := mergedReleaseFake()
	cfg := config.Default()
	cfg.IncludeVInReleaseName = false
	var stdout bytes.Buffer

	if err := doRelease(t.Context(), dir, cfg, "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doRelease() error = %v", err)
	}
	if len(f.CreatedReleases) != 1 || f.CreatedReleases[0].Name != "0.1.0" || f.CreatedReleases[0].Tag != "v0.1.0" {
		t.Fatalf("CreatedReleases = %+v, want release 0.1.0 at tag v0.1.0", f.CreatedReleases)
	}
}

// mergedReleaseFake is a forge holding one merged, pending release pull
// request for the implicit main component, number 9, at v0.1.0.
func TestDoReleaseReportsTheReleaseFieldsInTheComponentsOutput(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "chore: initial commit")
	f := mergedReleaseFake()
	var stdout bytes.Buffer

	out := &runOutputs{}
	if err := doRelease(withOutputs(t.Context(), out), dir, config.Default(), "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doRelease() error = %v", err)
	}

	got, err := out.render([]string{"main"})
	if err != nil {
		t.Fatalf("render() error = %v", err)
	}
	for _, want := range []string{
		"released=true\n", `"tag":"v0.1.0","major":0,"minor":1,"patch":0,"sha":"deadbeef"`,
		`"body":"## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the thing"`,
		`"html_url":"https://github.com/owner/repo/releases/tag/v0.1.0"`,
		`"upload_url":"https://uploads.github.com/repos/owner/repo/releases/9001/assets{?name,label}"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("outputs = %q, want it to contain %q", got, want)
		}
	}
}
