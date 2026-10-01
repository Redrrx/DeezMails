#!/usr/bin/env bash

# This file is strictly for demo it only serves through a functional enough frontend.

set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

if ! command -v bun >/dev/null 2>&1; then
	echo "Error: Bun 1.4.2 or newer is required." >&2
	exit 1
fi

export PORT="${PORT:-8080}"

echo "Preparing frontend..."
bun install --cwd "$ROOT_DIR/frontend" --frozen-lockfile
bun run --cwd "$ROOT_DIR/frontend" build:demo

echo
echo "Starting demo at http://127.0.0.1:${PORT}"
echo "Stop with Ctrl+C."
exec bun run --cwd "$ROOT_DIR/frontend" preview:demo --port "$PORT"
