// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package version

// Bump names the kind of version increment applied.
type Bump string

// The bump kinds a release can have.
const (
	BumpNone    Bump = "none"
	BumpPatch   Bump = "patch"
	BumpMinor   Bump = "minor"
	BumpMajor   Bump = "major"
	BumpInitial Bump = "initial"
)

// Change is the bump-relevant part of one history entry.
type Change struct {
	Type     string
	Breaking bool
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
func Next(last Semver, lastOK bool, changes []Change, initialVersion string, bumpMinorPreMajor bool) (result Result, released bool) {
	bump, reason := highestBump(changes)
	if bump == BumpNone {
		return Result{}, false
	}

	if !lastOK {
		return Result{Version: initialVersion, Bump: BumpInitial, Reason: "initial version"}, true
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
	case BumpNone, BumpInitial:
		// unreachable: highestBump never returns these
	}
	return Result{Version: next.String(), Bump: bump, Reason: reason}, true
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
	case BumpNone, BumpInitial:
		return 0
	default:
		return 0
	}
}
