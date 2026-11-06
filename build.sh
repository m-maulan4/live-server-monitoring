#!/usr/bin/env bash
# build.sh — build cross-platform (amd64 + arm64), static, stripped.
# Usage: ./build.sh   → servmon-amd64 + servmon-arm64
set -euo pipefail

cd "$(dirname "$0")"
LDFLAGS="-s -w"

echo "→ amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$LDFLAGS" -o servmon-amd64 .

echo "→ arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="$LDFLAGS" -o servmon-arm64 .

echo "OK:"
file servmon-amd64 servmon-arm64 | sed 's/^/  /'
