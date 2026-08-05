#!/bin/sh
set -eu

repo_root=$(CDPATH='' cd "$(dirname "$0")/.." && pwd)
input_file=${INPUT_FILE:-$repo_root/urls.txt}
output_file=${OUTPUT_FILE:-$repo_root/urls.alive.txt}
sandrone_cli=${SANDRONE_CLI:-$repo_root/sandrone}

if ! command -v "$sandrone_cli" >/dev/null 2>&1; then
  printf 'error: command not found: %s\n' "$sandrone_cli" >&2
  exit 127
fi

if ! command -v jq >/dev/null 2>&1; then
  printf '%s\n' 'error: command not found: jq' >&2
  exit 127
fi

if [ ! -f "$input_file" ]; then
  printf 'error: input file not found: %s\n' "$input_file" >&2
  exit 66
fi

umask 077
output_tmp=$(mktemp "${output_file}.tmp.XXXXXX")
probe_tmp=$(mktemp "${TMPDIR:-/tmp}/sandrone-probe.XXXXXX")

cleanup() {
  rm -f "$output_tmp" "$probe_tmp"
}

trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

checked=0
kept=0

while IFS= read -r raw_line || [ -n "$raw_line" ]; do
  url=$(printf '%s' "$raw_line" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')

  case "$url" in
    ''|'#'*) continue ;;
  esac

  checked=$((checked + 1))
  : >"$probe_tmp"

  if "$sandrone_cli" probe --input-url "$url" --output "$probe_tmp" 2>/dev/null &&
    jq -e 'any(.results[]?; .alive == true)' "$probe_tmp" >/dev/null 2>&1; then
    printf '%s\n' "$url" >>"$output_tmp"
    kept=$((kept + 1))
    printf '[%d] alive\n' "$checked" >&2
  else
    printf '[%d] unavailable or no live nodes\n' "$checked" >&2
  fi
done <"$input_file"

mv "$output_tmp" "$output_file"
printf 'done: kept %d of %d subscriptions in %s\n' "$kept" "$checked" "$output_file" >&2
