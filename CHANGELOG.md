# Changelog

`mygo build` takes the section of the version it builds as the release notes of the
update it signs, so the heading below must exist before a tag is pushed. Keep the
heading plain (`## 0.3.2`) or Keep a Changelog's (`## [0.3.2] - 2026-11-14`).

## 0.3.5

### Changed

- The window is set in the folio family: parchment pages and ivory cards in the light
  appearance, warm ink in the dark one, one cinnabar accent, and headings in the display
  serif. The download page and the app are now one palette.
- The title bar's brand lockup reads `trove` in both languages; the page's own name sits in
  the crumb beside it.
- Confirmations are the app's own card instead of the platform alert, so a destructive
  action is the danger ink on paper rather than the system's accent button.

### Fixed

- The scrollbar thumb is warm paper in both appearances instead of the toolkit's neutral
  grey.

## 0.3.4

### Fixed

- The window's close button now quits the app. It used to postpone the close and
  leave the window where it was, so the click looked like it did nothing.

### Changed

- A fresh install starts in English whatever the system language. The language is
  a setting inside the app, and "follow the system" is no longer one of its
  values.
- Settings no longer carries the auto-tagging signpost. Auto-tagging lives in the
  tag library's own tab, beside the tags it writes and the run that writes them.

## 0.3.3

### Changed

- Nothing behaves differently in this release. It exists so an installed 0.3.2
  has a newer version to find, which is how the update path is verified from
  the outside: the assets, the signed manifest and the delta are built exactly
  as a real release's are.

## 0.3.2

### Changed

- The source and the design documents no longer cite another app's stylesheet or
  commit. Every rule states its value and its reason in this repository's own words.

## 0.3.1

### Added

- `trove sync` and `trove export` run without a window, so a scheduler can keep the
  library current overnight.
- The app checks for updates itself and offers the new version in a native update
  window, with the release notes from this file.
- An empty library offers the way in — **Connect X**, or **Import…** for a backup —
  instead of describing posts that are not there yet.

### Changed

- Release builds are signed with a Developer ID and notarized when the repository's
  signing secrets are present, and every release asset carries a build attestation
  and a SHA-256 checksum.
- The README and the site lead with a screenshot and say who should not install
  trove.

## 0.3.0

- The first tagged release: sync, tag library, auto-tag suggestions, bundle export and
  import, English and Simplified Chinese.
