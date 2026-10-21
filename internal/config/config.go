// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package config loads release-me.yaml, the keys this milestone reads:
// initial-version, bump-minor-pre-major, hidden-types, fallback-type,
// token-env.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ostendorf-cloud/release-me/internal/conventional"
	"gopkg.in/yaml.v3"
)

// Defaults applied for a key left out of release-me.yaml, or for a missing
// file entirely.
const (
	DefaultInitialVersion = "0.1.0"
	DefaultTokenEnv       = "GITHUB_TOKEN"
	// DefaultFallbackType is conventional.FallbackType, named here too so
	// callers of this package never need to import internal/conventional
	// just to read the default.
	DefaultFallbackType = conventional.FallbackType
)

// DefaultHiddenTypes is the hidden-types list applied when the key is left
// out of release-me.yaml. DefaultFallbackType is hidden by default.
var DefaultHiddenTypes = []string{"chore", "docs", "ci", "style", "refactor", "test", "build", DefaultFallbackType}

// bumpingTypes are refused as fallback-type: text that does not parse must
// never move the version.
var bumpingTypes = map[string]bool{"feat": true, "fix": true, "perf": true, "revert": true}

// Config is the subset of release-me.yaml this milestone reads.
type Config struct {
	InitialVersion    string
	BumpMinorPreMajor bool
	HiddenTypes       []string
	FallbackType      string
	TokenEnv          string
}

// raw mirrors Config with pointer fields, so Load can tell a key left out of
// the file apart from one set to its zero value.
type raw struct {
	InitialVersion    *string   `yaml:"initial-version"`
	BumpMinorPreMajor *bool     `yaml:"bump-minor-pre-major"`
	HiddenTypes       *[]string `yaml:"hidden-types"`
	FallbackType      *string   `yaml:"fallback-type"`
	TokenEnv          *string   `yaml:"token-env"`
}

// Default is the configuration applied when release-me.yaml is absent.
func Default() Config {
	return Config{
		InitialVersion: DefaultInitialVersion,
		HiddenTypes:    DefaultHiddenTypes,
		FallbackType:   DefaultFallbackType,
		TokenEnv:       DefaultTokenEnv,
	}
}

// Load reads release-me.yaml at path. A missing file yields Default(). An
// unknown key is an error.
func Load(path string) (cfg Config, err error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("config: open %s: %w", path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("config: close %s: %w", path, cerr)
		}
	}()

	var r raw
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&r); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}

	cfg = Default()
	if r.InitialVersion != nil {
		cfg.InitialVersion = *r.InitialVersion
	}
	if r.BumpMinorPreMajor != nil {
		cfg.BumpMinorPreMajor = *r.BumpMinorPreMajor
	}
	if r.HiddenTypes != nil {
		cfg.HiddenTypes = *r.HiddenTypes
	}
	if r.FallbackType != nil {
		cfg.FallbackType = *r.FallbackType
	}
	if bumpingTypes[cfg.FallbackType] {
		return Config{}, fmt.Errorf(
			"config: parse %s: fallback-type %q bumps the version, unparseable text must not",
			path, cfg.FallbackType,
		)
	}
	if r.TokenEnv != nil {
		cfg.TokenEnv = *r.TokenEnv
	}
	return cfg, nil
}
