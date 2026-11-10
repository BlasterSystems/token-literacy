#!/usr/bin/env bash
# Cross-builds release artifacts per docs/RELEASING.md:
#   tokenledger_<version>_<os>_<arch>.tar.gz (macOS)
#   tokenledger_<version>_<os>_<arch>.zip    (Windows)
#   checksums.txt
# Usage: scripts/build-release.sh [version]  (default: 0.0.0-dev)
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${1:-0.0.0-dev}"
LDFLAGS="-s -w -X main.version=${VERSION}"
DIST="dist"
mkdir -p "$DIST"
rm -f "${DIST}"/tokenledger_"${VERSION}"_* "${DIST}/checksums.txt"

stage() { # goos goarch
  local goos="$1" goarch="$2"
  echo "building ${goos}/${goarch}"
  local bin="tokenledger"
  [ "$goos" = "windows" ] && bin="tokenledger.exe"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$DIST/$bin" ./cmd/tokenledger
  if [ "$goos" = "windows" ]; then
    (cd "$DIST" && zip -q "tokenledger_${VERSION}_${goos}_${goarch}.zip" "$bin")
  else
    (cd "$DIST" && tar -czf "tokenledger_${VERSION}_${goos}_${goarch}.tar.gz" "$bin")
  fi
  rm -f "$DIST/$bin"
}

stage darwin arm64
stage darwin amd64
stage windows amd64
stage windows arm64

# macOS universal binary via lipo (when available), shipped inside its own tarball.
if command -v lipo >/dev/null 2>&1; then
  echo "building darwin universal"
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$LDFLAGS" -o "$DIST/tokenledger-arm64" ./cmd/tokenledger
  CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "$DIST/tokenledger-amd64" ./cmd/tokenledger
  lipo -create "$DIST/tokenledger-arm64" "$DIST/tokenledger-amd64" -output "$DIST/tokenledger"
  rm -f "$DIST/tokenledger-arm64" "$DIST/tokenledger-amd64"
  (cd "$DIST" && tar -czf "tokenledger_${VERSION}_darwin_universal.tar.gz" tokenledger)
  rm -f "$DIST/tokenledger"
fi

# SHA-256 checksums (sha256sum on Linux, shasum on macOS/BSD).
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$DIST" && sha256sum tokenledger_"${VERSION}"_* > checksums.txt)
else
  (cd "$DIST" && shasum -a 256 tokenledger_"${VERSION}"_* > checksums.txt)
fi

echo "artifacts in ${DIST}/:"
ls -1 "${DIST}"/tokenledger_"${VERSION}"_* "$DIST/checksums.txt"
