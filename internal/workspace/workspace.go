// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

// Package workspace reads the package manifests of a repository's
// components, finds the components that depend on each other, and returns
// a manifest's new content with a local dependency moved to its new
// version. It reads and returns content only: the command layer reads the
// manifests and writes the result.
package workspace

import (
	"fmt"
	"slices"
)

// The package managers release-me.yaml's "workspaces" key accepts.
const (
	// Node reads a component's package.json: an npm, pnpm or yarn
	// workspace package.
	Node = "node"
)

// manager is one package manager's manifest file name, reader and
// dependency rewrite.
type manager struct {
	file  string
	parse func(content []byte) (Package, error)
	set   func(content, name, version string) (string, error)
	// linksLocal reports whether a dependency range resolves to the local
	// package at localVersion when Options.LocalRangesOnly is set; nil for
	// a manager that always links a local dependency.
	linksLocal func(rng, localVersion string) bool
}

// managers maps every package manager "workspaces" accepts to its
// manifest.
var managers = map[string]manager{
	Node: {file: "package.json", parse: parseNode, set: setNodeDependency, linksLocal: nodeLinksLocal},
}

// Options tunes which dependencies link two members.
type Options struct {
	// LocalRangesOnly links a node dependency only where it resolves to
	// the local package anyway: through a local protocol such as
	// "workspace:" or "file:", or a range the local package's current
	// version satisfies. Any other dependency on a local package name is
	// left to the registry. It is release-please's "always-link-local"
	// turned off; without it every dependency on a local package name
	// links.
	LocalRangesOnly bool
}

// Managers lists every package manager "workspaces" accepts, in the order
// errors report them.
var Managers = []string{Node}

// ManifestFile returns the file name a component's manifest carries for
// manager, relative to the component's path, "" for an unknown manager.
func ManifestFile(manager string) string {
	return managers[manager].file
}

// Package is one manifest as read: the name other manifests depend on it
// by, its version, and every dependency it declares.
type Package struct {
	Name         string
	Version      string
	Dependencies []Dependency
}

// Dependency is one dependency a manifest declares: the name it depends on
// and the version range it asks for, as written.
type Dependency struct {
	Name  string
	Range string
}

// Parse reads content, a manifest of manager.
func Parse(manager string, content []byte) (Package, error) {
	m, ok := managers[manager]
	if !ok {
		return Package{}, fmt.Errorf("workspace: unknown package manager %q", manager)
	}
	return m.parse(content)
}

// SetDependency returns content with every declaration of the dependency
// name moved to version, keeping the range operator each one carries: a
// "^1.2.0" becomes "^1.3.0". A declaration whose range names no version to
// move, such as "workspace:*" or "*", is left as it is. An error is
// returned when content declares name in a shape the manager's rewrite
// cannot read.
func SetDependency(manager, content, name, version string) (string, error) {
	m, ok := managers[manager]
	if !ok {
		return "", fmt.Errorf("workspace: unknown package manager %q", manager)
	}
	return m.set(content, name, version)
}

// Member is one component's manifest of one package manager.
type Member struct {
	// Component is the component's name.
	Component string
	Manager   string
	Package   Package
}

// Link is one dependency of a member on another member's package.
type Link struct {
	// Dependent and Dependency are the components on either side.
	Dependent, Dependency string
	// Manager is the package manager whose manifests link the two.
	Manager string
	// Name is the dependency's package name, as the dependent declares it.
	Name string
}

// Links returns every dependency a member declares on another member's
// package of the same package manager, in members' order, each
// dependent's links in its manifest's order. A dependency on the member's
// own package is no link, and with opts.LocalRangesOnly neither is one
// whose range would not resolve to the local package.
func Links(members []Member, opts Options) []Link {
	var links []Link
	for _, m := range members {
		for _, dep := range m.Package.Dependencies {
			for _, target := range members {
				if target.Component == m.Component || target.Manager != m.Manager || target.Package.Name != dep.Name {
					continue
				}
				if local := managers[m.Manager].linksLocal; opts.LocalRangesOnly && local != nil &&
					!local(dep.Range, target.Package.Version) {
					continue
				}
				link := Link{Dependent: m.Component, Dependency: target.Component, Manager: m.Manager, Name: dep.Name}
				if !slices.Contains(links, link) {
					links = append(links, link)
				}
			}
		}
	}
	return links
}

// Dependents returns every component that must release because a
// component in released does: one that depends on a released component,
// directly or through another dependent, in members' order, the released
// components themselves left out. opts decides the links, as in Links.
func Dependents(members []Member, released []string, opts Options) []string {
	links := Links(members, opts)
	releasing := slices.Clone(released)
	var out []string
	for changed := true; changed; {
		changed = false
		for _, m := range members {
			if slices.Contains(releasing, m.Component) {
				continue
			}
			if slices.ContainsFunc(links, func(l Link) bool {
				return l.Dependent == m.Component && slices.Contains(releasing, l.Dependency)
			}) {
				releasing = append(releasing, m.Component)
				out = append(out, m.Component)
				changed = true
			}
		}
	}
	return out
}

// semverPattern matches a full version, pre-release and build metadata
// included, the shape every released version carries.
const semverPattern = `\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?`
