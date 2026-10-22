// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ostendorf-cloud/release-me/internal/config"
)

// errComponentNotAllowedForLocked is returned when --component names a
// component while versioning is locked: locked always releases every
// component together, so limiting to one is not a meaningful
// restriction. Every command that loops over components is a usage error
// in that combination, exit code 2.
var errComponentNotAllowedForLocked = errors.New("locked versioning releases the family, --component is not allowed")

// rejectComponentForLocked returns errComponentNotAllowedForLocked when
// cfg.Versioning is locked and component names one. Every command checks
// this once, right after loading release-me.yaml, before resolving which
// components to act on.
func rejectComponentForLocked(cfg config.Config, component string) error {
	if cfg.Versioning == config.VersioningLocked && component != "" {
		return errComponentNotAllowedForLocked
	}
	return nil
}

// resolveComponents returns the components a command should loop over:
// every configured component, in release-me.yaml's order, or the single
// one named component when it is not empty. An empty result is never
// returned; an unknown component name is a configuration error.
func resolveComponents(cfg config.Config, component string) ([]config.ResolvedComponent, error) {
	all := cfg.Resolve()
	if component == "" {
		return all, nil
	}
	for _, c := range all {
		if c.Name == component {
			return []config.ResolvedComponent{c}, nil
		}
	}
	return nil, fmt.Errorf("release-me: unknown component %q", component)
}

// printLine writes line to w, prefixed with "<name>: " when multi, the
// loop carries more than one component this run.
func printLine(w io.Writer, name string, multi bool, line string) {
	if multi {
		fmt.Fprintf(w, "%s: %s\n", name, line)
		return
	}
	fmt.Fprintln(w, line)
}

// printBlock writes a multi-line block, such as a rendered changelog, to w.
// Every line is prefixed with "<name>: " when multi; text is printed
// unchanged otherwise, nothing is written for an empty text.
func printBlock(w io.Writer, name string, multi bool, text string) {
	if text == "" {
		return
	}
	if !multi {
		fmt.Fprint(w, text)
		return
	}
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		fmt.Fprintf(w, "%s: %s\n", name, line)
	}
}
