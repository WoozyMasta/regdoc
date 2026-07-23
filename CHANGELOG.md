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

## Unreleased

### Added

* Automatic source link/image URL discovery for Woodpecker CI via
  `CI_FORGE_TYPE`, `CI_REPO_URL`, and `CI_COMMIT_SHA`.

### Changed

* Replace `--base-url`/`-b`/`REGDOC_BASE_URL` with `--link-base-url`
  and `--image-base-url` (`REGDOC_LINK_BASE_URL`/`REGDOC_IMAGE_BASE_URL`),
  an all-or-nothing manual override: a single base URL cannot correctly
  describe both Markdown links and images on any supported forge.
* CI-discovered link and image URLs are now always pinned to a full commit SHA
  instead of a branch name, across GitLab CI, GitHub Actions, Gitea Actions,
  Forgejo Actions, Bitbucket Pipelines, and Woodpecker CI.

## [0.1.0][] - 2026-07-23

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/regdoc/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
