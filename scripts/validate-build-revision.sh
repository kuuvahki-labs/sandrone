#!/bin/sh

LC_ALL=C
export LC_ALL

case "${REVISION-}" in
  "")
    printf '%s\n' ok
    ;;
  *[!0-9A-Fa-f]*)
    printf '%s\n' invalid
    ;;
  *)
    length=${#REVISION}
    if [ "$length" -eq 40 ] || [ "$length" -eq 64 ]; then
      printf '%s\n' ok
    else
      printf '%s\n' invalid
    fi
    ;;
esac
