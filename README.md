# trove

[中文说明](./README.zh-CN.md)

trove is a native macOS and Windows app for people who save too much on X and still want to find the
right post later. It syncs X bookmarks into a local SQLite database and gives them a proper workspace:
browse, search, filter, tag, and review — without a hosted backend.

The app renders its own interface with [MyGo](https://github.com/egoist/mygo): there is no webview,
no HTML and no JavaScript in the product.

## What it does

- Sync bookmarks from X using your own `auth_token` + `ct0` cookies (stored in the macOS Keychain or Windows Credential Manager).
- Keep every bookmark locally in SQLite.
- Browse, search and filter by tag, author, media and saved view.
- Organize bookmarks with a tag library, including aliases and parent tags.
- Suggest tags with heuristics or an LLM provider; review suggestions before applying them.
- Show a tag-relation graph and batch actions over a selection.
- Import legacy `exportVersion: 2` JSON backups and export a native bundle.
- Stay local-first and bilingual (English / Simplified Chinese).

## Requirements

- macOS 12 or later (Apple silicon or Intel), or Windows 10/11 x64.
- macOS release builds need a Developer ID for signing and notarization; Windows builds use NSIS for
the installer.
- The pinned Go toolchain under `.toolchain/` (Go 1.27.2). It is mandatory: it fixes the version,
  forbids implicit toolchain switching and routes module downloads through a reachable mirror.
- Python 3 with Pillow for the screenshot-based checks.

## Build and run

Source the toolchain from bash (not zsh — `env.sh` locates itself through `BASH_SOURCE`):

```bash
bash -c 'source .toolchain/env.sh && go tool mygo build -skip-dmg'
```

The bundle lands at `build/darwin-arm64/trove.app`. Launch the binary directly rather than using
`open`, so a running instance cannot be reused by LaunchServices:

```bash
build/darwin-arm64/trove.app/Contents/MacOS/trove
```

To build both Mac architectures and the Windows portable executable, pass a platform list to MyGo:

```bash
go tool mygo build -platform darwin/arm64,darwin/amd64
go tool mygo build -platform windows/amd64
```

On Windows, run the built `trove.exe` or the NSIS `Setup.exe` from `build/windows-amd64`.

## Releases

Pushing a tag `v<version>` (matching `version` in `mygo.json`) runs
`.github/workflows/release.yml`, which builds both Mac architectures and the Windows installer and
attaches them to a GitHub release under the names the site links to:

| Asset | Platform |
|---|---|
| `trove-macos-arm64.dmg` | macOS, Apple silicon |
| `trove-macos-x64.dmg` | macOS, Intel |
| `trove-windows-x64-setup.exe` | Windows, x64 |

The builds are ad-hoc signed development builds, not notarized. Renaming an asset also means
changing `FILES` in `site/public/app.js`.

## Test and verify

```bash
bash -c 'source .toolchain/env.sh && go test ./...'
```

The full quality gate is the canonical pre-commit check. It runs the source checkers,
`gofmt`/`go vet`/build, the test suite with and without the race detector, a MyGo build, a
launch/single-instance/quit smoke test, and two pixel checks against a live capture:

```bash
bash -c 'source .toolchain/env.sh && bash tools/gate.sh'
```

The pixel checks read captures under `docs/screenshots`, which are local-only.

## Repository layout

```text
cmd/trove             Composition root, window, menu and capture entry point
internal/domain       Pure rules (no dependencies)
internal/ports        Interfaces the use cases need
internal/usecase      Workspace, tagging, sync, suggestions, settings
internal/storage      SQLite repositories and migrations
internal/xsource      X GraphQL client and query catalog
internal/llm          Model providers, adapters and errors
internal/autotag      Automatic tagging pipeline
internal/credentials  Keychain / Credential Manager and browser cookie reader
internal/media        Media download and disk cache
internal/bundle       Native .trovebundle format
internal/importexport Legacy JSON import and export
internal/ui           MyGo workspace and all drawing
cmd/mediaprobe        Development media diagnostic
cmd/seed              Seeds a fictional library for the site's screenshots
site/                 Local static product page (not deployed automatically)
tools/                Source checkers, capture tooling, quality gate
testdata/             Frozen equivalence fixtures
docs/                 Architecture, data, testing and design
```

Screenshots under `docs/screenshots/` are captured locally and are not versioned.

## Data and privacy

Bookmarks, tags and settings live in a local SQLite file. X credentials and LLM API keys are stored
in the macOS Keychain or Windows Credential Manager. No account, telemetry or hosted service is
involved.
