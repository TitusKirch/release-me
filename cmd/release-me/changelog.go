// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
)

// runChangelog implements the changelog command: it prints the rendered
// entries for the next version, nothing when there is nothing to release.
func runChangelog(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("changelog", flag.ContinueOnError)
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

	out := changelog.Render(rel.version.Version, time.Now(), rel.entries, rel.hiddenTypes)
	fmt.Fprint(stdout, out)
	return 0
}
