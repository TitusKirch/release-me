// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"testing"

	"github.com/ostendorf-cloud/release-me/internal/updater"
)

func TestUpdatePlainReplacesTheVersionAFileHolds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{name: "keeps the trailing newline", content: "1.4.0\n", want: "1.5.0-rc.1\n"},
		{name: "adds no newline to a file without one", content: "1.4.0", want: "1.5.0-rc.1"},
		{name: "keeps a Windows line ending", content: "1.4.0\r\n", want: "1.5.0-rc.1\r\n"},
		{name: "keeps blank lines around the version", content: "\n1.4.0\n\n", want: "\n1.5.0-rc.1\n\n"},
		{name: "replaces a pre-release version", content: "1.5.0-beta.2\n", want: "1.5.0-rc.1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := updater.UpdatePlain(tc.content, "1.5.0-rc.1")
			if err != nil {
				t.Fatalf("UpdatePlain() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("UpdatePlain() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUpdatePlainRejectsAFileThatIsNotAPlainVersionFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
	}{
		{name: "rejects an empty file", content: ""},
		{name: "rejects a file holding only whitespace", content: "\n\n"},
		{name: "rejects a file holding more than the version", content: "version 1.4.0\n"},
		{name: "rejects a file holding several lines", content: "1.4.0\nbuild 512\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := updater.UpdatePlain(tc.content, "2.0.0"); err == nil {
				t.Fatal("UpdatePlain() error = nil, want an error")
			}
		})
	}
}
