// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package forgetest provides a fake forge.Forge for tests, so callers never
// reach the network.
package forgetest

import (
	"context"

	"github.com/ostendorf-cloud/release-me/internal/forge"
)

// Response is the configured answer for one pull request number.
type Response struct {
	PullRequest forge.PullRequest
	Err         error
}

// Fake is a forge.Forge backed by a fixed map of responses, one per pull
// request number. A number without a configured response yields
// forge.ErrNotFound with a zero forge.PullRequest.
type Fake struct {
	Responses map[int]Response
}

// PullRequest implements forge.Forge.
func (f *Fake) PullRequest(_ context.Context, _, _ string, number int) (forge.PullRequest, error) {
	r, ok := f.Responses[number]
	if !ok {
		return forge.PullRequest{}, forge.ErrNotFound
	}
	return r.PullRequest, r.Err
}
