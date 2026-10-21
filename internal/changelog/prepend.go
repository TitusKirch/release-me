// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package changelog

import "strings"

// header is the top heading CHANGELOG.md carries.
const header = "# Changelog\n"

// Prepend inserts rendered, the output of Render for one version, right
// after the top heading of existing CHANGELOG.md content. The heading is
// created when existing has none, which also covers a missing file, read as
// "". Prepend returns existing unchanged when rendered is empty.
func Prepend(existing, rendered string) string {
	if rendered == "" {
		return existing
	}
	rendered = strings.TrimRight(rendered, "\n")

	body := strings.TrimLeft(strings.TrimPrefix(existing, header), "\n")
	if body == "" {
		return header + "\n" + rendered + "\n"
	}
	return header + "\n" + rendered + "\n\n" + body
}
