#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
export BUN_INSTALL="${BUN_INSTALL:-$HOME/.bun}"
export PATH="$BUN_INSTALL/bin:$PATH"

if ! command -v go >/dev/null || ! command -v gcc >/dev/null || ! command -v curl >/dev/null || ! command -v unzip >/dev/null; then
	ROOT_RUN=()
	if (( EUID != 0 )); then
		if ! command -v sudo >/dev/null; then
			echo "Error: sudo is required to install system dependencies." >&2
			exit 1
		fi
		ROOT_RUN=(sudo)
	fi
	if command -v apt-get >/dev/null; then
		"${ROOT_RUN[@]}" apt-get update
		"${ROOT_RUN[@]}" apt-get install -y golang-go build-essential curl unzip ca-certificates
	elif command -v dnf >/dev/null; then
		"${ROOT_RUN[@]}" dnf install -y golang gcc glibc-devel curl unzip ca-certificates
	elif command -v pacman >/dev/null; then
		"${ROOT_RUN[@]}" pacman -S --needed --noconfirm go base-devel curl unzip ca-certificates
	elif command -v apk >/dev/null; then
		"${ROOT_RUN[@]}" apk add go build-base curl unzip ca-certificates bash
	else
		echo "Error: install Go, GCC, curl and unzip with your system package manager." >&2
		exit 1
	fi
fi

BUN_VERSION="$(sed -n 's/.*"packageManager": "bun@\([^"]*\)".*/\1/p' "$ROOT_DIR/frontend/package.json")"
if [[ -z "$BUN_VERSION" ]]; then
	echo "Error: frontend/package.json does not specify a Bun version." >&2
	exit 1
fi
if ! command -v bun >/dev/null || [[ "$(printf '%s\n%s\n' "$BUN_VERSION" "$(bun --version)" | sort -V | head -n 1)" != "$BUN_VERSION" ]]; then
	curl -fsSL https://bun.sh/install | bash -s -- "bun-v$BUN_VERSION"
fi

echo "Installing project dependencies..."
GOTOOLCHAIN=auto go -C "$ROOT_DIR/backend" mod download
bun install --cwd "$ROOT_DIR/frontend" --frozen-lockfile
echo "Dependencies ready."
