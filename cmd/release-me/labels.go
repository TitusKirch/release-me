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

// missingLabels returns the configured pending, tagged and pre-release type
// labels that do not exist on the repository, the check doCheck and doPR
// both apply before writing anything. unauthorized is true when the forge
// rejected the credentials rather than failing for any other reason.
func missingLabels(ctx context.Context, f forge.Forge, owner, repoName string, cfg config.Config) (missing []string, unauthorized bool, err error) {
	repoLabels, err := f.RepositoryLabels(ctx, owner, repoName)
	if err != nil {
		if errors.Is(err, forge.ErrUnauthorized) {
			return nil, true, err
		}
		return nil, false, err
	}
	wanted := []string{cfg.Labels.Pending, cfg.Labels.Tagged, cfg.Labels.Alpha, cfg.Labels.Beta, cfg.Labels.Rc}
	for _, want := range wanted {
		// An empty pre-release label turns that type off: there is no
		// label to expect on the repository.
		if want != "" && !slices.Contains(repoLabels, want) {
			missing = append(missing, want)
		}
	}
	return missing, false, nil
}
