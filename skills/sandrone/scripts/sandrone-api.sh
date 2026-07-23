#!/bin/sh
set -eu

usage() {
  printf '%s\n' 'usage: sandrone-api.sh METHOD PATH [BODY_FILE|-]' >&2
  exit 64
}

[ "$#" -ge 2 ] && [ "$#" -le 3 ] || usage
method=$1
path=$2
body=${3-}

case "$method" in
  GET|POST|PUT|DELETE) ;;
  *)
    printf '%s\n' 'unsupported HTTP method' >&2
    exit 64
    ;;
esac

case "${SANDRONE_URL-}" in
  http://*) authority=${SANDRONE_URL#http://} ;;
  https://*) authority=${SANDRONE_URL#https://} ;;
  *)
    printf '%s\n' 'SANDRONE_URL must be an absolute HTTP(S) URL' >&2
    exit 64
    ;;
esac

case "$authority" in
  ''|[/?#]*)
    printf '%s\n' 'SANDRONE_URL must be an absolute HTTP(S) URL' >&2
    exit 64
    ;;
esac

case "$path" in
  /*) ;;
  *)
    printf '%s\n' 'PATH must begin with /' >&2
    exit 64
    ;;
esac

carriage_return=$(printf '\r')
line_feed='
'

case "$SANDRONE_URL" in
  *"$carriage_return"*|*"$line_feed"*)
    printf '%s\n' 'SANDRONE_URL must not contain CR or LF' >&2
    exit 64
    ;;
esac

case "${SANDRONE_TOKEN-}" in
  *"$carriage_return"*|*"$line_feed"*)
    printf '%s\n' 'SANDRONE_TOKEN must not contain CR or LF' >&2
    exit 64
    ;;
esac

case "$path" in
  *"$carriage_return"*|*"$line_feed"*)
    printf '%s\n' 'PATH must not contain CR or LF' >&2
    exit 64
    ;;
esac

base=$SANDRONE_URL
while [ "${base%/}" != "$base" ]; do
  base=${base%/}
done

umask 077
response_file=
header_file=

cleanup() {
  if [ -n "$response_file" ]; then
    rm -f "$response_file"
  fi
  if [ -n "$header_file" ]; then
    rm -f "$header_file"
  fi
}

trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

response_file=$(mktemp "${TMPDIR:-/tmp}/sandrone-api-response.XXXXXX")
header_file=$(mktemp "${TMPDIR:-/tmp}/sandrone-api-header.XXXXXX")
chmod 0600 "$header_file"

set -- \
  --disable \
  --silent \
  --show-error \
  --request "$method" \
  --output "$response_file" \
  --write-out '%{http_code}'

if [ -n "${SANDRONE_TOKEN-}" ]; then
  printf 'Authorization: Bearer %s\n' "$SANDRONE_TOKEN" >"$header_file"
  set -- "$@" --header "@$header_file"
fi

case "$body" in
  '') ;;
  -) set -- "$@" --header 'Content-Type: application/json' --data-binary '@-' ;;
  *) set -- "$@" --header 'Content-Type: application/json' --data-binary "@$body" ;;
esac

set -- "$@" "$base$path"

if status=$(curl "$@"); then
  curl_exit=0
else
  curl_exit=$?
fi

cat "$response_file"

if [ "$curl_exit" -ne 0 ]; then
  printf 'sandrone-api: transport failure (curl exit %s)\n' "$curl_exit" >&2
  exit "$curl_exit"
fi

case "$status" in
  2??) exit 0 ;;
  *)
    printf 'sandrone-api: HTTP status %s\n' "$status" >&2
    exit 1
    ;;
esac
