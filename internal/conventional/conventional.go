// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package conventional parses commit subjects and pull request titles into
// Conventional Commits data: type, scope, description, whether the change
// is breaking, the version a "Release-As" footer forces and the components
// a "Release-Component" footer names.
package conventional

import (
	"regexp"
	"slices"
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
	// ReleaseAs is the value of a "Release-As:" footer in the body, trimmed
	// and otherwise as written, "" when the body carries none. Parse never
	// validates it: internal/version decides whether it is a version the
	// next release may take.
	ReleaseAs string
	// Components are the component names "Release-Component:" footers in
	// the body name, in the order written, each once, nil when the body
	// carries none. Parse never checks them against a configuration: the
	// command layer decides which names are components.
	Components []string
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
// A "Release-As:" footer, key case-insensitive as in release-please, sets
// ReleaseAs; with more than one in the same body, the last one counts, the
// way git reads a repeated trailer. A "Release-Component:" footer, key
// case-insensitive too, sets Components; it may repeat and each value may
// list several names separated by commas, every one of them counts.
func Parse(text, fallbackType string) Message {
	header, body, _ := strings.Cut(text, "\n")
	header = strings.TrimSpace(header)

	msg := parseHeader(header, fallbackType)
	if hasBreakingFooter(body) {
		msg.Breaking = true
	}
	msg.ReleaseAs = releaseAsFooter(body)
	msg.Components = releaseComponentFooters(body)
	return msg
}

// releaseComponentPattern matches a "Release-Component:" footer line in
// git trailer form, the key at the start of the line in any case.
var releaseComponentPattern = regexp.MustCompile(`(?i)^release-component:[ \t]*(.*)$`)

// releaseComponentFooters returns every component name the
// "Release-Component:" footer lines in body name, split on commas, trimmed,
// each once in the order first written, nil when there is none.
func releaseComponentFooters(body string) []string {
	var names []string
	for _, line := range strings.Split(body, "\n") {
		m := releaseComponentPattern.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		for _, name := range strings.Split(m[1], ",") {
			if name = strings.TrimSpace(name); name != "" && !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	return names
}

// releaseAsPattern matches a "Release-As:" footer line in git trailer
// form, the key at the start of the line in any case.
var releaseAsPattern = regexp.MustCompile(`(?i)^release-as:[ \t]*(.*)$`)

// releaseAsFooter returns the trimmed value of the last "Release-As:"
// footer line in body, "" when there is none or its value is empty.
func releaseAsFooter(body string) string {
	value := ""
	for _, line := range strings.Split(body, "\n") {
		m := releaseAsPattern.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		if v := strings.TrimSpace(m[1]); v != "" {
			value = v
		}
	}
	return value
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

// Override is the parsed "release-me" override block from an ordinary pull
// request body. Present is false when the body carries no such block.
// Present with no Entries is an explicit empty block: it hides the pull
// request from the history entirely.
type Override struct {
	Present bool
	Entries []string
}

// overrideBlock matches a fenced block with info string "release-me",
// exactly, so it never matches the differently named "release-me-notes"
// block of a release pull request. Both fences are anchored to the start of
// their own line, so a stray "```release-me" inside a sentence never
// matches.
var overrideBlock = regexp.MustCompile("(?sm)^```release-me[ \t]*$\n(.*?)^```[ \t]*$")

// ParseOverride extracts the "release-me" fenced block from a pull request
// body, one Conventional Commits line per entry.
func ParseOverride(body string) Override {
	m := overrideBlock.FindStringSubmatch(body)
	if m == nil {
		return Override{}
	}

	var entries []string
	for _, line := range strings.Split(m[1], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entries = append(entries, line)
	}
	return Override{Present: true, Entries: entries}
}
