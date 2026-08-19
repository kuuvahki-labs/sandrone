#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$script_dir/.." && pwd)
make_command=${MAKE-make}

cd "$repo_root"
"$make_command" ruleset-catalog build-webui
sh "$script_dir/verify-vercel-assets.sh"
