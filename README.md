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

Docker Hub and Harbor also use the Docker credential store
and credential helpers when explicit credentials are absent.
Select `--provider` explicitly when the registry hostname is ambiguous.

Do not pass secrets as command-line arguments.
Use environment variables, `--password-stdin`, or `--token-stdin`.

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

`--base-url` turns relative file and image links into source repository links.
When flag is absent, `regdoc` determines the URL from CI metadata in this order;
an explicit `--base-url` always takes precedence:

* GitLab CI:
  `CI_PROJECT_URL` and `CI_DEFAULT_BRANCH`;
  links use raw files from the default branch.
* GitHub Actions, Gitea/Forgejo Actions:
  `GITHUB_SERVER_URL`, `GITHUB_REPOSITORY`, and `GITHUB_SHA`;
  links are pinned to a commit.
* Bitbucket Pipelines:
 `BITBUCKET_GIT_HTTP_ORIGIN` and `BITBUCKET_COMMIT`;
  links are also pinned to a commit.

When those variables are absent or the CI environment is unsupported,
relative links remain relative.

The same CI data populates the generated header
when `--title`, `--source-name`, and `--source-url` are not set:

* GitLab CI:
  `CI_PROJECT_TITLE`, `CI_PROJECT_PATH`, `CI_PROJECT_URL`.
* GitHub Actions, Gitea/Forgejo Actions:
  `GITHUB_SERVER_URL` and `GITHUB_REPOSITORY`;
  the title is the final repository path component.
* Bitbucket Pipelines:
  `BITBUCKET_REPO_SLUG`, `BITBUCKET_REPO_FULL_NAME`,
  and `BITBUCKET_GIT_HTTP_ORIGIN`.

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
