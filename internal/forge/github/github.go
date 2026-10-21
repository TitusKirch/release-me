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

	labels := make([]string, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		labels = append(labels, l.GetName())
	}
	return forge.PullRequest{
		Number: number,
		Title:  pr.GetTitle(),
		Body:   pr.GetBody(),
		Labels: labels,
		URL:    pr.GetHTMLURL(),
	}, nil
}
