#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$ROOT/web"
STATIC_DIR="$ROOT/internal/entry/webui/static"

cd "$WEB_DIR"
pnpm install --frozen-lockfile
pnpm build

mkdir -p "$STATIC_DIR"
find "$STATIC_DIR" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
cp -R "$WEB_DIR/build/client/." "$STATIC_DIR/"
