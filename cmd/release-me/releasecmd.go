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

	out := &runOutputs{}
	ctx := withOutputs(context.Background(), out)
	dir, cfg, f, logger, err := setup(ctx, stderr, verbose)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := rejectComponentForLocked(cfg, component); err != nil {
		fmt.Fprintf(stderr, "release-me: %v\n", err)
		return 2
	}
	if err := rejectComponentForShared(cfg, component); err != nil {
		fmt.Fprintf(stderr, "release-me: %v\n", err)
		return 2
	}

	if err := doRelease(ctx, dir, cfg, component, f, logger, dryRun, stdout); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := emitOutputs(cfg, component, out); err != nil {
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
	labels := tagrelease.Labels{
		Pending: cfg.Labels.Pending, Tagged: cfg.Labels.Tagged,
		Alpha: cfg.Labels.Alpha, Beta: cfg.Labels.Beta, Rc: cfg.Labels.Rc,
	}

	if cfg.Versioning == config.VersioningLocked {
		return doReleaseLocked(ctx, cfg, owner, repoName, f, labels, dryRun, logger, stdout)
	}
	if sharedOutsideLocked(cfg) {
		return doReleaseShared(ctx, cfg, owner, repoName, f, labels, dryRun, logger, stdout)
	}

	any := false
	for _, comp := range components {
		results, err := tagrelease.Run(
			ctx, f, owner, repoName,
			tagrelease.Component{Name: comp.Name, Branch: comp.Branch, TagPrefix: comp.TagPrefix, Multi: comp.Multi, MajorTag: comp.MajorTag},
			labels, cfg.DraftUntilAssetsAttach, dryRun, logger,
		)
		if err != nil {
			return fmt.Errorf("release-me: %w", err)
		}
		for _, r := range results {
			any = true
			printLine(stdout, comp.Name, multi, r.Message)
			if r.MajorTagMessage != "" {
				printLine(stdout, comp.Name, multi, r.MajorTagMessage)
			}
			reportReleased(ctx, stdout, dryRun, r)
		}
	}
	if !any {
		fmt.Fprintln(stdout, "nothing to release")
	}
	return nil
}

// doReleaseLocked tags and releases every configured component together,
// from the one release pull request a locked family shares: every merged,
// pending-labeled pull request on branch "release-me/main" releases every
// component at once, the same merge commit and version for each.
func doReleaseLocked(
	ctx context.Context, cfg config.Config, owner, repoName string, f forge.Forge, labels tagrelease.Labels, dryRun bool, logger *slog.Logger, stdout io.Writer,
) error {
	resolved := cfg.Resolve()
	comps := make([]tagrelease.Component, 0, len(resolved))
	for _, comp := range resolved {
		comps = append(comps, tagrelease.Component{Name: comp.Name, Branch: comp.Branch, TagPrefix: comp.TagPrefix, Multi: comp.Multi, MajorTag: comp.MajorTag})
	}
	branch := resolved[0].Branch

	results, err := tagrelease.RunLocked(ctx, f, owner, repoName, branch, comps, labels, cfg.DraftUntilAssetsAttach, dryRun, logger)
	if err != nil {
		return fmt.Errorf("release-me: %w", err)
	}

	multi := len(resolved) > 1
	any := false
	for _, r := range results {
		any = true
		printLine(stdout, r.Name, multi, r.Message)
		if r.MajorTagMessage != "" {
			printLine(stdout, r.Name, multi, r.MajorTagMessage)
		}
		reportReleased(ctx, stdout, dryRun, r)
	}
	if !any {
		fmt.Fprintln(stdout, "nothing to release")
	}
	return nil
}

// doReleaseShared tags and releases every component a merged shared
// release pull request names, on branch "release-me/main", each at its own
// version, then every merged, still pending pull request on a component's
// own branch, "release-me/<name>": one merged right before the switch to
// "release-pull-request: shared" still gets its tag and release instead of
// staying pending forever.
func doReleaseShared(
	ctx context.Context, cfg config.Config, owner, repoName string, f forge.Forge, labels tagrelease.Labels,
	dryRun bool, logger *slog.Logger, stdout io.Writer,
) error {
	resolved := cfg.Resolve()
	branch := resolved[0].Branch
	comps := make([]tagrelease.Component, 0, len(resolved))
	for _, comp := range resolved {
		comps = append(comps, tagrelease.Component{Name: comp.Name, Branch: comp.Branch, TagPrefix: comp.TagPrefix, Multi: comp.Multi, MajorTag: comp.MajorTag})
	}

	results, err := tagrelease.RunShared(ctx, f, owner, repoName, branch, comps, labels, cfg.DraftUntilAssetsAttach, dryRun, logger)
	if err != nil {
		return fmt.Errorf("release-me: %w", err)
	}
	for _, comp := range comps {
		own := config.PerComponentBranch(comp.Name)
		if own == branch {
			continue
		}
		comp.Branch = own
		leftover, err := tagrelease.Run(ctx, f, owner, repoName, comp, labels, cfg.DraftUntilAssetsAttach, dryRun, logger)
		if err != nil {
			return fmt.Errorf("release-me: %w", err)
		}
		results = append(results, leftover...)
	}

	if len(results) == 0 {
		fmt.Fprintln(stdout, "nothing to release")
		return nil
	}
	for _, r := range results {
		printLine(stdout, r.Name, true, r.Message)
		if r.MajorTagMessage != "" {
			printLine(stdout, r.Name, true, r.MajorTagMessage)
		}
		reportReleased(ctx, stdout, dryRun, r)
	}
	return nil
}
