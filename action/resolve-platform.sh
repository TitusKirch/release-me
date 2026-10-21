#!/usr/bin/env bash
# Copyright 2026 Simon Ostendorf
# SPDX-License-Identifier: Apache-2.0
# Maps the runner's os and arch to the goos and goarch a release archive is
# named with. RUNNER_OS and RUNNER_ARCH are set by the runner already.
set -euo pipefail

case "${RUNNER_OS}" in
  Linux) os=linux ;;
  macOS) os=darwin ;;
  *)
    echo "::error::unsupported runner os ${RUNNER_OS}" >&2
    exit 1
    ;;
esac

case "${RUNNER_ARCH}" in
  X64) arch=amd64 ;;
  ARM64) arch=arm64 ;;
  *)
    echo "::error::unsupported runner arch ${RUNNER_ARCH}" >&2
    exit 1
    ;;
esac

{
  echo "os=${os}"
  echo "arch=${arch}"
} >>"${GITHUB_OUTPUT}"
