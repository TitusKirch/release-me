// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"slices"
	"strings"
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

func TestFromReleasePleaseRejectsExplicitlyEmptyReleaseLabels(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"label", "release-label"} {
		for _, value := range []string{"", " , , "} {
			t.Run(key+"="+value, func(t *testing.T) {
				t.Parallel()
				configJSON := `{"` + key + `":"` + value + `","packages":{".":{"release-type":"go"}}}`
				_, err := migrate.FromReleasePlease([]byte(configJSON), nil, files(nil))
				if err == nil || !strings.Contains(err.Error(), key) {
					t.Fatalf("FromReleasePlease() error = %v, want an error naming %s", err, key)
				}
			})
		}
	}
}

func TestFromReleasePleaseAllowsExplicitlyEmptyExtraLabels(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", " , , "} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			configJSON := `{"extra-label":"` + value + `","packages":{".":{"release-type":"go"}}}`
			got, err := migrate.FromReleasePlease([]byte(configJSON), nil, files(nil))
			if err != nil {
				t.Fatalf("FromReleasePlease() error = %v", err)
			}
			if labels := load(t, got.YAML).Labels; len(labels.Extra) != 0 || len(labels.Pending) == 0 || len(labels.Tagged) == 0 {
				t.Fatalf("Labels = %+v, want no extras and the default release labels", labels)
			}
		})
	}
}
