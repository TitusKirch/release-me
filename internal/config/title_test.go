// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/releasepr"
)

func TestLoadReadsTheReleasePullRequestTitlePatterns(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name            string
		content         string
		wantTitle       string
		wantSharedTitle string
		wantShared      []string
	}{
		{
			name:       "left out keeps the defaults",
			content:    "initial-version: 0.1.0\n",
			wantShared: []string{releasepr.DefaultSharedTitlePattern},
		},
		{
			name:      "a per-component pattern",
			content:   "release-pull-request-title: \"chore(${branch}): release ${component} ${version}\"\n",
			wantTitle: "chore(${branch}): release ${component} ${version}",
			wantShared: []string{
				releasepr.DefaultSharedTitlePattern,
			},
		},
		{
			name: "a shared pattern",
			content: "release-pull-request: shared\nrelease-pull-request-shared-title: \"chore: ship ${releases}\"\n" +
				"components:\n  api:\n    path: api\n  web:\n    path: web\n",
			wantSharedTitle: "chore: ship ${releases}",
			wantShared:      []string{"chore: ship ${releases}", releasepr.DefaultSharedTitlePattern},
		},
		{
			name: "a locked pattern",
			content: "versioning: locked\nrelease-pull-request-shared-title: \"chore: release ${version} for ${components}\"\n" +
				"components:\n  api:\n    path: api\n  web:\n    path: web\n",
			wantSharedTitle: "chore: release ${version} for ${components}",
			wantShared:      []string{"chore: release ${version} for ${components}", releasepr.DefaultLockedTitlePattern},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := config.Load(writeFile(t, tc.content))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got.ReleasePullRequestTitle != tc.wantTitle || got.ReleasePullRequestSharedTitle != tc.wantSharedTitle {
				t.Fatalf("titles = (%q, %q), want (%q, %q)",
					got.ReleasePullRequestTitle, got.ReleasePullRequestSharedTitle, tc.wantTitle, tc.wantSharedTitle)
			}
			var shared []string
			for _, p := range got.SharedTitlePatterns() {
				shared = append(shared, p.String())
			}
			if strings.Join(shared, "|") != strings.Join(tc.wantShared, "|") {
				t.Fatalf("SharedTitlePatterns() = %q, want %q", shared, tc.wantShared)
			}
		})
	}
}

func TestLoadRejectsATitlePatternReleaseCannotReadBack(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "a per-component pattern without the version",
			content: "release-pull-request-title: \"chore(${component}): release\"\n",
			want:    "release-pull-request-title \"chore(${component}): release\" carries no ${version}",
		},
		{
			name:    "a per-component pattern without the component",
			content: "release-pull-request-title: \"chore: release v${version}\"\n",
			want:    "carries no ${component}",
		},
		{
			name:    "release-please's scope placeholder",
			content: "release-pull-request-title: \"chore${scope}: release ${component} ${version}\"\n",
			want:    "carries ${scope}, which is none of",
		},
		{
			name: "a shared pattern with one version for independent components",
			content: "release-pull-request: shared\nrelease-pull-request-shared-title: \"chore: release ${version}\"\n" +
				"components:\n  api:\n    path: api\n  web:\n    path: web\n",
			want: "release-pull-request-shared-title \"chore: release ${version}\" carries ${version}, which is none of",
		},
		{
			name:    "a pattern whose title bumps the version",
			content: "release-pull-request-title: \"feat(${component}): release ${version}\"\n",
			want:    `renders "feat(api): release 1.2.3", a conventional feat that bumps the version`,
		},
		{
			name: "a locked pattern without the version",
			content: "versioning: locked\nrelease-pull-request-shared-title: \"chore: release ${components}\"\n" +
				"components:\n  api:\n    path: api\n",
			want: "carries no ${version}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(writeFile(t, tc.content))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load() error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
