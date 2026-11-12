# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

Two implementations of the same product — trove, a local-first manager for X/Twitter bookmarks — live here:

- **Chrome MV3 extension** (`src/`, `tests/`, repo root): React 19 + TypeScript, IndexedDB, esbuild. This is the **reference implementation**: its UI, copy, storage formats, sync protocol and auto-tag behaviour are the source of truth that the native app replicates.
- **Native macOS app** (`apps/macos/`): Go, rendering its own UI with [MyGo](https://github.com/egoist/mygo) — no webview, no HTML, no JavaScript. It has its own SQLite database and imports the extension's backup format.

The extension and `apps/macos` are independent builds; `apps/macos` has its own `go.mod`, `.gitignore`, and pinned toolchain.

`docs/`, `specs/` and `plans/` are **gitignored but present locally**. They are the project's design record and are authoritative — read the relevant one before large changes. Most important: `docs/macos-architecture.md`, `docs/test-and-acceptance.md`, `docs/shell-contract.md`, `docs/ui-spec.md`, `docs/ui-surface-index.md`, `docs/native-ui-replication-plan.md`, `docs/ui-implementation-plan.md`, `docs/sqlite-schema.md`, `docs/domain-migration-map.md`, `docs/x-sync-protocol.md`, `docs/ai-autotagging.md`, `docs/security-and-privacy.md`, `docs/release-runbook.md`.

## Commands

### Extension (run from repository root)

```bash
npm install
npm run build      # esbuild + Tailwind -> build/chrome-mv3
npm run dev        # same pipeline in watch mode
npm test           # node:test over tests/**/*.test.ts(x)
npm run typecheck
npm run lint
```

Run one test file or filter by name (do not pass the file to `npm test`, which appends to the glob):

```bash
node --import tsx --test tests/options/OptionsApp.test.tsx
node --import tsx --test --test-name-pattern "accepts a suggestion" tests/options/suggestions.test.tsx
```

Manual check: `npm run build`, then in Chrome → `chrome://extensions` → Developer mode → Load unpacked → `build/chrome-mv3`. CI (`.github/workflows/ci.yml`) runs typecheck, lint and test on Node 24.

### Native macOS app (Apple silicon)

The pinned toolchain under `.toolchain/` is mandatory — it fixes Go 1.27.2, forbids implicit toolchain switching (`GOTOOLCHAIN=local`), enforces `CGO_ENABLED=0`, and routes module downloads through a mirror (the direct Go proxy is unreachable here). `env.sh` locates itself via `BASH_SOURCE`, so **source it from bash, not zsh** (under zsh `go` is simply not found):

```bash
bash -c 'source .toolchain/env.sh && cd apps/macos && go test ./...'
```

From an interactive bash shell, or inside `bash -c`:

```bash
source .toolchain/env.sh
cd apps/macos

go test ./...
go test ./internal/domain -run TestCanonicalTagKeyMatchesTypeScript -v   # one package / one test
go test -race ./...
gofmt -l .                        # must print nothing
go vet ./...
go tool mygo vet .                # MyGo-specific lifetime/goroutine checks
CGO_ENABLED=0 go build -o /dev/null .
go tool mygo build -skip-dmg      # -> build/darwin-arm64/trove.app
```

The full quality gate runs every step even when one fails and is the canonical pre-commit check. It needs the exported toolchain environment in the same shell, so run it from the repo root through bash:

```bash
bash -c 'source .toolchain/env.sh && bash apps/macos/tools/gate.sh'
```

It runs the source checkers, gofmt/vet/build, `go test` and `go test -race`, `mygo build`, a launch/single-instance/quit smoke test (`tools/smoke.sh`), and two pixel checks on a live capture (`tools/panes.sh`, `tools/geometry.sh`). The pixel checks read captures under `docs/screenshots`, which are local-only.

Other capture tooling: `tools/baselines.sh` and `tools/criteria.sh` regenerate screenshots from the built app. The app understands `TROVE_CAPTURE`, `TROVE_CAPTURE_STATE`, `TROVE_CAPTURE_WIDTH`, `TROVE_CAPTURE_HEIGHT`, `TROVE_CAPTURE_DELAY`, `TROVE_CAPTURE_LOCALE`, `TROVE_DATA_DIR`, `TROVE_HEADLESS` and `TROVE_SMOKE`.

## Architecture: Chrome extension

**Message-passing, one writer.** The background service worker (`src/background.ts` → `src/background/index.ts`) owns all sync, auto-tag and LLM work plus every IndexedDB write. Popup, options page and content script never write the database directly; they send typed messages defined in `src/lib/runtime/messages.ts` and receive progress broadcasts. Options is the full manager (`src/options/OptionsApp.tsx`); the popup is a small status/sync surface (`src/popup/`).

**Build.** `scripts/build.mjs` bundles four entries with esbuild (`options` ESM + code splitting, `background` ESM, `popup` IIFE, `content` IIFE) and runs Tailwind. It also **generates `manifest.json`, `options.html` and `popup.html`** — there is no checked-in manifest; edit permissions, content-script matches and web-accessible resources in that script.

**Content script.** `src/content/` runs on `x.com/home` and the bookmarks timeline, parses tweet DOM (`tweet-dom.ts`), and provides inline tagging (`site-controller.ts`, `tag-popover.ts`, `site-client.ts`). It must never see the LLM API key: `src/background.ts` calls `restrictStorageToTrustedContexts()` for exactly this reason, and backup/export code guards the key too.

**Storage.** IndexedDB `x-bookmark-manager`, `DB_VERSION = 9` in `src/lib/storage/db.ts` (stores: bookmarks, tags, bookmark-tags, sync-runs, raw-payloads, tag-suggestions, llm-analysis; the upgrade path deletes legacy knowledge-card/folder stores). Settings live in `chrome.storage.local` under `settings`, schema 7, normalized on read in `src/lib/storage/settings.ts`. Provider API keys live in `secrets`, keyed per provider.

**Domain libraries.**
- `src/lib/x/` — GraphQL bookmarks client, cookie/CSRF auth, pagination, parsing, error kinds.
- `src/lib/autotag/` — pipeline lifecycle (waves, budgets, cancellation, stale-run takeover, idempotent analysis ledger), heuristics, suggestion actions, vocabulary review.
- `src/lib/llm/` — provider catalog as pure data (`providers.ts`), api-id → adapter registry (`openai-chat`, `anthropic-messages`, `google-generative-ai`, `azure-openai-chat`), shared retry/backoff transport, capability resolution, error classification. Adapters are only imported by the background worker.
- `src/lib/storage/`, `src/lib/search/`, `src/lib/tags/`, `src/lib/workspace/`, `src/lib/export/`, `src/lib/graph/`.

**Tests.** `node:test` + `tsx`; imports keep the `.ts` extension. Storage tests import `fake-indexeddb/auto`. UI tests use `tests/helpers/render.tsx` (one shared JSDOM per file, plus React act/settle helpers) and `tests/helpers/runtime.ts` (a fake `chrome` whose `sendMessage` drives the real background message handler).

## Architecture: native macOS app

**Before changing `apps/macos`, read `apps/macos/AGENTS.md`** — the one-page contract for how work is
chosen, verified and recorded (board, lanes, DoR/DoD, evidence). The current slice lives in
`docs/loop/board.md`; the visual grammar in `docs/loop/grammar.md`.

**Layered Go, composition root in `main.go`:**

```
domain   pure rules, no dependencies
ports    interfaces the usecases need (repositories, bookmark source, clock)
usecase  workspace, tagging, syncer, suggestions, settings
adapter  storage/sqlite, xsource, llm, credentials, media, bundle, importexport
ui       MyGo workspace; talks only to usecases
```

`main.go` is the only place that chooses concrete implementations. Storage is `modernc.org/sqlite` (pure Go, migrations in `internal/storage/sqlite/migrations.go`). X credentials are the user's `auth_token` + `ct0` cookies, pasted by the user and stored in the macOS Keychain via the `/usr/bin/security` CLI (`internal/credentials/keychain`) — the app intentionally does not read browser cookie stores. LLM providers mirror the extension's catalog (`internal/llm/providers.go`, adapters in `internal/llm/adapters.go`). Import/export reads the extension's `exportVersion: 2` backup format (`internal/importexport/legacy.go`, `dto.go`).

**MyGo rules that shape the UI code.** `internal/ui/State` holds data only; a view may be rebuilt several times per event, so `View` must be side-effect free and must not do I/O or store elements/handles. Side effects run through event handlers or guarded jobs; slow work publishes via `Window.Update`. Keyboard commands use `c.OnShortcut` / `handle.OnShortcut`. UI tests use `ui.NewTester` (no window) and are the normal way to reproduce layout/state defects. All user-visible strings go through `internal/i18n`; `AssertComplete()` runs at startup because a missing key is a build defect. Copy and translations mirror the extension's `src/options/i18n/copy.ts`, and the native catalogs carry both locales.

**"Replication" means the extension is the spec.** For UI decisions the authority order is `src/` CSS + TSX → target screenshots → guess (`docs/native-ui-replication-plan.md` §0), and precise values must cite a `src/` file:line. New or renamed drawing methods must be added to `docs/ui-surface-index.md` in the same change.

**Source checkers are part of the build.** `tools/check-text.sh` fails on hardcoded user-visible strings; `tools/check-wiring.sh` fails when an interface method becomes unreachable or a field is never read (both use explicit allowlists with reasons — do not grow them casually); `tools/selfcheck.sh` proves each checker still catches its defect; `tools/panes.sh` and `tools/geometry.sh` assert the shell's real geometry against a live capture. `docs/shell-contract.md` records each source-vs-native structural decision.

## Conventions and gotchas

- Never launch the native app with `open` in a check: LaunchServices reuses a running instance, which has produced false passes. Launch the bundle binary directly (as `tools/smoke.sh` does) and refuse to run when an instance is already alive.
- The extension build output (`build/chrome-mv3`) and the app bundle (`apps/macos/build/`) are ignored; never edit generated output.
- Go module downloads must go through the mirror configured by `.toolchain/env.sh`; pin tools in `go.mod` (`go tool mygo ...`), never `@latest`.
- `.toolchain/env.sh` is tracked while the rest of `.toolchain/` is ignored — it records the pinned checksum the build depends on.
- The extension keeps the LLM key out of content scripts and backups deliberately; preserve that boundary when touching storage, messaging or backup code.
- Commit messages in this repository are short English sentences, conventionally prefixed `feat:`, `fix:`, `test:`, `refactor:`, `perf:`.
