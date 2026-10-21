// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/updater"
)

func TestUpdateMarkerReplacesTheVersionOnMarkedLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		version string
		want    string
	}{
		{
			name:    "replaces a quoted version before a slash comment",
			content: `const Version = "1.2.3" // release-me: version`,
			version: "2.0.0",
			want:    `const Version = "2.0.0" // release-me: version`,
		},
		{
			name:    "replaces a quoted version before a hash comment",
			content: `version: "1.2.3" # release-me: version`,
			version: "2.0.0",
			want:    `version: "2.0.0" # release-me: version`,
		},
		{
			name:    "replaces a bare version literal",
			content: `version = 1.2.3 # release-me: version`,
			version: "1.3.0",
			want:    `version = 1.3.0 # release-me: version`,
		},
		{
			name:    "keeps the original comment spacing",
			content: `VERSION="1.2.3"  #release-me: version`,
			version: "1.3.0",
			want:    `VERSION="1.3.0"  #release-me: version`,
		},
		{
			name: "updates every marked line and leaves the rest untouched",
			content: `package main

const Version = "1.2.3" // release-me: version

func other() string {
	return "1.2.3"
}

const Fallback = "1.2.3" // release-me: version
`,
			version: "2.0.0",
			want: `package main

const Version = "2.0.0" // release-me: version

func other() string {
	return "1.2.3"
}

const Fallback = "2.0.0" // release-me: version
`,
		},
		{
			name:    "replaces only the first version literal before the marker",
			content: `const Image = "go1.21-1.2.3" // release-me: version`,
			version: "1.3.0",
			want:    `const Image = "go1.21-1.3.0" // release-me: version`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := updater.UpdateMarker(tc.content, tc.version)
			if err != nil {
				t.Fatalf("UpdateMarker() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("UpdateMarker() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUpdateMarkerRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
	}{
		{name: "rejects content with no marker line", content: "const Version = \"1.2.3\"\n"},
		{name: "rejects a marker line with no version literal", content: "const Version = current // release-me: version\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := updater.UpdateMarker(tc.content, "2.0.0"); err == nil {
				t.Fatal("UpdateMarker() error = nil, want an error")
			}
		})
	}
}

func TestUpdatersRegistersTheMarkerUpdater(t *testing.T) {
	t.Parallel()
	if _, ok := updater.Updaters[updater.Marker]; !ok {
		t.Fatalf("Updaters[%q] missing", updater.Marker)
	}
}
