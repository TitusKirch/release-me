// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/forge/forgetest"
)

// mainBranch is the release branch of the implicit "main" component a
// repository without "components" carries, config.Default().Resolve()'s
// single entry.
const mainBranch = "release-me/main"

// newPRRepo creates a git repository with a local identity and an "origin"
// remote whose push URL, through git's pushInsteadOf configuration, is
// rewritten to a local bare repository, so a real "git push" succeeds
// without touching the network. The fetch URL that history.Remote parses
// stays the configured GitHub one. It returns the repository directory and
// the bare repository directory.
func newPRRepo(t *testing.T) (dir, origin string) {
	t.Helper()
	origin = t.TempDir()
	runGit(t, origin, "init", "-q", "--bare", "-b", "main")

	dir = t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.name", "release-me tests")
	runGit(t, dir, "config", "user.email", "release-me-tests@example.com")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	runGit(t, dir, "remote", "add", "origin", "git@github.com:owner/repo.git")
	runGit(t, dir, "config", "url."+origin+".pushInsteadOf", "git@github.com:owner/repo.git")
	return dir, origin
}

// showBareBranchFile reads path at mainBranch from a bare
// repository, failing the test when the git invocation fails.
func showBareBranchFile(t *testing.T, bareDir, path string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "--git-dir", bareDir, "show", mainBranch+":"+path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git show %s:%s: %v", mainBranch, path, err)
	}
	return string(out)
}

// bareRefExists reports whether ref exists in a bare repository.
func bareRefExists(t *testing.T, bareDir, ref string) bool {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "--git-dir", bareDir, "rev-parse", "--verify", ref)
	return cmd.Run() == nil
}

// bareBranchSHA resolves mainBranch to its commit SHA in a bare
// repository, the head SHA a real forge would report for an open pull
// request.
func bareBranchSHA(t *testing.T, bareDir string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "--git-dir", bareDir, "rev-parse", mainBranch)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", mainBranch, err)
	}
	return strings.TrimSpace(string(out))
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// newFake returns a Fake forge configured with the two labels pr and check
// expect to find on the repository, and the test repository's default
// branch, so tests only need to override what they actually exercise.
func newFake() *forgetest.Fake {
	return &forgetest.Fake{
		RepoLabels:        []string{"release: pending", "release: tagged", "release: alpha", "release: beta", "release: rc"},
		DefaultBranchName: "main",
	}
}

func TestDoPRCreatesTheReleasePullRequest(t *testing.T) {
	dir, origin := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	var stdout bytes.Buffer

	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doPR() error = %v", err)
	}

	if len(f.CreatedPullRequests) != 1 {
		t.Fatalf("CreatedPullRequests = %+v, want one", f.CreatedPullRequests)
	}
	created := f.CreatedPullRequests[0]
	if created.Base != "main" || created.Head != mainBranch || created.Title != "chore(main): release v0.1.0" {
		t.Fatalf("created = %+v", created)
	}
	if want := []string{"release: pending"}; !equalStrings(created.Labels, want) {
		t.Fatalf("labels = %v, want %v", created.Labels, want)
	}
	if !strings.Contains(stdout.String(), "created chore(main): release v0.1.0") {
		t.Fatalf("stdout = %q", stdout.String())
	}

	changelog := showBareBranchFile(t, origin, "CHANGELOG.md")
	if !strings.Contains(changelog, "add the next command") {
		t.Fatalf("CHANGELOG.md on the release branch = %q", changelog)
	}
}

func TestDoPRUpdatesWhenNewEntriesArriveSinceTheLastRun(t *testing.T) {
	dir, origin := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	var first bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &first); err != nil {
		t.Fatalf("doPR() first run error = %v", err)
	}
	created := f.CreatedPullRequests[0]

	commitEntry(t, dir, "fix: stop a panic")
	f.ListResponses = []forgetest.ListResponse{
		{Head: mainBranch, State: "open", PullRequests: []forge.PullRequest{
			{Number: 1, Title: created.Title, Body: created.Body, HeadSHA: bareBranchSHA(t, origin)},
		}},
	}

	var second bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &second); err != nil {
		t.Fatalf("doPR() second run error = %v", err)
	}

	if len(f.UpdatedPullRequests) != 1 {
		t.Fatalf("UpdatedPullRequests = %+v, want one", f.UpdatedPullRequests)
	}
	if !strings.Contains(second.String(), "updated chore(main): release v0.1.0") {
		t.Fatalf("stdout = %q", second.String())
	}
	if !strings.Contains(f.UpdatedPullRequests[0].Body, "stop a panic") {
		t.Fatalf("updated body = %q, want the new entry", f.UpdatedPullRequests[0].Body)
	}

	changelog := showBareBranchFile(t, origin, "CHANGELOG.md")
	if !strings.Contains(changelog, "stop a panic") {
		t.Fatalf("CHANGELOG.md on the release branch = %q", changelog)
	}
}

func TestDoPRKeepsTheNotesBlockAcrossAnUpdate(t *testing.T) {
	dir, origin := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	var first bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &first); err != nil {
		t.Fatalf("doPR() first run error = %v", err)
	}
	created := f.CreatedPullRequests[0]

	withNotes, ok := addNotesBlock(created.Body, "Upgrade note: run the migration first.")
	if !ok {
		t.Fatalf("could not add a notes block to %q", created.Body)
	}
	f.ListResponses = []forgetest.ListResponse{
		{Head: mainBranch, State: "open", PullRequests: []forge.PullRequest{
			{Number: 1, Title: created.Title, Body: withNotes, HeadSHA: bareBranchSHA(t, origin)},
		}},
	}

	commitEntry(t, dir, "fix: stop a panic")

	var second bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &second); err != nil {
		t.Fatalf("doPR() second run error = %v", err)
	}

	if len(f.UpdatedPullRequests) != 1 {
		t.Fatalf("UpdatedPullRequests = %+v, want one", f.UpdatedPullRequests)
	}
	updatedBody := f.UpdatedPullRequests[0].Body
	if !strings.Contains(updatedBody, "Upgrade note: run the migration first.") {
		t.Fatalf("updated body = %q, want the notes block kept", updatedBody)
	}
	if !strings.Contains(updatedBody, "stop a panic") {
		t.Fatalf("updated body = %q, want the new entry", updatedBody)
	}
}

// addNotesBlock inserts a release-me-notes fenced block, holding notes,
// right after the first line of body, the same shape releasepr.Body
// renders it in.
func addNotesBlock(body, notes string) (string, bool) {
	firstLine, rest, found := strings.Cut(body, "\n")
	if !found {
		return "", false
	}
	return firstLine + "\n\n```release-me-notes\n" + notes + "\n```\n" + rest, true
}

func TestDoPRChangesNothingWhenAlreadyUpToDate(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	var first bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &first); err != nil {
		t.Fatalf("doPR() first run error = %v", err)
	}
	created := f.CreatedPullRequests[0]

	f.ListResponses = []forgetest.ListResponse{
		{Head: mainBranch, State: "open", PullRequests: []forge.PullRequest{
			{Number: 1, Title: created.Title, Body: created.Body},
		}},
	}

	var second bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &second); err != nil {
		t.Fatalf("doPR() second run error = %v", err)
	}

	if len(f.CreatedPullRequests) != 1 || len(f.UpdatedPullRequests) != 0 {
		t.Fatalf("expected no further create or update, created=%d updated=%d", len(f.CreatedPullRequests), len(f.UpdatedPullRequests))
	}
	if !strings.Contains(second.String(), "release pull request up to date") {
		t.Fatalf("stdout = %q", second.String())
	}
}

func TestDoPRDryRunWritesNothing(t *testing.T) {
	dir, origin := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	var stdout bytes.Buffer

	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), true, &stdout); err != nil {
		t.Fatalf("doPR() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "would create chore(main): release v0.1.0") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if len(f.CreatedPullRequests) != 0 || len(f.UpdatedPullRequests) != 0 {
		t.Fatalf("dry run called the forge: created=%+v updated=%+v", f.CreatedPullRequests, f.UpdatedPullRequests)
	}
	if bareRefExists(t, origin, "refs/heads/"+mainBranch) {
		t.Fatal("dry run pushed a branch, want none")
	}
}

func TestDoPRPrintsNothingToReleaseWithoutAReleasableChange(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "chore: initial commit")

	f := newFake()
	var stdout bytes.Buffer

	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doPR() error = %v", err)
	}
	if stdout.String() != "nothing to release\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "nothing to release\n")
	}
	if len(f.CreatedPullRequests) != 0 {
		t.Fatalf("CreatedPullRequests = %+v, want none", f.CreatedPullRequests)
	}
}

func TestDoPRBumpsAConfiguredMarkerFileOnTheReleaseBranch(t *testing.T) {
	dir, origin := newPRRepo(t)
	writeAndCommit(t, dir, "version.go", "package main\n\nconst Version = \"0.0.0\" // release-me: version\n", "chore: add the version file")
	commitEntry(t, dir, "feat: add the next command")

	cfg := config.Default()
	cfg.Files = []config.File{{Path: "version.go", Updater: "marker"}}
	f := newFake()
	var stdout bytes.Buffer

	if err := doPR(t.Context(), dir, cfg, "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doPR() error = %v", err)
	}

	got := showBareBranchFile(t, origin, "version.go")
	want := "package main\n\nconst Version = \"0.1.0\" // release-me: version\n"
	if got != want {
		t.Fatalf("version.go on the release branch = %q, want %q", got, want)
	}
}

func TestDoPRBumpsATypedUpdaterFileOnTheReleaseBranch(t *testing.T) {
	dir, origin := newPRRepo(t)
	writeAndCommit(t, dir, "Chart.yaml", "apiVersion: v2\nname: chart\nversion: 0.0.0\n", "chore: add the chart")
	commitEntry(t, dir, "feat: add the next command")

	cfg := config.Default()
	cfg.Files = []config.File{{Path: "Chart.yaml", Updater: "helm"}}
	f := newFake()
	var stdout bytes.Buffer

	if err := doPR(t.Context(), dir, cfg, "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doPR() error = %v", err)
	}

	got := showBareBranchFile(t, origin, "Chart.yaml")
	want := "apiVersion: v2\nname: chart\nversion: 0.1.0\n"
	if got != want {
		t.Fatalf("Chart.yaml on the release branch = %q, want %q", got, want)
	}
}

func TestDoPRFailsNamingThePathWhenAConfiguredFileIsMissing(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	cfg := config.Default()
	cfg.Files = []config.File{{Path: "does-not-exist.go", Updater: "marker"}}
	f := newFake()
	var stdout bytes.Buffer

	err := doPR(t.Context(), dir, cfg, "", f, discardLogger(), false, &stdout)
	if err == nil {
		t.Fatal("doPR() error = nil, want an error naming the missing file")
	}
	if !strings.Contains(err.Error(), "does-not-exist.go") {
		t.Fatalf("doPR() error = %q, want it to name the path", err)
	}
}

func TestDoPRFailsNamingThePathAndKeyWhenTheKeyIsNotFound(t *testing.T) {
	dir, _ := newPRRepo(t)
	writeAndCommit(t, dir, "values.yaml", "replicaCount: 1\n", "chore: add the values file")
	commitEntry(t, dir, "feat: add the next command")

	cfg := config.Default()
	cfg.Files = []config.File{{Path: "values.yaml", Updater: "yaml", Key: "image.tag"}}
	f := newFake()
	var stdout bytes.Buffer

	err := doPR(t.Context(), dir, cfg, "", f, discardLogger(), false, &stdout)
	if err == nil {
		t.Fatal("doPR() error = nil, want an error naming the path and key")
	}
	if !strings.Contains(err.Error(), "values.yaml") || !strings.Contains(err.Error(), "image.tag") {
		t.Fatalf("doPR() error = %q, want it to name the path and key", err)
	}
}

func TestDoPRDetectsAFilesConfigChangeEvenWithTheSameTitleAndBody(t *testing.T) {
	dir, origin := newPRRepo(t)
	writeAndCommit(t, dir, "version.go", "package main\n\nconst Version = \"0.0.0\" // release-me: version\n", "chore: add the version file")
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	var first bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &first); err != nil {
		t.Fatalf("doPR() first run error = %v", err)
	}
	created := f.CreatedPullRequests[0]
	f.ListResponses = []forgetest.ListResponse{
		{Head: mainBranch, State: "open", PullRequests: []forge.PullRequest{
			{Number: 1, Title: created.Title, Body: created.Body, HeadSHA: bareBranchSHA(t, origin)},
		}},
	}

	// release-me.yaml now lists version.go, with nothing else changed:
	// no new commit, so the title and the changelog entries are identical
	// to the first run.
	cfg := config.Default()
	cfg.Files = []config.File{{Path: "version.go", Updater: "marker"}}

	var second bytes.Buffer
	if err := doPR(t.Context(), dir, cfg, "", f, discardLogger(), false, &second); err != nil {
		t.Fatalf("doPR() second run error = %v", err)
	}

	if strings.Contains(second.String(), "up to date") {
		t.Fatalf("stdout = %q, want the file bump to be pushed, not a no-op", second.String())
	}
	got := showBareBranchFile(t, origin, "version.go")
	want := "package main\n\nconst Version = \"0.1.0\" // release-me: version\n"
	if got != want {
		t.Fatalf("version.go on the release branch = %q, want %q", got, want)
	}
}

func TestDoPRRendersAnExplicitLineWhenEveryEntryIsHidden(t *testing.T) {
	dir, origin := newPRRepo(t)
	commitEntry(t, dir, "feat: internal rework nobody sees")

	cfg := config.Default()
	cfg.HiddenTypes = []string{"feat"}
	f := newFake()
	var stdout bytes.Buffer

	if err := doPR(t.Context(), dir, cfg, "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doPR() error = %v", err)
	}

	changelogFile := showBareBranchFile(t, origin, "CHANGELOG.md")
	if !strings.Contains(changelogFile, noUserFacingChangesLine) {
		t.Fatalf("CHANGELOG.md on the release branch = %q, want the explicit line", changelogFile)
	}
	if strings.Contains(changelogFile, "### ") {
		t.Fatalf("CHANGELOG.md on the release branch = %q, want no empty section heading", changelogFile)
	}
}

func TestDoPRUsesTheForgesDefaultBranchAsBase(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	f.DefaultBranchName = "trunk"
	var stdout bytes.Buffer

	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doPR() error = %v", err)
	}
	if got := f.CreatedPullRequests[0].Base; got != "trunk" {
		t.Fatalf("base = %q, want %q", got, "trunk")
	}
}

func TestDoPRFallsBackToOriginHEADWhenTheForgeDefaultBranchFails(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")
	runGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")

	f := newFake()
	f.DefaultBranchErr = errors.New("boom")
	var stdout bytes.Buffer

	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &stdout); err != nil {
		t.Fatalf("doPR() error = %v", err)
	}
	if got := f.CreatedPullRequests[0].Base; got != "trunk" {
		t.Fatalf("base = %q, want %q", got, "trunk")
	}
}

func TestDoPRFailsCleanlyWhenTheBaseBranchCannotBeResolved(t *testing.T) {
	dir, _ := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	f.DefaultBranchErr = errors.New("boom")
	var stdout bytes.Buffer

	err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &stdout)
	if err == nil {
		t.Fatal("doPR() error = nil, want an error")
	}
	if len(f.CreatedPullRequests) != 0 {
		t.Fatalf("CreatedPullRequests = %+v, want none", f.CreatedPullRequests)
	}
}

func TestDoPRRejectsAMissingLabelBeforeWritingAnything(t *testing.T) {
	dir, origin := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	f.RepoLabels = []string{"release: pending"} // "release: tagged" is missing

	err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, io.Discard)
	if err == nil {
		t.Fatal("doPR() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "release: tagged") {
		t.Fatalf("error = %v, want it to name the missing label", err)
	}
	if bareRefExists(t, origin, "refs/heads/"+mainBranch) {
		t.Fatal("a branch was pushed despite the missing label")
	}
}

func TestDoPRKeepsAHumanAddedLabelOnUpdate(t *testing.T) {
	dir, origin := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	var first bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &first); err != nil {
		t.Fatalf("doPR() first run error = %v", err)
	}
	created := f.CreatedPullRequests[0]
	f.PRLabels = map[int][]string{1: {"release: pending", "needs human"}}
	f.ListResponses = []forgetest.ListResponse{
		{Head: mainBranch, State: "open", PullRequests: []forge.PullRequest{
			{
				Number: 1, Title: created.Title, Body: created.Body, Labels: []string{"release: pending", "needs human"},
				HeadSHA: bareBranchSHA(t, origin),
			},
		}},
	}

	commitEntry(t, dir, "fix: stop a panic")

	var second bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &second); err != nil {
		t.Fatalf("doPR() second run error = %v", err)
	}

	got := f.PRLabels[1]
	want := []string{"release: pending", "needs human"}
	if !equalStrings(got, want) {
		t.Fatalf("Labels() = %v, want %v, the human-added label kept", got, want)
	}
}

// TestDoPRFollowsThePreReleaseLabelThroughATypeChangeBackToStableAndToRC
// walks a single release pull request through a label alpha, then beta,
// then no pre-release label at all, then rc, exercising "pr reads the
// label and renders the pre-release version", "removing the label returns
// it to the stable version" and "the counter counts up from the last
// pre-release tag of that type".
func TestDoPRFollowsThePreReleaseLabelThroughATypeChangeBackToStableAndToRC(t *testing.T) {
	dir, origin := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	var out bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &out); err != nil {
		t.Fatalf("doPR() create error = %v", err)
	}
	pr := f.CreatedPullRequests[0]
	if pr.Title != "chore(main): release v0.1.0" {
		t.Fatalf("created title = %q, want the stable version", pr.Title)
	}

	setLabel := func(label string) {
		labels := []string{"release: pending"}
		if label != "" {
			labels = append(labels, label)
		}
		f.ListResponses = []forgetest.ListResponse{
			{Head: mainBranch, State: "open", PullRequests: []forge.PullRequest{
				{Number: 1, Title: pr.Title, Body: pr.Body, Labels: labels, HeadSHA: bareBranchSHA(t, origin)},
			}},
		}
	}
	runAndCheck := func(step, wantTitle string) {
		t.Helper()
		var buf bytes.Buffer
		if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &buf); err != nil {
			t.Fatalf("doPR() %s error = %v", step, err)
		}
		if len(f.UpdatedPullRequests) == 0 {
			t.Fatalf("%s: no update recorded", step)
		}
		updated := f.UpdatedPullRequests[len(f.UpdatedPullRequests)-1]
		if updated.Title != wantTitle {
			t.Fatalf("%s: title = %q, want %q", step, updated.Title, wantTitle)
		}
		pr.Body = updated.Body
	}

	setLabel("release: alpha")
	runAndCheck("alpha", "chore(main): release v0.1.0-alpha.1")

	setLabel("release: beta")
	runAndCheck("beta", "chore(main): release v0.1.0-beta.1")

	setLabel("")
	runAndCheck("label removed", "chore(main): release v0.1.0")

	setLabel("release: rc")
	runAndCheck("first rc", "chore(main): release v0.1.0-rc.1")

	// Simulate release having tagged the first rc: the next one for the
	// same base version and type counts up from it.
	runGit(t, dir, "tag", "v0.1.0-rc.1")
	runAndCheck("second rc", "chore(main): release v0.1.0-rc.2")
}

func TestDoPRRejectsTwoPreReleaseLabelsAtOnce(t *testing.T) {
	dir, origin := newPRRepo(t)
	commitEntry(t, dir, "feat: add the next command")

	f := newFake()
	var out bytes.Buffer
	if err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, &out); err != nil {
		t.Fatalf("doPR() create error = %v", err)
	}
	pr := f.CreatedPullRequests[0]
	f.ListResponses = []forgetest.ListResponse{
		{Head: mainBranch, State: "open", PullRequests: []forge.PullRequest{
			{
				Number: 1, Title: pr.Title, Body: pr.Body,
				Labels: []string{"release: pending", "release: beta", "release: rc"}, HeadSHA: bareBranchSHA(t, origin),
			},
		}},
	}

	err := doPR(t.Context(), dir, config.Default(), "", f, discardLogger(), false, io.Discard)
	if err == nil {
		t.Fatal("doPR() error = nil, want an error naming both pre-release labels")
	}
	if !strings.Contains(err.Error(), "release: beta") || !strings.Contains(err.Error(), "release: rc") {
		t.Fatalf("error = %v, want it to name both labels", err)
	}
}

// writeAndCommit writes content to path inside dir and commits it.
func writeAndCommit(t *testing.T, dir, path, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	runGit(t, dir, "add", path)
	runGit(t, dir, "commit", "-q", "-m", message)
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
