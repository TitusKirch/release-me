// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"slices"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

func TestFromReleasePleaseTranslatesTheTitlePatterns(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		configJSON      string
		wantTitle       string
		wantSharedTitle string
		wantWarning     string
	}{
		{
			name: "release-please's default for a root package",
			configJSON: `{"pull-request-title-pattern": "chore${scope}: release${component} ${version}",
			  "packages": {".": {"release-type": "go"}}}`,
			wantTitle: "chore(${branch}): release ${component} ${version}",
		},
		{
			name: "a pattern naming the component of every package",
			configJSON: `{"pull-request-title-pattern": "chore${scope}: release${component} ${version}",
			  "separate-pull-requests": true,
			  "packages": {"api": {"component": "api", "release-type": "go"}, "web": {"component": "web", "release-type": "go"}}}`,
			wantTitle: "chore(${branch}): release ${component} ${version}",
		},
		{
			name: "a pattern without the component or the version",
			configJSON: `{"pull-request-title-pattern": "chore: cut a release",
			  "packages": {".": {"release-type": "go"}}}`,
			wantWarning: `pull-request-title-pattern: "chore: cut a release" carries no ${component}, ` +
				"which release reads back from the title after the merge; " +
				"release-me reads every component and its version back from the title, left out",
		},
		{
			name: "a group pattern for a locked family",
			configJSON: `{"group-pull-request-title-pattern": "chore${scope}: release ${version}",
			  "plugins": [{"type": "linked-versions", "groupName": "all", "components": ["api", "web"]}],
			  "packages": {"api": {"component": "api", "release-type": "go"}, "web": {"component": "web", "release-type": "go"}}}`,
			wantWarning: `group-pull-request-title-pattern: "chore(${branch}): release ${version}" carries no ${components}, ` +
				"which release reads back from the title after the merge; " +
				"release-me reads every component and its version back from the title, left out",
		},
		{
			name: "a group pattern for a shared pull request with independent versions",
			configJSON: `{"group-pull-request-title-pattern": "chore${scope}: release ${branch}",
			  "packages": {"api": {"component": "api", "release-type": "go"}, "web": {"component": "web", "release-type": "go"}}}`,
			wantWarning: `group-pull-request-title-pattern: "chore(${branch}): release ${branch}" carries ${branch} twice, ` +
				"release reads each value back from one place; " +
				"release-me reads every component and its version back from the title, left out",
		},
		{
			name: "a group pattern without a shared pull request",
			configJSON: `{"group-pull-request-title-pattern": "chore: release ${version}", "separate-pull-requests": true,
			  "packages": {"api": {"component": "api", "release-type": "go"}, "web": {"component": "web", "release-type": "go"}}}`,
			wantWarning: "group-pull-request-title-pattern: release-me opens one release pull request per component here, left out",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := migrate.FromReleasePlease([]byte(tc.configJSON), nil, files(nil))
			if err != nil {
				t.Fatalf("FromReleasePlease() error = %v", err)
			}
			cfg := load(t, got.YAML)
			if cfg.ReleasePullRequestTitle != tc.wantTitle || cfg.ReleasePullRequestSharedTitle != tc.wantSharedTitle {
				t.Fatalf("titles = (%q, %q), want (%q, %q)\n%s",
					cfg.ReleasePullRequestTitle, cfg.ReleasePullRequestSharedTitle, tc.wantTitle, tc.wantSharedTitle, got.YAML)
			}
			if tc.wantWarning != "" && !slices.Contains(got.Warnings, tc.wantWarning) {
				t.Fatalf("Warnings = %q, want it to contain %q", got.Warnings, tc.wantWarning)
			}
		})
	}
}
