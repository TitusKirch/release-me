#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Builds release-me from the checkout at GITHUB_ACTION_PATH/.., which is
# this repository's own working copy only for "uses: ./action"; a remote
# "uses: ostendorf-cloud/release-me/action@ref" builds the downloaded
# action's copy instead, not the caller's checkout.
set -euo pipefail

cd "${GITHUB_ACTION_PATH}/.."
go build -ldflags "-X main.Version=${VERSION}" -o "${RUNNER_TEMP}/release-me" ./cmd/release-me
