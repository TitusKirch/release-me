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
// against each other should not rely on this method.
func (v Semver) Less(other Semver) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

// String renders v as "X.Y.Z", without a "v" prefix.
func (v Semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
