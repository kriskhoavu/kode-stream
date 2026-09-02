# Homebrew Tap Bootstrap

First-time tap setup only. Day-to-day releases live in `deploy/homebrew/release.md`.

## 1) Create the tap repository

The repo must be named `homebrew-<name>`; `brew tap kriskhoavu/tap` resolves to
`kriskhoavu/homebrew-tap`.

```bash
gh repo create kriskhoavu/homebrew-tap --public --description "Homebrew tap for kode-stream"
```

## 2) Seed the formula

```bash
git clone https://github.com/kriskhoavu/homebrew-tap.git
mkdir -p homebrew-tap/Formula
cp cmd/scripts/distribution/kode-stream.rb homebrew-tap/Formula/kode-stream.rb
```

The template ships placeholder checksums and `v0.0.0` URLs. It is not installable until
`update_formula.py` points it at a real release, which the release runbook does for you.

The formula class name is derived from the filename: `kode-stream.rb` must declare
`class KodeStream`.

## 3) First publish

Publish a release first (`deploy/homebrew/release.md` step 1), then point the tap at it:

```bash
cmd/scripts/distribution/update_homebrew_formula_from_release.sh <version> ../homebrew-tap
```

## 4) Verify before announcing

```bash
brew install kriskhoavu/tap/kode-stream
brew test kriskhoavu/tap/kode-stream
brew audit --strict --formula kriskhoavu/tap/kode-stream
```

To test formula edits before pushing, tap a local checkout — Homebrew refuses to install a
formula by path, and `brew tap` clones at the current commit, so commit first:

```bash
brew tap kriskhoavu/localtest /path/to/homebrew-tap
brew install kriskhoavu/localtest/kode-stream
brew untap kriskhoavu/localtest
```

## Retiring an old formula

`plan-manager.rb` was removed when the project was renamed. Deleting a formula breaks
`brew upgrade` for anyone who installed it; `deprecate!` with a migration note is gentler if the
old name has users.
