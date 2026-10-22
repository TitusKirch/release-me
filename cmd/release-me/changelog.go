// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/history"
)

// runChangelog implements the changelog command: it prints the rendered
// entries for the next version per component, nothing for a component with
// nothing to release.
func runChangelog(args []string, stdout, stderr io.Writer) int {
	verbose, component, ok := parseFlags("changelog", args, stderr)
	if !ok {
		return 2
	}

	ctx := context.Background()
	dir, cfg, f, logger, err := setup(ctx, stderr, verbose)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := rejectComponentForLocked(cfg, component); err != nil {
		fmt.Fprintf(stderr, "release-me: %v\n", err)
		return 2
	}

	components, err := resolveComponents(cfg, component)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	multi := len(components) > 1

	owner, repoName, err := history.Remote(ctx, dir)
	if err != nil {
		fmt.Fprintf(stderr, "release-me: origin remote: %v\n", err)
		return 1
	}
	logger.Debug("resolved repository", "owner", owner, "repo", repoName)

	if cfg.Versioning == config.VersioningIndependent {
		for _, comp := range components {
			rel, released, err := computeRelease(ctx, dir, cfg, comp, owner, repoName, f, logger)
			if err != nil {
				fmt.Fprintf(stderr, "%v\n", err)
				return 1
			}
			if !released {
				continue
			}
			printBlock(stdout, comp.Name, multi, changelog.Render(rel.version.Version, time.Now(), rel.entries, rel.hiddenTypes))
		}
		return 0
	}

	members, err := gatherFamily(ctx, dir, cfg, owner, repoName, f, logger)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	releases, released := familyReleases(cfg, members)
	if !released {
		return 0
	}
	for _, comp := range components {
		rel, ok := releases[comp.Name]
		if !ok {
			continue
		}
		printBlock(stdout, comp.Name, multi, changelog.Render(rel.version.Version, time.Now(), rel.entries, rel.hiddenTypes))
	}
	return 0
}
