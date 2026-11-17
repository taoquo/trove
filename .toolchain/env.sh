#!/usr/bin/env bash
# Pinned Go toolchain for trove, the native macOS app.
#
# The toolchain, its module cache and its config live inside the workspace, so a
# checkout is reproducible and self-contained.
#
# The build cache does not. Go expires cache entries only by age -- five days since
# last use, scanned at most once a day, with no size cap -- so a week of editing and
# gate runs keeps every intermediate state of the largest package. Inside the
# workspace that was 13 GB of compiled history in a 14 GB checkout, and nothing was
# ever reclaimed. It is content-addressed and disposable, so it belongs in the
# platform cache directory: `go clean -cache` reclaims it, backup tools skip it.
#
# Usage:  source .toolchain/env.sh
#
# Pinned versions:
#   Go  1.27.4  darwin/arm64
#     module golang.org/toolchain@v0.0.1-go1.27.4.darwin-arm64
#     h1:87DnSk/ftpTKvVhs/5KkUqpRmq8SWxkmKzbTca6PBTk=
#     from the goproxy.cn mirror this network can reach, verified by the checksum database
#   MyGo  pinned as a Go tool in the root go.mod (never @latest)

TROVE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export GOROOT="$TROVE_ROOT/.toolchain/go"
export GOPATH="$TROVE_ROOT/.toolchain/gopath"
export GOMODCACHE="$GOPATH/pkg/mod"
export GOENV="$TROVE_ROOT/.toolchain/goenv"

# Caches live under the platform's cache directory, not in the checkout:
#   macOS  ~/Library/Caches/trove/go-build
#   other  ${XDG_CACHE_HOME:-$HOME/.cache}/trove/go-build
if [ -n "${XDG_CACHE_HOME:-}" ]; then
  TROVE_CACHE="$XDG_CACHE_HOME/trove"
elif [ "$(uname -s)" = "Darwin" ]; then
  TROVE_CACHE="$HOME/Library/Caches/trove"
else
  TROVE_CACHE="$HOME/.cache/trove"
fi
export GOCACHE="$TROVE_CACHE/go-build"

# Interrupted builds leave their go-buildNNN directory behind, so the build's temp
# directory is the system's: macOS sweeps it, and it never lands in the checkout.
export GOTMPDIR="${TMPDIR:-/tmp}"

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

mkdir -p "$GOPATH" "$GOCACHE"
export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"
