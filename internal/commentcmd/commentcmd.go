// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package commentcmd parses a "release-as" or "release-notes" command from
// a comment on the release pull request, and decides whether the
// commenter's author association authorizes it. It reaches no forge and
// writes nothing: cmd/release-me's comment command acts on the Outcome it
// returns.
package commentcmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ostendorf-cloud/release-me/internal/version"
)

// Kind names a recognized comment command word.
type Kind string

// The command words a comment may name after the configured mention.
const (
	// KindReleaseAs is "release-as X.Y.Z": force the next version.
	KindReleaseAs Kind = "release-as"
	// KindReleaseNotes is "release-notes <text>": set the release-me-notes
	// block.
	KindReleaseNotes Kind = "release-notes"
)

// writeAssociations are the GitHub "author_association" values Parse
// treats as close to write access. See Authorized's own doc comment for
// exactly what this does and does not guarantee.
var writeAssociations = map[string]bool{"OWNER": true, "MEMBER": true, "COLLABORATOR": true}

// Authorized reports whether association, a comment event's
// "author_association" field, is OWNER, MEMBER or COLLABORATOR.
//
// This is close to, but not the same as, write access or higher on the
// repository: MEMBER is any member of the owning organization, regardless
// of that member's own permission on this particular repository, and
// COLLABORATOR includes a collaborator added with only read or triage
// access. Checking the repository's actual collaborator permission would
// need an extra API call per comment; author_association is already on
// the webhook payload GitHub computed server side, so Parse accepts the
// wider set these three values actually describe instead.
func Authorized(association string) bool {
	return writeAssociations[association]
}

// Outcome is what Parse resolved one comment body to.
type Outcome struct {
	// Matched is true when some line of body, once stripped of quoted,
	// fenced, inline code and HTML comment text, starts with mention,
	// whether or not a recognized command word follows it.
	Matched bool
	// Allowed reports whether the comment's author association is close
	// to write access or higher; see Authorized. Meaningless when Matched
	// is false.
	Allowed bool
	// Command is the recognized command word after the first Matched
	// occurrence of mention that one follows, or "" when body names
	// mention with no recognized command word anywhere after any
	// occurrence of it.
	Command Kind
	// Version is Command's KindReleaseAs argument, normalized to "X.Y.Z"
	// without a "v" prefix. Set only when Command is KindReleaseAs and
	// Valid is true.
	Version string
	// Notes is Command's KindReleaseNotes argument, the free text that
	// follows the command word to the end of the comment, trimmed. Set
	// only when Command is KindReleaseNotes; Valid is always true for it.
	Notes string
	// Valid reports whether Command's argument parsed. Always true for
	// KindReleaseNotes; false for KindReleaseAs when Version did not parse
	// as a real semantic version. Meaningless when Command is "".
	Valid bool
	// Reason explains, in one line, why Command is "" or Valid is false.
	Reason string
}

// commandWord matches the first whitespace-delimited word of a trimmed
// tail and the rest of it, body's remaining text to the end for
// KindReleaseNotes to use verbatim.
var commandWord = regexp.MustCompile(`(?s)^(\S+)[ \t\r\n]*(.*)$`)

// sanitize strips HTML comments, fenced code blocks, inline code spans and
// blockquote lines from body, so Parse never scans text the commenter
// quoted, fenced or commented out rather than typed as a live command. A
// GitHub "Quote reply" prefixes every quoted line with ">", which this
// drops entirely rather than merely ignoring the prefix, since a quoted
// command is never meant to run.
//
// Each construct is removed through its own well-formed close when one
// exists. An HTML comment or a fenced code block that never closes is
// removed to the end of body instead of left alone: CommonMark and
// GitHub's own renderer read an unterminated "<!--" or an unterminated
// fence as running to the end of the document, inert the whole way, not
// as a syntax error that leaves the rest of the comment live. A regex
// requiring a literal closing delimiter cannot express that "or to the
// end" fallback, so this scans body itself, one construct at a time,
// left to right, rather than matching closed pairs and hoping every
// comment happens to close them.
//
// An inline code span is different: CommonMark pairs an opening run of N
// backticks with the next run of exactly N backticks, and one that never
// finds a matching run is not a code span at all, just literal backtick
// characters, read as ordinary text by the renderer too. sanitize mirrors
// that: a backtick run with no matching close is kept, unstripped, so
// Parse still scans whatever follows it as live text, the same thing a
// reader of the rendered comment would see.
func sanitize(body string) string {
	var out strings.Builder
	for i := 0; i < len(body); {
		switch {
		case strings.HasPrefix(body[i:], "<!--"):
			if j := strings.Index(body[i+4:], "-->"); j >= 0 {
				i += 4 + j + 3
			} else {
				i = len(body)
			}

		case body[i] == '`' || body[i] == '~':
			c := body[i]
			n := runLength(body, i, c)
			switch {
			case n >= 3 && atFenceOpen(body, i):
				if end := findRunOfAtLeast(body, i+n, c, n); end >= 0 {
					i = end
				} else {
					i = len(body)
				}
			case c == '`':
				if end := findRunOfExactly(body, i+n, n); end >= 0 {
					i = end
				} else {
					out.WriteString(body[i : i+n])
					i += n
				}
			default:
				// A tilde run that is not a fence opener has no other
				// special meaning in CommonMark; keep it literal.
				out.WriteString(body[i : i+n])
				i += n
			}

		default:
			out.WriteByte(body[i])
			i++
		}
	}
	return stripBlockquoteLines(out.String())
}

// runLength returns the length of the run of byte c starting at i.
func runLength(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// atFenceOpen reports whether i opens a line, up to three leading spaces
// or tabs allowed, the same indentation CommonMark still reads a code
// fence through.
func atFenceOpen(s string, i int) bool {
	lineStart := strings.LastIndexByte(s[:i], '\n') + 1
	indent := s[lineStart:i]
	if len(indent) > 3 {
		return false
	}
	for j := 0; j < len(indent); j++ {
		if indent[j] != ' ' && indent[j] != '\t' {
			return false
		}
	}
	return true
}

// findRunOfAtLeast returns the index right after the first run of c with
// length at least minLen, found at or after i, or -1 when body carries
// none: the fence closing rule, which accepts a closing delimiter at
// least as long as the opener, a shorter run of the same character
// skipped over as ordinary content.
func findRunOfAtLeast(s string, i int, c byte, minLen int) int {
	for i < len(s) {
		if s[i] == c {
			n := runLength(s, i, c)
			if n >= minLen {
				return i + n
			}
			i += n
		} else {
			i++
		}
	}
	return -1
}

// findRunOfExactly returns the index right after the first run of
// exactly n backticks found at or after i, or -1 when body carries none:
// the inline code span closing rule, which only a run of the same length
// as the opener ends, a longer or shorter run skipped over as content.
func findRunOfExactly(s string, i, n int) int {
	for i < len(s) {
		if s[i] == '`' {
			run := runLength(s, i, '`')
			if run == n {
				return i + n
			}
			i += run
		} else {
			i++
		}
	}
	return -1
}

// stripBlockquoteLines drops every line of body that CommonMark reads as
// part of a blockquote: one starting, after only leading whitespace,
// with ">", and every immediately following non-blank line that lazily
// continues it without repeating the marker, the same way CommonMark's
// own lazy continuation rule reads a quoted paragraph that is not
// re-prefixed on every line. A blank line ends the continuation.
func stripBlockquoteLines(body string) string {
	lines := strings.Split(body, "\n")
	kept := make([]string, 0, len(lines))
	inQuote := false
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		switch {
		case strings.HasPrefix(trimmed, ">"):
			inQuote = true
		case inQuote && trimmed != "":
			// Lazy continuation: still part of the blockquote above.
		default:
			inQuote = false
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// Parse sanitizes body, then scans its lines for one that starts, after
// only leading whitespace, with mention at a real token boundary, and
// resolves the recognized command word that follows the first such line
// one does, returning what it resolved and whether association
// authorizes it. mention is compared literally; a repository's configured
// "comment-mention" is passed as is.
//
// Requiring mention to open its own line, not merely appear anywhere in
// body, is deliberate: a command embedded mid sentence, in a blockquote, a
// fenced block, an inline code span or an HTML comment must never execute,
// including when a maintainer's own "Quote reply" repeats a refused
// command verbatim to say no to it. sanitize already drops every one of
// those constructs; the line-start rule is a second, independent reason
// each of them fails to match even if sanitize somehow missed one.
//
// A comment that never names mention at all returns Matched false: the
// comment was not directed at release-me, and the caller replies to
// nothing. A comment naming mention with no command word after any
// occurrence of it, "the mention alone", or one followed by an
// unrecognized word, returns Matched true and Command "", so the caller
// still explains why nothing happened. mention is matched at a word
// boundary, so a longer handle sharing its prefix, "@release-me-fan" for
// mention "@release-me", never matches.
func Parse(body, mention, association string) Outcome {
	var out Outcome
	if mention == "" {
		return out
	}

	clean := sanitize(body)
	offset := 0
	for _, line := range strings.Split(clean, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		leading := len(line) - len(trimmed)

		if strings.HasPrefix(trimmed, mention) {
			after := trimmed[len(mention):]
			if after == "" || isBoundary(after[0]) {
				out.Matched = true
				out.Allowed = Authorized(association)

				afterAbs := offset + leading + len(mention)
				tail := strings.TrimLeft(clean[afterAbs:], " \t\r\n")
				if m := commandWord.FindStringSubmatch(tail); m != nil {
					if done := resolveCommand(&out, m[1], m[2]); done {
						return out
					}
				}
				// No recognized command word on this line: keep scanning
				// in case a later line carries a real command.
			}
		}
		offset += len(line) + 1
	}
	return out
}

// resolveCommand fills out from word and rest, the command word and
// remaining text commandWord matched on one line. done is true when word
// is recognized, so Parse stops scanning; false for an unrecognized word,
// so Parse keeps looking at later lines.
func resolveCommand(out *Outcome, word, rest string) (done bool) {
	switch word {
	case string(KindReleaseAs):
		out.Command = KindReleaseAs
		applyReleaseAs(out, rest)
		return true
	case string(KindReleaseNotes):
		out.Command = KindReleaseNotes
		out.Notes = strings.TrimSpace(rest)
		out.Valid = true
		return true
	default:
		return false
	}
}

// applyReleaseAs parses rest's first token as a semantic version, filling
// out.Version and out.Valid on success, out.Reason on failure.
func applyReleaseAs(out *Outcome, rest string) {
	arg := firstToken(rest)
	if arg == "" {
		out.Reason = "release-as needs a version, for example \"release-as 1.2.3\""
		return
	}
	normalized, ok := parseVersion(arg)
	if !ok {
		out.Reason = fmt.Sprintf("%q after release-as is not a parseable X.Y.Z version", arg)
		return
	}
	out.Version = normalized
	out.Valid = true
}

// firstToken returns s's first whitespace-delimited token, "" when s holds
// none.
func firstToken(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// parseVersion parses s, an optional "v" prefix stripped, as a semantic
// version through version.ParseTag, the same parser a tag name is checked
// against. normalized is s rendered back without the "v" prefix and
// without build metadata, ok is false when s does not parse.
func parseVersion(s string) (normalized string, ok bool) {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	sv, ok := version.ParseTag("v" + s)
	if !ok {
		return "", false
	}
	return sv.String(), true
}

// isBoundary reports whether b cannot continue a mention token: every byte
// except a letter, a digit, "-", "_" and "@". Parse only ever checks the
// byte right after a line-starting match, since the mention is already
// known to open the line.
func isBoundary(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9', b == '-', b == '_', b == '@':
		return false
	default:
		return true
	}
}
