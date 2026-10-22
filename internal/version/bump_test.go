// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package version_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/version"
)

func TestNextComputesTheReleasedVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name              string
		last              version.Semver
		lastOK            bool
		changes           []version.Change
		initialVersion    string
		bumpMinorPreMajor bool
		want              version.Result
	}{
		{
			name:    "bumps minor for a feature",
			last:    version.Semver{Major: 1, Minor: 2, Patch: 3},
			lastOK:  true,
			changes: []version.Change{{Type: "feat"}},
			want:    version.Result{Version: "1.3.0", Bump: version.BumpMinor, Reason: "feat"},
		},
		{
			name:    "bumps patch for a fix",
			last:    version.Semver{Major: 1, Minor: 2, Patch: 3},
			lastOK:  true,
			changes: []version.Change{{Type: "fix"}},
			want:    version.Result{Version: "1.2.4", Bump: version.BumpPatch, Reason: "fix"},
		},
		{
			name:    "bumps patch for a perf change",
			last:    version.Semver{Major: 1, Minor: 2, Patch: 3},
			lastOK:  true,
			changes: []version.Change{{Type: "perf"}},
			want:    version.Result{Version: "1.2.4", Bump: version.BumpPatch, Reason: "perf"},
		},
		{
			name:    "bumps patch for a revert",
			last:    version.Semver{Major: 1, Minor: 2, Patch: 3},
			lastOK:  true,
			changes: []version.Change{{Type: "revert"}},
			want:    version.Result{Version: "1.2.4", Bump: version.BumpPatch, Reason: "revert"},
		},
		{
			name:    "bumps major for a breaking change",
			last:    version.Semver{Major: 1, Minor: 2, Patch: 3},
			lastOK:  true,
			changes: []version.Change{{Type: "feat", Breaking: true}},
			want:    version.Result{Version: "2.0.0", Bump: version.BumpMajor, Reason: "breaking change"},
		},
		{
			name:              "bumps minor for a breaking change below one pre major",
			last:              version.Semver{Major: 0, Minor: 4, Patch: 1},
			lastOK:            true,
			changes:           []version.Change{{Type: "feat", Breaking: true}},
			bumpMinorPreMajor: true,
			want:              version.Result{Version: "0.5.0", Bump: version.BumpMinor, Reason: "breaking change, minor below 1.0.0"},
		},
		{
			name:    "keeps major for a breaking change below one without pre major",
			last:    version.Semver{Major: 0, Minor: 4, Patch: 1},
			lastOK:  true,
			changes: []version.Change{{Type: "feat", Breaking: true}},
			want:    version.Result{Version: "1.0.0", Bump: version.BumpMajor, Reason: "breaking change"},
		},
		{
			name:    "picks the highest bump among multiple changes",
			last:    version.Semver{Major: 1, Minor: 0, Patch: 0},
			lastOK:  true,
			changes: []version.Change{{Type: "fix"}, {Type: "feat"}, {Type: "chore"}},
			want:    version.Result{Version: "1.1.0", Bump: version.BumpMinor, Reason: "feat"},
		},
		{
			name:           "uses the initial version when no tag exists and a change bumps",
			lastOK:         false,
			changes:        []version.Change{{Type: "chore"}, {Type: "feat"}},
			initialVersion: "0.3.0",
			want:           version.Result{Version: "0.3.0", Bump: version.BumpInitial, Reason: "initial version"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, released := version.Next(tc.last, tc.lastOK, tc.changes, tc.initialVersion, tc.bumpMinorPreMajor)
			if !released {
				t.Fatal("released = false, want true")
			}
			if got != tc.want {
				t.Fatalf("Next() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNextReportsNoRelease(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		last    version.Semver
		lastOK  bool
		changes []version.Change
	}{
		{
			name:    "no change bumps the version on a tagged repository",
			last:    version.Semver{Major: 1, Minor: 0, Patch: 0},
			lastOK:  true,
			changes: []version.Change{{Type: "chore"}, {Type: "docs"}, {Type: "other"}},
		},
		{
			name:   "there are no changes on a tagged repository",
			last:   version.Semver{Major: 1, Minor: 0, Patch: 0},
			lastOK: true,
		},
		{
			name:   "no tag exists and there are no changes",
			lastOK: false,
		},
		{
			name:    "no tag exists and no change bumps the version",
			lastOK:  false,
			changes: []version.Change{{Type: "chore"}, {Type: "docs"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, released := version.Next(tc.last, tc.lastOK, tc.changes, "0.1.0", false)
			if released {
				t.Fatal("released = true, want false")
			}
		})
	}
}

func TestChangedReportsWhetherAnyChangeBumpsTheVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		changes []version.Change
		want    bool
	}{
		{name: "no changes"},
		{name: "only hidden types", changes: []version.Change{{Type: "chore"}, {Type: "docs"}}},
		{name: "a fix bumps", changes: []version.Change{{Type: "chore"}, {Type: "fix"}}, want: true},
		{name: "a breaking change bumps", changes: []version.Change{{Type: "fix", Breaking: true}}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := version.Changed(tc.changes); got != tc.want {
				t.Fatalf("Changed(%+v) = %v, want %v", tc.changes, got, tc.want)
			}
		})
	}
}
