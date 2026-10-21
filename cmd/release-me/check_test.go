// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/forge/forgetest"
)

func TestDoCheckReportsEveryMissingItem(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		cfg        config.Config
		cfgErr     error
		token      string
		repoLabels []string
		want       []string
	}{
		{
			name:       "reports nothing when everything is present",
			cfg:        config.Config{Labels: config.Labels{Pending: "release: pending", Tagged: "release: tagged"}},
			token:      "a-token",
			repoLabels: []string{"release: pending", "release: tagged", "feature"},
			want:       nil,
		},
		{
			name:   "reports the config error and skips the label check",
			cfg:    config.Config{},
			cfgErr: errors.New("parse release-me.yaml: unknown key"),
			token:  "a-token",
			want:   []string{"config: parse release-me.yaml: unknown key"},
		},
		{
			name:       "reports the missing token",
			cfg:        config.Config{Labels: config.Labels{Pending: "release: pending", Tagged: "release: tagged"}},
			token:      "",
			repoLabels: []string{"release: pending", "release: tagged"},
			want:       []string{"token: environment variable GITHUB_TOKEN is not set"},
		},
		{
			name:       "reports a missing label",
			cfg:        config.Config{Labels: config.Labels{Pending: "release: pending", Tagged: "release: tagged"}},
			token:      "a-token",
			repoLabels: []string{"release: pending"},
			want:       []string{`label "release: tagged" does not exist on the repository`},
		},
		{
			name:       "reports both missing labels",
			cfg:        config.Config{Labels: config.Labels{Pending: "release: pending", Tagged: "release: tagged"}},
			token:      "a-token",
			repoLabels: nil,
			want: []string{
				`label "release: pending" does not exist on the repository`,
				`label "release: tagged" does not exist on the repository`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &forgetest.Fake{RepoLabels: tc.repoLabels}

			got := doCheck(t.Context(), tc.cfg, tc.cfgErr, "GITHUB_TOKEN", tc.token, "owner", "repo", f)
			if !equalStrings(got, tc.want) {
				t.Fatalf("doCheck() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDoCheckReportsARejectedTokenNotAMissingLabel(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Labels: config.Labels{Pending: "release: pending", Tagged: "release: tagged"}}
	wrapped := fmt.Errorf("github: list repository labels: %w: 401 Bad credentials", forge.ErrUnauthorized)
	f := &forgetest.Fake{RepoLabelsErr: wrapped}

	got := doCheck(t.Context(), cfg, nil, "GITHUB_TOKEN", "a-rejected-token", "owner", "repo", f)
	if len(got) != 1 || !strings.HasPrefix(got[0], "token:") {
		t.Fatalf("doCheck() = %v, want a single token line", got)
	}
}

func TestDoCheckDoesNotCheckLabelsWithoutAForge(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Labels: config.Labels{Pending: "release: pending", Tagged: "release: tagged"}}

	got := doCheck(t.Context(), cfg, errors.New("boom"), "GITHUB_TOKEN", "a-token", "owner", "repo", nil)
	want := []string{"config: boom"}
	if !equalStrings(got, want) {
		t.Fatalf("doCheck() = %v, want %v", got, want)
	}
}
