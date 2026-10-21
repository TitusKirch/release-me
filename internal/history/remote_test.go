// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history_test

import (
	"context"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/history"
)

func TestRemote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		remoteURL string
		wantOwner string
		wantName  string
	}{
		{
			name:      "parses the SSH shorthand form",
			remoteURL: "git@github.com:ostendorf-cloud/release-me.git",
			wantOwner: "ostendorf-cloud",
			wantName:  "release-me",
		},
		{
			name:      "parses the HTTPS form",
			remoteURL: "https://github.com/ostendorf-cloud/release-me.git",
			wantOwner: "ostendorf-cloud",
			wantName:  "release-me",
		},
		{
			name:      "parses the HTTPS form without the git suffix",
			remoteURL: "https://github.com/ostendorf-cloud/release-me",
			wantOwner: "ostendorf-cloud",
			wantName:  "release-me",
		},
		{
			name:      "parses the ssh scheme form",
			remoteURL: "ssh://git@github.com/ostendorf-cloud/release-me.git",
			wantOwner: "ostendorf-cloud",
			wantName:  "release-me",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := newRepo(t)
			runGit(t, dir, "remote", "add", "origin", tc.remoteURL)

			owner, name, err := history.Remote(context.Background(), dir)
			if err != nil {
				t.Fatalf("Remote() error = %v", err)
			}
			if owner != tc.wantOwner || name != tc.wantName {
				t.Fatalf("Remote() = (%q, %q), want (%q, %q)", owner, name, tc.wantOwner, tc.wantName)
			}
		})
	}
}

func TestRemoteFailsWithoutAnOriginRemote(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)

	_, _, err := history.Remote(context.Background(), dir)
	if err == nil {
		t.Fatal("Remote() error = nil, want an error")
	}
}
