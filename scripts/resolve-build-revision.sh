#!/bin/sh

set -eu

LC_ALL=C
export LC_ALL

if ! revision=$(git rev-parse --verify HEAD 2>/dev/null); then
  exit 0
fi
if ! status=$(git status --porcelain --untracked-files=normal 2>/dev/null); then
  exit 0
fi
if [ -n "$status" ]; then
  exit 0
fi

printf '%s\n' "$revision"
