// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// expoVersionParts matches the major, minor and patch at the start of a
// version. A pre-release or build suffix after them is ignored: an Android
// version code is a single integer, and release-please reads those three
// numbers off the version to compute one.
var expoVersionParts = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

// UpdateExpo replaces the versions a React Native Expo project's app.json
// carries under its "expo" key, as release-please's expo release type does:
//
//   - "expo.version", the project's version, which must be there;
//   - "expo.ios.buildNumber", the same version again, where the file has
//     that member;
//   - "expo.android.versionCode", an integer computed from the version and
//     the Expo SDK's major version, where the file has that member.
//
// key is the Expo SDK version, which only the version code needs; see
// expoSDKMajor for the forms it takes. The project's package.json version
// is bumped by a "files" entry of its own with the json updater, as it is
// for the node release type this one extends.
//
// Everything else is kept byte for byte, so unlike release-please, which
// parses the file and writes it back out, a member this updater does not
// name keeps its formatting exactly. The nesting is read off the lines, so
// "expo", "ios" and "android" each have to open on a line of their own,
// the same shape the json updater requires of a path.
//
// An error is returned when key is no Expo SDK version, when version has no
// major, minor and patch to compute a version code from, or when the file
// carries no "expo.version".
func UpdateExpo(content, key, version string) (string, error) {
	sdkMajor, err := expoSDKMajor(key)
	if err != nil {
		return "", fmt.Errorf("updater: expo: %w", err)
	}
	code, err := expoVersionCode(sdkMajor, version)
	if err != nil {
		return "", fmt.Errorf("updater: expo: %w", err)
	}

	out, ok := updateJSONPath(content, []string{"expo", "version"}, version)
	if !ok {
		return "", fmt.Errorf("updater: expo: no expo.version found")
	}
	// Both of the following are bumped by release-please only where the
	// file already carries them, since neither is required of an Expo
	// project: an app that ships to neither store has no build number and
	// no version code to keep in step.
	if next, ok := updateJSONPath(out, []string{"expo", "ios", "buildNumber"}, version); ok {
		out = next
	}
	versionCode := jsonNumberLine("versionCode")
	if next, ok := updateJSONValue(out, []string{"expo", "android", "versionCode"}, versionCode,
		func(old string) string { return quoteLike(old, strconv.Itoa(code)) }); ok {
		out = next
	}
	return out, nil
}

// jsonNumberLine matches a JSON object member named key whose value is a
// number, "\"versionCode\": 42", or a string holding one, which an app.json
// written by hand may carry instead. The member's trailing comma is left to
// the third group, so a bare value is not read together with it the way
// valueLinePattern's bare alternative would read it.
func jsonNumberLine(key string) *regexp.Regexp {
	return regexp.MustCompile(`^(\s*"` + regexp.QuoteMeta(key) + `"\s*:\s*)("\d+"|\d+)(\s*,?\s*)$`)
}

// expoSDKMajor reads the Expo SDK's major version off key, the "files"
// entry's key for the expo updater.
//
// release-please reads it from the "expo" dependency of the project's
// package.json, falling back to a major of 0 where the project declares
// none. An updater is handed one file and cannot read another, so release-me
// takes it from the configuration instead. key is that dependency's own
// value, so "54", "54.0.0" and a range such as "~54.0.0", "^54.0.0" or
// ">=54.0.0" are all accepted and only the major is read off.
func expoSDKMajor(key string) (int, error) {
	major, _, _ := strings.Cut(strings.TrimLeft(key, "^~>=v "), ".")
	n, err := strconv.Atoi(major)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("key %q is not an Expo SDK version to read a major version off", key)
	}
	return n, nil
}

// expoVersionCode returns the Android version code release-please computes
// for version under an Expo SDK of major version sdkMajor:
//
//	sdkMajor*10000000 + major*10000 + minor*100 + patch
//
// Android requires a version code to be an integer that only ever grows,
// since the Play Store refuses an upload carrying one it has already seen.
// The formula is the scheme Maxi Rosson described and release-please's
// app.json updater implements, which gives each part its own decimal room:
// patch and minor up to 99, major up to 999, the SDK major above them all.
// That is why the SDK major has to be known: it keeps the version code
// growing across an SDK upgrade even where the project's own version does
// not.
//
// A pre-release or build suffix does not reach the version code, so
// 1.5.0-rc.1 and 1.5.0 yield the same one. release-please computes it the
// same way, from the version's major, minor and patch alone.
func expoVersionCode(sdkMajor int, version string) (int, error) {
	m := expoVersionParts.FindStringSubmatch(version)
	if m == nil {
		return 0, fmt.Errorf("version %q has no major, minor and patch to compute a version code from", version)
	}
	parts := make([]int, 3)
	for i := range parts {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return 0, fmt.Errorf("version %q is out of range for a version code: %w", version, err)
		}
		parts[i] = n
	}
	return sdkMajor*10000000 + parts[0]*10000 + parts[1]*100 + parts[2], nil
}
