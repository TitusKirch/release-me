// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history_test

import (
	"context"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/history"
	"github.com/ostendorf-cloud/release-me/internal/version"
)

func TestLastTag(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		setup       func(t *testing.T, dir string)
		prefix      string
		wantTag     string
		wantVersion version.Semver
		wantOK      bool
	}{
		{
			name: "reports not found without a tag",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
			},
			prefix: "v",
			wantOK: false,
		},
		{
			name: "picks the highest semver tag",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
				tag(t, dir, "v0.1.0")
				commitEntry(t, dir, "feat: add a command", "")
				tag(t, dir, "v0.2.0")
				commitEntry(t, dir, "fix: a bug", "")
				tag(t, dir, "v0.10.0")
			},
			prefix:      "v",
			wantTag:     "v0.10.0",
			wantVersion: version.Semver{Major: 0, Minor: 10, Patch: 0},
			wantOK:      true,
		},
		{
			name: "ignores pre release tags",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
				tag(t, dir, "v1.0.0")
				commitEntry(t, dir, "feat: add a command", "")
				tag(t, dir, "v1.1.0-rc.1")
			},
			prefix:      "v",
			wantTag:     "v1.0.0",
			wantVersion: version.Semver{Major: 1, Minor: 0, Patch: 0},
			wantOK:      true,
		},
		{
			name: "ignores tags not matching the prefix",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
				tag(t, dir, "release-1.0.0")
			},
			prefix: "v",
			wantOK: false,
		},
		{
			name: "uses a custom prefix",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
				tag(t, dir, "component-1.2.0")
			},
			prefix:      "component-",
			wantTag:     "component-1.2.0",
			wantVersion: version.Semver{Major: 1, Minor: 2, Patch: 0},
			wantOK:      true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := newRepo(t)
			tc.setup(t, dir)

			gotTag, gotVersion, ok, err := history.LastTag(context.Background(), dir, tc.prefix)
			if err != nil {
				t.Fatalf("LastTag() error = %v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if gotTag != tc.wantTag {
				t.Fatalf("tag = %q, want %q", gotTag, tc.wantTag)
			}
			if gotVersion != tc.wantVersion {
				t.Fatalf("version = %+v, want %+v", gotVersion, tc.wantVersion)
			}
		})
	}
}
