// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
)

// runRun implements the run command, the CI entry point: pr, then release,
// for every configured component or the one named with --component.
func runRun(args []string, stdout, stderr io.Writer) int {
	verbose, dryRun, component, ok := parseWriteFlags("run", args, stderr)
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
	if err := rejectComponentForShared(cfg, component); err != nil {
		fmt.Fprintf(stderr, "release-me: %v\n", err)
		return 2
	}

	if err := doPR(ctx, dir, cfg, component, f, logger, dryRun, stdout); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if err := doRelease(ctx, dir, cfg, component, f, logger, dryRun, stdout); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	return 0
}
