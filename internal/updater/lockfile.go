// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"fmt"
	"regexp"
	"strings"
)

// UpdateNpmLock replaces the version a package-lock.json or
// npm-shrinkwrap.json records for the package it locks: the top-level
// "version" field and, in a lockfile version 2 or 3, the "version" of the
// root package, the entry keyed "" under "packages". Dependencies and
// workspace packages are left alone. Everything else is kept byte for
// byte, so the file stays in npm's own formatting. An error is returned
// when there is no top-level version field.
func UpdateNpmLock(content, version string) (string, error) {
	out, ok := updateJSONPath(content, []string{"version"}, version)
	if !ok {
		return "", fmt.Errorf("updater: npm-lock: no top-level version field found")
	}
	if next, ok := updateJSONPath(out, []string{"packages", "", "version"}, version); ok {
		out = next
	}
	return out, nil
}

// cargoTable matches any TOML table or array of tables header; cargoPackage
// the "[[package]]" one a Cargo.lock entry opens with.
var (
	cargoTable   = regexp.MustCompile(`^\s*\[`)
	cargoPackage = regexp.MustCompile(`^\s*\[\[package\]\]\s*$`)
	cargoName    = valueLinePattern(`\s*name\s*=\s*`)
	cargoVersion = valueLinePattern(`\s*version\s*=\s*`)
	cargoSource  = regexp.MustCompile(`^\s*source\s*=`)
)

// UpdateCargoLock replaces the version of the crate named key in a
// Cargo.lock: every "[[package]]" entry with that name and no "source",
// which is how Cargo records a crate of the workspace itself rather than
// one fetched from a registry or a git repository. Everything else is kept
// byte for byte. An error is returned when no such entry exists.
func UpdateCargoLock(content, key, version string) (string, error) {
	lines := strings.Split(content, "\n")
	found := false

	inPackage := false
	name, versionLine, hasSource := "", -1, false
	flush := func() {
		if inPackage && name == key && versionLine >= 0 && !hasSource {
			m := cargoVersion.FindStringSubmatch(lines[versionLine])
			lines[versionLine] = m[1] + quoteLike(m[2], version) + m[3]
			found = true
		}
		inPackage, name, versionLine, hasSource = false, "", -1, false
	}
	for i, line := range lines {
		if cargoTable.MatchString(line) {
			flush()
			inPackage = cargoPackage.MatchString(line)
			continue
		}
		if !inPackage {
			continue
		}
		switch {
		case cargoName.MatchString(line):
			name = strings.Trim(cargoName.FindStringSubmatch(line)[2], `"'`)
		case cargoVersion.MatchString(line):
			versionLine = i
		case cargoSource.MatchString(line):
			hasSource = true
		}
	}
	flush()
	if !found {
		return "", fmt.Errorf("updater: cargo-lock: no workspace crate named %q", key)
	}
	return strings.Join(lines, "\n"), nil
}

// UpdateGemfileLock replaces the version of the gem named key in a
// Gemfile.lock: every specs entry for it, "    name (version)", indented by
// four spaces, the depth Bundler lists a resolved gem at; the constraints
// a gem's dependencies are listed with sit deeper and are left alone. A
// pre-release version is written the way Bundler records it, every "-"
// becoming ".pre.", so "1.2.0-rc.1" is "1.2.0.pre.rc.1". Everything else is
// kept byte for byte. An error is returned when no such entry exists.
func UpdateGemfileLock(content, key, version string) (string, error) {
	spec := regexp.MustCompile(`^(    ` + regexp.QuoteMeta(key) + ` \()(\d[^)\s]*)(\)\s*)$`)
	gemVersion := strings.ReplaceAll(version, "-", ".pre.")
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		if m := spec.FindStringSubmatch(line); m != nil {
			lines[i] = m[1] + gemVersion + m[3]
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("updater: gemfile-lock: no specs entry for gem %q", key)
	}
	return strings.Join(lines, "\n"), nil
}
