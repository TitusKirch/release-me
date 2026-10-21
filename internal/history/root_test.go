// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/history"
)

func TestRoot(t *testing.T) {
	t.Parallel()
	dir := newRepo(t)
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cases := []struct {
		name string
		dir  string
	}{
		{name: "from the repository root", dir: dir},
		{name: "from a subdirectory", dir: sub},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := history.Root(context.Background(), tc.dir)
			if err != nil {
				t.Fatalf("Root() error = %v", err)
			}
			want, err := filepath.EvalSymlinks(dir)
			if err != nil {
				t.Fatalf("EvalSymlinks: %v", err)
			}
			gotResolved, err := filepath.EvalSymlinks(got)
			if err != nil {
				t.Fatalf("EvalSymlinks: %v", err)
			}
			if gotResolved != want {
				t.Fatalf("Root() = %q, want %q", gotResolved, want)
			}
		})
	}
}
