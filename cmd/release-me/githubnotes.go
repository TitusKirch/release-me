// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/forge"
)

// sectionBody renders comp's section body, with no heading: Sections of
// its entries by default, the forge's generated release notes fitted
// under a version heading by changelog.ForgeNotes with "changelog-type:
// github". The notes cover the commits up to the checkout's HEAD, the one
// the release pull request is cut from, since fromTag, "" letting the
// forge pick the previous release, a first release's whole history when
// there is none. With placeholder, a body with nothing visible becomes
// noUserFacingChangesLine, the line a release pull request carries
// instead of nothing; without it, "" stays "".
func sectionBody(
	ctx context.Context, f forge.Forge, dir, owner, repoName string, comp config.ResolvedComponent,
	entries []changelog.Entry, opts changelog.Options, tag, fromTag string, placeholder bool,
) (string, error) {
	body := ""
	if comp.ChangelogType == config.ChangelogTypeGitHub {
		target, err := gitOutput(ctx, dir, "rev-parse", "HEAD")
		if err != nil {
			return "", fmt.Errorf("release-me: component %s: release notes target: %w", comp.Name, err)
		}
		notes, err := f.GenerateReleaseNotes(ctx, owner, repoName, tag, fromTag, target)
		if err != nil {
			return "", fmt.Errorf("release-me: component %s: %w", comp.Name, err)
		}
		body = changelog.ForgeNotes(notes)
	} else {
		body = changelog.Sections(entries, opts)
	}
	if body == "" && placeholder {
		body = noUserFacingChangesLine
	}
	return body, nil
}

// renderSection renders comp's whole changelog section for the changelog
// command, heading included, the way pr writes it: "" when nothing is
// visible, the same as changelog.Render.
func renderSection(
	ctx context.Context, f forge.Forge, dir, owner, repoName string, comp config.ResolvedComponent, rel release,
) (string, error) {
	tag := comp.TagPrefix + rel.version.Version
	body, err := sectionBody(ctx, f, dir, owner, repoName, comp, rel.entries, rel.options, tag, rel.fromTag, false)
	if err != nil || body == "" {
		return "", err
	}
	link := compareURL(f, owner, repoName, rel.fromTag, tag)
	return changelog.RenderHeading(rel.version.Version, time.Now(), link, rel.options.DateFormat) + "\n\n" + body + "\n", nil
}
