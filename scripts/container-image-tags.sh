#!/bin/sh

set -eu

LC_ALL=C
export LC_ALL

: "${IMAGE:?IMAGE is required}"
: "${GITHUB_SHA:?GITHUB_SHA is required}"

if [ "${GITHUB_EVENT_NAME-}" != "push" ]; then
  exit 0
fi

short_revision=$(printf '%.12s' "$GITHUB_SHA")
revision_tag="${IMAGE}:sha-${short_revision}"

if [ "${GITHUB_REF_TYPE-}" != "tag" ]; then
  printf '%s\n' "$revision_tag"
  exit 0
fi

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
version_file=${VERSION_FILE-"$script_dir/../internal/buildinfo/VERSION"}
version=$(tr -d '\r\n' <"$version_file")
VERSION=$version
export VERSION
if [ "$(sh "$script_dir/validate-build-version.sh")" != "ok" ] || [ -z "$version" ]; then
  printf 'invalid release version %s\n' "$version" >&2
  exit 1
fi

expected_tag="v${version}"
if [ "${GITHUB_REF_NAME-}" != "$expected_tag" ]; then
  printf 'release tag %s does not match VERSION %s\n' "${GITHUB_REF_NAME-}" "$expected_tag" >&2
  exit 1
fi

latest_tag=$(
  git tag --list 'v*' --sort=-version:refname \
    | grep -E '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' \
    | sed -n '1p' \
    || true
)

printf '%s\n' "$revision_tag" "${IMAGE}:${version}"
if [ -n "$latest_tag" ] && [ "${GITHUB_REF_NAME-}" = "$latest_tag" ]; then
  printf '%s\n' "${IMAGE}:latest"
fi
