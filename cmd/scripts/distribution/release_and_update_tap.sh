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

cat <<'WARNING'
NOTE: pushing a v* tag triggers .github/workflows/release.yml, which builds and
publishes the release assets itself. This script also builds locally and uploads
with --clobber, so running both against one tag races: whichever finishes last
wins, and the tap can end up pointing at assets whose checksums came from the
other build. Prefer the CI path in deploy/homebrew/release.md. Use this script
only when Actions is unavailable, and let the workflow finish or cancel it first.
WARNING

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

# package-lock.json is committed. Falling back to npm install here would hide a
# missing or unusable lockfile, which is exactly what broke the release workflow.
npm ci
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

echo "==> Updating Homebrew formula"
python3 "$ROOT_DIR/cmd/scripts/distribution/update_formula.py" "$FORMULA_FILE" "$VERSION" "$SUMS_FILE"

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
