// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"io"
)

// parseWriteFlags parses the --verbose and --dry-run flags shared by pr,
// release and run. ok is false when flag.ContinueOnError already reported
// the error on stderr; the caller should return exit code 2.
func parseWriteFlags(name string, args []string, stderr io.Writer) (verbose, dryRun, ok bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	v := fs.Bool("verbose", false, "enable debug logging on stderr")
	d := fs.Bool("dry-run", false, "print what would change, write nothing")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return false, false, false
	}
	return *v, *d, true
}
