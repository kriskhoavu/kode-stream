# Chrome Extension Architecture

The Chrome extension is an unpacked Local-mode showcase. It packages the React UI as a Manifest V3 extension page.
All privileged work stays in a separately started local Kode Stream server, which keeps enforcing the guarded
filesystem and Git APIs.

```mermaid
flowchart LR
  chrome[Chrome extension page\nReact UI, Manifest V3] -->|HTTP JSON API| server[Local Kode Stream server\n127.0.0.1:4317]
  server --> guarded[Guarded workspace, Git,\nand system APIs]
  guarded --> repo[Registered local Git workspace]
  server --> storage{Local app-state storage}
  storage --> datadir[(datadir\nYAML / JSONL)]
  storage --> sqlite[(SQLite database)]
  chrome -. no direct file access .-> repo
```

Source: [chrome-extension.mmd](chrome-extension.mmd).

Origin rules and limits: [extension reference](../../../wiki/platform/extension-reference.md). Build, load and
troubleshooting: [Chrome extension showcase](../../../deploy/chrome-extension.md).
