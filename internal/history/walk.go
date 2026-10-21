// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// Entry is one first parent commit since the last tag.
type Entry struct {
	// SHA is the commit hash, used to resolve the paths it touches for a
	// component, through Touches.
	SHA string
	// PullRequestNumber is the pull request number parsed from a
	// "Merge pull request #N from ..." subject, or 0 for a direct commit.
	PullRequestNumber int
	// Subject is the commit subject: the merge subject for a merge commit,
	// or the commit's own subject for a direct commit.
	Subject string
	// Body is the commit body. For a merge commit it is the fallback text
	// used when the forge has no pull request for PullRequestNumber.
	Body string
}

var mergeSubject = regexp.MustCompile(`^Merge pull request #(\d+) from `)

const (
	recordSep = "\x1e"
	fieldSep  = "\x1f"
)

// Walk reads the first parent commit history from since, exclusive, to HEAD.
// since is a tag name, or "" to read the whole history.
func Walk(ctx context.Context, dir, since string) ([]Entry, error) {
	rangeArg := "HEAD"
	if since != "" {
		rangeArg = since + "..HEAD"
	}

	out, err := run(ctx, dir, "log", "--first-parent", "--format="+recordSep+"%H"+fieldSep+"%s"+fieldSep+"%b", rangeArg)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, record := range strings.Split(out, recordSep) {
		if record == "" {
			continue
		}
		sha, rest, _ := strings.Cut(record, fieldSep)
		subject, body, _ := strings.Cut(rest, fieldSep)
		body = strings.TrimRight(body, "\n")

		entry := Entry{SHA: sha, Subject: subject, Body: body}
		if m := mergeSubject.FindStringSubmatch(subject); m != nil {
			if number, err := strconv.Atoi(m[1]); err == nil {
				entry.PullRequestNumber = number
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
