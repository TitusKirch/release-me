// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history

import (
	"context"
	"strings"
)

// emptyTree is git's well known hash of the empty tree, the diff base used
// for a commit that has no parent.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Touches returns the paths, relative to the repository root, that the
// commit sha changed. A merge commit is compared directly against its
// first parent's tree, the same diff a caller would see for the squashed
// equivalent of the branch it merged, instead of the combined diff git log
// shows for a merge by default. A commit with no parent, the repository's
// root commit, is compared against the empty tree.
func Touches(ctx context.Context, dir, sha string) ([]string, error) {
	parent := emptyTree
	if out, err := run(ctx, dir, "rev-parse", "--verify", "--quiet", sha+"^1"); err == nil {
		parent = strings.TrimSpace(out)
	}

	out, err := run(ctx, dir, "diff-tree", "--no-commit-id", "--name-only", "-r", parent, sha)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// Belongs reports whether any of paths, the shape Touches returns, falls
// under componentPath: equal to it, or nested inside it. componentPath "."
// is the sentinel for the component at the repository root and matches
// every path, including an empty list.
func Belongs(paths []string, componentPath string) bool {
	if componentPath == "." {
		return true
	}
	prefix := componentPath + "/"
	for _, p := range paths {
		if p == componentPath || strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}
