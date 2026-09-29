# gh-sync

`gh-sync` is a Forgejo CI action that publishes CI-produced files to a GitHub
release. It uses the GitHub REST API directly and does not require the GitHub
CLI.

The Forgejo repository remains the source of truth. This action does not mirror
branches, commits, tags, issues, pull requests, or any other repository state.

## Usage

```yaml
- name: Sync release to GitHub
  uses: https://pubcode.archuser.org/actions/gh-sync@v1
  with:
    repository: firebadnofire/example
    token: ${{ secrets.GH_SYNC_TOKEN }}
    tag: ${{ forgejo.ref_name }}
    name: Example ${{ forgejo.ref_name }}
    body: Built and published by Forgejo CI.
    files: |
      dist/*.tar.gz
      dist/*.zip
      dist/SHA256SUMS
    overwrite: true
```

The token must be able to read and write releases in the target GitHub
repository. Keep it in a Forgejo secret; `gh-sync` never writes the token to its
logs.

### Inputs

| Input | Required | Default | Description |
| --- | --- | --- | --- |
| `repository` | yes | | GitHub repository in `owner/repository` form |
| `token` | yes | | GitHub personal access token |
| `tag` | no | current ref | Release tag; falls back to `FORGEJO_REF_NAME`, then `GITHUB_REF_NAME` |
| `name` | no | release tag | Release display name |
| `body` | no | empty | Release description |
| `files` | no | empty | Newline-separated local paths or glob patterns |
| `draft` | no | `false` | Create a new release as a draft |
| `prerelease` | no | `false` | Create a new release as a prerelease |
| `overwrite` | no | `true` | Delete and replace same-named existing assets |

Every nonblank `files` pattern must match at least one regular file.
Directories are not uploaded. Files are deduplicated by resolved local path,
and the basename of each file becomes its GitHub asset name.

If the release already exists, it is reused. With the default
`overwrite: true`, an existing asset with the same filename is deleted before
the new file is uploaded. With `overwrite: false`, the action fails instead.
Consequently, repeating a job for the same repository, tag, and files updates
the release assets rather than creating duplicates.

## How the action runs

The action is defined as a Docker action. Forgejo builds the multi-stage
`Dockerfile`, then runs a small statically linked executable in an Alpine image
with CA certificates. A consuming runner therefore needs Docker action support,
but it does not need Go or `gh` installed.

Publish releases of this action by tagging commits in this repository (for
example, `v1.0.0`) and maintaining the desired major-version tag (for example,
`v1`). The executable does not need to be committed; it is compiled by the
Docker build.

## Command-line use

The same program can be built and run directly:

```sh
go build -o gh-sync ./cmd/gh-sync

INPUT_TOKEN="$GH_SYNC_TOKEN" ./gh-sync \
  -repository firebadnofire/example \
  -tag v1.2.3 \
  -files $'dist/*.tar.gz\ndist/SHA256SUMS'
```

Flags correspond to the action inputs. Defaults are read from the standard
`INPUT_REPOSITORY`, `INPUT_TOKEN`, `INPUT_TAG`, `INPUT_NAME`, `INPUT_BODY`,
`INPUT_FILES`, `INPUT_DRAFT`, `INPUT_PRERELEASE`, and `INPUT_OVERWRITE`
environment variables. Flags override those values.

For GitHub Enterprise Server or API-compatible testing, set
`GH_SYNC_API_URL` or pass `-api-url`. The default is
`https://api.github.com`.

## Integration self-test

The manually triggered Forgejo workflow
`.forgejo/workflows/self-test.yml` tests the current checkout end to end against
the dedicated GitHub publishing endpoint `firebadnofire/sync-test`. Configure a
Forgejo repository secret named `GH_SYNC_TOKEN` with permission to read and
write releases in that GitHub repository, then open the repository's Actions
page and run **gh-sync integration self-test**. The job targets the
`ubuntu-22.04` label advertised by the online Linux runner.

The workflow checks out the revision being tested and invokes the root action
locally with `uses: ./`, so Forgejo builds the current `Dockerfile` rather than
using a previously published `gh-sync` tag. It uploads two small diagnostic
assets to the stable `gh-sync-self-test` release, changes the primary asset, and
invokes the action again with `overwrite: true`. A separate local verification
action then queries the GitHub API, requires exactly one asset with each expected
name, downloads the primary asset, and checks for the final
`phase=overwrite` marker.

The dedicated release and its two assets intentionally remain in place and are
reused on subsequent runs. A concurrency group serializes this workflow on
Forgejo versions that support workflow concurrency; Forgejo documents this as a
best-effort safeguard. The workflow never writes the token into an artifact or
prints it.

Normal unit tests remain offline and do not need the secret or any GitHub
credentials.

## Development

The project uses only the Go standard library.

```sh
gofmt -w cmd internal
go test ./...
go vet ./...
```

API tests use local `httptest` servers and never contact GitHub.
