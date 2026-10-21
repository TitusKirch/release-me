// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"slices"

	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/forge"
)

// missingLabels returns the configured pending and tagged labels that do
// not exist on the repository, the check doCheck and doPR both apply
// before writing anything. unauthorized is true when the forge rejected
// the credentials rather than failing for any other reason.
func missingLabels(ctx context.Context, f forge.Forge, owner, repoName string, cfg config.Config) (missing []string, unauthorized bool, err error) {
	repoLabels, err := f.RepositoryLabels(ctx, owner, repoName)
	if err != nil {
		if errors.Is(err, forge.ErrUnauthorized) {
			return nil, true, err
		}
		return nil, false, err
	}
	for _, want := range []string{cfg.Labels.Pending, cfg.Labels.Tagged} {
		if !slices.Contains(repoLabels, want) {
			missing = append(missing, want)
		}
	}
	return missing, false, nil
}
