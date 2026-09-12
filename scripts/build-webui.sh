#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$ROOT/web"
STATIC_DIR="$ROOT/internal/entry/webui/static"

if [ -n "${WEBUI_PREBUILT_DIR:-}" ]; then
  ASSET_DIR="$(cd -- "$WEBUI_PREBUILT_DIR" && pwd)"
else
  cd "$WEB_DIR"
  pnpm install --frozen-lockfile
  pnpm build
  ASSET_DIR="$WEB_DIR/build/client"
fi

if [ ! -s "$ASSET_DIR/index.html" ]; then
  printf '%s\n' "Web UI assets are missing or empty: $ASSET_DIR/index.html" >&2
  exit 1
fi

case "$ASSET_DIR/" in
  "$STATIC_DIR/"*)
    printf '%s\n' 'Web UI assets must be outside the embed destination' >&2
    exit 1
    ;;
esac
case "$STATIC_DIR/" in
  "$ASSET_DIR/"*)
    printf '%s\n' 'Web UI assets must not contain the embed destination' >&2
    exit 1
    ;;
esac

mkdir -p "$STATIC_DIR"
find "$STATIC_DIR" -mindepth 1 -maxdepth 1 ! -name .gitkeep -exec rm -rf {} +
cp -R "$ASSET_DIR/." "$STATIC_DIR/"
