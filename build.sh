#!/usr/bin/env bash
# build.sh — build cross-platform (amd64 + arm64), static, stripped.
# Usage: ./build.sh   → servmon-amd64 + servmon-arm64
set -euo pipefail

cd "$(dirname "$0")"
LDFLAGS="-s -w"
FLAGS="-trimpath -buildvcs=false"   # -buildvcs=false: tak butuh metadata git/CI
ARCHES=(amd64 arm64)

go vet ./...   # gate cepat: cegah build binary rusak

for arch in "${ARCHES[@]}"; do
  echo "→ $arch..."
  # tulis ke .tmp lalu mv: binary lama tak korup kalau build gagal di tengah
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" \
    go build $FLAGS -ldflags="$LDFLAGS" -o "servmon-$arch.tmp" .
  mv "servmon-$arch.tmp" "servmon-$arch"
done

echo "OK:"
file servmon-amd64 servmon-arm64 | sed 's/^/  /'
