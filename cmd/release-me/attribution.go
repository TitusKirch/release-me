// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/conventional"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/history"
)

// attribution decides which history entries count for a component: by the
// files each changed, history.Touches, and by the components its
// "Release-Component" footers name, through history.Counts.
//
// A footer counts wherever release-me reads the entry's text, the same
// places a "Release-As" footer counts: the commit's own message, the body
// of the pull request it belongs to, and, in config.ChangelogSourceNested,
// the messages of a merge commit's sub commits.
type attribution struct {
	dir, owner, repoName string
	forge                forge.Forge
	known                []string
	logger               *slog.Logger
	// warned holds the commit and name of every unknown footer value
	// already logged, so one commit read twice warns once.
	warned map[string]bool
}

// newAttribution returns the attribution for one run over cfg's
// components. f should be the same forge the run resolves pull request
// text through, a pullRequestCache, so the attribution's reads cost no
// extra request.
func newAttribution(dir string, cfg config.Config, owner, repoName string, f forge.Forge, logger *slog.Logger) *attribution {
	var known []string
	for _, c := range cfg.Resolve() {
		known = append(known, c.Name)
	}
	return &attribution{
		dir: dir, owner: owner, repoName: repoName, forge: f, known: known, logger: logger,
		warned: map[string]bool{},
	}
}

// filter keeps only the entries that count for comp. comp.Path "." keeps
// every entry without reading any commit's touched files, but still reads
// the footers, so a footer naming an unknown component warns there too.
func (a *attribution) filter(ctx context.Context, entries []history.Entry, comp config.ResolvedComponent) ([]history.Entry, error) {
	filtered := make([]history.Entry, 0, len(entries))
	for _, e := range entries {
		named, err := a.named(ctx, e, comp)
		if err != nil {
			return nil, err
		}
		if comp.Path == "." {
			filtered = append(filtered, e)
			continue
		}
		paths, err := history.Touches(ctx, a.dir, e.SHA)
		if err != nil {
			return nil, fmt.Errorf("commit %s: %w", e.SHA, err)
		}
		if history.Counts(paths, named, comp.Name, comp.Path) {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// named returns the configured components e's "Release-Component" footers
// name, logging and dropping every name no component carries.
func (a *attribution) named(ctx context.Context, e history.Entry, comp config.ResolvedComponent) ([]string, error) {
	texts := []string{entryText(e)}
	if e.PullRequestNumber > 0 {
		pr, err := a.forge.PullRequest(ctx, a.owner, a.repoName, e.PullRequestNumber)
		switch {
		case err == nil:
			texts = append(texts, prText(pr.Title, pr.Body))
		case errors.Is(err, forge.ErrNotFound):
		default:
			return nil, fmt.Errorf("release-me: pull request #%d: %w", e.PullRequestNumber, err)
		}
		if comp.ChangelogSource == config.ChangelogSourceNested {
			subs, err := history.SubCommits(ctx, a.dir, e.SHA)
			if err != nil {
				return nil, fmt.Errorf("release-me: merge commit %s: sub commits: %w", e.SHA, err)
			}
			for _, s := range subs {
				texts = append(texts, entryText(s))
			}
		}
	}

	var named []string
	for _, text := range texts {
		for _, name := range conventional.Parse(text, "").Components {
			if slices.Contains(named, name) {
				continue
			}
			if !slices.Contains(a.known, name) {
				a.warnUnknown(e, name)
				continue
			}
			named = append(named, name)
		}
	}
	return named, nil
}

// warnUnknown logs a "Release-Component" footer naming no configured
// component, once per commit and name, never failing the run: the name is
// ignored, the same way a refused "Release-As" footer is.
func (a *attribution) warnUnknown(e history.Entry, name string) {
	key := e.SHA + "\x00" + name
	if a.warned[key] {
		return
	}
	a.warned[key] = true
	attrs := []any{"commit", e.SHA, "value", name}
	if e.PullRequestNumber > 0 {
		attrs = append(attrs, "pull_request", e.PullRequestNumber)
	}
	a.logger.Warn("ignoring a Release-Component footer: no component has that name", attrs...)
}

// pullRequestCache is a forge.Forge that reads every pull request once:
// attribution and assembleEntries both resolve an entry's pull request,
// and the second read is served from memory. Only PullRequest is cached,
// every other method goes to the wrapped forge unchanged.
type pullRequestCache struct {
	forge.Forge
	prs map[pullRequestKey]cachedPullRequest
}

type pullRequestKey struct {
	owner, repo string
	number      int
}

type cachedPullRequest struct {
	pr  forge.PullRequest
	err error
}

// newPullRequestCache wraps f in a pullRequestCache. A run should keep the
// cache for the reads of one computation only, never across a write to the
// pull requests it caches.
func newPullRequestCache(f forge.Forge) *pullRequestCache {
	return &pullRequestCache{Forge: f, prs: map[pullRequestKey]cachedPullRequest{}}
}

// PullRequest implements forge.Forge, reading each pull request from the
// wrapped forge once, its error included.
func (c *pullRequestCache) PullRequest(ctx context.Context, owner, repo string, number int) (forge.PullRequest, error) {
	key := pullRequestKey{owner: owner, repo: repo, number: number}
	if hit, ok := c.prs[key]; ok {
		return hit.pr, hit.err
	}
	pr, err := c.Forge.PullRequest(ctx, owner, repo, number)
	c.prs[key] = cachedPullRequest{pr: pr, err: err}
	return pr, err
}
