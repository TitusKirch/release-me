// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

func TestFromReleasePleaseExplainsWhyDateFormatIsLeftOut(t *testing.T) {
	t.Parallel()
	configJSON := `{"date-format": "%d.%m.%Y", "packages": {".": {"release-type": "go", "date-format": "%Y"}}}`
	got, err := migrate.FromReleasePlease([]byte(configJSON), nil, files(nil))
	if err != nil {
		t.Fatalf("FromReleasePlease() error = %v", err)
	}
	reason := "release-please formats the date its generic updater writes with it, not the changelog's, " +
		"which release-me's changelog-date-format sets, left out"
	want := []string{"date-format: " + reason, "package .: date-format: " + reason}
	var w []string
	for _, warning := range got.Warnings {
		if strings.Contains(warning, "date-format") {
			w = append(w, warning)
		}
	}
	if !reflect.DeepEqual(w, want) {
		t.Fatalf("warnings = %q, want %q", w, want)
	}
	if load(t, got.YAML).ChangelogStyle.DateFormat != "" {
		t.Fatalf("YAML = %s, want no changelog-date-format", got.YAML)
	}
}
