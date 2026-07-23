#!/bin/sh

LC_ALL=C
export LC_ALL

case "${VERSION-}" in
  "")
    printf '%s\n' ok
    ;;
  *[!0-9A-Za-z.+-]*)
    printf '%s\n' invalid
    ;;
  *)
    printf '%s\n' ok
    ;;
esac
