# Homebrew Release Procedure

Canonical procedure for publishing a Kode Stream release and updating the Homebrew tap.

Last exercised end to end on **v2.0.0**. The troubleshooting section records failures that
actually occurred, not hypothetical ones.

## Scope

- Publish a GitHub release via CI.
- Point `kriskhoavu/homebrew-tap` at the new artifacts.
- Verify the formula installs before anyone else gets it.

## Prerequisites

- `git`, `gh`, `npm`, `go`, `python3`, `shasum`, `brew`.
- GitHub auth for `kriskhoavu/kode-stream` and `kriskhoavu/homebrew-tap` (`gh auth status`).
- Tap cloned locally, by default `../homebrew-tap`.

## Preflight

CI builds on Linux; local development is macOS. Everything below has broken a release before,
so check it rather than assuming.

```bash
go test ./...                 # must pass; see "macOS-only passes" in Troubleshooting
npm ci && npm run typecheck && npm test -- --run && npm run build
grep -o '"resolved": "https://[^/"]*' package-lock.json | sort -u  # only registry.npmjs.org
git ls-files --error-unmatch package-lock.json     # must be tracked
```

`package-lock.json` must be **committed** and must resolve from `registry.npmjs.org`. The
workflow uses `actions/setup-node` with `cache: npm` and installs with `npm ci`; both require
the lockfile, and a lockfile resolved against an internal mirror cannot be fetched from a
GitHub runner.

If your `~/.npmrc` points at a private registry, regenerate the lockfile in a clean directory
so those URLs never enter it:

```bash
mkdir /tmp/lockgen && cp package.json /tmp/lockgen/ && cd /tmp/lockgen
NPM_CONFIG_USERCONFIG=/dev/null npm install --package-lock-only \
  --registry=https://registry.npmjs.org/ --no-audit --no-fund
cp package-lock.json "$OLDPWD/" && cd "$OLDPWD"
```

Regenerating in place fails: npm reconciles against the existing `node_modules` and aborts
with `Cannot read properties of null (reading 'edgesOut')`.

## 1) Publish the release (CI)

Tagging is the whole trigger. `.github/workflows/release.yml` runs `verify`, builds
darwin arm64/amd64, linux amd64 and windows amd64, generates `SHA256SUMS`, and publishes.

```bash
git tag -a v<version> -m "kode-stream v<version>"
git push origin v<version>
gh run watch "$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')" \
  -R kriskhoavu/kode-stream --interval 30
```

To retry after a fix, move the tag — CI only triggers on the tag push:

```bash
git push origin :refs/tags/v<version> && git tag -d v<version>
# re-tag and push again
```

Iterate on workflow changes without burning tags using `gh workflow run release.yml --ref main`;
the `verify` and `build` jobs run, `publish` stays tag-gated.

Signing is opt-in. `sign-macos` and `sign-windows` are skipped unless the `MACOS_CERT_*` /
`WINDOWS_CERT_*` secrets exist — currently they do not, so **published binaries are unsigned**.
macOS users get a Gatekeeper prompt on first run.

## 2) Verify the release before touching the tap

Never trust the published `SHA256SUMS` blindly — recompute it:

```bash
mkdir /tmp/rel && cd /tmp/rel
gh release download v<version> -R kriskhoavu/kode-stream
shasum -a 256 kode-stream_<version>_* | diff - <(grep kode-stream SHA256SUMS) && echo MATCH
tar -xzf kode-stream_<version>_darwin_arm64.tar.gz && ./kode-stream   # prints Usage, exits 2
```

## 3) Update the tap

```bash
cmd/scripts/distribution/update_homebrew_formula_from_release.sh <version> ../homebrew-tap
```

Downloads `SHA256SUMS` from the release, rewrites the formula via `update_formula.py`, commits
and pushes. The updater rewrites the version **inside every release URL** and replaces all three
platform checksums together, and exits non-zero if any asset is missing from `SHA256SUMS`, if the
formula is unchanged, or if any URL is left on a different version. See "silent checksum
mismatch" in Troubleshooting for why that strictness exists.

## 4) Validate the published formula

Do this before telling anyone the release is out. `brew` caches taps, so refresh first.

```bash
brew untap kriskhoavu/tap 2>/dev/null
brew install kriskhoavu/tap/kode-stream
brew test kriskhoavu/tap/kode-stream
brew audit --strict --formula kriskhoavu/tap/kode-stream
kode-stream doctor
```

`brew audit --strict` must exit 0. Two rules the formula is shaped around:

- **no `version` line** — audit rejects it as redundant with the version scanned from the URL,
  which is why URLs carry the version literally instead of interpolating `#{version}`.
- **`desc` under 80 characters**, not starting with the formula name.

## Fallback: release without CI

`cmd/scripts/distribution/release_and_update_tap.sh <version> ../homebrew-tap` builds every
target locally and uploads them.

Use it only when Actions is unavailable. Pushing the tag triggers the workflow, which publishes
the same asset names, while the script uploads its own build with `--clobber`. Running both
against one tag is a race: the tap can end up carrying checksums from the build that lost. Let
the workflow finish, or cancel it, before the script uploads.

## Troubleshooting

**`Dependencies lock file is not found`** — `package-lock.json` is not committed. `setup-node`
fails before any build step. Commit it; do not switch the workflow to `npm install`.

**`npm error Exit handler never called!`** — npm's generic crash, almost never an npm bug here.
Read the debug log it points at; the real error is inside. On v2.0.0 it was hundreds of
`ENOTFOUND` on an internal mirror host, from a lockfile resolved against it rather than the
public registry. Surface it
with:

```yaml
run: |
  npm ci --no-audit --no-fund || { tail -n 150 /home/runner/.npm/_logs/*-debug-0.log; exit 1; }
```

Worse, on Node 20 the crash did **not** fail the step: the build continued against a
half-installed tree and failed later with `Cannot find module 'react'`. CI is pinned to Node 22.

**Tests that only pass on macOS** — timing-sensitive tests can pass locally and fail on the
faster Linux runner. Two were fixed for v2.0.0:

- an adapter-level test asserted a lock the adapter never takes (the *service* layer holds it);
  it passed on macOS only because `git switch` there outran the 50 ms window.
- `Manager.Close()` signalled processes without waiting for the goroutines it spawned, so session
  records were still being written while `t.TempDir()` cleanup ran.

Reproduce with `go test ./... -count=3 -race` before blaming the runner, and prefer asserting at
the layer that owns the guarantee.

**Silent checksum mismatch in the tap** — the old updater matched a `version "x.y.z"` line. When
that line was removed for `brew audit --strict`, it silently updated the checksums and left the
URLs on the previous release: a valid-looking formula where every install fails its checksum
check. `update_formula.py` now fails loudly instead. Always run step 4.

**`No available formula ... Did you mean kriskhoavu/tap/plan-manager?`** — a stale tap clone.
`brew untap kriskhoavu/tap` then reinstall, or
`git -C "$(brew --repository)/Library/Taps/kriskhoavu/homebrew-tap" fetch origin && git reset --hard origin/main`.

**`Homebrew requires formulae to be in a tap`** — you cannot `brew install` a formula by path.
Tap a local checkout to test before publishing:
`brew tap kriskhoavu/localtest /path/to/homebrew-tap`. `brew tap` clones at the current commit,
so commit first and re-tap after amending.

**`curl .../SHA256SUMS` returns 404** — the tag exists but assets do not; the workflow failed or
is still running.

## Cloud Release Checklist

- Build frontend assets with `npm run build`.
- Build and tag the Cloud image with `docker build -t kode-stream:<version> .`.
- Run `/api/health` against the image with `KODE_STREAM_STORAGE_OPTION=database`, Postgres, and required Cloud
  environment variables.
- Confirm `/api/health` reports the expected database `migrationVersion`.
- Confirm Cloud startup rejects `KODE_STREAM_STORAGE_OPTION=datadir`.
- Back up Postgres before upgrade and rehearse restore from snapshot or dump.
- Smoke local storage options with `KODE_STREAM_STORAGE_OPTION=database ./run.sh restart` and
  `KODE_STREAM_STORAGE_OPTION=datadir ./run.sh restart`, or use `./run.sh smoke-storage`.
- Smoke Settings manual sync in both directions and confirm a target backup appears under `backups/storage-sync/`.
- Smoke branch re-index by loading a non-current branch and switching back to the active branch.
- Verify reverse proxy WebSocket upgrade for `/api/agents/channel`.
- Verify the Homebrew package exposes `kode-stream agent start`, `status`, and `doctor`.
- Smoke `kodestream://connect` deep-link registration on macOS.
- Connect Cloud Agent, register a local repository, and confirm Cloud shows redacted path metadata.
- Confirm hosted Git, terminal, AI, runtime, and verification routes do not execute without the owner agent.
- Back up and restore `KODE_STREAM_DATA_DIR` during upgrade and rollback rehearsal.

## Related Files

- `.github/workflows/release.yml` — builds and publishes; the tag is the trigger
- `cmd/scripts/distribution/update_formula.py` — rewrites URLs and checksums, fails loudly
- `cmd/scripts/distribution/update_homebrew_formula_from_release.sh` — tap update from a release
- `cmd/scripts/distribution/release_and_update_tap.sh` — local-build fallback, races CI
- `cmd/scripts/distribution/kode-stream.rb` — formula template for bootstrap
- `deploy/homebrew/homebrew-tap-bootstrap.md`
