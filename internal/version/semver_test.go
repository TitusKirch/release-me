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

func TestSemverStringRendersThePreReleaseIdentifier(t *testing.T) {
	t.Parallel()
	got := version.Semver{Major: 1, Minor: 2, Patch: 3, Pre: "alpha.1"}.String()
	if got != "1.2.3-alpha.1" {
		t.Fatalf("String() = %q, want %q", got, "1.2.3-alpha.1")
	}
}

func TestSemverWithPreReleaseSetsThePreField(t *testing.T) {
	t.Parallel()
	base := version.Semver{Major: 1, Minor: 2, Patch: 3}
	got := base.WithPreRelease("alpha", 1)
	want := version.Semver{Major: 1, Minor: 2, Patch: 3, Pre: "alpha.1"}
	if got != want {
		t.Fatalf("WithPreRelease() = %+v, want %+v", got, want)
	}
}

func TestSemverPreReleaseType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		v        version.Semver
		wantType string
		wantN    int
		wantOK   bool
	}{
		{name: "parses a pre-release identifier", v: version.Semver{Pre: "alpha.3"}, wantType: "alpha", wantN: 3, wantOK: true},
		{name: "reports no pre-release for a stable version", v: version.Semver{}, wantOK: false},
		{name: "rejects an identifier without a counter", v: version.Semver{Pre: "alpha"}, wantOK: false},
		{name: "rejects a non numeric counter", v: version.Semver{Pre: "alpha.x"}, wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			preType, n, ok := tc.v.PreReleaseType()
			if ok != tc.wantOK {
				t.Fatalf("PreReleaseType() ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && (preType != tc.wantType || n != tc.wantN) {
				t.Fatalf("PreReleaseType() = (%q, %d), want (%q, %d)", preType, n, tc.wantType, tc.wantN)
			}
		})
	}
}

func TestSemverLessPrecedenceOrdersPreReleasesBelowTheRelease(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b version.Semver
		want bool
	}{
		{
			name: "a pre-release sorts below the same release",
			a:    version.Semver{Major: 1, Pre: "alpha.1"}, b: version.Semver{Major: 1},
			want: true,
		},
		{
			name: "the release sorts above its own pre-release",
			a:    version.Semver{Major: 1}, b: version.Semver{Major: 1, Pre: "alpha.1"},
			want: false,
		},
		{
			name: "alpha sorts below beta",
			a:    version.Semver{Major: 1, Pre: "alpha.1"}, b: version.Semver{Major: 1, Pre: "beta.1"},
			want: true,
		},
		{
			name: "beta sorts below rc",
			a:    version.Semver{Major: 1, Pre: "beta.3"}, b: version.Semver{Major: 1, Pre: "rc.1"},
			want: true,
		},
		{
			name: "a lower counter of the same type sorts lower",
			a:    version.Semver{Major: 1, Pre: "alpha.1"}, b: version.Semver{Major: 1, Pre: "alpha.2"},
			want: true,
		},
		{
			name: "a higher counter of the same type is not lower",
			a:    version.Semver{Major: 1, Pre: "alpha.10"}, b: version.Semver{Major: 1, Pre: "alpha.2"},
			want: false,
		},
		{
			name: "a lower major always sorts lower regardless of pre-release",
			a:    version.Semver{Major: 1, Pre: "rc.1"}, b: version.Semver{Major: 2, Pre: "alpha.1"},
			want: true,
		},
		{
			name: "equal versions are not lower",
			a:    version.Semver{Major: 1, Pre: "alpha.1"}, b: version.Semver{Major: 1, Pre: "alpha.1"},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.a.LessPrecedence(tc.b); got != tc.want {
				t.Fatalf("%+v.LessPrecedence(%+v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
