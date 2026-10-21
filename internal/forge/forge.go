// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package forge defines the interface release-me uses to read pull request
// data from a code hosting platform. internal/forge/github implements it for
// GitHub; the core packages depend only on this package, never on a concrete
// implementation.
package forge

import (
	"context"
	"errors"
)

// ErrNotFound is returned when the forge has no pull request for the given
// number.
var ErrNotFound = errors.New("pull request not found")

// PullRequest is the subset of pull request data milestone 1 needs.
type PullRequest struct {
	Number int
	Title  string
	Body   string
	Labels []string
	URL    string
}

// Forge reads pull request data from a code hosting platform.
//
// PullRequest may return a partially populated PullRequest alongside
// ErrNotFound: an implementation that can derive a link from owner, repo and
// number alone should still set URL, so a caller can show a link even when
// the pull request itself could not be read.
type Forge interface {
	PullRequest(ctx context.Context, owner, repo string, number int) (PullRequest, error)
}
