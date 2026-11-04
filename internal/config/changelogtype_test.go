// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/config"
)

func TestLoadReadsChangelogType(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load(writeFile(t, "changelog-type: github\ncomponents:\n  api:\n    path: api\n"+
		"  web:\n    path: web\n    changelog-type: default\n"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got := cfg.Resolve()
	if got[0].ChangelogType != config.ChangelogTypeGitHub || got[1].ChangelogType != config.ChangelogTypeDefault {
		t.Fatalf("ChangelogType = api %q, web %q, want github from the top level and web's own default",
			got[0].ChangelogType, got[1].ChangelogType)
	}

	for _, content := range []string{"changelog-type: gitlab\n", "components:\n  api:\n    path: api\n    changelog-type: gitlab\n"} {
		_, err := config.Load(writeFile(t, content))
		if err == nil || !strings.Contains(err.Error(), `changelog-type "gitlab" is not default or github`) {
			t.Fatalf("Load(%q) error = %v, want the unknown type refused", content, err)
		}
	}
}
