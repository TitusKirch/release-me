// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
)

func TestLoadReadsTheChangelogStyleKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    config.ChangelogStyle
	}{
		{name: "every key left out", content: "initial-version: 1.0.0\n", want: config.ChangelogStyle{}},
		{
			name:    "changelog-sentence-case",
			content: "changelog-sentence-case: true\n",
			want:    config.ChangelogStyle{SentenceCase: true},
		},
		{name: "changelog-authors", content: "changelog-authors: true\n", want: config.ChangelogStyle{Authors: true}},
		{name: "changelog-commit-links", content: "changelog-commit-links: true\nchangelog-issue-references: true\n", want: config.ChangelogStyle{CommitLinks: true, IssueReferences: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Load(writeFile(t, tc.content))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.ChangelogStyle != tc.want {
				t.Fatalf("ChangelogStyle = %+v, want %+v", cfg.ChangelogStyle, tc.want)
			}
		})
	}
}

// changelogStyleKeys is every top-level changelog style key, each set to
// a value other than its default.
var changelogStyleKeys = []string{
	"changelog-sentence-case: true", "changelog-authors: true", "changelog-commit-links: true",
	"changelog-issue-references: true",
}

func TestResolveGivesEveryComponentTheTopLevelChangelogStyle(t *testing.T) {
	t.Parallel()
	path := writeFile(t, strings.Join(changelogStyleKeys, "\n")+"\ncomponents:\n  api:\n    path: api\n  web:\n    path: web\n")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, comp := range cfg.Resolve() {
		if comp.ChangelogStyle != cfg.ChangelogStyle {
			t.Fatalf("component %s ChangelogStyle = %+v, want the top level's %+v", comp.Name, comp.ChangelogStyle, cfg.ChangelogStyle)
		}
	}
}

func TestLoadRejectsAChangelogStyleKeyInsideAComponent(t *testing.T) {
	t.Parallel()
	for _, key := range changelogStyleKeys {
		_, err := config.Load(writeFile(t, "components:\n  api:\n    path: api\n    "+key+"\n"))
		if err == nil {
			t.Fatalf("Load() error = nil, want the unknown component key %q refused", key)
		}
	}
}
