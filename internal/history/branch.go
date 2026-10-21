// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history

import (
	"context"
	"fmt"
	"strings"
)

// originHEADPrefix is what "git symbolic-ref refs/remotes/origin/HEAD"
// reports the origin remote's default branch ref as.
const originHEADPrefix = "refs/remotes/origin/"

// OriginHEAD returns the branch name the "origin" remote reports as its
// HEAD, through "git symbolic-ref refs/remotes/origin/HEAD", stripped of
// its "refs/remotes/origin/" prefix. It is the base branch fallback for the
// release pull request when the forge's default branch cannot be read; the
// branch currently checked out, including a detached HEAD, is never used.
func OriginHEAD(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "symbolic-ref", originHEADPrefix+"HEAD")
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(out)
	if !strings.HasPrefix(name, originHEADPrefix) {
		return "", fmt.Errorf("history: origin HEAD %q is not under %s", name, originHEADPrefix)
	}
	return strings.TrimPrefix(name, originHEADPrefix), nil
}
