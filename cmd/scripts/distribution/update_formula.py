#!/usr/bin/env python3
"""Point the Homebrew formula at a released version.

Rewrites the version inside every release URL and replaces the sha256 that
follows each platform's url line. Every platform present in the formula must
have a checksum in SHA256SUMS, and the file must actually change, otherwise
this exits non-zero.

That strictness is deliberate. An earlier version matched a `version "x.y.z"`
line that no longer exists in the formula, so it silently updated the
checksums while leaving the URLs on the previous release. The formula stayed
syntactically valid and every `brew install` failed its checksum check.

Usage: update_formula.py <formula-path> <version> <sha256sums-path>
"""

import re
import sys
from pathlib import Path

PLATFORMS = ("darwin_arm64", "darwin_amd64", "linux_amd64")
ARCHIVE = {"darwin_arm64": "tar.gz", "darwin_amd64": "tar.gz", "linux_amd64": "tar.gz"}


def parse_sums(path: Path) -> dict[str, str]:
    sums: dict[str, str] = {}
    for line in path.read_text().splitlines():
        parts = line.split()
        if len(parts) == 2:
            sums[parts[1]] = parts[0]
    return sums


def main() -> int:
    if len(sys.argv) != 4:
        print(__doc__.strip())
        return 2
    formula_path, version, sums_path = Path(sys.argv[1]), sys.argv[2].lstrip("v"), Path(sys.argv[3])

    text = original = formula_path.read_text()
    sums = parse_sums(sums_path)

    # Move every release URL onto the new version.
    text = re.sub(
        r"/releases/download/v[0-9][^/]*/kode-stream_[0-9][^_]*_",
        f"/releases/download/v{version}/kode-stream_{version}_",
        text,
    )

    updated = []
    for platform in PLATFORMS:
        pattern = re.compile(
            r'(kode-stream_' + re.escape(version) + r'_' + platform
            + r'\.' + ARCHIVE[platform] + r'"\n\s+sha256\s+")([^"]+)(")'
        )
        if not pattern.search(text):
            continue  # formula does not ship this platform
        asset = f"kode-stream_{version}_{platform}.{ARCHIVE[platform]}"
        checksum = sums.get(asset)
        if not checksum:
            print(f"error: {asset} missing from {sums_path}", file=sys.stderr)
            return 1
        text = pattern.sub(lambda m, c=checksum: m.group(1) + c + m.group(3), text, count=1)
        updated.append(platform)

    if not updated:
        print("error: formula contains no recognised kode-stream release URLs", file=sys.stderr)
        return 1
    if text == original:
        print(f"error: formula already matches v{version}; nothing to publish", file=sys.stderr)
        return 1

    stale = re.findall(r"/releases/download/v([0-9][^/]*)/", text)
    if any(s != version for s in stale):
        print(f"error: formula still references other versions: {sorted(set(stale))}", file=sys.stderr)
        return 1

    formula_path.write_text(text)
    print(f"Updated {formula_path} to v{version} ({', '.join(updated)})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
