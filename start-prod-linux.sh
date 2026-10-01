#!/usr/bin/env bash

# Build and run DeezMails using environment variables or .env.

set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"
export PATH="${BUN_INSTALL:-$HOME/.bun}/bin:$PATH"
export GOTOOLCHAIN="${GOTOOLCHAIN:-auto}"

if ! command -v go >/dev/null 2>&1; then
	echo "Error: Go 1.27.1 or newer is required." >&2
	exit 1
fi
if ! command -v bun >/dev/null 2>&1; then
	echo "Error: Bun 1.4.2 or newer is required." >&2
	exit 1
fi

echo "Preparing frontend..."
bun install --cwd "$ROOT_DIR/frontend" --frozen-lockfile
bun run --cwd "$ROOT_DIR/frontend" build

echo
echo "Starting production..."
echo "Stop with Ctrl+C."
exec go run ./backend/cmd/deezmails
