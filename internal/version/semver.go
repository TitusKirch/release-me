// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package version computes the next semantic version from conventional
// commit entry types.
//
// With no previous tag, the next version is exactly the configured initial
// version, applied only once at least one change bumps the version.
package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Semver is a parsed semantic version, without its "v" tag prefix.
type Semver struct {
	Major int
	Minor int
	Patch int
	Pre   string
}

var tagPattern = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

// ParseTag parses a "v"-prefixed semantic version tag such as "v1.2.3" or
// "v1.2.3-rc.1". ok is false when tag does not match that shape.
func ParseTag(tag string) (v Semver, ok bool) {
	m := tagPattern.FindStringSubmatch(tag)
	if m == nil {
		return Semver{}, false
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return Semver{}, false
	}
	minor, err := strconv.Atoi(m[2])
	if err != nil {
		return Semver{}, false
	}
	patch, err := strconv.Atoi(m[3])
	if err != nil {
		return Semver{}, false
	}
	return Semver{Major: major, Minor: minor, Patch: patch, Pre: m[4]}, true
}

// IsPreRelease reports whether v carries a pre-release identifier.
func (v Semver) IsPreRelease() bool {
	return v.Pre != ""
}

// Less reports whether v is lower precedence than other. Pre-release
// identifiers are not compared; callers that need to order pre-releases
// against each other should use LessPrecedence instead.
func (v Semver) Less(other Semver) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

// LessPrecedence reports whether v has lower semver precedence than other,
// following semver.org's precedence rules in full: major, minor and patch
// compared numerically first; when those are equal, a version carrying a
// pre-release identifier always sorts below the same major.minor.patch
// without one; when both carry one, their dot-separated identifiers are
// compared left to right, a numeric identifier compared numerically and
// always lower precedence than an alphanumeric one, and the version with
// fewer identifiers sorting lower when every identifier compared so far is
// equal.
func (v Semver) LessPrecedence(other Semver) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	if v.Patch != other.Patch {
		return v.Patch < other.Patch
	}
	if v.Pre == other.Pre {
		return false
	}
	if v.Pre == "" {
		return false
	}
	if other.Pre == "" {
		return true
	}
	return lessPreReleaseIdentifiers(v.Pre, other.Pre)
}

// lessPreReleaseIdentifiers compares two dot-separated pre-release
// identifier strings per semver precedence rule 11.
func lessPreReleaseIdentifiers(a, b string) bool {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		if aParts[i] == bParts[i] {
			continue
		}
		aNum, aIsNum := atoiOK(aParts[i])
		bNum, bIsNum := atoiOK(bParts[i])
		switch {
		case aIsNum && bIsNum:
			return aNum < bNum
		case aIsNum:
			return true
		case bIsNum:
			return false
		default:
			return aParts[i] < bParts[i]
		}
	}
	return len(aParts) < len(bParts)
}

func atoiOK(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// WithPreRelease returns a copy of v with Pre set to "<preType>.<n>", one
// pre-release build of v's major.minor.patch, for example "alpha.1".
func (v Semver) WithPreRelease(preType string, n int) Semver {
	v.Pre = fmt.Sprintf("%s.%d", preType, n)
	return v
}

// PreReleaseType parses v.Pre as the "<type>.<N>" shape WithPreRelease
// renders, returning the type and the counter. ok is false when v carries
// no pre-release identifier, or one not in that shape.
func (v Semver) PreReleaseType() (preType string, n int, ok bool) {
	if v.Pre == "" {
		return "", 0, false
	}
	preType, numStr, found := strings.Cut(v.Pre, ".")
	if !found {
		return "", 0, false
	}
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return "", 0, false
	}
	return preType, n, true
}

// String renders v as "X.Y.Z", or "X.Y.Z-<pre>" when v carries a
// pre-release identifier, without a "v" prefix or build metadata.
func (v Semver) String() string {
	if v.Pre == "" {
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	return fmt.Sprintf("%d.%d.%d-%s", v.Major, v.Minor, v.Patch, v.Pre)
}
