// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package github_test

import (
	"context"
	"encoding/json"
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

func TestPullRequests(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("head"); got != "owner:release-me/main" {
			t.Errorf("head query = %q, want %q", got, "owner:release-me/main")
		}
		if got := r.URL.Query().Get("state"); got != "open" {
			t.Errorf("state query = %q, want %q", got, "open")
		}
		fmt.Fprint(w, `[{"number": 9, "title": "chore(main): release v0.2.0", "merged": false}]`)
	})
	f := newTestForge(t, mux)

	got, err := f.PullRequests(context.Background(), "owner", "repo", "release-me/main", "open")
	if err != nil {
		t.Fatalf("PullRequests() error = %v", err)
	}
	want := []forge.PullRequest{{Number: 9, Title: "chore(main): release v0.2.0", Labels: []string{}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PullRequests() = %+v, want %+v", got, want)
	}
}

func TestCreatePullRequestCreatesAndLabelsIt(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/owner/repo/pulls", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"number": 12, "title": "chore(main): release v0.1.0", "body": "Release v0.1.0"}`)
	})
	var labeled []string
	mux.HandleFunc("/api/v3/repos/owner/repo/issues/12/labels", func(w http.ResponseWriter, r *http.Request) {
		var body []string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode labels body: %v", err)
		}
		labeled = body
		fmt.Fprint(w, `[]`)
	})
	f := newTestForge(t, mux)

	got, err := f.CreatePullRequest(context.Background(), "owner", "repo", "main", "release-me/main",
		"chore(main): release v0.1.0", "Release v0.1.0", []string{"release: pending"})
	if err != nil {
		t.Fatalf("CreatePullRequest() error = %v", err)
	}
	want := forge.PullRequest{Number: 12, Title: "chore(main): release v0.1.0", Body: "Release v0.1.0", Labels: []string{"release: pending"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CreatePullRequest() = %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(labeled, []string{"release: pending"}) {
		t.Fatalf("labeled = %v, want %v", labeled, []string{"release: pending"})
	}
}

func TestUpdatePullRequestEditsWithoutTouchingLabels(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/owner/repo/pulls/12", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"number": 12, "title": "chore(main): release v0.2.0", "body": "Release v0.2.0", "labels": [{"name": "release: pending"}]}`)
	})
	mux.HandleFunc("/api/v3/repos/owner/repo/issues/12/labels", func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("UpdatePullRequest() must not touch labels")
	})
	f := newTestForge(t, mux)

	got, err := f.UpdatePullRequest(context.Background(), "owner", "repo", 12, "chore(main): release v0.2.0", "Release v0.2.0")
	if err != nil {
		t.Fatalf("UpdatePullRequest() error = %v", err)
	}
	want := forge.PullRequest{Number: 12, Title: "chore(main): release v0.2.0", Body: "Release v0.2.0", Labels: []string{"release: pending"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UpdatePullRequest() = %+v, want %+v", got, want)
	}
}

func TestAddLabelsAddsToTheIssue(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	var added []string
	mux.HandleFunc("/api/v3/repos/owner/repo/issues/12/labels", func(w http.ResponseWriter, r *http.Request) {
		var body []string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode labels body: %v", err)
		}
		added = body
		fmt.Fprint(w, `[]`)
	})
	f := newTestForge(t, mux)

	if err := f.AddLabels(context.Background(), "owner", "repo", 12, []string{"release: pending"}); err != nil {
		t.Fatalf("AddLabels() error = %v", err)
	}
	if want := []string{"release: pending"}; !reflect.DeepEqual(added, want) {
		t.Fatalf("added = %v, want %v", added, want)
	}
}

func TestDefaultBranch(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/owner/repo", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"default_branch": "trunk"}`)
	})
	f := newTestForge(t, mux)

	got, err := f.DefaultBranch(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("DefaultBranch() error = %v", err)
	}
	if got != "trunk" {
		t.Fatalf("DefaultBranch() = %q, want %q", got, "trunk")
	}
}

func TestLabels(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/owner/repo/issues/12/labels", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[{"name": "release: pending"}, {"name": "priority: high"}]`)
	})
	f := newTestForge(t, mux)

	got, err := f.Labels(context.Background(), "owner", "repo", 12)
	if err != nil {
		t.Fatalf("Labels() error = %v", err)
	}
	if want := []string{"release: pending", "priority: high"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Labels() = %v, want %v", got, want)
	}
}

func TestSetLabelsReplacesTheFullSet(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	var replaced []string
	mux.HandleFunc("/api/v3/repos/owner/repo/issues/12/labels", func(w http.ResponseWriter, r *http.Request) {
		var body []string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode labels body: %v", err)
		}
		replaced = body
		fmt.Fprint(w, `[]`)
	})
	f := newTestForge(t, mux)

	if err := f.SetLabels(context.Background(), "owner", "repo", 12, []string{"release: tagged"}); err != nil {
		t.Fatalf("SetLabels() error = %v", err)
	}
	if want := []string{"release: tagged"}; !reflect.DeepEqual(replaced, want) {
		t.Fatalf("replaced = %v, want %v", replaced, want)
	}
}

func TestRepositoryLabels(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/owner/repo/labels", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[{"name": "release: pending"}, {"name": "release: tagged"}]`)
	})
	f := newTestForge(t, mux)

	got, err := f.RepositoryLabels(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("RepositoryLabels() error = %v", err)
	}
	if want := []string{"release: pending", "release: tagged"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RepositoryLabels() = %v, want %v", got, want)
	}
}

func TestRepositoryLabelsReturnsErrUnauthorizedOnA401(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/owner/repo/labels", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message": "Bad credentials"}`)
	})
	f := newTestForge(t, mux)

	_, err := f.RepositoryLabels(context.Background(), "owner", "repo")
	if !errors.Is(err, forge.ErrUnauthorized) {
		t.Fatalf("RepositoryLabels() error = %v, want %v", err, forge.ErrUnauthorized)
	}
}

func TestCreateTag(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		handler   http.HandlerFunc
		wantErrIs error
		wantErr   bool
	}{
		{
			name: "creates the tag reference",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, `{"ref": "refs/tags/v0.1.0", "object": {"sha": "deadbeef"}}`)
			},
		},
		{
			name: "returns ErrTagExists when GitHub refuses a duplicate ref",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				fmt.Fprint(w, `{"message": "Reference already exists"}`)
			},
			wantErrIs: forge.ErrTagExists,
		},
		{
			name: "wraps a server error",
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
			mux.HandleFunc("/api/v3/repos/owner/repo/git/refs", tc.handler)
			f := newTestForge(t, mux)

			err := f.CreateTag(context.Background(), "owner", "repo", "v0.1.0", "deadbeef")
			switch {
			case tc.wantErrIs != nil:
				if !errors.Is(err, tc.wantErrIs) {
					t.Fatalf("CreateTag() error = %v, want %v", err, tc.wantErrIs)
				}
			case tc.wantErr:
				if err == nil {
					t.Fatal("CreateTag() error = nil, want an error")
				}
			default:
				if err != nil {
					t.Fatalf("CreateTag() error = %v", err)
				}
			}
		})
	}
}

func TestCreateRelease(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	var body map[string]any
	mux.HandleFunc("/api/v3/repos/owner/repo/releases", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode release body: %v", err)
		}
		fmt.Fprint(w, `{"id": 1, "tag_name": "v0.1.0"}`)
	})
	f := newTestForge(t, mux)

	if err := f.CreateRelease(context.Background(), "owner", "repo", "v0.1.0", "v0.1.0", "Release v0.1.0"); err != nil {
		t.Fatalf("CreateRelease() error = %v", err)
	}
	if body["draft"] != false || body["prerelease"] != false {
		t.Fatalf("body = %v, want draft and prerelease false", body)
	}
	if body["tag_name"] != "v0.1.0" || body["body"] != "Release v0.1.0" {
		t.Fatalf("body = %v", body)
	}
}

func TestCreateReleaseReturnsErrReleaseExistsOnADuplicateTag(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/owner/repo/releases", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"message": "Validation Failed"}`)
	})
	f := newTestForge(t, mux)

	err := f.CreateRelease(context.Background(), "owner", "repo", "v0.1.0", "v0.1.0", "Release v0.1.0")
	if !errors.Is(err, forge.ErrReleaseExists) {
		t.Fatalf("CreateRelease() error = %v, want %v", err, forge.ErrReleaseExists)
	}
}

func TestMergeCommitReturnsTheMergeCommitSHA(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/owner/repo/pulls/12", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"number": 12, "merged": true, "merge_commit_sha": "deadbeef"}`)
	})
	f := newTestForge(t, mux)

	got, err := f.MergeCommit(context.Background(), "owner", "repo", 12)
	if err != nil {
		t.Fatalf("MergeCommit() error = %v", err)
	}
	if got != "deadbeef" {
		t.Fatalf("MergeCommit() = %q, want %q", got, "deadbeef")
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
