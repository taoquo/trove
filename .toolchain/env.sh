#!/usr/bin/env bash
# Pinned Go toolchain for the x-master macOS app (apps/macos).
#
# Everything lives inside the workspace so the toolchain and module cache are
# reproducible, sandbox-local, and never read or written outside it.
#
# Usage:  source .toolchain/env.sh
#
# Pinned versions (see docs/macos-architecture.md §1.1):
#   Go  1.27.2  darwin/arm64
#     sha256 76812b213b1b2302c978d28fa52fa92d541704b9e7d9d5db8002c50e4018c4c5
#     source https://dl.google.com/go/go1.27.2.darwin-arm64.tar.gz
#   MyGo  pinned as a Go tool in apps/macos/go.mod (never @latest)

XM_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export GOROOT="$XM_ROOT/.toolchain/go"
export GOPATH="$XM_ROOT/.toolchain/gopath"
export GOMODCACHE="$GOPATH/pkg/mod"
export GOCACHE="$XM_ROOT/.toolchain/gocache"
export GOTMPDIR="$XM_ROOT/.toolchain/tmp"
export GOENV="$XM_ROOT/.toolchain/goenv"

# Forbid implicit toolchain switching: a go.mod asking for a newer Go must fail
# loudly rather than silently downloading one. Keeps builds pinned.
export GOTOOLCHAIN=local

# This network cannot reach proxy.golang.org or github.com directly (both time
# out), so module resolution must go through a mirror. goproxy.cn also mirrors
# the checksum database, which keeps GOSUMDB verification on.
export GOPROXY="https://goproxy.cn,direct"
export GOSUMDB="sum.golang.org"
export GOFLAGS="-mod=mod"

# No cgo anywhere: the app must build with CGO_ENABLED=0 (docs/macos-architecture.md §1.3).
# NOTE: the Keychain decision D7 may revise this — see §1.1.2.
export CGO_ENABLED=0

mkdir -p "$GOPATH" "$GOCACHE" "$GOTMPDIR"
export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"
