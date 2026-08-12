# regdoc

`regdoc` combines Markdown documentation from a project into one document,
normalizes links and structure,
and publishes the result to the container registry alongside the image.

It lets you reuse README files, changelogs, links,
and metadata from the source repository
and keep them available to users in Docker Hub, Quay, or Harbor.

## Installation

Download a binary for your platform from the
[releases page](https://github.com/WoozyMasta/regdoc/releases),
or install it with Go:

```sh
go install github.com/woozymasta/regdoc/cmd/regdoc@latest
```

## Container images

```sh
ghcr.io/woozymasta/regdoc:latest
docker.io/woozymasta/regdoc:latest
```

## Publish documentation

By default, `regdoc` finds `README.md` and `CHANGELOG.md` in the work directory,
merges them, and publishes the result.
The provider is determined from the image hostname.

Docker Hub uses a username with a password or personal access token:

```sh
REGDOC_USERNAME="example" REGDOC_TOKEN="$DOCKERHUB_TOKEN" \
  regdoc example/service
```

Publishing a Docker Hub description requires a PAT with `Delete` permission;
image push permission alone is not sufficient.

For Quay, provide an OAuth token:

```sh
REGDOC_TOKEN="$QUAY_TOKEN" \
  regdoc quay.io/example/service
```

Harbor requires a username and password:

```sh
REGDOC_USERNAME="example" REGDOC_PASSWORD="$HARBOR_PASSWORD" \
  regdoc registry.example/team/service
```

Docker Hub and Harbor also use Docker-compatible credentials
when explicit credentials are absent, including repository-scoped entries.
Select `--provider` explicitly when the registry hostname is ambiguous.

Do not pass secrets as command-line arguments.
Use environment variables, `--password-stdin`, or `--token-stdin`.

## Skipping stale publishes

The registry description is a single field per repository,
shared across every tag.
When parallel release lines (e.g. a `3.x` and a `2.x` branch)
build independently, CI can publish out of version order
and leave the description showing older docs than what was already there.

Add an explicit tag to `IMAGE` to guard against this:

```sh
regdoc quay.io/example/service:3.1.1
```

When `IMAGE` carries an explicit tag,
`regdoc` lists the tags already published in the repository
using the available registry credentials, with no extra permissions needed.
It finds the highest existing stable release and skips publishing
(a no-op, not an error) if the tag being published is older.
Equal versions still publish, so re-running the same tag's CI job is safe.

A tag-order check runs immediately before publishing to minimize race window.
It is a best-effort guard, not an atomic guarantee;
serialize publishing jobs per repository in CI when strict ordering is required.

A tag that isn't a valid stable SemVer release
(`:latest`, `:nightly`, `:sha-abcdef`, prereleases)
is treated the same as no tag at all: publish unconditionally.
Without an explicit tag, behavior is unchanged from today.

`--skip-tag-check` bypasses the gate and publishes regardless of tag order.

`--version-format` selects how tags are compared:
`semver` (the default, shown above),
`calver`, `numeric`, `lexical`, `pep440`, `debian`, or `rpm`.
`--calver-format` picks the calendar layout when `--version-format=calver`:

```sh
regdoc --version-format=calver --calver-format=ym-dot \
  quay.io/example/service:2026.07
```

A tag that does not match the configured format
is treated the same as no tag at all: publish unconditionally.

## Release version in the header

The generated header shows a `Release` line, when a version is known.
`--release-version` sets it explicitly;
without it, `regdoc` falls back to `IMAGE`'s explicit tag
(the same tag the stale-publish gate above reads),
shown as-is with no validation:

```sh
regdoc --release-version=3.1.1 quay.io/example/service
```

When the source forge and project URL are known
(see "Documents, links, and images" below),
the release is linked to that version's tag page in the forge -
not a Releases page, since a Release object is optional and a git tag is not.
Without a known forge or without any version at all,
the line is either plain text or omitted entirely.

## Typical CI invocation

This example selects the provider explicitly, supplies project metadata,
and uses a known corporate registry limit.
Files are published in the listed order.

```sh
REGDOC_TOKEN="$QUAY_TOKEN" \
  regdoc \
  --provider=quay \
  --title="$CI_PROJECT_TITLE" \
  --source-url="$CI_PROJECT_URL" \
  --short-description="$CI_PROJECT_DESCRIPTION" \
  --doc-body-limit=65536 \
  --cut-heading-level=2 \
  --cut-retries=3 \
  quay.io/example/service \
  README.md docs/*.md CHANGELOG.md
```

`docs/*.md` is expanded by `regdoc` itself, including in PowerShell.
Matches are added in lexical order.

> [!NOTE]
> Supplying an explicit file list disables automatic selection
> of `README.md` and `CHANGELOG.md`.

## CI examples

GitHub Actions runs the job in the image:

```yaml
jobs:
  publish-documentation:
    runs-on: ubuntu-latest
    container: ghcr.io/woozymasta/regdoc:latest
    steps:
      - uses: actions/checkout@v6
      - env:
          REGDOC_TOKEN: ${{ secrets.QUAY_TOKEN }}
        run: regdoc quay.io/example/service README.md 'docs/*.md' CHANGELOG.md
```

GitLab CI requires an empty entrypoint so the runner can start its shell:

```yaml
publish-documentation:
  image:
    name: ghcr.io/woozymasta/regdoc:latest
    entrypoint: [""]
  script:
    - regdoc quay.io/example/service README.md 'docs/*.md' CHANGELOG.md
```

Set `REGDOC_TOKEN`, `REGDOC_USERNAME`, and `REGDOC_PASSWORD`
as protected CI variables when required by the target registry.

## Local preview

`--output` disables publishing:
no registry detection, credential lookup, or network requests occur.

Write Markdown to stdout:

```sh
regdoc --output - quay.io/example/service
```

Save HTML:

```sh
regdoc --format html --output description.html quay.io/example/service
```

HTML is useful for registries with limited Markdown support,
such as legacy Quay UI versions that do not render tables.

## Documents, links, and images

Without explicit files, `regdoc` finds `README.md` and `CHANGELOG.md`
under `--root` and adds them in that order.
An explicit list disables autodiscovery:

```sh
regdoc quay.io/example/service README.md docs/*.md CHANGELOG.md
```

`--link-base-url` and `--image-base-url` turn relative file
and image links into source repository links.
A Markdown link needs a different route than a Markdown image
on every supported forge, so the two are separate flags:
set both or neither, setting only one is a configuration error.

When both are absent, `regdoc` determines them from CI metadata,
trying providers in this order and stopping at the first one
whose CI sentinel matches -
an incomplete profile leaves relative links untouched
rather than falling back to another provider or a default branch:

* GitLab CI (`GITLAB_CI=true`):
  `CI_PROJECT_URL` and `CI_COMMIT_SHA`;
  links use `-/blob/<sha>/`, images use `-/raw/<sha>/`.
* Bitbucket Pipelines (`BITBUCKET_BUILD_NUMBER` set):
  `BITBUCKET_GIT_HTTP_ORIGIN` and `BITBUCKET_COMMIT`;
  links use `src/<sha>/`, images use `raw/<sha>/`.
* Forgejo Actions (`FORGEJO_ACTIONS=true`, Forgejo Runner v7.0.0+ only):
  `FORGEJO_SERVER_URL`, `FORGEJO_REPOSITORY`, `FORGEJO_SHA`,
  falling back per-field to the `GITHUB_*`-compatible aliases Forgejo also sets;
  links use `src/commit/<sha>/`, images use `raw/commit/<sha>/`.
  Older runners never set `FORGEJO_ACTIONS`
  and are picked up by the GitHub Actions profile below instead -
  there is no reliable way to tell them apart.
* Gitea Actions (`GITEA_ACTIONS=true`):
  `GITHUB_SERVER_URL`, `GITHUB_REPOSITORY`, `GITHUB_SHA`
  (Gitea has no native variables of its own for these);
  same route shape as Forgejo.
* GitHub Actions (`GITHUB_ACTIONS=true`):
  `GITHUB_SERVER_URL`, `GITHUB_REPOSITORY`, `GITHUB_SHA`;
  links use `blob/<sha>/`, images use `raw/<sha>/`.
* Woodpecker CI (`CI_FORGE_TYPE` set):
  `CI_REPO_URL` and `CI_COMMIT_SHA`, with the route shape
  selected by the reported forge type
  (`github`, `gitlab`, `gitea`, `forgejo`, `bitbucket`, `bitbucket_dc`).

All discovered links are pinned to a full commit SHA, never a branch name.
An unsupported forge, a proxy, or a third-party CI system with no
forge-identifying variable (Drone, CircleCI, Jenkins, and similar)
can opt in explicitly:

```sh
regdoc \
  --link-base-url="https://git.example/team/project/blob/0123456789abcdef/" \
  --image-base-url="https://git.example/team/project/raw/0123456789abcdef/" \
  quay.io/example/service
```

These bases include the forge route and revision.
The source project URL shown in the header
is configured separately with `--source-url`.

The same override also covers a deliberate choice
to point documentation at a mutable ref instead of the pinned commit,
e.g. always linking to a project's default branch:

```sh
regdoc \
  --link-base-url="$CI_PROJECT_URL/-/blob/$CI_DEFAULT_BRANCH/" \
  --image-base-url="$CI_PROJECT_URL/-/raw/$CI_DEFAULT_BRANCH/" \
  quay.io/example/service
```

The same CI data populates the generated header
when `--title`, `--source-name`, and `--source-url` are not set:

* GitLab CI:
  `CI_PROJECT_TITLE`, `CI_PROJECT_PATH`, `CI_PROJECT_URL`.
* Bitbucket Pipelines:
  `BITBUCKET_REPO_SLUG`, `BITBUCKET_REPO_FULL_NAME`,
  and `BITBUCKET_GIT_HTTP_ORIGIN`.
* Forgejo Actions, Gitea Actions, GitHub Actions:
  `GITHUB_SERVER_URL` and `GITHUB_REPOSITORY`
  (or the native `FORGEJO_*` variables when Forgejo sets them);
  the title is the final repository path component.
* Woodpecker CI:
  `CI_REPO_URL`; the title is the final path component.

Explicit `--title`, `--source-name`,
and `--source-url` always win over discovered metadata.

For private projects, local images can be embedded as base64 in the document:

```sh
regdoc --embed-images quay.io/example/service
```

Only files inside `--root` are embedded; external URLs remain unchanged.
Data URIs increase description size, so account for selected registry limit.

> [!IMPORTANT]
> In addition to their size, not every registry can render base64 images in
> document links. Verify the result with the target registry.

## Reference

The complete option list and default values are available in [CLI.md](CLI.md)
and in the command itself:

```sh
regdoc --help
regdoc docs md -
```
