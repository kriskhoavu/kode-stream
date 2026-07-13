#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "Usage: cmd/scripts/distribution/update_homebrew_formula_from_release.sh <version> [tap-path]"
  echo "Example: cmd/scripts/distribution/update_homebrew_formula_from_release.sh 1.0.0 ../homebrew-tap"
  exit 2
fi

if [[ "$VERSION" == v* ]]; then
  VERSION="${VERSION#v}"
fi

TAP_PATH="${2:-../homebrew-tap}"
TAG="v${VERSION}"
FORMULA_FILE="$TAP_PATH/Formula/kode-stream.rb"
SUMS_FILE="/tmp/SHA256SUMS-v${VERSION}"

if [[ ! -f "$FORMULA_FILE" ]]; then
  echo "Formula file not found: $FORMULA_FILE"
  exit 1
fi

echo "==> Downloading checksums for ${TAG}"
curl -fL "https://github.com/kriskhoavu/kode-stream/releases/download/${TAG}/SHA256SUMS" -o "$SUMS_FILE"

ARM64_SHA="$(awk '/kode-stream_'"${VERSION}"'_darwin_arm64.tar.gz/{print $1}' "$SUMS_FILE")"
AMD64_SHA="$(awk '/kode-stream_'"${VERSION}"'_darwin_amd64.tar.gz/{print $1}' "$SUMS_FILE")"

if [[ -z "$ARM64_SHA" || -z "$AMD64_SHA" ]]; then
  echo "Could not extract darwin checksums from $SUMS_FILE"
  exit 1
fi

echo "==> Updating formula"
VERSION_ENV="$VERSION" ARM64_SHA_ENV="$ARM64_SHA" AMD64_SHA_ENV="$AMD64_SHA" FORMULA_ENV="$FORMULA_FILE" python3 - <<'PY'
import os
import re
from pathlib import Path

formula = Path(os.environ["FORMULA_ENV"])
version = os.environ["VERSION_ENV"]
arm = os.environ["ARM64_SHA_ENV"]
amd = os.environ["AMD64_SHA_ENV"]

text = formula.read_text()
text = re.sub(r'version\s+"[^"]+"', f'version "{version}"', text)
text = re.sub(
    r'(darwin_arm64\.tar\.gz"\n\s+sha256\s+")([^"]+)(")',
    rf'\g<1>{arm}\3',
    text,
    count=1,
)
text = re.sub(
    r'(darwin_amd64\.tar\.gz"\n\s+sha256\s+")([^"]+)(")',
    rf'\g<1>{amd}\3',
    text,
    count=1,
)
formula.write_text(text)
print(f"Updated {formula}")
PY

echo "==> Committing tap update"
git -C "$TAP_PATH" add Formula/kode-stream.rb
if git -C "$TAP_PATH" diff --cached --quiet; then
  echo "No formula changes to commit."
  exit 0
fi

git -C "$TAP_PATH" commit -m "kode-stream: update to v${VERSION}"
git -C "$TAP_PATH" push
echo "==> Done"
