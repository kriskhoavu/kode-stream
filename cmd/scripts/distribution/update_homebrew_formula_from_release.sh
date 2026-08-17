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

echo "==> Updating formula"
python3 "$ROOT_DIR/cmd/scripts/distribution/update_formula.py" "$FORMULA_FILE" "$VERSION" "$SUMS_FILE"

echo "==> Committing tap update"
git -C "$TAP_PATH" add Formula/kode-stream.rb
if git -C "$TAP_PATH" diff --cached --quiet; then
  echo "No formula changes to commit."
  exit 0
fi

git -C "$TAP_PATH" commit -m "kode-stream: update to v${VERSION}"
git -C "$TAP_PATH" push
echo "==> Done"
