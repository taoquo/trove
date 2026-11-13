# trove site

A local static product page. It is plain HTML, CSS and JavaScript — no build step, no framework,
and no third-party requests.

Preview it from the repository root:

```bash
python3 -m http.server 8080 -d site/public
```

Then open <http://localhost:8080/>.

The page is intentionally not deployed from this repository yet. When a host and domain are
chosen, the files under `site/public/` are ready to upload as-is. Download buttons point at the
stable asset names of the latest GitHub release:

- `trove-macos-arm64.dmg`
- `trove-macos-x64.dmg`
- `trove-windows-x64-setup.exe`

Until a release with those names exists, the buttons resolve to GitHub's 404 page. That is
expected for a local preview.

## Screenshots

The six page images are captures of a seeded demo library, not of anyone's account. The fixture
(`testdata/site-fixture.json`) invents every author, handle and post; avatars render as initials,
so no real profile ever appears on the page. Rebuild them with:

From bash, with the pinned toolchain sourced (`source .toolchain/env.sh`):

```bash
go run ./cmd/seed -fixture testdata/site-fixture.json -locale en -data-dir /tmp/trove-site-en
go run ./cmd/seed -fixture testdata/site-fixture.json -locale zh-CN -data-dir /tmp/trove-site-zh

TROVE_DATA_DIR=/tmp/trove-site-en TROVE_HEADLESS=1 TROVE_CAPTURE_WIDTH=1297 \
  TROVE_CAPTURE_STATE=workspace TROVE_CAPTURE=/tmp/shot-en-workspace.png \
  build/darwin-arm64/trove.app/Contents/MacOS/trove
```

Capture `workspace`, `detail` and `tags` for both locales, then convert each with
`cwebp -q 82 -resize 1600 0` into `site/public/img/`. The page keeps a second sentence beside the
figures saying the samples are invented; keep that promise if the fixture changes.
