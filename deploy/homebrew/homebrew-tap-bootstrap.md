# Homebrew Tap Bootstrap

First-time tap setup. Releases follow [release.md](release.md).

1. Create the tap. The name must be `homebrew-<name>`, so `brew tap kriskhoavu/tap` resolves to
   `kriskhoavu/homebrew-tap`:

   ```bash
   gh repo create kriskhoavu/homebrew-tap --public --description "Homebrew tap for kode-stream"
   ```

2. Seed the formula:

   ```bash
   git clone https://github.com/kriskhoavu/homebrew-tap.git
   mkdir -p homebrew-tap/Formula
   cp cmd/scripts/distribution/kode-stream.rb homebrew-tap/Formula/kode-stream.rb
   ```

   The template has placeholder checksums and `v0.0.0` URLs. `kode-stream.rb` must declare `class KodeStream`.

3. Publish a release ([release.md](release.md) step 1), then point the tap at it:

   ```bash
   cmd/scripts/distribution/update_homebrew_formula_from_release.sh <version> ../homebrew-tap
   ```

4. Verify before announcing:

   ```bash
   brew install kriskhoavu/tap/kode-stream
   brew test kriskhoavu/tap/kode-stream
   brew audit --strict --formula kriskhoavu/tap/kode-stream
   ```

To test formula edits before pushing, commit them, then tap the local checkout:

```bash
brew tap kriskhoavu/localtest /path/to/homebrew-tap
brew install kriskhoavu/localtest/kode-stream
brew untap kriskhoavu/localtest
```

The tap carries only `kode-stream.rb`. Deleting a formula breaks `brew upgrade` for its users. Prefer `deprecate!`
with a migration note.
