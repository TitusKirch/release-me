// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package forgetest_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/forge/forgetest"
)

func TestFakePullRequest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		responses map[int]forgetest.Response
		number    int
		want      forge.PullRequest
		wantErr   error
	}{
		{
			name:   "returns the configured response",
			number: 7,
			responses: map[int]forgetest.Response{
				7: {PullRequest: forge.PullRequest{Number: 7, Title: "feat: add the next command", URL: "https://example.com/pull/7"}},
			},
			want: forge.PullRequest{Number: 7, Title: "feat: add the next command", URL: "https://example.com/pull/7"},
		},
		{
			name:    "returns not found for an unconfigured number",
			number:  9,
			wantErr: forge.ErrNotFound,
		},
		{
			name:   "returns a configured not found with a URL",
			number: 9,
			responses: map[int]forgetest.Response{
				9: {PullRequest: forge.PullRequest{URL: "https://example.com/pull/9"}, Err: forge.ErrNotFound},
			},
			want:    forge.PullRequest{URL: "https://example.com/pull/9"},
			wantErr: forge.ErrNotFound,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := &forgetest.Fake{Responses: tc.responses}

			got, err := f.PullRequest(context.Background(), "owner", "repo", tc.number)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("PullRequest() error = %v, want %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("PullRequest() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
