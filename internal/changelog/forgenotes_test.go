// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package changelog_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
)

// gitHubNotes is the shape GitHub's "generate release notes" returns, with
// release.yml categories and a first-time contributor.
const gitHubNotes = "<!-- Release notes generated using configuration in .github/release.yml at main -->\r\n\r\n" +
	"## What's Changed\r\n" +
	"### Features\r\n" +
	"* feat: add the widget by @octocat in https://github.com/owner/repo/pull/12\r\n" +
	"#### Internal\r\n" +
	"* chore: bump yaml.v3 by @dependabot[bot] in https://github.com/owner/repo/pull/13\r\n\r\n" +
	"## New Contributors\r\n" +
	"* @octocat made their first contribution in https://github.com/owner/repo/pull/12\r\n\r\n" +
	"```text\r\n## not a heading\r\n```\r\n\r\n" +
	"**Full Changelog**: https://github.com/owner/repo/compare/v1.1.0...v1.2.0\r\n"

func TestForgeNotesFitsGitHubsNotesUnderAVersionHeading(t *testing.T) {
	t.Parallel()
	got := changelog.ForgeNotes(gitHubNotes)
	want := "<!-- Release notes generated using configuration in .github/release.yml at main -->\n\n" +
		"### What's Changed\n" +
		"### Features\n" +
		"* feat: add the widget by @octocat in https://github.com/owner/repo/pull/12\n" +
		"**Internal**\n" +
		"* chore: bump yaml.v3 by @dependabot[bot] in https://github.com/owner/repo/pull/13\n\n" +
		"### New Contributors\n" +
		"* @octocat made their first contribution in https://github.com/owner/repo/pull/12\n\n" +
		"```text\n ## not a heading\n```\n\n" +
		"**Full Changelog**: https://github.com/owner/repo/compare/v1.1.0...v1.2.0"
	if got != want {
		t.Fatalf("ForgeNotes() = %q, want %q", got, want)
	}
}

func TestForgeNotesReadBackAsTheTopSection(t *testing.T) {
	t.Parallel()
	body := changelog.ForgeNotes(gitHubNotes)
	section := changelog.RenderHeading("1.2.0", fixtureDate, "https://github.com/owner/repo/compare/v1.1.0...v1.2.0", "") +
		"\n\n" + body + "\n"
	content := changelog.Prepend("# Changelog\n\n## v1.1.0 (2026-10-01)\n\n### Bug Fixes\n\n- fix the parser\n", section)

	if got, ok := changelog.TopSection(content); !ok || got != body {
		t.Fatalf("TopSection() = %q, %v, want the notes back whole", got, ok)
	}
	if v, ok := changelog.HeadingVersion(content); !ok || v != "1.2.0" {
		t.Fatalf("HeadingVersion() = %q, %v, want 1.2.0", v, ok)
	}
}
