// Copyright 2026 Simon Ostendorf
// SPDX-License-Identifier: Apache-2.0

package history

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// sshRemote matches the SCP-like shorthand "git@host:owner/repo(.git)".
var sshRemote = regexp.MustCompile(`^git@[^:]+:([^/]+)/(.+?)(\.git)?$`)

// urlRemote matches "https://host/owner/repo(.git)" and
// "ssh://[user@]host/owner/repo(.git)".
var urlRemote = regexp.MustCompile(`^(?:https?|ssh)://[^/]+/([^/]+)/(.+?)(\.git)?$`)

// Remote parses the owner and repository name from the origin remote URL, in
// its SSH shorthand, ssh:// or HTTPS form.
func Remote(ctx context.Context, dir string) (owner, name string, err error) {
	out, err := run(ctx, dir, "remote", "get-url", "origin")
	if err != nil {
		return "", "", err
	}
	url := strings.TrimSpace(out)

	if m := sshRemote.FindStringSubmatch(url); m != nil {
		return m[1], m[2], nil
	}
	if m := urlRemote.FindStringSubmatch(url); m != nil {
		return m[1], m[2], nil
	}
	return "", "", fmt.Errorf("history: origin remote %q is neither an SSH nor an HTTPS GitHub URL", url)
}
