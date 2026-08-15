#!/bin/sh

set -eu

LC_ALL=C
export LC_ALL

: "${IMAGE:?IMAGE is required}"

if [ "${GITHUB_REF_TYPE-}" != "tag" ]; then
  exit 0
fi

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
version_file=${VERSION_FILE-"$script_dir/../internal/buildinfo/VERSION"}
sh "$script_dir/validate-release-tag.sh"
version=$(tr -d '\r\n' <"$version_file")

latest_tag=$(
  git tag --list 'v*' --sort=-version:refname \
    | grep -E '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' \
    | sed -n '1p' \
    || true
)

printf '%s\n' "${IMAGE}:${GITHUB_REF_NAME}"
if [ -n "$latest_tag" ] && [ "${GITHUB_REF_NAME-}" = "$latest_tag" ]; then
  printf '%s\n' "${IMAGE}:latest"
fi
