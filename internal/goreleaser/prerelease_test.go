// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package goreleaser holds no production code. Its only file proves, against
// the real goreleaser binary and the repository's own .goreleaser.yaml, that
// publish.yaml's "assets" job never overwrites a pre-release tag's
// "prerelease" flag back to false. See issue #41.
package goreleaser_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	fixtureOwner = "ostendorf-cloud"
	fixtureRepo  = "release-me"
	fixtureTag   = "v0.1.0-rc.1"
)

// TestPublishWorkflowKeepsAPreReleaseTagMarkedAsPrerelease reproduces
// publish.yaml's "assets" job: it runs the real goreleaser binary, configured
// with this repository's own .goreleaser.yaml release section, against a
// fake GitHub API that already holds the pre-release release-me's own
// Forge.CreateRelease call created for the tag. It captures every PATCH body
// goreleaser's GitHub client actually sends and asserts none of them carries
// "prerelease":false, which is the regression publish.yaml hit for
// v0.1.0-rc.1 before .goreleaser.yaml set release.prerelease to auto.
//
// Reverting that line in .goreleaser.yaml makes this test fail: the
// extracted release block below no longer sets release.prerelease, so
// goreleaser's GitHub client sends the zero value, false, in the first
// PATCH, same as it did against the live API for v0.1.0-rc.1.
func TestPublishWorkflowKeepsAPreReleaseTagMarkedAsPrerelease(t *testing.T) {
	if testing.Short() {
		t.Skip("drives the real goreleaser binary and a go build, skipped with -short")
	}
	if _, err := exec.LookPath("goreleaser"); err != nil {
		t.Skip("goreleaser not found on PATH, install it with mise install")
	}

	var (
		mu          sync.Mutex
		patchBodies []map[string]any
	)
	release := map[string]any{
		"id":         float64(1),
		"tag_name":   fixtureTag,
		"name":       fixtureTag,
		"draft":      false,
		"prerelease": true,
		"body":       "",
		"html_url":   "https://example.invalid/releases/" + fixtureTag,
		"upload_url": "",
	}

	// go-github treats a WithEnterpriseURLs base URL as a GitHub Enterprise
	// Server host and prefixes every API path with /api/v3, unlike the
	// public api.github.com host.
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/api/v3/repos/%s/%s", fixtureOwner, fixtureRepo), func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"full_name": fixtureOwner + "/" + fixtureRepo})
	})
	mux.HandleFunc(fmt.Sprintf("/api/v3/repos/%s/%s/releases/tags/%s", fixtureOwner, fixtureRepo, fixtureTag), func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		writeJSON(t, w, release)
	})
	mux.HandleFunc(fmt.Sprintf("/api/v3/repos/%s/%s/releases/1", fixtureOwner, fixtureRepo), func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding PATCH body: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		patchBodies = append(patchBodies, body)
		for k, v := range body {
			release[k] = v
		}
		updated := make(map[string]any, len(release))
		for k, v := range release {
			updated[k] = v
		}
		mu.Unlock()

		writeJSON(t, w, updated)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	dir := t.TempDir()
	initFixtureRepo(t, dir)
	writeFixtureConfig(t, dir, server.URL)

	cmd := exec.CommandContext(t.Context(), "goreleaser", "release", "--skip=announce,validate", "--timeout=2m")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GITHUB_TOKEN=prerelease-reproduction-test-token")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goreleaser release failed: %v\n%s", err, out)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(patchBodies) == 0 {
		t.Fatalf("goreleaser never updated the release, expected at least one PATCH request\n%s", out)
	}
	for i, body := range patchBodies {
		prerelease, sent := body["prerelease"]
		if sent && prerelease != true {
			t.Errorf("PATCH #%d sent prerelease=%v, want true: the fake release was already a pre-release, so this would clear GitHub's flag\nfull body: %#v\ngoreleaser output:\n%s", i, prerelease, body, out)
		}
	}
}

// initFixtureRepo creates a one-commit, one-tag git repository in dir, with
// a single trivial main package goreleaser can build. A remote named origin
// is required: goreleaser's git pipe calls "git ls-remote --get-url", which
// errors without one, even though it never contacts the network.
func initFixtureRepo(t *testing.T, dir string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	runGit(t, dir, "init", "-q")
	runGit(t, dir, "remote", "add", "origin", fmt.Sprintf("https://github.com/%s/%s.git", fixtureOwner, fixtureRepo))
	runGit(t, dir, "config", "user.email", "reproduction@example.invalid")
	runGit(t, dir, "config", "user.name", "reproduction")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "fixture")
	runGit(t, dir, "tag", fixtureTag)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// writeFixtureConfig writes dir/.goreleaser.yaml: a minimal build plus the
// repository's own .goreleaser.yaml "release" block, read from disk as-is,
// with github_urls pointed at the fake server, release.github set to the
// fixture repository (so goreleaser does not try to detect it from a real
// GitHub remote), and release.skip_upload set, since no artifact upload
// endpoint is faked here.
func writeFixtureConfig(t *testing.T, dir, apiURL string) {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read the repository's .goreleaser.yaml: %v", err)
	}

	var real map[string]any
	if err := yaml.Unmarshal(raw, &real); err != nil {
		t.Fatalf("parse the repository's .goreleaser.yaml: %v", err)
	}
	release, ok := real["release"].(map[string]any)
	if !ok {
		t.Fatalf(".goreleaser.yaml has no release section to test")
	}
	release["github"] = map[string]any{"owner": fixtureOwner, "name": fixtureRepo}
	release["skip_upload"] = "true"

	fixture := map[string]any{
		"version":      2,
		"project_name": "fixture",
		"builds": []map[string]any{{
			"id":     "fixture",
			"main":   "./main.go",
			"binary": "fixture",
			"env":    []string{"CGO_ENABLED=0"},
			"goos":   []string{runtime.GOOS},
			"goarch": []string{runtime.GOARCH},
		}},
		"archives": []map[string]any{{"formats": []string{"tar.gz"}}},
		"checksum": map[string]any{"name_template": "checksums.txt"},
		"release":  release,
		"github_urls": map[string]any{
			"api":    apiURL,
			"upload": apiURL,
		},
	}

	out, err := yaml.Marshal(fixture)
	if err != nil {
		t.Fatalf("render the fixture .goreleaser.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".goreleaser.yaml"), out, 0o644); err != nil {
		t.Fatalf("write the fixture .goreleaser.yaml: %v", err)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encoding fake GitHub API response: %v", err)
	}
}
