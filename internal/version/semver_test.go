// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package version_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/version"
)

func TestParseTagReadsMajorMinorPatchAndPreRelease(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		tag    string
		want   version.Semver
		wantOK bool
	}{
		{name: "parses a release tag", tag: "v1.2.3", want: version.Semver{Major: 1, Minor: 2, Patch: 3}, wantOK: true},
		{name: "parses a zero version", tag: "v0.1.0", want: version.Semver{Major: 0, Minor: 1, Patch: 0}, wantOK: true},
		{name: "parses a pre release tag", tag: "v1.0.0-rc.1", want: version.Semver{Major: 1, Minor: 0, Patch: 0, Pre: "rc.1"}, wantOK: true},
		{name: "parses a build metadata suffix", tag: "v1.0.0+build.5", want: version.Semver{Major: 1, Minor: 0, Patch: 0}, wantOK: true},
		{name: "rejects a tag without the v prefix", tag: "1.2.3", wantOK: false},
		{name: "rejects a non semver tag", tag: "v1.2", wantOK: false},
		{name: "rejects an unrelated tag", tag: "release-2024", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := version.ParseTag(tc.tag)
			if ok != tc.wantOK {
				t.Fatalf("ParseTag(%q) ok = %v, want %v", tc.tag, ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Fatalf("ParseTag(%q) = %+v, want %+v", tc.tag, got, tc.want)
			}
		})
	}
}

func TestSemverLessOrdersByPrecedence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b version.Semver
		want bool
	}{
		{name: "lower major is less", a: version.Semver{Major: 1}, b: version.Semver{Major: 2}, want: true},
		{name: "lower minor is less", a: version.Semver{Minor: 1}, b: version.Semver{Minor: 2}, want: true},
		{name: "lower patch is less", a: version.Semver{Patch: 1}, b: version.Semver{Patch: 2}, want: true},
		{name: "equal versions are not less", a: version.Semver{Major: 1}, b: version.Semver{Major: 1}, want: false},
		{name: "higher major is not less", a: version.Semver{Major: 2}, b: version.Semver{Major: 1}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.a.Less(tc.b); got != tc.want {
				t.Fatalf("%+v.Less(%+v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestSemverStringRendersWithoutAVPrefix(t *testing.T) {
	t.Parallel()
	got := version.Semver{Major: 1, Minor: 2, Patch: 3}.String()
	if got != "1.2.3" {
		t.Fatalf("String() = %q, want %q", got, "1.2.3")
	}
}
