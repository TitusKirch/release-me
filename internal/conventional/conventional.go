// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package conventional parses commit subjects and pull request titles into
// Conventional Commits data: type, scope, description and whether the change
// is breaking.
package conventional

import (
	"regexp"
	"strings"
)

// FallbackType is the default fallback type, see internal/config's
// fallback-type key.
const FallbackType = "other"

// Message is one parsed commit or pull request.
type Message struct {
	Type        string
	Scope       string
	Description string
	Breaking    bool
}

// headerPattern matches a lowercase type only, so "Feat:" or "WIP:" fall to
// the fallback type instead of being read as a type of their own.
var headerPattern = regexp.MustCompile(`^([a-z]+)(\(([^()]+)\))?(!)?:\s*(.+)$`)

// Parse parses text into a Message. Text is a commit subject or pull request
// title as the first line, optionally followed by a body below it.
//
// A first line matching "type(scope)!: description", type in lowercase,
// yields that type, scope and description, breaking when "!" is present. A
// first line that does not match becomes a Message of fallbackType with the
// first line as its description. A "BREAKING CHANGE:" or "BREAKING-CHANGE:"
// footer anywhere in the body marks the change breaking even without "!".
func Parse(text, fallbackType string) Message {
	header, body, _ := strings.Cut(text, "\n")
	header = strings.TrimSpace(header)

	msg := parseHeader(header, fallbackType)
	if hasBreakingFooter(body) {
		msg.Breaking = true
	}
	return msg
}

func parseHeader(header, fallbackType string) Message {
	m := headerPattern.FindStringSubmatch(header)
	if m == nil {
		return Message{Type: fallbackType, Description: header}
	}
	return Message{
		Type:        m[1],
		Scope:       m[3],
		Description: strings.TrimSpace(m[5]),
		Breaking:    m[4] == "!",
	}
}

func hasBreakingFooter(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "BREAKING CHANGE:") || strings.HasPrefix(line, "BREAKING-CHANGE:") {
			return true
		}
	}
	return false
}
