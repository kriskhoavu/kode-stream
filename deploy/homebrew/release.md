# Homebrew Release Procedure

Publish a GitHub release through CI, point `kriskhoavu/homebrew-tap` at it, and check the formula installs before
anyone else gets it.

## Prerequisites

- `git`, `gh`, `npm`, `go`, `python3`, `shasum`, `brew`.
- GitHub auth for `kriskhoavu/kode-stream` and `kriskhoavu/homebrew-tap` (`gh auth status`).
- The tap cloned locally, by default at `../homebrew-tap`.

## Preflight

CI builds on Linux. Each check guards a known release failure.

```bash
go test ./... -count=3 -race   # catches timing tests that pass only on macOS
npm ci && npm run typecheck && npm test -- --run && npm run build
grep -o '"resolved": "https://[^/"]*' package-lock.json | sort -u  # must be registry.npmjs.org only
git ls-files --error-unmatch package-lock.json                     # must be tracked
```

CI installs with `npm ci` and caches with `setup-node`, so the lockfile must be committed and resolve from the public
registry. If your `~/.npmrc` points at a private registry, regenerate the lockfile in a clean directory:

```bash
mkdir /tmp/lockgen && cp package.json /tmp/lockgen/ && cd /tmp/lockgen
NPM_CONFIG_USERCONFIG=/dev/null npm install --package-lock-only \
  --registry=https://registry.npmjs.org/ --no-audit --no-fund
cp package-lock.json "$OLDPWD/" && cd "$OLDPWD"
```

Regenerating in place fails with `Cannot read properties of null (reading 'edgesOut')`.

## 1. Publish The Release

The tag push triggers `.github/workflows/release.yml`. It runs `verify`, builds darwin arm64 and amd64, linux amd64 and
windows amd64, writes `SHA256SUMS` and publishes.

```bash
git tag -a v<version> -m "kode-stream v<version>"
git push origin v<version>
gh run watch "$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')" \
  -R kriskhoavu/kode-stream --interval 30
```

To retry after a fix, move the tag:

```bash
git push origin :refs/tags/v<version> && git tag -d v<version>   # then tag and push again
```

`gh workflow run release.yml --ref main` runs `verify` and `build` without a tag. `publish` needs the tag.

Signing runs only when the `MACOS_CERT_*` or `WINDOWS_CERT_*` secrets exist. They do not, so binaries are unsigned and
macOS shows a Gatekeeper prompt on first run.

## 2. Verify The Release

Recompute the checksums yourself:

```bash
mkdir /tmp/rel && cd /tmp/rel
gh release download v<version> -R kriskhoavu/kode-stream
shasum -a 256 kode-stream_<version>_* | diff - <(grep kode-stream SHA256SUMS) && echo MATCH
tar -xzf kode-stream_<version>_darwin_arm64.tar.gz && ./kode-stream   # prints Usage, exits 2
```

## 3. Update The Tap

```bash
cmd/scripts/distribution/update_homebrew_formula_from_release.sh <version> ../homebrew-tap
```

The script downloads `SHA256SUMS`, rewrites the formula with `update_formula.py`, commits and pushes. It rewrites the
version in every URL and all checksums together. It fails if an asset is missing, the formula is unchanged, or any URL
stays on another version.

## 4. Validate The Formula

```bash
brew untap kriskhoavu/tap 2>/dev/null
brew install kriskhoavu/tap/kode-stream
brew test kriskhoavu/tap/kode-stream
brew audit --strict --formula kriskhoavu/tap/kode-stream   # must exit 0
kode-stream doctor
```

The formula has no `version` line, because the audit rejects it as redundant. So URLs carry the version literally.
`desc` stays under 80 characters and does not start with the formula name.

## Fallback Without CI

`cmd/scripts/distribution/release_and_update_tap.sh <version> ../homebrew-tap` builds every target locally and uploads
with `--clobber`. Use it only when Actions is down. If the tag push also started the workflow, let it finish or cancel
it first, or the tap may carry checksums from the losing build.

## Troubleshooting

| Error                                         | Cause and fix                                                                                  |
|-----------------------------------------------|------------------------------------------------------------------------------------------------|
| `Dependencies lock file is not found`         | `package-lock.json` is not committed. Commit it. Do not switch CI to `npm install`              |
| `npm error Exit handler never called!`        | Read the debug log it names. A lockfile from a private mirror shows as `ENOTFOUND` errors. On Node 20 the step passes and the build later fails with `Cannot find module 'react'`, so CI pins Node 22 |
| Tests pass on macOS, fail in CI               | A test asserts a guarantee at the wrong layer, or shutdown does not wait for its goroutines. Reproduce with `-count=3 -race` |
| Every install fails its checksum              | A URL stayed on the old version. `update_formula.py` refuses this. Always run step 4           |
| `No available formula ... Did you mean ...?`  | A stale tap clone. `brew untap kriskhoavu/tap` and reinstall                                    |
| `Homebrew requires formulae to be in a tap`   | Tap a local checkout: `brew tap kriskhoavu/localtest /path/to/homebrew-tap`. Commit first      |
| `SHA256SUMS` returns 404                      | The workflow failed or is still running                                                        |

## Cloud Image

1. `npm run build`, then `docker build -f deploy/docker/image/kode-stream/Dockerfile -t kode-stream:<version> .`.
2. Start it on Postgres with the Cloud environment. Check `/api/health` and the expected `migrationVersion`.
3. Confirm startup rejects `KODE_STREAM_STORAGE_OPTION=datadir`.
4. Run `./run.sh smoke-storage` and Settings sync in both directions. A backup must appear under
   `backups/storage-sync/`.
5. Follow [Cloud deployment](../cloud-deployment.md) for proxy, Agent and upgrade checks.

## Related Files

- `.github/workflows/release.yml`: builds and publishes on a tag
- `cmd/scripts/distribution/update_formula.py`: rewrites URLs and checksums
- `cmd/scripts/distribution/update_homebrew_formula_from_release.sh`: updates the tap from a release
- `cmd/scripts/distribution/release_and_update_tap.sh`: local-build fallback
- `cmd/scripts/distribution/kode-stream.rb`: formula template
