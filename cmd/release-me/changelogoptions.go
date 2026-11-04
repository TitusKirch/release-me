// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/ostendorf-cloud/release-me/internal/changelog"
	"github.com/ostendorf-cloud/release-me/internal/config"
)

// changelogOptions translates comp's changelog settings into the options
// its entries render with: hidden-types and changelog-date-format.
func changelogOptions(comp config.ResolvedComponent) changelog.Options {
	return changelog.Options{HiddenTypes: comp.HiddenTypes, DateFormat: comp.ChangelogStyle.DateFormat}
}
