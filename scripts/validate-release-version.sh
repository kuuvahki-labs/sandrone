#!/bin/sh

LC_ALL=C
export LC_ALL

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
