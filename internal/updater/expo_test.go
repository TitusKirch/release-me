// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/updater"
)

const appJSON = `{
  "expo": {
    "name": "Release Notes",
    "slug": "release-notes",
    "version": "1.4.0",
    "orientation": "portrait",
    "ios": {
      "bundleIdentifier": "cloud.ostendorf.notes",
      "buildNumber": "1.4.0"
    },
    "android": {
      "package": "cloud.ostendorf.notes",
      "versionCode": 540010400
    }
  }
}
`

func TestUpdateExpoBumpsTheVersionTheBuildNumberAndTheVersionCode(t *testing.T) {
	t.Parallel()
	got, err := updater.UpdateExpo(appJSON, "54", "1.5.0")
	if err != nil {
		t.Fatalf("UpdateExpo() error = %v", err)
	}
	want := appJSON
	for _, r := range []struct{ from, to string }{
		{`    "version": "1.4.0",`, `    "version": "1.5.0",`},
		{`      "buildNumber": "1.4.0"`, `      "buildNumber": "1.5.0"`},
		{`      "versionCode": 540010400`, `      "versionCode": 540010500`},
	} {
		next := replaceLine(want, r.from, r.to)
		if next == want {
			t.Fatalf("fixture has no line %q", r.from)
		}
		want = next
	}
	if got != want {
		t.Fatalf("UpdateExpo() =\n%s\nwant\n%s", got, want)
	}
}

// TestUpdateExpoComputesTheAndroidVersionCode pins the arithmetic
// release-please's app.json updater performs: the Expo SDK's major version
// times ten million, plus the version's major times ten thousand, its minor
// times a hundred and its patch.
func TestUpdateExpoComputesTheAndroidVersionCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		key     string
		version string
		want    string
	}{
		{
			name:    "the SDK major carries the version code's high digits",
			key:     "54",
			version: "1.5.0",
			want:    "540010500",
		},
		{
			name:    "an SDK downgrade lowers the version code with it",
			key:     "53",
			version: "1.5.0",
			want:    "530010500",
		},
		{
			name:    "each part of the version keeps its own decimal room",
			key:     "54",
			version: "2.11.7",
			want:    "540021107",
		},
		{
			name:    "a pre-release yields the version code of its release",
			key:     "54",
			version: "1.5.0-rc.1",
			want:    "540010500",
		},
		{
			name:    "an SDK major of zero leaves the version alone in the version code",
			key:     "0",
			version: "1.5.0",
			want:    "10500",
		},
		{
			name:    "the key may be the package.json range the SDK is declared as",
			key:     "~54.0.0",
			version: "1.5.0",
			want:    "540010500",
		},
		{
			name:    "the key may be a caret range",
			key:     "^54.0.0",
			version: "1.5.0",
			want:    "540010500",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := updater.UpdateExpo(appJSON, tc.key, tc.version)
			if err != nil {
				t.Fatalf("UpdateExpo() error = %v", err)
			}
			line := `      "versionCode": ` + tc.want
			if !strings.Contains(got, line+"\n") {
				t.Fatalf("UpdateExpo() =\n%s\nwant a line %q", got, line)
			}
		})
	}
}

func TestUpdateExpoLeavesOutTheMembersTheFileDoesNotCarry(t *testing.T) {
	t.Parallel()
	content := "{\n  \"expo\": {\n    \"name\": \"Release Notes\",\n    \"version\": \"1.4.0\"\n  }\n}\n"
	got, err := updater.UpdateExpo(content, "54", "1.5.0")
	if err != nil {
		t.Fatalf("UpdateExpo() error = %v", err)
	}
	want := replaceLine(content, `    "version": "1.4.0"`, `    "version": "1.5.0"`)
	if got != want {
		t.Fatalf("UpdateExpo() =\n%s\nwant\n%s", got, want)
	}
}

func TestUpdateExpoLeavesAVersionCodeOutsideTheAndroidSettingsAlone(t *testing.T) {
	t.Parallel()
	content := "{\n  \"versionCode\": 7,\n  \"expo\": {\n    \"version\": \"1.4.0\",\n" +
		"    \"extra\": {\n      \"versionCode\": 8\n    }\n  }\n}\n"
	got, err := updater.UpdateExpo(content, "54", "1.5.0")
	if err != nil {
		t.Fatalf("UpdateExpo() error = %v", err)
	}
	want := replaceLine(content, `    "version": "1.4.0",`, `    "version": "1.5.0",`)
	if got != want {
		t.Fatalf("UpdateExpo() =\n%s\nwant\n%s", got, want)
	}
}

func TestUpdateExpoKeepsAQuotedVersionCodeQuoted(t *testing.T) {
	t.Parallel()
	content := replaceLine(appJSON, `      "versionCode": 540010400`, `      "versionCode": "540010400"`)
	got, err := updater.UpdateExpo(content, "54", "1.5.0")
	if err != nil {
		t.Fatalf("UpdateExpo() error = %v", err)
	}
	if !strings.Contains(got, `      "versionCode": "540010500"`+"\n") {
		t.Fatalf("UpdateExpo() =\n%s\nwant a quoted version code", got)
	}
}

func TestUpdateExpoRejectsAFileOrKeyItCannotBump(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		key     string
		version string
	}{
		{
			name:    "rejects an app.json without an expo.version",
			content: "{\n  \"expo\": {\n    \"name\": \"Release Notes\"\n  }\n}\n",
			key:     "54",
			version: "1.5.0",
		},
		{
			name:    "rejects a version outside the expo key",
			content: "{\n  \"version\": \"1.4.0\"\n}\n",
			key:     "54",
			version: "1.5.0",
		},
		{
			name:    "rejects a key that is no Expo SDK version",
			content: appJSON,
			key:     "latest",
			version: "1.5.0",
		},
		{
			name:    "rejects an empty key",
			content: appJSON,
			key:     "",
			version: "1.5.0",
		},
		{
			name:    "rejects a version without a patch to compute a version code from",
			content: appJSON,
			key:     "54",
			version: "1.5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := updater.UpdateExpo(tc.content, tc.key, tc.version); err == nil {
				t.Fatal("UpdateExpo() error = nil, want an error")
			}
		})
	}
}
