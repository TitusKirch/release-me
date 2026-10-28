// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"errors"
	"fmt"
	"strings"
)

// Bump names the kind of version increment applied.
type Bump string

// The bump kinds a release can have.
const (
	BumpNone    Bump = "none"
	BumpPatch   Bump = "patch"
	BumpMinor   Bump = "minor"
	BumpMajor   Bump = "major"
	BumpInitial Bump = "initial"
	// BumpForced is a version a "Release-As" footer forces, see ReleaseAs.
	BumpForced Bump = "forced"
)

// Change is the bump-relevant part of one history entry.
type Change struct {
	Type     string
	Breaking bool
	// ReleaseAs is the value of the entry's "Release-As" footer, "" when
	// it carries none. A Change may carry only ReleaseAs, no Type: it then
	// forces a version without bumping one.
	ReleaseAs string
}

// Result is the computed next version.
type Result struct {
	Version string
	Bump    Bump
	Reason  string
}

// Next computes the next version from changes since the last tag.
//
// lastOK is false when no version tag exists yet; Next then returns
// initialVersion, still only once at least one change bumps the version.
// released is false when there is nothing to release: either no changes at
// all, or no change of a type that bumps the version.
//
// A usable "Release-As" footer among changes, see ReleaseAs, wins outright
// over the computed bump and the initial version, and releases even when
// no change bumps the version. refused lists every footer value ReleaseAs
// could not use, whatever Next returns otherwise: Next never logs, so the
// caller reports them, once per computation.
func Next(
	last Semver, lastOK bool, changes []Change, initialVersion string, bumpMinorPreMajor bool,
) (result Result, released bool, refused []Refusal) {
	forced, ok, refused := ReleaseAs(last, lastOK, changes)
	if ok {
		return Result{Version: forced.String(), Bump: BumpForced, Reason: "Release-As footer"}, true, refused
	}

	bump, reason := highestBump(changes)
	if bump == BumpNone {
		return Result{}, false, refused
	}

	if !lastOK {
		return Result{Version: initialVersion, Bump: BumpInitial, Reason: "initial version"}, true, refused
	}

	next := last
	switch bump {
	case BumpMajor:
		if bumpMinorPreMajor && last.Major < 1 {
			next = Semver{Major: last.Major, Minor: last.Minor + 1}
			bump = BumpMinor
			reason += ", minor below 1.0.0"
		} else {
			next = Semver{Major: last.Major + 1}
		}
	case BumpMinor:
		next = Semver{Major: last.Major, Minor: last.Minor + 1}
	case BumpPatch:
		next = Semver{Major: last.Major, Minor: last.Minor, Patch: last.Patch + 1}
	case BumpNone, BumpInitial, BumpForced:
		// unreachable: highestBump never returns these
	}
	return Result{Version: next.String(), Bump: bump, Reason: reason}, true, refused
}

// Changed reports whether changes contains at least one change that bumps
// the version. Next returns released true for exactly these changes,
// regardless of the previous version: callers that need to know whether a
// component has anything to release, without computing its next version
// yet, can use Changed instead.
func Changed(changes []Change) bool {
	bump, _ := highestBump(changes)
	return bump != BumpNone
}

func highestBump(changes []Change) (bump Bump, reason string) {
	bump = BumpNone
	for _, c := range changes {
		var b Bump
		var r string
		switch {
		case c.Breaking:
			b, r = BumpMajor, "breaking change"
		case c.Type == "feat":
			b, r = BumpMinor, "feat"
		case c.Type == "fix":
			b, r = BumpPatch, "fix"
		case c.Type == "perf":
			b, r = BumpPatch, "perf"
		case c.Type == "revert":
			b, r = BumpPatch, "revert"
		default:
			continue
		}
		if severity(b) > severity(bump) {
			bump, reason = b, r
		}
	}
	return bump, reason
}

func severity(b Bump) int {
	switch b {
	case BumpMajor:
		return 3
	case BumpMinor:
		return 2
	case BumpPatch:
		return 1
	case BumpNone, BumpInitial, BumpForced:
		return 0
	default:
		return 0
	}
}

// Refusal is one "Release-As" footer value ReleaseAs could not use, with
// the reason in one line.
type Refusal struct {
	Value  string
	Reason string
}

// The reasons ParseForced refuses a value for, in the one-line form a log
// line or a reply names them in.
var (
	ErrNotAVersion = errors.New("does not parse as an X.Y.Z version")
	ErrPreRelease  = errors.New("is a pre-release version")
	ErrNotGreater  = errors.New("is not greater than the last released version")
)

// ParseForced is the one rule every forced version follows, a
// "Release-As" footer, a release-me-version block and a release-as
// comment command alike: value parses as a plain X.Y.Z version, with an
// optional "v" or "V" prefix and an optional pre-release and build
// suffix, and is strictly greater than last, the pre-release suffix not
// compared. With no last tag yet (lastOK false) any such version passes.
// A pre-release value passes only when allowPreRelease is true. The error
// wraps ErrNotAVersion, ErrPreRelease or ErrNotGreater; its text is the
// reason alone, ErrNotGreater's naming last.
func ParseForced(value string, last Semver, lastOK, allowPreRelease bool) (Semver, error) {
	v, ok := ParseTag("v" + strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V"))
	switch {
	case !ok:
		return Semver{}, ErrNotAVersion
	case v.IsPreRelease() && !allowPreRelease:
		return Semver{}, ErrPreRelease
	case lastOK && !last.Less(v):
		return Semver{}, fmt.Errorf("%w %s", ErrNotGreater, last)
	default:
		return v, nil
	}
}

// ReleaseAs picks the version the "Release-As" footers among changes
// force. A value is usable when ParseForced accepts it against last, a
// pre-release value refused. Of several usable values the highest wins,
// whatever order the commits carrying them came in. A pre-release value
// is refused because the footer stays in the history until the next
// stable tag, so it would force the same pre-release again after it
// shipped; a pre-release label on the release pull request builds on the
// forced version instead. ok is false when no value is usable; refused
// lists every distinct value that is not, once, in the order changes
// first carries it, so a commit attributed to several components of one
// family is refused once.
func ReleaseAs(last Semver, lastOK bool, changes []Change) (forced Semver, ok bool, refused []Refusal) {
	seen := make(map[string]bool)
	for _, c := range changes {
		if c.ReleaseAs == "" {
			continue
		}
		v, err := ParseForced(c.ReleaseAs, last, lastOK, false)
		switch {
		case err != nil:
			if seen[c.ReleaseAs] {
				continue
			}
			seen[c.ReleaseAs] = true
			reason := err.Error()
			if errors.Is(err, ErrPreRelease) {
				reason += ", set one with a pre-release label instead"
			}
			refused = append(refused, Refusal{Value: c.ReleaseAs, Reason: reason})
		case !ok || forced.Less(v):
			forced, ok = v, true
		}
	}
	return forced, ok, refused
}
