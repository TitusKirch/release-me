// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
	"github.com/ostendorf-cloud/release-me/internal/config"
	"github.com/ostendorf-cloud/release-me/internal/forge"
	"github.com/ostendorf-cloud/release-me/internal/history"
	"github.com/ostendorf-cloud/release-me/internal/version"
)

// preReleaseKind pairs a pre-release type with the label that selects it.
type preReleaseKind struct {
	Type  string
	Label string
}

// preReleaseKinds lists the enabled pre-release types in the fixed order
// alpha, beta, rc: the order a pull request carrying more than one label is
// reported in. A type whose label is configured empty is turned off and
// left out.
func preReleaseKinds(lbl config.Labels) []preReleaseKind {
	all := []preReleaseKind{
		{Type: "alpha", Label: lbl.Alpha},
		{Type: "beta", Label: lbl.Beta},
		{Type: "rc", Label: lbl.Rc},
	}
	kinds := make([]preReleaseKind, 0, len(all))
	for _, k := range all {
		if k.Label != "" {
			kinds = append(kinds, k)
		}
	}
	return kinds
}

// selectPreReleaseType reads which of the alpha, beta and rc labels prLabels
// carries. ok is false when none of them is present: the release stays
// stable, the next run's version returns to it the moment the label it
// carried is removed. err names every label present when more than one is.
func selectPreReleaseType(prLabels []string, lbl config.Labels) (preType string, ok bool, err error) {
	var found []preReleaseKind
	for _, k := range preReleaseKinds(lbl) {
		if slices.Contains(prLabels, k.Label) {
			found = append(found, k)
		}
	}
	switch len(found) {
	case 0:
		return "", false, nil
	case 1:
		return found[0].Type, true, nil
	default:
		names := make([]string, len(found))
		for i, k := range found {
			names[i] = k.Label
		}
		return "", false, fmt.Errorf("more than one pre-release label: %s", strings.Join(names, ", "))
	}
}

// preReleaseVersion computes the pre-release version for preType on top of
// base, the version a stable release would get, together with the history
// tag to collect changelog entries since. since is "" when ownTagPrefix
// carries no earlier pre-release tag yet for base: the caller then keeps
// the entries it already collected since the last stable tag. N starts at
// 1 for preType and counts up from the highest N an earlier pre-release
// tag of that same type and base already carries, scanning every prefix
// in familyTagPrefixes: the counter is shared across the whole family in a
// linked or locked run, ownTagPrefix on its own, as the only entry of
// familyTagPrefixes, for an independent component. A type change does not
// inherit the counter of the type it replaces.
func preReleaseVersion(
	ctx context.Context, dir, ownTagPrefix string, familyTagPrefixes []string, base version.Semver, preType string,
) (v version.Semver, since string, err error) {
	n := 1
	for _, prefix := range familyTagPrefixes {
		tags, err := history.PreReleaseTags(ctx, dir, prefix, base)
		if err != nil {
			return version.Semver{}, "", fmt.Errorf("release-me: pre-release tags: %w", err)
		}
		for _, t := range tags {
			typ, num, ok := t.Version.PreReleaseType()
			if ok && typ == preType && num+1 > n {
				n = num + 1
			}
		}
		if prefix == ownTagPrefix && len(tags) > 0 {
			since = tags[len(tags)-1].Tag
		}
	}
	return base.WithPreRelease(preType, n), since, nil
}

// preReleaseEntries computes the pre-release version string for preType on
// top of rel's stable version, together with the changelog entries to
// render for it: the entries since the previous pre-release tag for that
// base version when one exists, rel's own entries, already collected since
// the last stable tag, otherwise.
func preReleaseEntries(
	ctx context.Context, f forge.Forge, dir string, cfg config.Config, comp config.ResolvedComponent, familyTagPrefixes []string,
	owner, repoName string, rel release, preType string,
) (versionStr string, entries []changelog.Entry, err error) {
	base, ok := version.ParseTag("v" + rel.version.Version)
	if !ok {
		return "", nil, fmt.Errorf("next version %q does not parse", rel.version.Version)
	}

	preVersion, since, err := preReleaseVersion(ctx, dir, comp.TagPrefix, familyTagPrefixes, base, preType)
	if err != nil {
		return "", nil, err
	}
	if since == "" {
		return preVersion.String(), rel.entries, nil
	}

	raw, err := walkEntries(ctx, dir, since, comp.ChangelogSource)
	if err != nil {
		return "", nil, fmt.Errorf("history since %s: %w", since, err)
	}
	raw, err = filterByComponent(ctx, dir, raw, comp.Path)
	if err != nil {
		return "", nil, err
	}
	_, restricted, err := assembleEntries(ctx, f, owner, repoName, raw, cfg.FallbackType, comp.ChangelogSource, dir)
	if err != nil {
		return "", nil, err
	}
	return preVersion.String(), restricted, nil
}
