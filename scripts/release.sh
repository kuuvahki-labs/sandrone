#!/bin/sh
set -eu

LC_ALL=C
export LC_ALL

validate_version() {
	case "${VERSION-}" in
	  "")
	    printf '%s\n' 'release VERSION must not be empty' >&2
	    exit 1
	    ;;
	  *[!0-9A-Za-z.-]*)
	    printf '%s\n' 'release VERSION must contain only ASCII letters, digits, dots, and hyphens' >&2
	    exit 1
	    ;;
	esac

	if [ "${#VERSION}" -gt 127 ]; then
	  printf '%s\n' 'release VERSION must be at most 127 characters' >&2
	  exit 1
	fi
}

validate_tag() {
	: "${GITHUB_REF_NAME:?GITHUB_REF_NAME is required}"

	script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
	version_file=${VERSION_FILE-"$script_dir/../internal/buildinfo/VERSION"}
	version=$(tr -d '\r\n' <"$version_file")
	VERSION=$version
	export VERSION
	validate_version

	expected_tag="v${version}"
	if [ "$GITHUB_REF_NAME" != "$expected_tag" ]; then
		printf 'release tag %s does not match VERSION %s\n' "$GITHUB_REF_NAME" "$expected_tag" >&2
		exit 1
	fi
}

latest_stable_tag() {
	git tag --list 'v*' --sort=-version:refname \
		| grep -E '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' \
		| sed -n '1p' \
		|| true
}

next_version() {
	latest_tag=$(latest_stable_tag)
	if [ -z "$latest_tag" ]; then
		printf '%s\n' 'no stable release tag found' >&2
		exit 1
	fi

	version=${latest_tag#v}
	major=${version%%.*}
	remainder=${version#*.}
	minor=${remainder%%.*}
	patch=${remainder#*.}

	printf '%s.%s.%s\n' "$major" "$minor" "$((patch + 1))"
}

image_tags() {
	: "${IMAGE:?IMAGE is required}"

	if [ "${GITHUB_REF_TYPE-}" != "tag" ]; then
	  exit 0
	fi

	validate_tag
	latest_tag=$(latest_stable_tag)

	printf '%s\n' "${IMAGE}:${GITHUB_REF_NAME}"
	if [ -n "$latest_tag" ] && [ "${GITHUB_REF_NAME-}" = "$latest_tag" ]; then
	  printf '%s\n' "${IMAGE}:latest"
	fi
}

case "${1-}" in
	validate-version) validate_version ;;
	validate-tag) validate_tag ;;
	next-version) next_version ;;
	image-tags) image_tags ;;
	*)
		printf '%s\n' 'usage: release.sh {validate-version|validate-tag|next-version|image-tags}' >&2
		exit 2
		;;
esac
