// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// Entry is one commit a changelog source mode turned into a changelog
// candidate: a first parent commit for Walk and WalkSquash, or any other
// commit for AllEntries and SubCommits.
type Entry struct {
	// SHA is the commit hash, used to resolve the paths it touches for a
	// component, through Touches.
	SHA string
	// PullRequestNumber is the pull request number the entry belongs to,
	// 0 when it belongs to none: parsed from a GitHub "Merge pull request
	// #N from ..." subject or a GitLab "See merge request group/project!N"
	// body for Walk, from a trailing "(#N)" on the commit subject for
	// WalkSquash, or inherited from the first parent merge that introduced
	// the commit for AllEntries. SubCommits never sets it: a sub commit
	// is never itself a pull request.
	PullRequestNumber int
	// Subject is the commit subject, the merge subject for a Walk merge
	// commit, stripped of its trailing "(#N)" for a WalkSquash commit
	// that carries one, or the commit's own subject otherwise.
	Subject string
	// Body is the commit body. For an entry with a PullRequestNumber it
	// is also the fallback text used when the forge has no pull request
	// for that number.
	Body string
	// Squashed is true only for a WalkSquash entry whose PullRequestNumber
	// came from a trailing "(#N)" on the commit's own subject: Subject is
	// then the real, already cleaned commit title, which a caller should
	// never replace with a pull request's own title, forge reachable or
	// not. It is false for a genuine merge commit, even one WalkSquash
	// still recognizes by its "Merge pull request #N from ..." subject,
	// and for every entry Walk, AllEntries or SubCommits produces.
	Squashed bool
}

// mergeSubject matches GitHub's merge commit subject, naming the pull
// request number directly.
var mergeSubject = regexp.MustCompile(`^Merge pull request #(\d+) from `)

// gitlabMergeSubject matches GitLab's merge commit subject, which names
// neither the source nor the merge request; the number is read from the
// body instead, through gitlabMergeRequest.
var gitlabMergeSubject = regexp.MustCompile(`^Merge branch '.+' into '.+'$`)

// gitlabMergeRequest matches the merge request reference GitLab appends to
// the merge commit body, "See merge request group/project!N".
var gitlabMergeRequest = regexp.MustCompile(`See merge request \S*!(\d+)`)

const (
	recordSep = "\x1e"
	fieldSep  = "\x1f"
)

// logFormat is the git log format Walk, WalkSquash, AllEntries and
// SubCommits all share: one record per commit, SHA, subject and body
// separated by fieldSep, records separated by recordSep.
const logFormat = "--format=" + recordSep + "%H" + fieldSep + "%s" + fieldSep + "%b"

// record is one parsed commit before it is turned into an Entry: a plain
// SHA, subject and body, with no pull request number resolved yet.
type record struct {
	SHA, Subject, Body string
}

// parseRecords splits raw git log output in logFormat's shape into records.
func parseRecords(out string) []record {
	var records []record
	for _, chunk := range strings.Split(out, recordSep) {
		if chunk == "" {
			continue
		}
		sha, rest, _ := strings.Cut(chunk, fieldSep)
		subject, body, _ := strings.Cut(rest, fieldSep)
		records = append(records, record{SHA: sha, Subject: subject, Body: strings.TrimRight(body, "\n")})
	}
	return records
}

// Walk reads the first parent commit history from since, exclusive, to HEAD.
// since is a tag name, or "" to read the whole history.
func Walk(ctx context.Context, dir, since string) ([]Entry, error) {
	out, err := run(ctx, dir, "log", "--first-parent", logFormat, rangeArg(since))
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, r := range parseRecords(out) {
		entry := Entry{SHA: r.SHA, Subject: r.Subject, Body: r.Body}
		if m := mergeSubject.FindStringSubmatch(r.Subject); m != nil {
			if number, err := strconv.Atoi(m[1]); err == nil {
				entry.PullRequestNumber = number
			}
		} else if gitlabMergeSubject.MatchString(r.Subject) {
			if m := gitlabMergeRequest.FindStringSubmatch(r.Body); m != nil {
				if number, err := strconv.Atoi(m[1]); err == nil {
					entry.PullRequestNumber = number
				}
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// rangeArg builds the git log range argument for since, exclusive, to HEAD:
// "HEAD" to read the whole history when since is "", "since..HEAD"
// otherwise.
func rangeArg(since string) string {
	if since == "" {
		return "HEAD"
	}
	return since + "..HEAD"
}
