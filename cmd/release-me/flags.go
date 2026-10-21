// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"io"
)

// componentUsage is the --component flag's help text, shared by every
// command that loops over components.
const componentUsage = "limit to one configured component"

// parseFlags parses the --verbose and --component flags shared by next,
// changelog and check, which write nothing and so have no --dry-run flag.
// ok is false when flag.ContinueOnError already reported the error on
// stderr; the caller should return exit code 2.
func parseFlags(name string, args []string, stderr io.Writer) (verbose bool, component string, ok bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	v := fs.Bool("verbose", false, "enable debug logging on stderr")
	c := fs.String("component", "", componentUsage)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return false, "", false
	}
	return *v, *c, true
}

// parseWriteFlags parses the --verbose, --dry-run and --component flags
// shared by pr, release and run. ok is false when flag.ContinueOnError
// already reported the error on stderr; the caller should return exit
// code 2.
func parseWriteFlags(name string, args []string, stderr io.Writer) (verbose, dryRun bool, component string, ok bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	v := fs.Bool("verbose", false, "enable debug logging on stderr")
	d := fs.Bool("dry-run", false, "print what would change, write nothing")
	c := fs.String("component", "", componentUsage)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return false, false, "", false
	}
	return *v, *d, *c, true
}
