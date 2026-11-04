// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"reflect"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/workspace"
)

// acmeMembers is a workspace of four packages: web depends on ui and
// core, ui on core, and docs on nothing local.
var acmeMembers = []workspace.Member{
	{Component: "core", Manager: workspace.Node, Package: workspace.Package{Name: "@acme/core", Version: "1.2.0"}},
	{Component: "ui", Manager: workspace.Node, Package: workspace.Package{
		Name: "@acme/ui", Version: "0.8.0",
		Dependencies: []workspace.Dependency{{Name: "@acme/core", Range: "^1.2.0"}, {Name: "vue", Range: "^3.5.13"}},
	}},
	{Component: "web", Manager: workspace.Node, Package: workspace.Package{
		Name: "@acme/web", Version: "0.4.0",
		Dependencies: []workspace.Dependency{
			{Name: "@acme/ui", Range: "workspace:*"}, {Name: "@acme/core", Range: "^1.2.0"}, {Name: "@acme/core", Range: "workspace:^1.2.0"},
		},
	}},
	{Component: "docs", Manager: workspace.Node, Package: workspace.Package{
		Name: "@acme/docs", Version: "2.0.0",
		Dependencies: []workspace.Dependency{{Name: "vitepress", Range: "^1.6.0"}},
	}},
}

func TestLinksFindsEveryDependencyOnAnotherMember(t *testing.T) {
	t.Parallel()
	got := workspace.Links(acmeMembers)
	want := []workspace.Link{
		{Dependent: "ui", Dependency: "core", Manager: workspace.Node, Name: "@acme/core"},
		{Dependent: "web", Dependency: "ui", Manager: workspace.Node, Name: "@acme/ui"},
		{Dependent: "web", Dependency: "core", Manager: workspace.Node, Name: "@acme/core"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Links() = %+v, want %+v", got, want)
	}
}

func TestDependentsReleasesEveryComponentDependingOnAReleasedOne(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		released []string
		want     []string
	}{
		{name: "a released dependency releases its dependents transitively", released: []string{"core"}, want: []string{"ui", "web"}},
		{name: "a released leaf releases its own dependents only", released: []string{"ui"}, want: []string{"web"}},
		{name: "a dependent already released is not added again", released: []string{"core", "web"}, want: []string{"ui"}},
		{name: "a package nobody depends on releases nothing else", released: []string{"docs"}, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := workspace.Dependents(acmeMembers, tc.released); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Dependents(%q) = %q, want %q", tc.released, got, tc.want)
			}
		})
	}
}

func TestLinksIgnoresAPackageOfAnotherManagerWithTheSameName(t *testing.T) {
	t.Parallel()
	members := []workspace.Member{
		{Component: "core", Manager: "other", Package: workspace.Package{Name: "core"}},
		{Component: "web", Manager: workspace.Node, Package: workspace.Package{
			Name: "web", Dependencies: []workspace.Dependency{{Name: "core", Range: "^1.0.0"}},
		}},
	}
	if got := workspace.Links(members); len(got) != 0 {
		t.Fatalf("Links() = %+v, want none across package managers", got)
	}
}
