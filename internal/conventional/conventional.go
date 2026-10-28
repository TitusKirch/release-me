// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package conventional parses commit subjects and pull request titles into
// Conventional Commits data: type, scope, description, whether the change
// is breaking and the version a "Release-As" footer forces.
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
	// ReleaseAs is the value of a "Release-As:" trailer in the body's
	// trailer block, see Trailers, trimmed and otherwise as written, ""
	// when the block carries none. Parse never validates it:
	// internal/version decides whether it is a version the next release
	// may take.
	ReleaseAs string
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
// A "Release-As:" trailer in the body's trailer block, see Trailers, key
// case-insensitive as in release-please, sets ReleaseAs; with more than
// one in the block, the last one counts, the way git reads a repeated
// trailer.
func Parse(text, fallbackType string) Message {
	header, body, _ := strings.Cut(text, "\n")
	header = strings.TrimSpace(header)

	msg := parseHeader(header, fallbackType)
	if hasBreakingFooter(body) {
		msg.Breaking = true
	}
	msg.ReleaseAs = releaseAsFooter(text)
	return msg
}

// Trailer is one "Key: value" line of a message's trailer block, the key
// and the value as written, the value trimmed.
type Trailer struct {
	Key   string
	Value string
}

// trailerLine matches one git trailer line: a key of letters, digits and
// "-" at the very start of the line, or Conventional Commits' own
// "BREAKING CHANGE" spelled with a space, then ":" and the value.
var trailerLine = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]*|BREAKING CHANGE):[ \t]*(.*)$`)

// Trailers returns every trailer of text's trailer block, in order, nil
// when text carries none. Text has the shape Parse takes: the first line
// is the subject or title and never part of the block.
//
// The trailer block is the last paragraph of the text below the first
// line, paragraphs separated by blank lines, and only when every
// non-empty line in it is a trailer line, the way git interpret-trailers
// finds it. A fence line, an HTML comment, a quote, a list item or a line
// of prose anywhere in that paragraph disqualifies the whole paragraph,
// so a "Key: value" line quoted or fenced in a pull request body is never
// read as a trailer. A key is matched as written; comparing it without
// regard to case, and whether the first or last of a repeated key
// counts, is the caller's decision.
func Trailers(text string) []Trailer {
	_, body, _ := strings.Cut(text, "\n")
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")

	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	start := end
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}

	var trailers []Trailer
	for _, line := range lines[start:end] {
		m := trailerLine.FindStringSubmatch(strings.TrimRight(line, " \t\r"))
		if m == nil {
			return nil
		}
		trailers = append(trailers, Trailer{Key: m[1], Value: strings.TrimSpace(m[2])})
	}
	return trailers
}

// releaseAsFooter returns the value of the last non-empty "Release-As"
// trailer in text's trailer block, key in any case, "" when there is
// none.
func releaseAsFooter(text string) string {
	value := ""
	for _, t := range Trailers(text) {
		if strings.EqualFold(t.Key, "Release-As") && t.Value != "" {
			value = t.Value
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
