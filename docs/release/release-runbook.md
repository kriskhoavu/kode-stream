# Release Runbook

Canonical runbook for packaging Plan Manager, publishing GitHub releases, and updating Homebrew tap metadata.

## Scope

- Build and package cross-platform artifacts.
- Publish or update a GitHub release tag.
- Generate and use `SHA256SUMS`.
- Update Homebrew formula with the release version and macOS checksums.

## Prerequisites

- `git`, `gh`, `npm`, `go`, `python3`, `zip`, `tar`, `shasum`.
- GitHub authentication for `kriskhoavu/plan-manager` and `kriskhoavu/homebrew-tap`.
- Tap repo cloned as sibling directory: `../homebrew-tap`.

## Primary Automation

Use the one-shot script from the `plan-manager` repository root:

```bash
./scripts/distribution/release_and_update_tap.sh 1.0.0 ../homebrew-tap
```

What it does:

1. Builds frontend and cross-platform binaries.
2. Packages release artifacts into `release/<version>/`.
3. Generates `SHA256SUMS`.
4. Creates/pushes `v<version>` tag if needed.
5. Creates or updates GitHub release assets.
6. Downloads `SHA256SUMS` from the release.
7. Updates `../homebrew-tap/Formula/plan-manager.rb` checksums.
8. Commits and pushes the tap formula update.

## Homebrew-only Helper

If release assets already exist and you only need to update the tap formula:

```bash
./scripts/distribution/update_homebrew_formula_from_release.sh 1.0.0 ../homebrew-tap
```

This script:

- pulls `SHA256SUMS` from `v<version>` release,
- updates formula `version`, `darwin_arm64` and `darwin_amd64` `sha256` values,
- commits and pushes the tap update.

## Validation

```bash
brew update
brew tap kriskhoavu/homebrew-tap
brew install plan-manager
plan-manager doctor
brew test plan-manager
```

## Troubleshooting

- `curl .../SHA256SUMS` returns 404: release tag exists but assets are missing.
- `npm ci` fails due to missing lockfile: script falls back to `npm install`.
- no tap commit created: formula already matches target version and checksums.

## Related Files

- `scripts/distribution/release_and_update_tap.sh`
- `scripts/distribution/update_homebrew_formula_from_release.sh`
- `packaging/homebrew/Formula/plan-manager.rb`
- `docs/distribution/homebrew-tap-setup.md`
