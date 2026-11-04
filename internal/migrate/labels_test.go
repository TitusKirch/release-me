// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"slices"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

func TestFromReleasePleaseCarriesEveryLabelOver(t *testing.T) {
	t.Parallel()
	configJSON := `{"label": "autorelease: pending, release", "release-label": "autorelease: tagged",
	  "extra-label": "team: platform,automerge", "packages": {".": {"release-type": "go"}}}`
	got, err := migrate.FromReleasePlease([]byte(configJSON), nil, files(nil))
	if err != nil {
		t.Fatalf("FromReleasePlease() error = %v", err)
	}
	l := load(t, got.YAML).Labels
	if !slices.Equal(l.Pending, []string{"autorelease: pending", "release"}) ||
		!slices.Equal(l.Tagged, []string{"autorelease: tagged"}) ||
		!slices.Equal(l.Extra, []string{"team: platform", "automerge"}) {
		t.Fatalf("Labels = %+v\n%s", l, got.YAML)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("Warnings = %q, want none", got.Warnings)
	}
}
