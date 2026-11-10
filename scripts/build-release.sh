#!/usr/bin/env bash
# Cross-builds release artifacts and their SHA-256 checksums.
# Usage: scripts/build-release.sh [version]  (default: 0.0.0-dev)
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-0.0.0-dev}"
OUT="dist/tokenledger-${VERSION}"
LDFLAGS="-s -w -X main.version=${VERSION}"
mkdir -p dist
rm -f "${OUT}"-*

build() { # goos goarch ext
  local goos="$1" goarch="$2" ext="${3:-}"
  echo "building ${goos}/${goarch}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "${OUT}-${goos}-${goarch}${ext}" ./cmd/tokenledger
}

build darwin arm64
build darwin amd64
build windows amd64 .exe
build windows arm64 .exe

# macOS universal binary via lipo (when available).
if command -v lipo >/dev/null 2>&1; then
  lipo -create "${OUT}-darwin-arm64" "${OUT}-darwin-amd64" -output "${OUT}-darwin-universal"
fi

# SHA-256 checksums (shasum on macOS/BSD, sha256sum on Linux).
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "${OUT}"-* > "${OUT}.sha256"
else
  shasum -a 256 "${OUT}"-* > "${OUT}.sha256"
fi
echo "artifacts in dist/:"
ls -1 "${OUT}"-*
