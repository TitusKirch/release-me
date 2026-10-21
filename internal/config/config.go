// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package config loads release-me.yaml: initial-version,
// bump-minor-pre-major, hidden-types, fallback-type, token-env, files,
// changelog, labels, major-tag and components.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	stdpath "path"
	"strings"

	"github.com/ostendorf-cloud/release-me/internal/conventional"
	"github.com/ostendorf-cloud/release-me/internal/updater"
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
	// DefaultChangelogPath is the changelog file path applied when the
	// "changelog" key is left out of release-me.yaml.
	DefaultChangelogPath = "CHANGELOG.md"
	// DefaultPendingLabel and DefaultTaggedLabel are the label names
	// applied when the "labels" block, or one of its keys, is left out.
	DefaultPendingLabel = "release: pending"
	DefaultTaggedLabel  = "release: tagged"
	// DefaultAlphaLabel, DefaultBetaLabel and DefaultRcLabel are the
	// pre-release type label names applied when the "labels" block, or one
	// of its keys, is left out.
	DefaultAlphaLabel = "release: alpha"
	DefaultBetaLabel  = "release: beta"
	DefaultRcLabel    = "release: rc"
)

// DefaultHiddenTypes is the hidden-types list applied when the key is left
// out of release-me.yaml. DefaultFallbackType is hidden by default.
var DefaultHiddenTypes = []string{"chore", "docs", "ci", "style", "refactor", "test", "build", DefaultFallbackType}

// bumpingTypes are refused as fallback-type: text that does not parse must
// never move the version.
var bumpingTypes = map[string]bool{"feat": true, "fix": true, "perf": true, "revert": true}

// File is one entry of the "files" key: a file release-me bumps the version
// in, through the named updater. Key is the dotted path, identifier or
// other locator the updater needs to find the value, empty for an updater
// that takes none.
type File struct {
	Path    string
	Updater string
	Key     string
}

// Labels is the "labels" key: the label names release-me expects on the
// repository. Alpha, Beta and Rc select a pre-release type on the release
// pull request; Pending and Tagged apply to every release, stable or not.
type Labels struct {
	Pending string
	Tagged  string
	Alpha   string
	Beta    string
	Rc      string
}

// Config is the release-me.yaml configuration.
type Config struct {
	InitialVersion    string
	BumpMinorPreMajor bool
	HiddenTypes       []string
	FallbackType      string
	TokenEnv          string
	Files             []File
	ChangelogPath     string
	Labels            Labels
	// MajorTag is the top-level "major-tag" key: when true, a stable
	// release moves the major tag "vX" to the release commit. A component
	// overrides it through its own "major-tag" key.
	MajorTag bool
	// Components is the "components" key, in the order release-me.yaml
	// declares them, or nil when the key is absent: a repository without
	// it is one component at the repository root, as Resolve reports.
	Components []Component
}

// Component is one entry of the "components" key: a name mapped to a path
// and optional overrides of the top-level initial-version,
// bump-minor-pre-major, hidden-types, files and major-tag. A nil field was
// left out of release-me.yaml and falls back to the top-level value, the
// same left-out-versus-zero-value distinction Load applies to the top
// level itself.
type Component struct {
	Name              string
	Path              string
	InitialVersion    *string
	BumpMinorPreMajor *bool
	HiddenTypes       *[]string
	Files             *[]File
	MajorTag          *bool
}

// ResolvedComponent is one component ready for a command to release: its
// name and path, the tag prefix, release branch and changelog path derived
// from them, and its settings with the top-level ones applied where it
// carries no override of its own.
type ResolvedComponent struct {
	Name          string
	Path          string
	TagPrefix     string
	Branch        string
	ChangelogPath string
	// Multi is true when release-me.yaml declares a "components" key. It
	// is false for the single implicit "main" component, which keeps
	// today's tag prefix "v" and release name instead of the generic
	// "<name>/v" prefix and "<name> vX.Y.Z" release name a configured
	// component carries.
	Multi             bool
	InitialVersion    string
	BumpMinorPreMajor bool
	HiddenTypes       []string
	Files             []File
	MajorTag          bool
}

// Resolve returns the components a command should loop over, in
// release-me.yaml's order: the configured "components", each merged with
// the top-level settings it does not override, or the single implicit
// "main" component at the repository root when the "components" key is
// absent.
func (cfg Config) Resolve() []ResolvedComponent {
	if len(cfg.Components) == 0 {
		return []ResolvedComponent{{
			Name:              "main",
			Path:              ".",
			TagPrefix:         "v",
			Branch:            "release-me/main",
			ChangelogPath:     cfg.ChangelogPath,
			InitialVersion:    cfg.InitialVersion,
			BumpMinorPreMajor: cfg.BumpMinorPreMajor,
			HiddenTypes:       cfg.HiddenTypes,
			Files:             cfg.Files,
			MajorTag:          cfg.MajorTag,
		}}
	}

	out := make([]ResolvedComponent, 0, len(cfg.Components))
	for _, c := range cfg.Components {
		rc := ResolvedComponent{
			Name:              c.Name,
			Path:              c.Path,
			TagPrefix:         c.Name + "/v",
			Branch:            "release-me/" + c.Name,
			ChangelogPath:     stdpath.Join(c.Path, "CHANGELOG.md"),
			Multi:             true,
			InitialVersion:    cfg.InitialVersion,
			BumpMinorPreMajor: cfg.BumpMinorPreMajor,
			HiddenTypes:       cfg.HiddenTypes,
			Files:             cfg.Files,
			MajorTag:          cfg.MajorTag,
		}
		if c.InitialVersion != nil {
			rc.InitialVersion = *c.InitialVersion
		}
		if c.BumpMinorPreMajor != nil {
			rc.BumpMinorPreMajor = *c.BumpMinorPreMajor
		}
		if c.HiddenTypes != nil {
			rc.HiddenTypes = *c.HiddenTypes
		}
		if c.Files != nil {
			rc.Files = *c.Files
		}
		if c.MajorTag != nil {
			rc.MajorTag = *c.MajorTag
		}
		out = append(out, rc)
	}
	return out
}

// rawFile mirrors File, the yaml shape of one "files" entry.
type rawFile struct {
	Path    string  `yaml:"path"`
	Updater *string `yaml:"updater"`
	Key     *string `yaml:"key"`
}

// rawLabels mirrors Labels with pointer fields, so Load can tell a key left
// out of the "labels" block apart from one set to its zero value.
type rawLabels struct {
	Pending *string `yaml:"pending"`
	Tagged  *string `yaml:"tagged"`
	Alpha   *string `yaml:"alpha"`
	Beta    *string `yaml:"beta"`
	Rc      *string `yaml:"rc"`
}

// raw mirrors Config with pointer fields, so Load can tell a key left out of
// the file apart from one set to its zero value.
type raw struct {
	InitialVersion    *string    `yaml:"initial-version"`
	BumpMinorPreMajor *bool      `yaml:"bump-minor-pre-major"`
	HiddenTypes       *[]string  `yaml:"hidden-types"`
	FallbackType      *string    `yaml:"fallback-type"`
	TokenEnv          *string    `yaml:"token-env"`
	Files             *[]rawFile `yaml:"files"`
	Changelog         *string    `yaml:"changelog"`
	Labels            *rawLabels `yaml:"labels"`
	MajorTag          *bool      `yaml:"major-tag"`
	// Components is decoded as a node, not a Go map, so decodeComponents
	// can read its entries back in release-me.yaml's order: a Go map
	// gives no such guarantee. A zero Kind means the "components" key was
	// left out: yaml.v3 only special cases decoding into a Node field by
	// value, never through a pointer.
	Components yaml.Node `yaml:"components"`
}

// rawComponent mirrors Component with pointer fields, the yaml shape of one
// "components" entry.
type rawComponent struct {
	Path              *string    `yaml:"path"`
	InitialVersion    *string    `yaml:"initial-version"`
	BumpMinorPreMajor *bool      `yaml:"bump-minor-pre-major"`
	HiddenTypes       *[]string  `yaml:"hidden-types"`
	Files             *[]rawFile `yaml:"files"`
	MajorTag          *bool      `yaml:"major-tag"`
}

// Default is the configuration applied when release-me.yaml is absent.
func Default() Config {
	return Config{
		InitialVersion: DefaultInitialVersion,
		HiddenTypes:    DefaultHiddenTypes,
		FallbackType:   DefaultFallbackType,
		TokenEnv:       DefaultTokenEnv,
		ChangelogPath:  DefaultChangelogPath,
		Labels: Labels{
			Pending: DefaultPendingLabel, Tagged: DefaultTaggedLabel,
			Alpha: DefaultAlphaLabel, Beta: DefaultBetaLabel, Rc: DefaultRcLabel,
		},
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
	if r.Changelog != nil {
		cfg.ChangelogPath = *r.Changelog
	}
	if r.Labels != nil {
		if r.Labels.Pending != nil {
			cfg.Labels.Pending = *r.Labels.Pending
		}
		if r.Labels.Tagged != nil {
			cfg.Labels.Tagged = *r.Labels.Tagged
		}
		if r.Labels.Alpha != nil {
			cfg.Labels.Alpha = *r.Labels.Alpha
		}
		if r.Labels.Beta != nil {
			cfg.Labels.Beta = *r.Labels.Beta
		}
		if r.Labels.Rc != nil {
			cfg.Labels.Rc = *r.Labels.Rc
		}
	}
	if r.Files != nil {
		files, err := decodeFiles(path, *r.Files)
		if err != nil {
			return Config{}, err
		}
		cfg.Files = files
	}
	if r.MajorTag != nil {
		cfg.MajorTag = *r.MajorTag
	}
	if r.Components.Kind != 0 {
		components, err := decodeComponents(path, &r.Components)
		if err != nil {
			return Config{}, err
		}
		if err := validateComponentPaths(path, components); err != nil {
			return Config{}, err
		}
		cfg.Components = components
	}
	return cfg, nil
}

// decodeFiles converts the yaml shape of a "files" key into []File,
// defaulting a left-out updater to the marker updater and rejecting one
// that names no known updater, that is missing a key it requires, or that
// carries a key it takes none of.
func decodeFiles(path string, raw []rawFile) ([]File, error) {
	files := make([]File, 0, len(raw))
	for _, rf := range raw {
		name := updater.Marker
		if rf.Updater != nil {
			name = *rf.Updater
		}
		entry, ok := updater.Updaters[name]
		if !ok {
			return nil, fmt.Errorf("config: parse %s: file %s: unknown updater %q", path, rf.Path, name)
		}
		key := ""
		if rf.Key != nil {
			key = *rf.Key
		}
		if entry.NeedsKey && key == "" {
			return nil, fmt.Errorf("config: parse %s: file %s: updater %q requires a key", path, rf.Path, name)
		}
		if !entry.NeedsKey && key != "" {
			return nil, fmt.Errorf("config: parse %s: file %s: updater %q takes no key", path, rf.Path, name)
		}
		files = append(files, File{Path: rf.Path, Updater: name, Key: key})
	}
	return files, nil
}

// decodeComponents converts the "components" mapping node into []Component,
// in the order its keys appear in release-me.yaml. Each entry's value is
// decoded with the same strict, unknown-key-rejecting rules as the rest of
// release-me.yaml.
func decodeComponents(path string, node *yaml.Node) ([]Component, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config: parse %s: components: must be a mapping of name to settings", path)
	}

	components := make([]Component, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		name := node.Content[i].Value
		valueNode := node.Content[i+1]

		data, err := yaml.Marshal(valueNode)
		if err != nil {
			return nil, fmt.Errorf("config: parse %s: component %s: %w", path, name, err)
		}
		var rc rawComponent
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&rc); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("config: parse %s: component %s: %w", path, name, err)
		}
		if rc.Path == nil {
			return nil, fmt.Errorf("config: parse %s: component %s: path is required", path, name)
		}

		c := Component{
			Name: name, Path: *rc.Path, InitialVersion: rc.InitialVersion,
			BumpMinorPreMajor: rc.BumpMinorPreMajor, HiddenTypes: rc.HiddenTypes, MajorTag: rc.MajorTag,
		}
		if rc.Files != nil {
			files, err := decodeFiles(path, *rc.Files)
			if err != nil {
				return nil, err
			}
			c.Files = &files
		}
		components = append(components, c)
	}
	return components, nil
}

// validateComponentPaths rejects two components whose paths overlap: equal,
// or one nested inside the other, including the root path ".", which
// overlaps with every other component.
func validateComponentPaths(path string, components []Component) error {
	for i := 0; i < len(components); i++ {
		for j := i + 1; j < len(components); j++ {
			a, b := components[i], components[j]
			if pathsOverlap(a.Path, b.Path) {
				return fmt.Errorf("config: parse %s: component %q and %q have overlapping paths %q and %q",
					path, a.Name, b.Name, a.Path, b.Path)
			}
		}
	}
	return nil
}

// pathsOverlap reports whether a and b name the same component path, or one
// is nested inside the other. "." is the repository root and overlaps with
// every path.
func pathsOverlap(a, b string) bool {
	return pathContains(a, b) || pathContains(b, a)
}

// pathContains reports whether child is parent itself or nested under it.
// parent "." contains every path.
func pathContains(parent, child string) bool {
	if parent == "." {
		return true
	}
	return child == parent || strings.HasPrefix(child, parent+"/")
}
