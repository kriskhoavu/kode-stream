#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "Usage: cmd/scripts/distribution/release_and_update_tap.sh <version> [tap-path]"
  echo "Example: cmd/scripts/distribution/release_and_update_tap.sh 1.0.0 ../homebrew-tap"
  exit 2
fi

if [[ "$VERSION" == v* ]]; then
  VERSION="${VERSION#v}"
fi

TAP_PATH="${2:-../homebrew-tap}"
TAG="v${VERSION}"
OUT_DIR="$ROOT_DIR/release/${VERSION}"
SUMS_FILE="/tmp/SHA256SUMS-v${VERSION}"
FORMULA_FILE="$TAP_PATH/Formula/kode-stream.rb"
REPO="kriskhoavu/kode-stream"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1"
    exit 1
  fi
}

echo "==> Checking required tools"
for c in git gh npm go python3 awk shasum zip tar; do
  require_cmd "$c"
done

if [[ ! -d "$TAP_PATH/.git" ]]; then
  echo "Tap repo not found at: $TAP_PATH"
  exit 1
fi

if [[ ! -f "$FORMULA_FILE" ]]; then
  echo "Formula file not found at: $FORMULA_FILE"
  exit 1
fi

echo "==> Building release artifacts for $TAG"
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

if [[ -f "$ROOT_DIR/package-lock.json" ]]; then
  npm ci
else
  echo "No package-lock.json found; using npm install"
  npm install
fi
npm run build

for target in "darwin arm64" "darwin amd64" "linux amd64" "windows amd64"; do
  GOOS="$(awk '{print $1}' <<<"$target")"
  GOARCH="$(awk '{print $2}' <<<"$target")"

  workdir="$(mktemp -d)"
  bin_name="kode-stream"
  if [[ "$GOOS" == "windows" ]]; then
    bin_name="kode-stream.exe"
  fi

  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
    go build -trimpath -ldflags "-s -w" -o "$workdir/$bin_name" ./cmd/kode-stream

  cp "$ROOT_DIR/README.md" "$workdir/"
  if compgen -G "$ROOT_DIR/LICENSE*" >/dev/null; then
    cp "$ROOT_DIR"/LICENSE* "$workdir/"
  fi

  base="kode-stream_${VERSION}_${GOOS}_${GOARCH}"
  if [[ "$GOOS" == "windows" ]]; then
    (cd "$workdir" && zip -qr "$OUT_DIR/${base}.zip" .)
  else
    tar -C "$workdir" -czf "$OUT_DIR/${base}.tar.gz" .
  fi

  rm -rf "$workdir"
done

echo "==> Generating checksums"
(
  cd "$OUT_DIR"
  shasum -a 256 kode-stream_* > SHA256SUMS
)

echo "==> Ensuring git tag exists on origin: $TAG"
if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Tag already exists locally: $TAG"
else
  git tag "$TAG"
fi

if git ls-remote --exit-code --tags origin "refs/tags/$TAG" >/dev/null 2>&1; then
  echo "Tag already exists on origin: $TAG"
else
  git push origin "$TAG"
fi

echo "==> Publishing GitHub release assets"
if gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then
  gh release upload "$TAG" "$OUT_DIR"/kode-stream_* "$OUT_DIR"/SHA256SUMS --repo "$REPO" --clobber
else
  gh release create "$TAG" "$OUT_DIR"/kode-stream_* "$OUT_DIR"/SHA256SUMS --repo "$REPO" --title "$TAG" --generate-notes
fi

echo "==> Downloading release SHA256SUMS"
curl -fL "https://github.com/kriskhoavu/kode-stream/releases/download/${TAG}/SHA256SUMS" -o "$SUMS_FILE"

ARM64_SHA="$(awk '/kode-stream_'"${VERSION}"'_darwin_arm64.tar.gz/{print $1}' "$SUMS_FILE")"
AMD64_SHA="$(awk '/kode-stream_'"${VERSION}"'_darwin_amd64.tar.gz/{print $1}' "$SUMS_FILE")"

if [[ -z "$ARM64_SHA" || -z "$AMD64_SHA" ]]; then
  echo "Could not extract darwin checksums from $SUMS_FILE"
  exit 1
fi

echo "==> Updating Homebrew formula"
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

echo "==> Committing and pushing tap update"
git -C "$TAP_PATH" add Formula/kode-stream.rb
if git -C "$TAP_PATH" diff --cached --quiet; then
  echo "No tap changes to commit."
else
  git -C "$TAP_PATH" commit -m "kode-stream: update to v${VERSION}"
  git -C "$TAP_PATH" push
fi

echo "==> Done"
echo "Release assets: $OUT_DIR"
echo "Checksums file: $SUMS_FILE"
echo "Next: brew tap kriskhoavu/homebrew-tap && brew install kode-stream"
