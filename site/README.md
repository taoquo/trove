# trove site

A local static product page. It is plain HTML, CSS and JavaScript — no build step, no framework,
and no third-party requests.

Preview it from the repository root:

```bash
python3 -m http.server 8080 -d site/public
```

Then open <http://localhost:8080/>.

It is served from Cloudflare Workers at <https://trove.taoquo.com/> — see
[Deploy](#deploy) below. Download buttons point at the stable asset names of the latest GitHub
release:

- `trove-macos-arm64.dmg`
- `trove-macos-x64.dmg`
- `trove-windows-x64-setup.exe`

`.github/workflows/release.yml` produces exactly these names from a `v<version>` tag, so the
buttons start working with the first release. Renaming an asset in the workflow means changing
`FILES` in `app.js` too.

## Screenshots

The six page images are captures of a seeded demo library, not of anyone's account. The fixture
(`testdata/site-fixture.json`) invents every author, handle and post; avatars render as initials,
so no real profile ever appears on the page. Rebuild them from bash with the pinned toolchain
sourced (`source .toolchain/env.sh`):

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

## Deploy

The page is a Cloudflare Worker with static assets, configured by `wrangler.jsonc`:

- worker `trove-site`, serving `./public` from the custom domain `trove.taoquo.com`
  (zone `taoquo.com`);
- `workers_dev` and preview URLs are off, so the custom domain is the only entry point;
- `public/_headers` sets CSP, `X-Frame-Options`, `Referrer-Policy`, HSTS and the image cache
  lifetime. Workers assets apply it in production and in `wrangler dev`, but not when the page is
  served by `python3 -m http.server`.

```bash
cd site
npm ci
npm run check        # wrangler deploy --dry-run: validates config and asset set
npm run dev          # local Workers runtime on http://127.0.0.1:8787
npm run deploy       # publishes to https://trove.taoquo.com/
```

`_headers` and `wrangler.jsonc` live outside `public/`, so they are never served as assets.
Deploy from `site/`; running wrangler from the repository root leaves its state in a root
`.wrangler/` directory, which is gitignored either way.

Rollback restores the previous Worker version, not any data (there is none):

```bash
npx wrangler deployments list
npx wrangler rollback <version-id>
```
