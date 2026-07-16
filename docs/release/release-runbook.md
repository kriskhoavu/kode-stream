# Release Runbook

Canonical runbook for packaging Kode Stream, publishing GitHub releases, and updating Homebrew tap metadata.

## Scope

- Build and package cross-platform artifacts.
- Publish or update a GitHub release tag.
- Generate and use `SHA256SUMS`.
- Update Homebrew formula with the release version and macOS checksums.

## Prerequisites

- `git`, `gh`, `npm`, `go`, `python3`, `zip`, `tar`, `shasum`.
- GitHub authentication for `kriskhoavu/kode-stream` and `kriskhoavu/homebrew-tap`.
- Tap repo cloned as sibling directory: `../homebrew-tap`.

## Primary Automation

Use the one-shot script from the `kode-stream` repository root:

```bash
../cmd/scripts/distribution/release_and_update_tap.sh 1.0.0 ../homebrew-tap
```

What it does:

1. Builds frontend and cross-platform binaries.
2. Packages release artifacts into `release/<version>/`.
3. Generates `SHA256SUMS`.
4. Creates/pushes `v<version>` tag if needed.
5. Creates or updates GitHub release assets.
6. Downloads `SHA256SUMS` from the release.
7. Updates `../homebrew-tap/Formula/kode-stream.rb` checksums.
8. Commits and pushes the tap formula update.

## Homebrew-only Helper

If release assets already exist and you only need to update the tap formula:

```bash
../cmd/scripts/distribution/update_homebrew_formula_from_release.sh 1.0.0 ../homebrew-tap
```

This script:

- pulls `SHA256SUMS` from `v<version>` release,
- updates formula `version`, `darwin_arm64` and `darwin_amd64` `sha256` values,
- commits and pushes the tap update.

## Validation

```bash
brew update
brew tap kriskhoavu/homebrew-tap
brew install kode-stream
kode-stream doctor
kode-stream agent doctor --cloud-url https://kode-stream.example.com --repo /path/to/repo
brew test kode-stream
```

## Cloud Release Checklist

- Build frontend assets with `npm run build`.
- Build and tag the Cloud image with `docker build -t kode-stream:<version> .`.
- Run `/api/health` against the image with required Cloud and Postgres environment variables.
- Confirm `/api/health` reports the expected database `migrationVersion`.
- Back up Postgres before upgrade and rehearse restore from snapshot or dump.
- Smoke branch re-index by loading a non-current branch and switching back to the active branch.
- Verify reverse proxy WebSocket upgrade for `/api/agents/channel`.
- Verify the Homebrew package exposes `kode-stream agent start`, `status`, and `doctor`.
- Smoke `kodestream://connect` deep-link registration on macOS.
- Connect Cloud Agent, register a local repository, and confirm Cloud shows redacted path metadata.
- Confirm hosted Git, terminal, AI, runtime, and verification routes do not execute without the owner agent.
- Back up and restore `KODE_STREAM_DATA_DIR` during upgrade and rollback rehearsal.

## Troubleshooting

- `curl .../SHA256SUMS` returns 404: release tag exists but assets are missing.
- `npm ci` fails due to missing lockfile: script falls back to `npm install`.
- no tap commit created: formula already matches target version and checksums.
- Cloud agent disconnected: check WebSocket proxy upgrade and idle timeout settings.
- Cloud role denied: check `KODE_STREAM_ADMIN_USERS` and OIDC email/subject claims.
- Cloud image unhealthy: check required Cloud environment variables and `/api/health`.

## Related Files

- `cmd/scripts/distribution/release_and_update_tap.sh`
- `cmd/scripts/distribution/update_homebrew_formula_from_release.sh`
- `packaging/homebrew/Formula/kode-stream.rb`
- `docs/distribution/homebrew-tap-setup.md`
