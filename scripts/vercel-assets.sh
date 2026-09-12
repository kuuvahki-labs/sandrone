#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$script_dir/.." && pwd)
case "${1-}" in
  build)
    cd "$repo_root"
    "${MAKE-make}" ruleset-catalog build-webui
    ;;
  verify) ;;
  *)
    printf '%s\n' 'usage: vercel-assets.sh build|verify' >&2
    exit 2
    ;;
esac

web_index=$repo_root/internal/entry/webui/static/index.html
ruleset_catalog=$repo_root/internal/service/catalog_builtin/catalog.json.gz

if [ ! -s "$web_index" ]; then
	printf '%s\n' 'Vercel asset contract failed: internal/entry/webui/static/index.html is missing or empty' >&2
	exit 1
fi
if [ ! -s "$ruleset_catalog" ]; then
	printf '%s\n' 'Vercel asset contract failed: internal/service/catalog_builtin/catalog.json.gz is missing or empty' >&2
	exit 1
fi
if ! gzip -t "$ruleset_catalog"; then
	printf '%s\n' 'Vercel asset contract failed: internal/service/catalog_builtin/catalog.json.gz is not valid gzip' >&2
	exit 1
fi
