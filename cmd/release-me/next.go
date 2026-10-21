// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
)

// runNext implements the next command: it prints the next version and the
// bump reason, nothing when there is nothing to release.
func runNext(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("next", flag.ContinueOnError)
	verbose := fs.Bool("verbose", false, "enable debug logging on stderr")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx := context.Background()
	dir, cfg, f, logger, err := setup(ctx, stderr, *verbose)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	rel, released, err := computeRelease(ctx, dir, cfg, f, logger)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if !released {
		return 0
	}

	fmt.Fprintf(stdout, "%s (%s)\n", rel.version.Version, rel.version.Reason)
	return 0
}
