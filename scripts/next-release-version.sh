#!/bin/sh

set -eu

LC_ALL=C
export LC_ALL

latest_tag=$(
	git tag --list 'v*' --sort=-version:refname \
		| grep -E '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' \
		| sed -n '1p' \
		|| true
)
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
