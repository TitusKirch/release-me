// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"strings"
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/updater"
)

const pomXML = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>

  <parent>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-parent</artifactId>
    <version>3.3.2</version>
  </parent>

  <groupId>cloud.ostendorf</groupId>
  <artifactId>release-notes</artifactId>
  <!-- the version release-me bumps -->
  <version>1.4.0</version>

  <dependencies>
    <dependency>
      <groupId>org.slf4j</groupId>
      <artifactId>slf4j-api</artifactId>
      <version>2.0.13</version>
    </dependency>
  </dependencies>
</project>
`

const csproj = utf8BOM + `<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <Version>
      0.3.1
    </Version>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Serilog" Version="4.0.0" />
  </ItemGroup>

</Project>
`

const utf8BOM = "\xef\xbb\xbf"

func TestUpdateXMLReplacesTheTextOfTheElementAtKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		key     string
		from    string
		to      string
	}{
		{
			name:    "bumps a pom.xml's project version, not its parent's or a dependency's",
			content: pomXML,
			key:     "project.version",
			from:    "  <version>1.4.0</version>",
			to:      "  <version>1.5.0-rc.1</version>",
		},
		{
			name:    "bumps a pom.xml's parent version when key names it",
			content: pomXML,
			key:     "project.parent.version",
			from:    "    <version>3.3.2</version>",
			to:      "    <version>1.5.0-rc.1</version>",
		},
		{
			name:    "bumps a .csproj's Version, keeping the byte order mark and the whitespace around it",
			content: csproj,
			key:     "Project.PropertyGroup.Version",
			from:    "      0.3.1",
			to:      "      1.5.0-rc.1",
		},
		{
			name:    "matches an element by its local name, ignoring its namespace prefix",
			content: "<pom:project xmlns:pom=\"http://maven.apache.org/POM/4.0.0\">\n  <pom:version>1.4.0</pom:version>\n</pom:project>\n",
			key:     "project.version",
			from:    "  <pom:version>1.4.0</pom:version>",
			to:      "  <pom:version>1.5.0-rc.1</pom:version>",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := updater.UpdateXML(tc.content, tc.key, "1.5.0-rc.1")
			if err != nil {
				t.Fatalf("UpdateXML() error = %v", err)
			}
			want := replaceLine(tc.content, tc.from, tc.to)
			if want == tc.content {
				t.Fatalf("fixture has no line %q", tc.from)
			}
			if got != want {
				t.Fatalf("UpdateXML() =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestUpdateXMLReplacesEveryElementAtKey(t *testing.T) {
	t.Parallel()
	content := `<Project>
  <PropertyGroup Condition="'$(Configuration)' == 'Debug'">
    <Version>0.3.1</Version>
  </PropertyGroup>
  <PropertyGroup Condition="'$(Configuration)' == 'Release'">
    <Version>0.3.1</Version>
  </PropertyGroup>
</Project>
`
	got, err := updater.UpdateXML(content, "Project.PropertyGroup.Version", "0.4.0")
	if err != nil {
		t.Fatalf("UpdateXML() error = %v", err)
	}
	if want := strings.ReplaceAll(content, "0.3.1", "0.4.0"); got != want {
		t.Fatalf("UpdateXML() =\n%s\nwant\n%s", got, want)
	}
}

func TestUpdateXMLRejectsAnElementItCannotBump(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		key     string
	}{
		{name: "rejects a path with no element", content: pomXML, key: "project.properties.revision"},
		{name: "rejects a path not starting at the root element", content: pomXML, key: "parent.version"},
		{name: "rejects an element holding child elements", content: pomXML, key: "project.parent"},
		{name: "rejects an empty-tagged element", content: "<project>\n  <version/>\n</project>\n", key: "project.version"},
		{name: "rejects an element holding a comment", content: "<project><version><!-- x -->1.0.0</version></project>", key: "project.version"},
		{name: "rejects content that is not XML", content: "<project><version>1.0.0</project>", key: "project.version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := updater.UpdateXML(tc.content, tc.key, "2.0.0"); err == nil {
				t.Fatal("UpdateXML() error = nil, want an error")
			}
		})
	}
}
