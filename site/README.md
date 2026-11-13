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
