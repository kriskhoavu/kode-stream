# Infrastructure Design: Chrome Extension Local Showcase

## Overview

PM-034 adds a local-only extension packaging path. The normal production build continues to emit assets for Go embed.
The new extension build emits a separate unpacked directory that Chrome can load manually from `chrome://extensions`.

## Build Artifacts

| Artifact                   | Producer            | Consumer                  | Purpose                                         |
|----------------------------|---------------------|---------------------------|-------------------------------------------------|
| `internal/server/frontend` | Existing Vite build | Go embedded local server  | Normal localhost app delivery.                  |
| `dist/chrome-extension`    | Extension build     | Chrome Load unpacked flow | Showcase extension page and MV3 manifest.       |
| Extension manifest         | Build script/file   | Chrome extension runtime  | Declares app page, icons, and localhost access. |

## Extension Manifest Requirements

| Area             | Requirement                                                            |
|------------------|------------------------------------------------------------------------|
| Manifest version | MV3.                                                                   |
| Host access      | Allow `http://127.0.0.1:4317/*` and `http://localhost:4317/*`.         |
| Permissions      | Keep minimal; use only extension storage if an options page is added.  |
| File URLs        | Do not require `Allow access to file URLs` for v1.                     |
| Downloads        | Do not require `Manage downloads` for v1.                              |
| Entry page       | Open the bundled app page in a normal extension tab or popup launcher. |

## Local Server Contract

| Contract          | Requirement                                                                  |
|-------------------|------------------------------------------------------------------------------|
| Bind address      | Local mode continues to bind to `127.0.0.1` by default.                      |
| Default port      | Extension defaults to `4317`.                                                |
| CORS              | Local API must allow the extension origin for JSON/SSE fetches if required.  |
| WebSocket         | Embedded AI session channel remains out of scope.                            |
| Security boundary | The server remains responsible for path validation and Git command guarding. |

## Design Decisions

| Decision                                     | Alternatives Considered            | Rationale                                                                    |
|----------------------------------------------|------------------------------------|------------------------------------------------------------------------------|
| Build extension as unpacked local artifact   | Store release or signed package    | Fits a showcase and avoids external publishing steps.                        |
| Keep localhost and extension builds separate | Replace the normal build           | Avoids regressions in Go embedded assets and Homebrew release packaging.     |
| Do not request file URL or downloads access  | Match MarkView permissions exactly | Kode Stream accesses workspace content through the local API, not `file://`. |
| Treat CORS as local-mode-only                | Broad CORS for all runtime modes   | Keeps Cloud mode locked behind its existing auth and proxy assumptions.      |
