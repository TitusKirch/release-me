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

func TestPreReleaseTags(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		setup  func(t *testing.T, dir string)
		prefix string
		base   version.Semver
		want   []history.PreReleaseTag
	}{
		{
			name: "reports none without a matching pre-release tag",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
				tag(t, dir, "v1.0.0")
			},
			prefix: "v",
			base:   version.Semver{Major: 1, Minor: 1, Patch: 0},
		},
		{
			name: "orders alpha below beta below rc by semver precedence, not creation order",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
				tag(t, dir, "v1.1.0-rc.1")
				tag(t, dir, "v1.1.0-alpha.1")
				tag(t, dir, "v1.1.0-beta.1")
			},
			prefix: "v",
			base:   version.Semver{Major: 1, Minor: 1, Patch: 0},
			want: []history.PreReleaseTag{
				{Tag: "v1.1.0-alpha.1", Version: version.Semver{Major: 1, Minor: 1, Patch: 0, Pre: "alpha.1"}},
				{Tag: "v1.1.0-beta.1", Version: version.Semver{Major: 1, Minor: 1, Patch: 0, Pre: "beta.1"}},
				{Tag: "v1.1.0-rc.1", Version: version.Semver{Major: 1, Minor: 1, Patch: 0, Pre: "rc.1"}},
			},
		},
		{
			name: "orders counters of the same type numerically",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
				tag(t, dir, "v1.1.0-alpha.10")
				tag(t, dir, "v1.1.0-alpha.2")
			},
			prefix: "v",
			base:   version.Semver{Major: 1, Minor: 1, Patch: 0},
			want: []history.PreReleaseTag{
				{Tag: "v1.1.0-alpha.2", Version: version.Semver{Major: 1, Minor: 1, Patch: 0, Pre: "alpha.2"}},
				{Tag: "v1.1.0-alpha.10", Version: version.Semver{Major: 1, Minor: 1, Patch: 0, Pre: "alpha.10"}},
			},
		},
		{
			name: "ignores a stable tag and a pre-release tag of a different base version",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
				tag(t, dir, "v1.1.0")
				tag(t, dir, "v1.2.0-alpha.1")
			},
			prefix: "v",
			base:   version.Semver{Major: 1, Minor: 1, Patch: 0},
		},
		{
			name: "uses a custom prefix",
			setup: func(t *testing.T, dir string) {
				commitEntry(t, dir, "chore: initial commit", "")
				tag(t, dir, "frontend/v0.4.0-alpha.1")
			},
			prefix: "frontend/v",
			base:   version.Semver{Major: 0, Minor: 4, Patch: 0},
			want: []history.PreReleaseTag{
				{Tag: "frontend/v0.4.0-alpha.1", Version: version.Semver{Minor: 4, Pre: "alpha.1"}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := newRepo(t)
			tc.setup(t, dir)

			got, err := history.PreReleaseTags(context.Background(), dir, tc.prefix, tc.base)
			if err != nil {
				t.Fatalf("PreReleaseTags() error = %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("PreReleaseTags() = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("PreReleaseTags()[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestTagExists(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	commitEntry(t, dir, "chore: initial commit", "")
	tag(t, dir, "duxt@v0.8.1")
	tag(t, dir, "duxt@v0.8.10")

	for _, tc := range []struct {
		tag  string
		want bool
	}{
		{tag: "duxt@v0.8.1", want: true},
		{tag: "duxt@v0.8", want: false},
		{tag: "duxt-typesense@v0.1.0", want: false},
	} {
		got, err := history.TagExists(context.Background(), dir, tc.tag)
		if err != nil {
			t.Fatalf("TagExists(%q) error = %v", tc.tag, err)
		}
		if got != tc.want {
			t.Fatalf("TagExists(%q) = %v, want %v", tc.tag, got, tc.want)
		}
	}
}
