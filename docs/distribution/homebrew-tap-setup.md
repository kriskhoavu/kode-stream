# Homebrew Tap Setup

Use this page for first-time tap bootstrap. Day-to-day releases now live in the release runbook:

- `docs/release/release-runbook.md`

## 1) Create the tap repository

```bash
gh repo create kriskhoavu/homebrew-tap --public --description "Homebrew tap for kode-stream"
```

## 2) Initialize tap structure

```bash
git clone https://github.com/kriskhoavu/homebrew-tap.git
mkdir -p homebrew-tap/Formula
cp packaging/homebrew/Formula/kode-stream.rb homebrew-tap/Formula/kode-stream.rb
```

## 3) First publish guidance

After bootstrap, run release automation from `kode-stream` root:

```bash
../cmd/scripts/distribution/release_and_update_tap.sh <version> ../homebrew-tap
```
