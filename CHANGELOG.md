<!-- markdownlint-disable MD024 -->
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## [0.2.0][] - 2026-07-27

### Added

* Skip publishing when `IMAGE` tag
  is older than the highest already-published tag,
  `--skip-tag-check` to bypass gate,
  `--version-format`/`--calver-format` to pick the comparison format.
* `--release-version` show a `Release` line in the generated header,
  linked to that version's tag page when the source forge is known.
  Defaults to `IMAGE`'s explicit tag.
* Automatic source link/image URL discovery for Woodpecker CI.

### Changed

* Replace `--base-url` with `--link-base-url`/`--image-base-url`,
  since links and images need different routes on every forge.
* CI-discovered link and image URLs are now pinned
  to a commit SHA instead of a branch.

[0.2.0]: https://github.com/WoozyMasta/rats/compare/v0.1.0...v0.2.0.

## [0.1.0][] - 2026-07-23

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/regdoc/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
