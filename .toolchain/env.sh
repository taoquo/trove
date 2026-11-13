#!/usr/bin/env bash
# Pinned Go toolchain for trove, the native macOS app.
#
# Everything lives inside the workspace so the toolchain and module cache are
# reproducible, sandbox-local, and never read or written outside it.
#
# Usage:  source .toolchain/env.sh
#
# Pinned versions:
#   Go  1.27.2  darwin/arm64
#     sha256 76812b213b1b2302c978d28fa52fa92d541704b9e7d9d5db8002c50e4018c4c5
#     source https://dl.google.com/go/go1.27.2.darwin-arm64.tar.gz
#   MyGo  pinned as a Go tool in the root go.mod (never @latest)

TROVE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export GOROOT="$TROVE_ROOT/.toolchain/go"
export GOPATH="$TROVE_ROOT/.toolchain/gopath"
export GOMODCACHE="$GOPATH/pkg/mod"
export GOCACHE="$TROVE_ROOT/.toolchain/gocache"
export GOTMPDIR="$TROVE_ROOT/.toolchain/tmp"
export GOENV="$TROVE_ROOT/.toolchain/goenv"

# Forbid implicit toolchain switching: a go.mod asking for a newer Go must fail
# loudly rather than silently downloading one. Keeps builds pinned.
export GOTOOLCHAIN=local

# This network cannot reach proxy.golang.org or github.com directly (both time
# out), so module resolution must go through a mirror. goproxy.cn also mirrors
# the checksum database, which keeps GOSUMDB verification on.
export GOPROXY="https://goproxy.cn,direct"
export GOSUMDB="sum.golang.org"
export GOFLAGS="-mod=mod"

# No cgo anywhere: the app must build with CGO_ENABLED=0.
export CGO_ENABLED=0

mkdir -p "$GOPATH" "$GOCACHE" "$GOTMPDIR"
export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"
