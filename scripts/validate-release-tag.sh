#!/bin/sh

set -eu

LC_ALL=C
export LC_ALL

: "${GITHUB_REF_NAME:?GITHUB_REF_NAME is required}"

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
version_file=${VERSION_FILE-"$script_dir/../internal/buildinfo/VERSION"}
version=$(tr -d '\r\n' <"$version_file")
VERSION=$version
export VERSION
if ! sh "$script_dir/validate-release-version.sh"; then
	exit 1
fi

expected_tag="v${version}"
if [ "$GITHUB_REF_NAME" != "$expected_tag" ]; then
	printf 'release tag %s does not match VERSION %s\n' "$GITHUB_REF_NAME" "$expected_tag" >&2
	exit 1
fi
