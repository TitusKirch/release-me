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
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// Kind names a recognized comment command word.
type Kind string

// The command words a comment may name after the configured mention.
const (
	// KindReleaseAs is "release-as X.Y.Z", or "release-as <component>
	// X.Y.Z" on a shared release pull request: force the next version.
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
	// Matched is true when some line of body's live text, with every
	// quoted, fenced, code span and HTML comment or block construct
	// removed, starts with mention, whether or not a recognized command
	// word follows it.
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
	// Component is the component a "release-as <component> X.Y.Z" command
	// names, "" for a plain "release-as X.Y.Z". Only a shared release pull
	// request with independent versions needs it; the caller decides
	// whether the name is known and allowed there.
	Component string
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

// markdown is the shared CommonMark/GFM parser liveText uses to tell live
// text apart from a fenced or indented code block, an HTML block or
// comment, a blockquote at any depth, or an inline code span: the exact
// set of constructs GitHub's own renderer treats as inert, with every one
// of CommonMark's own closing rules (fence length and character, a
// closer standing alone on its own line, indented-code-block detection,
// blockquote lazy continuation, code span backtick-run matching) already
// correctly implemented there, rather than re-approximated by hand here.
// A hand-rolled scanner previously attempted that approximation and, over
// three review rounds, kept missing real CommonMark edge cases, each one
// a fresh way to smuggle a command past the mention-matching below: an
// unterminated fence or comment, a multi-line double-backtick span, a
// closing fence embedded mid-line instead of alone on its own, and a
// plain indented code block. A conformant parser is the fix for the
// whole class, not another special case.
var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

// liveText returns the plain text content of body that CommonMark reads
// as live, read in document order: the content of a fenced or indented
// code block, an HTML block, a blockquote at any depth, and an inline
// code span or raw inline HTML are all skipped entirely, never scanned
// for mention, the same text a reader of the rendered comment would
// never see as a real command either. Each surviving source line keeps
// its own line in the result, so Parse's "mention opens its own line"
// rule still applies to exactly the text CommonMark would actually
// render live: an inline construct skipped mid-line, unlike a whole
// block, leaves a placeholder behind rather than vanishing outright, so
// text that only looked line-initial once the inline construct's own
// content was removed is never mistaken for text that really is.
func liveText(body string) string {
	source := []byte(body)
	doc := markdown.Parser().Parse(text.NewReader(source))

	var b strings.Builder
	walkLiveText(doc, source, &b)
	return b.String()
}

// walkLiveText appends n's live text content to b, skipping n entirely
// when it is one of the constructs CommonMark renders as inert, and
// recursing into every other node's children otherwise. A line break
// follows a block node's own content, so sibling blocks never merge onto
// one line.
func walkLiveText(n ast.Node, source []byte, b *strings.Builder) {
	switch n.Kind() {
	case ast.KindFencedCodeBlock, ast.KindCodeBlock, ast.KindHTMLBlock, ast.KindBlockquote:
		// A whole-line, block-level exclusion: CommonMark never lets one
		// of these share a source line with other inline content, so
		// dropping it drops entire lines, never merging onto one.
		return

	case ast.KindCodeSpan, ast.KindRawHTML:
		// An inline exclusion: unlike the block kinds above, a code span
		// or raw HTML tag can sit mid-line beside surviving text, on
		// either side. Returning with nothing written would let that
		// surviving text, which in the source started mid-line, be
		// mistaken for text that opens the line once this node's own
		// content is gone, exactly what the mention-opens-its-own-line
		// rule in Parse exists to rule out. The object replacement
		// character marks "something inert sat here": it is not
		// whitespace, so Parse's own leading-whitespace trim never
		// removes it, and it is not a character any real
		// "comment-mention" would plausibly open with.
		b.WriteRune('￼')
		return

	case ast.KindText:
		t, ok := n.(*ast.Text)
		if !ok {
			return
		}
		b.Write(t.Segment.Value(source))
		if t.SoftLineBreak() || t.HardLineBreak() {
			b.WriteByte('\n')
		}
		return
	}

	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		walkLiveText(c, source, b)
	}
	if n.Type() == ast.TypeBlock {
		b.WriteByte('\n')
	}
}

// Parse reduces body to its live text through liveText, then scans its
// lines for one that starts, after only leading whitespace, with mention
// at a real token boundary, and resolves the recognized command word
// that follows the first such line one does, returning what it resolved
// and whether association authorizes it. mention is compared literally;
// a repository's configured "comment-mention" is passed as is.
//
// Requiring mention to open its own line, not merely appear anywhere in
// body, is deliberate: a command embedded mid sentence, in a blockquote, a
// fenced block, an inline code span or an HTML comment must never execute,
// including when a maintainer's own "Quote reply" repeats a refused
// command verbatim to say no to it. liveText already drops every one of
// those constructs; the line-start rule is a second, independent reason
// each of them fails to match even if liveText ever somehow missed one.
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

	clean := liveText(body)
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

// applyReleaseAs parses rest as "X.Y.Z" or "<component> X.Y.Z", filling
// out.Version, out.Component and out.Valid on success, out.Reason on
// failure. A first token that parses as a version is the version, so a
// plain "release-as 1.2.3" never reads as naming a component.
func applyReleaseAs(out *Outcome, rest string) {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		out.Reason = "release-as needs a version, for example \"release-as 1.2.3\""
		return
	}
	if normalized, ok := parseVersion(fields[0]); ok {
		out.Version = normalized
		out.Valid = true
		return
	}
	if len(fields) < 2 {
		out.Reason = fmt.Sprintf("%q after release-as is not a parseable X.Y.Z version", fields[0])
		return
	}
	normalized, ok := parseVersion(fields[1])
	if !ok {
		out.Reason = fmt.Sprintf("%q after release-as %s is not a parseable X.Y.Z version", fields[1], fields[0])
		return
	}
	out.Component = fields[0]
	out.Version = normalized
	out.Valid = true
}

// parseVersion parses s through version.ParseForced, the rule every
// forced version follows, a pre-release value accepted. No last tag is
// checked here: the comment command learns it only once pr re-renders
// the release pull request with the override. normalized is s rendered
// back without the "v" prefix and without build metadata, ok is false
// when s does not parse.
func parseVersion(s string) (normalized string, ok bool) {
	sv, err := version.ParseForced(s, version.Semver{}, false, true)
	if err != nil {
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
