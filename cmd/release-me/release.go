// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/conventional"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/history"
	"github.com/ostendorf-cloud/release-me/internal/version"
)

// tagPrefix is the version tag prefix for the single component this
// milestone supports.
const tagPrefix = "v"

// release is the computed next version and changelog entries for one
// repository, ready for the next and changelog commands to print.
type release struct {
	version     version.Result
	entries     []changelog.Entry
	hiddenTypes []string
}

// computeRelease wires history, the forge and the conventional and version
// packages into one release. released is false when there is nothing to
// release.
func computeRelease(ctx context.Context, dir string, cfg config.Config, f forge.Forge, logger *slog.Logger) (release, bool, error) {
	owner, repoName, err := history.Remote(ctx, dir)
	if err != nil {
		return release{}, false, fmt.Errorf("release-me: origin remote: %w", err)
	}
	logger.Debug("resolved repository", "owner", owner, "repo", repoName)

	tag, last, hasTag, err := history.LastTag(ctx, dir, tagPrefix)
	if err != nil {
		return release{}, false, fmt.Errorf("release-me: last tag: %w", err)
	}
	logger.Debug("last tag", "tag", tag, "found", hasTag)

	since := ""
	if hasTag {
		since = tag
	}
	raw, err := history.Walk(ctx, dir, since)
	if err != nil {
		return release{}, false, fmt.Errorf("release-me: history: %w", err)
	}
	logger.Debug("history entries", "count", len(raw))

	changes, entries, err := assembleEntries(ctx, f, owner, repoName, raw, cfg.FallbackType)
	if err != nil {
		return release{}, false, err
	}

	result, released := version.Next(last, hasTag, changes, cfg.InitialVersion, cfg.BumpMinorPreMajor)
	if !released {
		return release{}, false, nil
	}
	return release{version: result, entries: entries, hiddenTypes: cfg.HiddenTypes}, true, nil
}

// assembleEntries resolves each history entry's text through the forge when
// it names a pull request, falling back to the commit body when the forge
// has no pull request for it, then parses the result as a conventional
// commit, unparseable text falling back to fallbackType.
func assembleEntries(ctx context.Context, f forge.Forge, owner, repoName string, raw []history.Entry, fallbackType string) ([]version.Change, []changelog.Entry, error) {
	changes := make([]version.Change, 0, len(raw))
	entries := make([]changelog.Entry, 0, len(raw))

	for _, e := range raw {
		text, url := entryText(e), ""

		if e.PullRequestNumber > 0 {
			pr, err := f.PullRequest(ctx, owner, repoName, e.PullRequestNumber)
			switch {
			case err == nil:
				text, url = prText(pr.Title, pr.Body), pr.URL
			case errors.Is(err, forge.ErrNotFound):
				text, url = e.Body, pr.URL
			default:
				return nil, nil, fmt.Errorf("release-me: pull request #%d: %w", e.PullRequestNumber, err)
			}
		}

		msg := conventional.Parse(text, fallbackType)
		changes = append(changes, version.Change{Type: msg.Type, Breaking: msg.Breaking})
		entries = append(entries, changelog.Entry{
			Type:              msg.Type,
			Scope:             msg.Scope,
			Description:       msg.Description,
			Breaking:          msg.Breaking,
			PullRequestNumber: e.PullRequestNumber,
			URL:               url,
		})
	}
	return changes, entries, nil
}

func entryText(e history.Entry) string {
	if e.Body == "" {
		return e.Subject
	}
	return e.Subject + "\n" + e.Body
}

func prText(title, body string) string {
	if body == "" {
		return title
	}
	return title + "\n" + body
}
