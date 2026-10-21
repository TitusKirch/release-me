// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/history"
	tagrelease "github.com/ostendorf-cloud/release-me/internal/release"
	"github.com/ostendorf-cloud/release-me/internal/releasepr"
)

// runRelease implements the release command: it tags and releases every
// merged release pull request carrying the pending label.
func runRelease(args []string, stdout, stderr io.Writer) int {
	verbose, dryRun, ok := parseWriteFlags("release", args, stderr)
	if !ok {
		return 2
	}

	ctx := context.Background()
	dir, cfg, f, logger, err := setup(ctx, stderr, verbose)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	if err := doRelease(ctx, dir, cfg, f, logger, dryRun, stdout); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

// doRelease tags, releases and swaps the label of every merged release pull
// request carrying the pending label.
func doRelease(ctx context.Context, dir string, cfg config.Config, f forge.Forge, logger *slog.Logger, dryRun bool, stdout io.Writer) error {
	owner, repoName, err := history.Remote(ctx, dir)
	if err != nil {
		return fmt.Errorf("release-me: origin remote: %w", err)
	}

	results, err := tagrelease.Run(ctx, f, owner, repoName, releasepr.Branch, cfg.Labels.Pending, cfg.Labels.Tagged, dryRun, logger)
	if err != nil {
		return fmt.Errorf("release-me: %w", err)
	}
	if len(results) == 0 {
		fmt.Fprintln(stdout, "nothing to release")
		return nil
	}
	for _, r := range results {
		fmt.Fprintln(stdout, r.Message)
	}
	return nil
}
