#!/bin/sh
set -eu

tag=gh-sync-self-test
release_name='gh-sync integration self-test'
release_body='Automated source release produced by the gh-sync Forgejo CI self-test.'
asset_paths='self-test/gh-sync-self-test.txt self-test/gh-sync-self-test-secondary.txt'

api_url=${FORGEJO_API_URL:-${GITHUB_API_URL:?Missing Forgejo API URL}}
repository=${FORGEJO_REPOSITORY:-${GITHUB_REPOSITORY:?Missing Forgejo repository}}
revision=${FORGEJO_SHA:-${GITHUB_SHA:?Missing source revision}}
token=${FORGEJO_TOKEN:?Missing Forgejo workflow token}
owner=${repository%%/*}
repo=${repository#*/}

case "$api_url" in
  https://*) ;;
  *)
    echo "self-test: Forgejo API URL must use HTTPS" >&2
    exit 1
    ;;
esac

response_dir=$(mktemp -d)
trap 'rm -rf "$response_dir"' EXIT HUP INT TERM

release_payload=$(jq -n \
  --arg tag "$tag" \
  --arg target "$revision" \
  --arg name "$release_name" \
  --arg body "$release_body" \
  '{tag_name: $tag, target_commitish: $target, name: $name, body: $body, draft: false, prerelease: false, hide_archive_links: false}')

api_request() {
  method=$1
  url=$2
  shift 2
  curl --proto '=https' --tlsv1.2 --fail-with-body --silent --show-error \
    --request "$method" \
    --header "Authorization: token ${token}" \
    "$@" \
    "$url"
}

fetch_release() {
  response_file=${response_dir}/fetch-release.json
  status=$(curl --proto '=https' --tlsv1.2 --silent --show-error \
    --output "$response_file" \
    --write-out '%{http_code}' \
    --header "Authorization: token ${token}" \
    "${api_url}/repos/${owner}/${repo}/releases/tags/${tag}")

  case "$status" in
    200)
      cat "$response_file"
      return 0
      ;;
    404)
      return 1
      ;;
    *)
      cat "$response_file" >&2
      echo "self-test: Forgejo release lookup failed: HTTP ${status}" >&2
      exit 1
      ;;
  esac
}

create_release() {
  response_file=${response_dir}/create-release.json
  status=$(curl --proto '=https' --tlsv1.2 --silent --show-error \
    --output "$response_file" \
    --write-out '%{http_code}' \
    --request POST \
    --header "Authorization: token ${token}" \
    --header 'Content-Type: application/json' \
    --data "$release_payload" \
    "${api_url}/repos/${owner}/${repo}/releases")

  case "$status" in
    200|201)
      cat "$response_file"
      return 0
      ;;
    409|422)
      if release_json=$(fetch_release); then
        printf '%s' "$release_json"
        return 0
      fi
      ;;
  esac

  cat "$response_file" >&2
  echo "self-test: Forgejo release creation failed: HTTP ${status}" >&2
  exit 1
}

if release_json=$(fetch_release); then
  release_id=$(printf '%s' "$release_json" | jq -r '.id')
  release_json=$(api_request PATCH "${api_url}/repos/${owner}/${repo}/releases/${release_id}" \
    --header 'Content-Type: application/json' \
    --data "$release_payload")
else
  release_json=$(create_release)
fi

release_id=$(printf '%s' "$release_json" | jq -r '.id')
if [ -z "$release_id" ] || [ "$release_id" = null ]; then
  echo 'self-test: Forgejo release response did not include an id' >&2
  exit 1
fi

for asset_path in $asset_paths; do
  if [ ! -f "$asset_path" ]; then
    echo "self-test: Forgejo release asset is missing: ${asset_path}" >&2
    exit 1
  fi

  asset_name=$(basename "$asset_path")
  existing_asset_id=$(api_request GET "${api_url}/repos/${owner}/${repo}/releases/${release_id}/assets" \
    | jq -r --arg name "$asset_name" '.[] | select(.name == $name) | .id' \
    | head -n 1)

  if [ -n "$existing_asset_id" ]; then
    api_request DELETE "${api_url}/repos/${owner}/${repo}/releases/${release_id}/assets/${existing_asset_id}" >/dev/null
  fi

  api_request POST "${api_url}/repos/${owner}/${repo}/releases/${release_id}/assets?name=${asset_name}" \
    --form "attachment=@${asset_path}" >/dev/null
  echo "self-test: published ${asset_name} to Forgejo release ${tag}"
done
