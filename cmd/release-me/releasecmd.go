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
)

// runRelease implements the release command: it tags and releases every
// merged release pull request carrying the pending label, for every
// configured component or the one named with --component.
func runRelease(args []string, stdout, stderr io.Writer) int {
	verbose, dryRun, component, ok := parseWriteFlags("release", args, stderr)
	if !ok {
		return 2
	}

	ctx := context.Background()
	dir, cfg, f, logger, err := setup(ctx, stderr, verbose)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	if err := doRelease(ctx, dir, cfg, component, f, logger, dryRun, stdout); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}

// doRelease tags, releases and swaps the label of every merged release pull
// request carrying the pending label, for every component component
// resolves to.
func doRelease(ctx context.Context, dir string, cfg config.Config, component string, f forge.Forge, logger *slog.Logger, dryRun bool, stdout io.Writer) error {
	owner, repoName, err := history.Remote(ctx, dir)
	if err != nil {
		return fmt.Errorf("release-me: origin remote: %w", err)
	}

	components, err := resolveComponents(cfg, component)
	if err != nil {
		return err
	}
	multi := len(components) > 1

	any := false
	for _, comp := range components {
		results, err := tagrelease.Run(
			ctx, f, owner, repoName,
			tagrelease.Component{Name: comp.Name, Branch: comp.Branch, TagPrefix: comp.TagPrefix, Multi: comp.Multi},
			tagrelease.Labels{
				Pending: cfg.Labels.Pending, Tagged: cfg.Labels.Tagged,
				Alpha: cfg.Labels.Alpha, Beta: cfg.Labels.Beta, Rc: cfg.Labels.Rc,
			},
			dryRun, logger,
		)
		if err != nil {
			return fmt.Errorf("release-me: %w", err)
		}
		for _, r := range results {
			any = true
			printLine(stdout, comp.Name, multi, r.Message)
		}
	}
	if !any {
		fmt.Fprintln(stdout, "nothing to release")
	}
	return nil
}
