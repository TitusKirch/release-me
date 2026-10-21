// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package changelog_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/changelog"
)

func TestPrepend(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		existing string
		rendered string
		want     string
	}{
		{
			name:     "creates the heading for a missing file",
			existing: "",
			rendered: "## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the first feature\n",
			want:     "# Changelog\n\n## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the first feature\n",
		},
		{
			name:     "inserts the new version above the existing ones",
			existing: "# Changelog\n\n## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the first feature\n",
			rendered: "## v0.2.0 (2026-02-03)\n\n### Bug Fixes\n\n- stop the panic\n",
			want: "# Changelog\n\n## v0.2.0 (2026-02-03)\n\n### Bug Fixes\n\n- stop the panic\n\n" +
				"## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the first feature\n",
		},
		{
			name:     "returns existing unchanged when rendered is empty",
			existing: "# Changelog\n\n## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the first feature\n",
			rendered: "",
			want:     "# Changelog\n\n## v0.1.0 (2026-01-02)\n\n### Features\n\n- add the first feature\n",
		},
		{
			name:     "returns an empty file unchanged when rendered is empty",
			existing: "",
			rendered: "",
			want:     "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := changelog.Prepend(tc.existing, tc.rendered)
			if got != tc.want {
				t.Fatalf("Prepend() = %q, want %q", got, tc.want)
			}
		})
	}
}
