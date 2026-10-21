// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package github_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	forgegithub "github.com/ostendorf-cloud/release-me/internal/forge/github"
)

// newTestForge points a Forge at a local httptest server instead of the real
// GitHub API, so the tests never reach the network.
func newTestForge(t *testing.T, mux *http.ServeMux) *forgegithub.Forge {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	f, err := forgegithub.New("", gogithub.WithEnterpriseURLs(srv.URL, srv.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return f
}

func TestPullRequest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		number    int
		handler   http.HandlerFunc
		want      forge.PullRequest
		wantErrIs error
		wantErr   bool
	}{
		{
			name:   "reads title, body, labels and URL",
			number: 5,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, `{
					"title": "feat(cli): add the next command",
					"body": "adds release-me next",
					"html_url": "https://github.com/owner/repo/pull/5",
					"labels": [{"name": "release: pending"}, {"name": "feature"}]
				}`)
			},
			want: forge.PullRequest{
				Number: 5,
				Title:  "feat(cli): add the next command",
				Body:   "adds release-me next",
				Labels: []string{"release: pending", "feature"},
				URL:    "https://github.com/owner/repo/pull/5",
			},
		},
		{
			name:   "returns not found with a fallback URL",
			number: 9,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, `{"message": "Not Found"}`)
			},
			want:      forge.PullRequest{URL: "https://github.com/owner/repo/pull/9"},
			wantErrIs: forge.ErrNotFound,
		},
		{
			name:   "wraps a server error",
			number: 3,
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			mux.HandleFunc(fmt.Sprintf("/api/v3/repos/owner/repo/pulls/%d", tc.number), tc.handler)
			f := newTestForge(t, mux)

			got, err := f.PullRequest(context.Background(), "owner", "repo", tc.number)
			switch {
			case tc.wantErr:
				if err == nil {
					t.Fatal("PullRequest() error = nil, want an error")
				}
				if errors.Is(err, forge.ErrNotFound) {
					t.Fatal("PullRequest() error wraps forge.ErrNotFound, want a plain error")
				}
			case tc.wantErrIs != nil:
				if !errors.Is(err, tc.wantErrIs) {
					t.Fatalf("PullRequest() error = %v, want %v", err, tc.wantErrIs)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("PullRequest() = %+v, want %+v", got, tc.want)
				}
			default:
				if err != nil {
					t.Fatalf("PullRequest() error = %v", err)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("PullRequest() = %+v, want %+v", got, tc.want)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		token string
	}{
		{name: "accepts an empty token for unauthenticated use", token: ""},
		{name: "accepts a token", token: "a-token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := forgegithub.New(tc.token); err != nil {
				t.Fatalf("New(%q) error = %v", tc.token, err)
			}
		})
	}
}
