// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package migrate_test

import (
	"fmt"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/migrate"
)

func TestFromReleasePleaseCarriesDraftPullRequestOver(t *testing.T) {
	t.Parallel()
	for _, draft := range []bool{false, true} {
		configJSON := fmt.Sprintf(`{"draft-pull-request": %v, "packages": {".": {"release-type": "go"}}}`, draft)
		got, err := migrate.FromReleasePlease([]byte(configJSON), nil, files(nil))
		if err != nil {
			t.Fatalf("FromReleasePlease() error = %v", err)
		}
		if cfg := load(t, got.YAML); cfg.ReleasePullRequestDraft != draft {
			t.Fatalf("ReleasePullRequestDraft = %v, want %v\n%s", cfg.ReleasePullRequestDraft, draft, got.YAML)
		}
	}
}
