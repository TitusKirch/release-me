// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/updater"
)

func TestUpdateGenericBumpsTheLinesReleasePleaseMarkersMark(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "bumps an inline version marker in any comment style",
			content: `const Version = "1.4.0" // x-release-please-version
VERSION = "1.4.0"  # x-release-please-version
untouched = "1.4.0"
`,
			want: `const Version = "1.5.0-rc.1" // x-release-please-version
VERSION = "1.5.0-rc.1"  # x-release-please-version
untouched = "1.4.0"
`,
		},
		{
			name: "bumps the major, minor and patch markers with their part of the version",
			content: `MAJOR = 1 # x-release-please-major
MINOR = 4 # x-release-please-minor
PATCH = 0 # x-release-please-patch
`,
			want: `MAJOR = 1 # x-release-please-major
MINOR = 5 # x-release-please-minor
PATCH = 0 # x-release-please-patch
`,
		},
		{
			name: "bumps every line of a block, the line closing it included",
			content: "# Install\n\n<!-- x-release-please-start-version -->\n" +
				"```sh\ncurl -fsSL https://example.com/notes/1.4.0/install.sh | sh\n```\n" +
				"Version 1.4.0 <!-- x-release-please-end -->\n\nSee 1.4.0's notes.\n",
			want: "# Install\n\n<!-- x-release-please-start-version -->\n" +
				"```sh\ncurl -fsSL https://example.com/notes/1.5.0-rc.1/install.sh | sh\n```\n" +
				"Version 1.5.0-rc.1 <!-- x-release-please-end -->\n\nSee 1.4.0's notes.\n",
		},
		{
			name: "matches an embedded number only for the major scope as release please does",
			content: `MAJOR = "v9" # x-release-please-major
MINOR = "v4" # x-release-please-minor
PATCH = "v8" # x-release-please-patch
`,
			want: `MAJOR = "v1" # x-release-please-major
MINOR = "v4" # x-release-please-minor
PATCH = "v8" # x-release-please-patch
`,
		},
		{
			name: "an inline scope overrides its enclosing block for that line only",
			content: `<!-- x-release-please-start-version -->
const Major = "9.4.7" // x-release-please-major
const Version = "9.4.7"
<!-- x-release-please-end -->
const Untouched = "9.4.7"
`,
			want: `<!-- x-release-please-start-version -->
const Major = "1.4.7" // x-release-please-major
const Version = "1.5.0-rc.1"
<!-- x-release-please-end -->
const Untouched = "9.4.7"
`,
		},
		{
			name: "an end marker outside a block leaves the line alone",
			content: `const Version = "1.4.0" // x-release-please-version
Version 1.4.0 <!-- x-release-please-end -->
`,
			want: `const Version = "1.5.0-rc.1" // x-release-please-version
Version 1.4.0 <!-- x-release-please-end -->
`,
		},
		{
			name:    "replaces a version's pre-release and build suffix too",
			content: "image: ghcr.io/acme/notes:1.5.0-beta.2+build.7 # x-release-please-version\n",
			want:    "image: ghcr.io/acme/notes:1.5.0-rc.1 # x-release-please-version\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := updater.UpdateGeneric(tc.content, "1.5.0-rc.1")
			if err != nil {
				t.Fatalf("UpdateGeneric() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("UpdateGeneric() =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestUpdateGenericRejectsMarkersItCannotApply(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
	}{
		{name: "rejects content with no marker", content: "const Version = \"1.4.0\"\n"},
		{name: "an end marker alone does not mark content for updating", content: "Version 1.4.0 <!-- x-release-please-end -->\n"},
		{name: "rejects a date marker", content: "Released 2026-01-05 <!-- x-release-please-date -->\n"},
		{name: "rejects a version-date block", content: "<!-- x-release-please-start-version-date -->\n1.4.0 (2026-01-05)\n<!-- x-release-please-end -->\n"},
		{name: "rejects a block that is never closed", content: "<!-- x-release-please-start-version -->\n1.4.0\n"},
		{name: "an inline marker takes precedence over an end marker on the same line", content: "<!-- x-release-please-start-version -->\n1.4.0 <!-- x-release-please-version x-release-please-end -->\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := updater.UpdateGeneric(tc.content, "2.0.0"); err == nil {
				t.Fatal("UpdateGeneric() error = nil, want an error")
			}
		})
	}
}

func TestGenericMarkersReplaceHyphenatedPrereleaseAndBuildSuffixes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, content, want string }{
		{name: "inline", content: "VERSION = \"1.2.3-preview-one.1+build-ci.7\" # x-release-please-version\n", want: "VERSION = \"2.0.0\" # x-release-please-version\n"},
		{name: "block", content: "<!-- x-release-please-start-version -->\nimage: notes:1.2.3-preview-one.1+build-ci.7\n<!-- x-release-please-end -->\n", want: "<!-- x-release-please-start-version -->\nimage: notes:2.0.0\n<!-- x-release-please-end -->\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := updater.UpdateGeneric(tc.content, "2.0.0")
			if err != nil || got != tc.want {
				t.Fatalf("generic = %q,%v; want %q", got, err, tc.want)
			}
		})
	}
}
