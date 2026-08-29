#!/bin/sh
set -eu

: "${VERSION:?VERSION is required}"

BUILD_TIME=${BUILD_TIME-$(date -u +%Y-%m-%dT%H:%M:%SZ)}

artifact_kind=${ARTIFACT_KIND-release}
REVISION=${REVISION-}
case "$artifact_kind" in
	release)
		if [ -z "$REVISION" ]; then
			printf '%s\n' 'release artifacts require REVISION' >&2
			exit 2
		fi
		;;
	snapshot)
		if [ "$VERSION" != dev ]; then
			printf '%s\n' 'snapshot artifacts require VERSION=dev' >&2
			exit 2
		fi
		if [ -n "$REVISION" ]; then
			printf '%s\n' 'snapshot artifacts require an empty REVISION' >&2
			exit 2
		fi
		;;
	*)
		printf 'unsupported artifact kind %s\n' "$artifact_kind" >&2
		exit 2
		;;
esac

release_targets=${RELEASE_TARGETS-"linux/amd64 linux/arm64"}
output_dir=${OUTPUT_DIR-dist}
make_command=${MAKE-make}

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$script_dir/.." && pwd)

if [ "$(VERSION="$VERSION" sh "$script_dir/validate-build-version.sh")" != ok ]; then
	printf '%s\n' 'VERSION must contain only ASCII letters, digits, dots, plus signs, and hyphens' >&2
	exit 2
fi
if [ "$(REVISION="$REVISION" sh "$script_dir/validate-build-revision.sh")" != ok ]; then
	printf '%s\n' 'REVISION must be a complete 40- or 64-character hexadecimal Git object ID' >&2
	exit 2
fi
if [ "$(BUILD_TIME="$BUILD_TIME" sh "$script_dir/validate-build-time.sh")" != ok ]; then
	printf '%s\n' 'BUILD_TIME must use UTC RFC3339 format YYYY-MM-DDTHH:MM:SSZ' >&2
	exit 2
fi

# RELEASE_TARGETS is a space-separated list by contract.
# shellcheck disable=SC2086
set -- $release_targets
if [ "$#" -eq 0 ]; then
	printf '%s\n' 'release target list must not be empty' >&2
	exit 2
fi
for target do
	case "$target" in
		linux/amd64 | linux/arm64) ;;
		*)
			printf 'unsupported release target %s\n' "$target" >&2
			exit 2
			;;
	esac
done

staging_dir=$(mktemp -d "${TMPDIR:-/tmp}/sandrone-release-artifacts.XXXXXX")
cleanup() {
	rm -rf "$staging_dir"
}
trap cleanup 0 1 2 15
artifact_dir=$staging_dir/artifacts
mkdir -p "$artifact_dir"

cd "$repo_root"
VERSION="$VERSION" REVISION="$REVISION" "$make_command" ruleset-catalog

for target do
	goos=${target%/*}
	goarch=${target#*/}
	package_dir=$staging_dir/package-$goos-$goarch
	mkdir -p "$package_dir"
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
		"$make_command" build-check \
		"BUILD_BIN=$package_dir/sandrone" \
		"VERSION=$VERSION" \
		"REVISION=$REVISION" \
		"BUILD_TIME=$BUILD_TIME"
	cp "$repo_root/LICENSE" "$package_dir/LICENSE"
	tar -czf "$artifact_dir/sandrone_${goos}_${goarch}.tar.gz" \
		-C "$package_dir" sandrone LICENSE
done

(
	cd "$artifact_dir"
	LC_ALL=C
	export LC_ALL
	for archive in *.tar.gz; do
		sha256sum "$archive"
	done
) >"$artifact_dir/checksums.txt"

mkdir -p "$output_dir"
mv "$artifact_dir"/* "$output_dir"/
