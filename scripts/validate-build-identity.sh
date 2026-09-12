#!/bin/sh
set -eu

LC_ALL=C
export LC_ALL

case "${VERSION-}" in
  *[!0-9A-Za-z.+-]*)
    printf '%s\n' 'VERSION must be empty or contain only ASCII letters, digits, dots, plus signs, and hyphens' >&2
    exit 2
    ;;
esac

case "${REVISION-}" in
  "")
    :
    ;;
  *[!0-9A-Fa-f]*)
    printf '%s\n' 'REVISION must be empty or a complete 40- or 64-character hexadecimal Git object ID' >&2
    exit 2
    ;;
  *)
    length=${#REVISION}
    if [ "$length" -eq 40 ] || [ "$length" -eq 64 ]; then
      :
    else
      printf '%s\n' 'REVISION must be empty or a complete 40- or 64-character hexadecimal Git object ID' >&2
      exit 2
    fi
    ;;
esac

case "${BUILD_TIME-}" in
  [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z)
    :
    ;;
  *)
    printf '%s\n' 'BUILD_TIME must use UTC RFC3339 format YYYY-MM-DDTHH:MM:SSZ' >&2
    exit 2
    ;;
esac
