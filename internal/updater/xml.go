// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// utf8BOM is the byte order mark an editor may put before a .csproj's or a
// pom.xml's first byte.
const utf8BOM = "\xef\xbb\xbf"

// xmlSpace is the whitespace XML allows around an element's text.
const xmlSpace = " \t\r\n"

// UpdateXML replaces the text of every element at key, a dotted path of
// element names from the document's root element down, such as
// "project.version" for a pom.xml or "Project.PropertyGroup.Version" for a
// .csproj. Names are matched without their namespace prefix, as
// release-please's pom.xml updater matches them. The element must hold
// text only; whitespace around the text is kept, and so is everything else
// in the file, byte for byte. An error is returned when no element sits at
// key, or when one that does is empty-tagged or holds markup.
func UpdateXML(content, key, version string) (string, error) {
	segments := strings.Split(key, ".")
	body := strings.TrimPrefix(content, utf8BOM)
	offset := len(content) - len(body)

	dec := xml.NewDecoder(strings.NewReader(body))
	// Only an element's text is replaced, and a version is plain ASCII,
	// so the bytes are read as they are whatever encoding the declaration
	// names.
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }

	type span struct{ start, end int }
	var (
		stack []string
		spans []span
		open  = -1
	)
	for {
		before := int(dec.InputOffset())
		tok, err := dec.RawToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("updater: xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if open >= 0 {
				return "", fmt.Errorf("updater: xml: element %q holds child elements, not a version", key)
			}
			stack = append(stack, t.Name.Local)
			if slices.Equal(stack, segments) {
				end := int(dec.InputOffset())
				if strings.HasSuffix(body[before:end], "/>") {
					return "", fmt.Errorf("updater: xml: element %q is empty-tagged, it holds no version to replace", key)
				}
				open = end
			}
		case xml.EndElement:
			// RawToken leaves matching start and end elements to the
			// caller.
			if len(stack) == 0 || stack[len(stack)-1] != t.Name.Local {
				return "", fmt.Errorf("updater: xml: unexpected end element %q", t.Name.Local)
			}
			if open >= 0 && slices.Equal(stack, segments) {
				spans = append(spans, span{start: open, end: before})
				open = -1
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) > 0 {
		return "", fmt.Errorf("updater: xml: element %q is never closed", stack[len(stack)-1])
	}
	if len(spans) == 0 {
		return "", fmt.Errorf("updater: xml: element %q not found", key)
	}

	var b strings.Builder
	last := 0
	for _, s := range spans {
		text := body[s.start:s.end]
		if strings.Contains(text, "<") {
			return "", fmt.Errorf("updater: xml: element %q holds markup, not a version", key)
		}
		lead := text[:len(text)-len(strings.TrimLeft(text, xmlSpace))]
		trail := text[len(lead):][len(strings.TrimRight(text[len(lead):], xmlSpace)):]
		b.WriteString(body[last:s.start])
		b.WriteString(lead + version + trail)
		last = s.end
	}
	b.WriteString(body[last:])
	return content[:offset] + b.String(), nil
}
