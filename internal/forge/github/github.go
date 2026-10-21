// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package github implements internal/forge.Forge on top of the GitHub REST
// API through go-github.
package github

import (
	"context"
	"fmt"
	"net/http"

	gogithub "github.com/google/go-github/v93/github"
	"github.com/ostendorf-cloud/release-me/internal/forge"
)

// Forge reads pull request data from the GitHub REST API.
type Forge struct {
	client *gogithub.Client
}

// New builds a Forge. An empty token makes unauthenticated requests.
func New(token string, opts ...gogithub.ClientOptionsFunc) (*Forge, error) {
	if token != "" {
		opts = append(opts, gogithub.WithAuthToken(token))
	}
	client, err := gogithub.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("github: new client: %w", err)
	}
	return &Forge{client: client}, nil
}

// PullRequest implements forge.Forge.
func (f *Forge) PullRequest(ctx context.Context, owner, repo string, number int) (forge.PullRequest, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, number)

	pr, resp, err := f.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return forge.PullRequest{URL: url}, forge.ErrNotFound
		}
		return forge.PullRequest{}, fmt.Errorf("github: get pull request #%d: %w", number, err)
	}
	result := toPullRequest(pr)
	result.Number = number
	return result, nil
}

// PullRequests implements forge.Forge.
func (f *Forge) PullRequests(ctx context.Context, owner, repo, head, state string) ([]forge.PullRequest, error) {
	opts := &gogithub.PullRequestListOptions{
		Head:        owner + ":" + head,
		State:       state,
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}

	var out []forge.PullRequest
	for {
		prs, resp, err := f.client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("github: list pull requests for %s: %w", head, err)
		}
		for _, pr := range prs {
			out = append(out, toPullRequest(pr))
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// CreatePullRequest implements forge.Forge.
func (f *Forge) CreatePullRequest(ctx context.Context, owner, repo, base, head, title, body string, labels []string) (forge.PullRequest, error) {
	pr, _, err := f.client.PullRequests.Create(ctx, owner, repo, gogithub.CreatePullRequest{
		Title: &title,
		Head:  head,
		Base:  base,
		Body:  &body,
	})
	if err != nil {
		return forge.PullRequest{}, fmt.Errorf("github: create pull request %s onto %s: %w", head, base, err)
	}
	if len(labels) > 0 {
		if err := f.AddLabels(ctx, owner, repo, pr.GetNumber(), labels); err != nil {
			return forge.PullRequest{}, err
		}
	}
	result := toPullRequest(pr)
	result.Labels = labels
	return result, nil
}

// UpdatePullRequest implements forge.Forge.
func (f *Forge) UpdatePullRequest(ctx context.Context, owner, repo string, number int, title, body string) (forge.PullRequest, error) {
	pr, _, err := f.client.PullRequests.Edit(ctx, owner, repo, number, &gogithub.PullRequest{
		Title: &title,
		Body:  &body,
	})
	if err != nil {
		return forge.PullRequest{}, fmt.Errorf("github: update pull request #%d: %w", number, err)
	}
	return toPullRequest(pr), nil
}

// Labels implements forge.Forge.
func (f *Forge) Labels(ctx context.Context, owner, repo string, number int) ([]string, error) {
	ls, _, err := f.client.Issues.ListLabelsByIssue(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, fmt.Errorf("github: list labels for pull request #%d: %w", number, err)
	}
	return labelNames(ls), nil
}

// AddLabels implements forge.Forge.
func (f *Forge) AddLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	if _, _, err := f.client.Issues.AddLabelsToIssue(ctx, owner, repo, number, labels); err != nil {
		return fmt.Errorf("github: add labels to pull request #%d: %w", number, err)
	}
	return nil
}

// SetLabels implements forge.Forge.
func (f *Forge) SetLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	if _, _, err := f.client.Issues.ReplaceLabelsForIssue(ctx, owner, repo, number, labels); err != nil {
		return fmt.Errorf("github: set labels for pull request #%d: %w", number, err)
	}
	return nil
}

// RepositoryLabels implements forge.Forge.
func (f *Forge) RepositoryLabels(ctx context.Context, owner, repo string) ([]string, error) {
	opts := &gogithub.ListOptions{PerPage: 100}

	var out []string
	for {
		ls, resp, err := f.client.Issues.ListLabels(ctx, owner, repo, opts)
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusUnauthorized {
				return nil, fmt.Errorf("github: list repository labels: %w: %w", forge.ErrUnauthorized, err)
			}
			return nil, fmt.Errorf("github: list repository labels: %w", err)
		}
		out = append(out, labelNames(ls)...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// DefaultBranch implements forge.Forge.
func (f *Forge) DefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	r, _, err := f.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("github: get repository %s/%s: %w", owner, repo, err)
	}
	return r.GetDefaultBranch(), nil
}

// CreateTag implements forge.Forge.
func (f *Forge) CreateTag(ctx context.Context, owner, repo, tag, sha string) error {
	_, resp, err := f.client.Git.CreateRef(ctx, owner, repo, gogithub.CreateRef{
		Ref: "refs/tags/" + tag,
		SHA: sha,
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnprocessableEntity {
			return forge.ErrTagExists
		}
		return fmt.Errorf("github: create tag %s: %w", tag, err)
	}
	return nil
}

// CreateRelease implements forge.Forge.
func (f *Forge) CreateRelease(ctx context.Context, owner, repo, tag, name, body string) error {
	draft, prerelease := false, false
	_, resp, err := f.client.Repositories.CreateRelease(ctx, owner, repo, gogithub.CreateReleaseRequest{
		TagName:    tag,
		Name:       &name,
		Body:       &body,
		Draft:      &draft,
		Prerelease: &prerelease,
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnprocessableEntity {
			return forge.ErrReleaseExists
		}
		return fmt.Errorf("github: create release %s: %w", tag, err)
	}
	return nil
}

// MergeCommit implements forge.Forge.
func (f *Forge) MergeCommit(ctx context.Context, owner, repo string, number int) (string, error) {
	pr, _, err := f.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return "", fmt.Errorf("github: get pull request #%d: %w", number, err)
	}
	return pr.GetMergeCommitSHA(), nil
}

// toPullRequest converts a go-github pull request into a forge.PullRequest.
func toPullRequest(pr *gogithub.PullRequest) forge.PullRequest {
	return forge.PullRequest{
		Number:  pr.GetNumber(),
		Title:   pr.GetTitle(),
		Body:    pr.GetBody(),
		Labels:  labelNames(pr.Labels),
		URL:     pr.GetHTMLURL(),
		Merged:  pr.GetMerged(),
		HeadSHA: pr.GetHead().GetSHA(),
	}
}

// labelNames reads the names off a slice of go-github labels.
func labelNames(ls []*gogithub.Label) []string {
	names := make([]string, 0, len(ls))
	for _, l := range ls {
		names = append(names, l.GetName())
	}
	return names
}
