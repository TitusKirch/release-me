// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// replaceFirstLine replaces the value on the first line of content pattern
// matches. pattern has three groups: everything before the value, the
// value, and everything after it; the value is rendered through quoteLike,
// so a quoted value stays quoted the same way. It reports whether a line
// matched.
func replaceFirstLine(content string, pattern *regexp.Regexp, version string) (string, bool) {
	return replaceFirstLineOutside(content, pattern, version, nil)
}

// replaceFirstLineOutside is replaceFirstLine restricted to a value that
// lies outside skip, the spans of content no value may be taken from, such
// as the heredocs of a mix.exs.
func replaceFirstLineOutside(content string, pattern *regexp.Regexp, version string, skip []span) (string, bool) {
	lines := strings.Split(content, "\n")
	offset := 0
	for i, line := range lines {
		// The value is the pattern's second group, so its span is the
		// third index pair FindStringSubmatchIndex reports.
		if m := pattern.FindStringSubmatchIndex(line); m != nil && !inSpan(skip, offset+m[4]) {
			start, stop := m[4], m[5]
			lines[i] = line[:start] + quoteLike(line[start:stop], version) + line[stop:]
			return strings.Join(lines, "\n"), true
		}
		offset += len(line) + 1
	}
	return "", false
}

// replaceAllGroup replaces every match of pattern's first group in content
// with version, for a release type whose version is not a field of its own
// but part of a longer string, and which may carry it more than once.
// Splicing from the last match keeps the earlier matches' offsets valid. It
// returns how many matches were replaced.
func replaceAllGroup(content string, pattern *regexp.Regexp, version string) (string, int) {
	matches := pattern.FindAllStringSubmatchIndex(content, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		content = content[:m[2]] + version + content[m[3]:]
	}
	return content, len(matches)
}

// quotedValue matches a single or double quoted string, the value most
// release types' version files assign.
const quotedValue = `("[^"]*"|'[^']*')`

// quotedScanValue matches a double quoted string and singleQuotedScanValue
// a single quoted one, backslash escapes honoured in both. They are the
// values the Elixir and Bazel scanners hand off to: those find the keyword
// themselves, skipping escapes as they go, so the pattern that reads the
// value off has to skip them the same way.
const (
	quotedScanValue       = `"(?:[^"\\]|\\.)*"`
	singleQuotedScanValue = `'(?:[^'\\]|\\.)*'`
)

// pythonVersion matches the version assignment of a Python package's
// version files: "__version__ = \"1.2.3\"" in an __init__.py or a
// version.py, the keyword argument "version=\"1.2.3\"," in setup.py, and
// the bare "version = 1.2.3" of setup.cfg. A bare value must start with a
// digit, so "version = attr: pkg.__version__" or
// "version=get_version()," is never mistaken for one.
var pythonVersion = regexp.MustCompile(`^(\s*(?:__version__|version)\s*=\s*)("[^"]*"|'[^']*'|\d[^\s#,)]*)(.*)$`)

// UpdatePython replaces the version a Python package's version file
// assigns: the first "__version__ = ..." or "version = ..." line, the
// shape of an __init__.py, a version.py, setup.py and setup.cfg alike. A
// quoted version stays quoted the same way. An error is returned when no
// such line holds a literal version.
func UpdatePython(content, version string) (string, error) {
	if out, ok := replaceFirstLine(content, pythonVersion, version); ok {
		return out, nil
	}
	return "", fmt.Errorf("updater: python: no __version__ or version assignment holding a version found")
}

// rubyVersion matches a version.rb's "VERSION = \"1.2.3\"" constant, a
// trailing ".freeze" included in what follows the value.
var rubyVersion = regexp.MustCompile(`^(\s*VERSION\s*=\s*)` + quotedValue + `(.*)$`)

// UpdateRuby replaces the string a Ruby gem's version.rb assigns to its
// VERSION constant. An error is returned when there is no such constant
// holding a string.
func UpdateRuby(content, version string) (string, error) {
	if out, ok := replaceFirstLine(content, rubyVersion, version); ok {
		return out, nil
	}
	return "", fmt.Errorf("updater: ruby: no VERSION constant holding a string found")
}

// elixirAttribute matches mix.exs's "@version \"1.2.3\"" module attribute,
// elixirProject the head of its project/0 definition in each of the shapes
// Elixir accepts, and elixirKeyword the "version: \"1.2.3\"" keyword of the
// list project/0 returns.
var (
	elixirAttribute = regexp.MustCompile(`^(\s*@version\s+)("[^"]*")(.*)$`)
	elixirProject   = regexp.MustCompile(`(?m)^[ \t]*defp?\s+project(?:\s*\(\s*\))?\s*(?:do\b|,\s*do:)`)
	elixirKeyword   = regexp.MustCompile(`version:\s*(` + quotedScanValue + `)`)
)

// UpdateElixir replaces the version of a mix.exs: its @version module
// attribute where it has one, which project/0 then refers to, otherwise
// the string of the "version:" keyword of the list project/0 returns, as
// release-please's mix.exs updater does. Nothing a heredoc holds is read,
// so a mix.exs whose documentation shows a version, an @version attribute
// or a whole project/0 definition keeps that example and has its real
// version bumped. An error is returned when neither is found.
func UpdateElixir(content, version string) (string, error) {
	heredocs := elixirHeredocs(content)
	if out, ok := replaceFirstLineOutside(content, elixirAttribute, version, heredocs); ok {
		return out, nil
	}
	if out, ok := replaceElixirKeyword(content, version, heredocs); ok {
		return out, nil
	}
	return "", fmt.Errorf("updater: elixir: no @version attribute or version keyword holding a string found")
}

// replaceElixirKeyword replaces the string of the "version:" keyword in the
// list mix.exs's project/0 returns. The search is scoped to that list, the
// way UpdateBazel scopes its search to the module() call, so that a version
// example in a comment or another function cannot be taken for the
// project's own version, and so that the keyword is found wherever it sits
// on its line. A definition head inside one of heredocs is not a
// definition but documentation of one, so it is skipped rather than
// scoped to. It reports whether the keyword was found.
func replaceElixirKeyword(content, version string, heredocs []span) (string, bool) {
	for _, head := range elixirProject.FindAllStringIndex(content, -1) {
		if inSpan(heredocs, head[0]) {
			continue
		}
		open := strings.IndexByte(content[head[1]:], '[')
		if open < 0 {
			return "", false
		}
		open += head[1] + 1
		end := closingDelimiter(content, open, '[', ']', heredocs)
		if end < 0 {
			return "", false
		}
		m := elixirVersionKeyword(content, open, end, heredocs)
		if m == nil {
			return "", false
		}
		return content[:m[2]] + quoteLike(content[m[2]:m[3]], version) + content[m[3]:], true
	}
	return "", false
}

// elixirVersionKeyword locates the top-level "version:" keyword and its
// string span in the keyword list content holds between from and to,
// skipping comments, heredocs, other strings and nested expressions such
// as a dependency's own version. The span it returns is an index into
// content, not into the list.
func elixirVersionKeyword(content string, from, to int, heredocs []span) []int {
	depth := 0
	for i := from; i < to; i++ {
		if end := spanEnd(heredocs, i); end > i {
			i = end - 1
			continue
		}
		switch c := content[i]; c {
		case '#':
			for i < to && content[i] != '\n' {
				i++
			}
		case '\'', '"':
			quote := c
			for i++; i < to; i++ {
				if content[i] == '\\' {
					i++
					continue
				}
				if content[i] == quote {
					break
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		default:
			if depth == 0 && strings.HasPrefix(content[i:to], "version:") {
				if i > from {
					prev := content[i-1]
					if prev == '_' || prev == ':' || prev >= 'a' && prev <= 'z' || prev >= 'A' && prev <= 'Z' || prev >= '0' && prev <= '9' {
						continue
					}
				}
				if m := elixirKeyword.FindStringSubmatchIndex(content[i:to]); m != nil && m[0] == 0 {
					for n := range m {
						m[n] += i
					}
					return m
				}
			}
		}
	}
	return nil
}

// span is a half open byte range of a file's content, [start, end).
type span struct {
	start, end int
}

// inSpan reports whether pos lies in one of spans.
func inSpan(spans []span, pos int) bool {
	for _, s := range spans {
		if pos >= s.start && pos < s.end {
			return true
		}
	}
	return false
}

// spanEnd returns the end of the span that starts at pos, or pos itself
// when none does, so that a scanner can skip a span it runs into.
func spanEnd(spans []span, pos int) int {
	for _, s := range spans {
		if s.start == pos {
			return s.end
		}
	}
	return pos
}

// elixirHeredocs returns the spans content's heredocs cover, both the
// triple double quote and the triple single quote kind, each from its
// opening delimiter to the end of its closing one.
//
// A heredoc opens where its delimiter is the last thing on a line and
// closes on the next line that holds nothing but whitespace before the
// same delimiter. That is why heredocs do not nest: inside a heredoc body
// the delimiter either closes it or is plain text, and the other kind of
// delimiter is always plain text. An unterminated heredoc runs to the end
// of content, which keeps a file Elixir itself would refuse from having
// anything read out of it.
func elixirHeredocs(content string) []span {
	var spans []span
	var delimiter string
	start := 0
	for offset := 0; offset < len(content); {
		line := content[offset:]
		if n := strings.IndexByte(line, '\n'); n >= 0 {
			line = line[:n]
		}
		next := offset + len(line) + 1
		switch {
		case delimiter != "":
			if rest := strings.TrimLeft(line, " \t"); strings.HasPrefix(rest, delimiter) {
				spans = append(spans, span{start, offset + len(line) - len(rest) + len(delimiter)})
				delimiter = ""
			}
		default:
			if i, d := elixirHeredocOpener(line); d != "" {
				delimiter, start = d, offset+i
			}
		}
		offset = next
	}
	if delimiter != "" {
		spans = append(spans, span{start, len(content)})
	}
	return spans
}

// elixirHeredocOpener returns the index and the kind of the heredoc
// delimiter that opens at the end of line, or an empty kind when the line
// opens none. A heredoc opens only where its delimiter is the last thing
// on its line, so a one line string such as @moduledoc "..." opens none,
// and only outside a comment, so a comment about a heredoc opens none
// either.
func elixirHeredocOpener(line string) (int, string) {
	trimmed := strings.TrimRight(line, " \t\r")
	for _, d := range []string{`"""`, `'''`} {
		if !strings.HasSuffix(trimmed, d) {
			continue
		}
		at := len(trimmed) - len(d)
		if commented(line, at) {
			return 0, ""
		}
		return at, d
	}
	return 0, ""
}

// commented reports whether a "#" comment has started in line before at,
// the strings line holds skipped so that a "#" inside one does not count.
func commented(line string, at int) bool {
	var quote byte
	for i := 0; i < at; i++ {
		switch c := line[i]; {
		case quote != 0:
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#':
			return true
		}
	}
	return false
}

// opamVersion matches an opam file's top-level "version: \"1.2.3\"" field,
// duneVersion a dune-project's "(version 1.2.3)" stanza.
var (
	opamVersion = regexp.MustCompile(`^(version:\s*)("[^"]*")(\s*(?:#.*)?)$`)
	duneVersion = regexp.MustCompile(`^(\(version\s+)([^\s)]+)(\).*)$`)
)

// UpdateOCaml replaces the version of an OCaml package file: the top-level
// "version:" field of an .opam or .opam.locked file, or the "(version ...)"
// stanza of a dune-project. An error is returned when neither is found.
func UpdateOCaml(content, version string) (string, error) {
	if out, ok := replaceFirstLine(content, opamVersion, version); ok {
		return out, nil
	}
	if out, ok := replaceFirstLine(content, duneVersion, version); ok {
		return out, nil
	}
	return "", fmt.Errorf("updater: ocaml: no opam version field or dune-project version stanza found")
}

// rDescriptionVersion matches the "Version: 1.2.3" field of an R package's
// DESCRIPTION file, a development suffix such as ".9000" included in the
// value.
var rDescriptionVersion = regexp.MustCompile(`^(Version:\s*)(\S+)(\s*)$`)

// UpdateR replaces the Version field of an R package's DESCRIPTION file.
// An error is returned when there is no such field.
func UpdateR(content, version string) (string, error) {
	if out, ok := replaceFirstLine(content, rDescriptionVersion, version); ok {
		return out, nil
	}
	return "", fmt.Errorf("updater: r: no Version field found")
}

// bazelModule matches the start of a MODULE.bazel's module() call, and
// bazelVersion its version attribute.
var (
	bazelModule  = regexp.MustCompile(`(?m)^module\s*\(`)
	bazelVersion = regexp.MustCompile(`\bversion\s*=\s*(` + quotedScanValue + `|` + singleQuotedScanValue + `)`)
)

// UpdateBazel replaces the version attribute of a MODULE.bazel's module()
// call. The version attributes of bazel_dep() and every other call are
// left alone. An error is returned when there is no module() call, or when
// it carries no version attribute.
func UpdateBazel(content, version string) (string, error) {
	loc := bazelModule.FindStringIndex(content)
	if loc == nil {
		return "", fmt.Errorf("updater: bazel: no module() call found")
	}
	end := closingDelimiter(content, loc[1], '(', ')', nil)
	if end < 0 {
		return "", fmt.Errorf("updater: bazel: the module() call is never closed")
	}
	call := content[loc[1]:end]
	m := bazelVersionArgument(call)
	if m == nil {
		return "", fmt.Errorf("updater: bazel: the module() call has no version attribute")
	}
	start, stop := loc[1]+m[2], loc[1]+m[3]
	return content[:start] + quoteLike(content[start:stop], version) + content[stop:], nil
}

// bazelVersionArgument locates the top-level keyword and its string span,
// skipping comments, other strings and nested expressions in the call.
func bazelVersionArgument(call string) []int {
	depth := 0
	for i := 0; i < len(call); i++ {
		switch c := call[i]; c {
		case '#':
			for i < len(call) && call[i] != '\n' {
				i++
			}
		case '\'', '"':
			quote := c
			for i++; i < len(call); i++ {
				if call[i] == '\\' {
					i++
					continue
				}
				if call[i] == quote {
					break
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		default:
			if depth == 0 && strings.HasPrefix(call[i:], "version") {
				if i > 0 {
					prev := call[i-1]
					if prev == '_' || prev >= 'a' && prev <= 'z' || prev >= 'A' && prev <= 'Z' || prev >= '0' && prev <= '9' {
						continue
					}
				}
				if m := bazelVersion.FindStringSubmatchIndex(call[i:]); m != nil && m[0] == 0 {
					for n := range m {
						m[n] += i
					}
					return m
				}
			}
		}
	}
	return nil
}

// closingDelimiter returns the index of the close byte that closes the
// group whose body starts at from, skipping delimiters inside strings,
// inside "#" comments and inside one of skip's spans, or -1 when the group
// is never closed. skip is nil for a language without heredocs.
func closingDelimiter(content string, from int, open, close byte, skip []span) int {
	depth := 1
	var quote byte
	for i := from; i < len(content); i++ {
		if quote == 0 {
			if end := spanEnd(skip, i); end > i {
				i = end - 1
				continue
			}
		}
		c := content[i]
		switch {
		case quote != 0:
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#':
			for i < len(content) && content[i] != '\n' {
				i++
			}
		case c == open:
			depth++
		case c == close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// pubspecVersion matches a pubspec.yaml's top-level "version: 1.2.3+4"
// field: group 2 the version, group 3 a "+build" suffix, if any.
var pubspecVersion = regexp.MustCompile(`^(version:\s*)(\d[^\s+#]*)(\+[^\s#]+)?(.*)$`)

// UpdateDart replaces the version of a Dart or Flutter package's
// pubspec.yaml. A numeric build number, "+4", counts up by one, the way
// release-please bumps it, since a Flutter app store refuses a build
// number it has seen before; any other build suffix is kept as it is. An
// error is returned when there is no top-level version field.
func UpdateDart(content, version string) (string, error) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		m := pubspecVersion.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		build := m[3]
		if n, err := strconv.Atoi(strings.TrimPrefix(build, "+")); err == nil {
			build = "+" + strconv.Itoa(n+1)
		}
		lines[i] = m[1] + version + build + m[4]
		return strings.Join(lines, "\n"), nil
	}
	return "", fmt.Errorf("updater: dart: no top-level version field found")
}

// terraformModuleVersion matches a "v"-prefixed version literal such as
// "v1.2.3", the shape a Terraform module records its own version in. The
// "v" sits behind a word boundary, so it is never read out of the middle of
// a longer word such as "rev1.2.3". Group 1 is the literal without the "v",
// which stays where it is.
var terraformModuleVersion = regexp.MustCompile(`\bv(` + versionLiteral.String() + `)`)

// UpdateTerraformModule replaces every "v"-prefixed version of a Terraform
// module's versions.tf or versions.tf.tmpl, as release-please's
// terraform-module release type does.
//
// A module's own version is not a field there: it is the last segment of
// the module_name its provider_meta block reports to the provider, such as
// "blueprints/terraform/terraform-google-notes/v1.4.0". No key names it and
// no comment marks it, which is why every "v"-prefixed version literal in
// the file is replaced rather than one named field. A version constraint
// carries no "v" - ">= 1.3.0" and "~> 5.0" alike - so the required_version
// and the required_providers of the same file are left alone, and a
// constraint is never mistaken for the module's version.
//
// release-please bumps two further files of such a module that this updater
// is not for: a metadata.yaml, whose top-level "version" field the yaml
// updater takes with key "version", and a README, whose
// "version = \"~> 1.4\"" usage example release-me does not rewrite.
//
// An error is returned when the file carries no "v"-prefixed version.
func UpdateTerraformModule(content, version string) (string, error) {
	out, n := replaceAllGroup(content, terraformModuleVersion, version)
	if n == 0 {
		return "", fmt.Errorf("updater: terraform-module: no v-prefixed version found")
	}
	return out, nil
}

// krmBlueprintVersion matches the version ending a KRM blueprint's
// "cnrm/<blueprint>/v1.2.3" attribution value. The blueprint segment stops
// at whitespace and at either quote, so one value's match never runs into
// the next. Group 1 is the version literal, the "cnrm/" prefix and the "v"
// kept.
var krmBlueprintVersion = regexp.MustCompile(`cnrm/[^\s"']*/v(` + versionLiteral.String() + `)`)

// UpdateKrmBlueprint replaces the version of a Kubernetes Resource Model
// blueprint's YAML, as release-please's krm-blueprint release type does.
//
// The version is not a key of its own: it is the last segment of the value
// of the "cnrm.cloud.google.com/blueprint" annotation, a
// "cnrm/<blueprint>/v1.4.0" string, so the updater anchors on that "cnrm/"
// prefix instead of on a key. That anchor is what keeps the other pinned
// versions a blueprint carries out of it, a kpt function image such as
// "gcr.io/kpt-fn/set-annotations:v0.1.4" under a
// "config.kubernetes.io/function" annotation above all. Every such value in
// the file is replaced, since one blueprint may annotate more than one
// resource. The file is edited in place rather than through a YAML round
// trip, which would drop its comments.
//
// A pre-release or build suffix on the old version is replaced along with
// it, so the new version is never left carrying the old one's suffix.
// release-please's own expression matches the three numbers only and leaves
// such a suffix behind.
//
// An error is returned when the file carries no such value, which is also
// how a YAML that is no blueprint is rejected: release-please searches a
// package's YAML files for the annotation itself, release-me is given the
// file to bump.
func UpdateKrmBlueprint(content, version string) (string, error) {
	out, n := replaceAllGroup(content, krmBlueprintVersion, version)
	if n == 0 {
		return "", fmt.Errorf("updater: krm-blueprint: no cnrm blueprint version found")
	}
	return out, nil
}
