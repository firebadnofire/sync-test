# gh-sync

`gh-sync` is a reusable Forgejo CI action that publishes files produced by a
Forgejo job as assets on a GitHub release. It talks directly to the GitHub REST
API; neither the GitHub CLI nor a Go installation is required in the consuming
job.

The Forgejo repository is the source of truth. `gh-sync` only creates or reuses
a GitHub release and uploads its requested assets. It does **not** mirror Git
history, branches, refs, issues, pull requests, or repository settings.

## Use from Forgejo Actions

Store a GitHub personal access token in a Forgejo Actions secret. A fine-grained
PAT needs **Contents: read and write** access to the target repository. For a
classic PAT, use an appropriate `public_repo` or `repo` scope for the target.

Call the action by its fully qualified Forgejo URL:

```yaml
name: Publish release to GitHub

on:
  push:
    tags:
      - 'v*'

jobs:
  publish:
    runs-on: ubuntu-22.04
    steps:
      - name: Check out and build the project
        # Use checkout/build steps suitable for your Forgejo runners.
        run: |
          ./build-release.sh

      - name: Publish the release on GitHub
        uses: https://pubcode.archuser.org/actions/gh-sync@v1
        with:
          repository: firebadnofire/example
          token: ${{ secrets.GH_KEY }}
          is_fine: false
          tag: ${{ forgejo.ref_name }}
          name: Example ${{ forgejo.ref_name }}
          body: |
            Release artifacts built by Forgejo CI.
          files: |
            dist/*.tar.gz
            dist/*.zip
            dist/SHA256SUMS
          overwrite: true
```

Replace `firebadnofire/example` with the destination GitHub repository. The
example uses a classic PAT named `GH_KEY`, so `is_fine` is `false`. For a
fine-grained PAT, set `is_fine: true`.

`@v1` selects the published major version of this action. A consumer that wants
an immutable dependency can replace `v1` with a full commit SHA:

```yaml
uses: https://pubcode.archuser.org/actions/gh-sync@<full-commit-sha>
```

The action is a Docker action. The Forgejo runner must support Docker actions,
but the job image itself does not need Go or `gh`; Forgejo builds the action's
multi-stage `Dockerfile` and runs its small compiled executable.

## Inputs

| Input | Required | Default | Meaning |
| --- | --- | --- | --- |
| `repository` | yes | | Destination GitHub repository in exact `owner/repository` form |
| `token` | yes | | GitHub personal access token; it is never logged |
| `is_fine` | no | `false` | `true` for a fine-grained PAT; `false` for a classic PAT |
| `tag` | no | current ref | GitHub release tag; falls back to `FORGEJO_REF_NAME`, then `GITHUB_REF_NAME` |
| `name` | no | release tag | Display name used when creating a release |
| `body` | no | empty | Description used when creating a release |
| `files` | no | empty | Newline-separated local file paths or glob patterns |
| `draft` | no | `false` | Create a new release as a draft |
| `prerelease` | no | `false` | Create a new release as a prerelease |
| `overwrite` | no | `true` | Replace an existing asset with the same filename |

Boolean inputs accept only `true` or `false`. If `tag` is omitted and neither
supported ref-name environment variable is available, the action fails.

Each nonblank entry in `files` is expanded as a local glob. Every specified
pattern must match at least one regular file. Directories are ignored, resolved
file paths are deduplicated, and two different files may not have the same
basename because the basename becomes the GitHub asset name. For example:

```text
dist/linux-amd64.tar.gz -> linux-amd64.tar.gz
```

## Release behavior

For the selected repository and tag, `gh-sync` performs these operations:

1. Look up the GitHub release by tag.
2. Create it with `name`, `body`, `draft`, and `prerelease` if it does not exist.
3. Reuse it if it already exists.
4. List its existing assets.
5. For each local file, delete the same-named remote asset when
   `overwrite: true`, then stream the new file with its exact content length.
6. Fail instead of deleting when `overwrite: false`.

Existing releases are reused as-is; creation-only metadata is not rewritten.
Running the same job again with the same tag and filenames therefore replaces
the release assets rather than accumulating duplicates.

Useful logs identify the repository, release, replacements, and uploaded
filenames. Authorization headers and token values are never logged. API errors
include the operation, HTTP status, and GitHub message with token values
redacted.

## Command-line use

The implementation can also run directly:

```sh
go build -o gh-sync ./cmd/gh-sync

INPUT_TOKEN="$GH_KEY" ./gh-sync \
  -repository firebadnofire/example \
  -is-fine=false \
  -tag v1.2.3 \
  -name 'Example v1.2.3' \
  -files $'dist/*.tar.gz\ndist/SHA256SUMS'
```

Flags override the corresponding action environment variables:

```text
INPUT_REPOSITORY
INPUT_TOKEN
INPUT_IS_FINE
INPUT_TAG
INPUT_NAME
INPUT_BODY
INPUT_FILES
INPUT_DRAFT
INPUT_PRERELEASE
INPUT_OVERWRITE
```

For GitHub Enterprise Server or API-compatible testing, set
`GH_SYNC_API_URL` or pass `-api-url`. Its default is
`https://api.github.com`.

## End-to-end self-test

[`.forgejo/workflows/self-test.yml`](.forgejo/workflows/self-test.yml) runs on
every pushed commit and also supports manual dispatch. It targets the available
`ubuntu-22.04` runner and installs the minimal Git, curl, CA-certificate, and
JSON tooling missing from that runner's base image.

The test uses one stable tag, `gh-sync-self-test`, and two small ASCII assets. It
follows the same source-and-mirror flow used by the neighboring `vpnctl`
project:

1. Create or reuse the `gh-sync-self-test` release in this Forgejo repository.
2. Publish both ASCII files as Forgejo release assets using the automatic
   `forgejo.token`.
3. Invoke the root action with `uses: ./`, which tests the code from the exact
   commit checked out by the workflow.
4. Publish those files to `firebadnofire/sync-test` using the classic GitHub PAT
   stored in the Forgejo secret `GH_KEY` and `is_fine: false`.
5. Change the primary file from `phase=initial` to `phase=overwrite`.
6. Replace the Forgejo assets and invoke `gh-sync` again for the same GitHub tag
   and filenames.
7. Independently query the GitHub API, require exactly one copy of each asset,
   download the primary asset, and require its final `phase=overwrite` marker.

The fixed concurrency group serializes runs that would otherwise modify the
same release. Both dedicated releases and their known assets intentionally
remain for reuse; the workflow does not create an ever-growing series of test
releases. It never places `GH_KEY` in an artifact or prints its value.

The normal unit tests remain offline and require no GitHub or Forgejo
credentials.

## Development

The project uses only the Go standard library:

```sh
gofmt -w cmd internal integration
go test ./...
go vet ./...
```

API tests use local `httptest` servers and never contact GitHub. To publish a
version of the action itself, tag a tested commit (for example `v1.0.0`) and
move the major `v1` tag to the desired compatible release. No compiled binary
needs to be committed because the Docker build produces it.
