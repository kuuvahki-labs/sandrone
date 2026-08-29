#!/bin/sh

LC_ALL=C
export LC_ALL

case "${BUILD_TIME-}" in
  [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z)
    printf '%s\n' ok
    ;;
  *)
    printf '%s\n' invalid
    ;;
esac
