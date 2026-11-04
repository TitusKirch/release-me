// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"fmt"
	"strings"
)

// plainSpace is the whitespace a plain version file may carry around its
// version, a trailing newline most of all.
const plainSpace = " \t\r\n"

// UpdatePlain replaces the content of a file that holds nothing but the
// version, such as a version.txt or a VERSION file, with version. The
// whitespace around it, a trailing newline or none, is kept. An error is
// returned when the file is empty or holds anything besides one version:
// such a file is not a plain version file, and the marker updater fits it.
func UpdatePlain(content, version string) (string, error) {
	lead := content[:len(content)-len(strings.TrimLeft(content, plainSpace))]
	value := strings.TrimRight(content[len(lead):], plainSpace)
	trail := content[len(lead)+len(value):]
	if value == "" {
		return "", fmt.Errorf("updater: plain: the file is empty, it holds no version to replace")
	}
	if strings.ContainsAny(value, plainSpace) {
		return "", fmt.Errorf("updater: plain: the file holds more than a version, mark the version's line for the marker updater instead")
	}
	return lead + version + trail, nil
}
