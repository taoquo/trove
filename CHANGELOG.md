# Changelog

`mygo build` takes the section of the version it builds as the release notes of the
update it signs, so the heading below must exist before a tag is pushed. Keep the
heading plain (`## 0.3.2`) or Keep a Changelog's (`## [0.3.2] - 2026-11-14`).

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
