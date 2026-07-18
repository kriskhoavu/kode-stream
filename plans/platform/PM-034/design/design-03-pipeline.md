# Pipeline Design: Chrome Extension Local Showcase

## Overview

The pipeline goal is local reproducibility, not public distribution. PM-034 adds commands and documentation that let a
developer build the extension, load it into Chrome, and run a focused acceptance pass against a running local server.

## Commands

| Command                   | Purpose                                                    |
|---------------------------|------------------------------------------------------------|
| `npm run build`           | Existing production build for embedded server assets.      |
| `npm run build:extension` | New extension artifact build into `dist/chrome-extension`. |
| `npm run typecheck`       | Existing frontend type safety gate.                        |
| `npm test -- --run`       | Existing frontend test gate.                               |
| `go test ./...`           | Backend regression gate for local API behavior.            |

## Manual Acceptance Flow

```text
Build extension artifact
    -> start kode-stream serve -port 4317
    -> load unpacked dist/chrome-extension in Chrome
    -> open extension app page
    -> run Files + Git showcase scenarios
    -> stop local server and confirm unavailable state
```

## Release Relationship

| Path             | PM-034 Behavior                                                         |
|------------------|-------------------------------------------------------------------------|
| GitHub release   | No new public artifact required for showcase.                           |
| Homebrew package | No change; users still install and run the local server normally.       |
| Chrome Web Store | Out of scope; evaluate only after unpacked extension acceptance passes. |
| Cloud deployment | No change; extension showcase targets Local mode only.                  |

## Design Decisions

| Decision                                 | Alternatives Considered                 | Rationale                                                           |
|------------------------------------------|-----------------------------------------|---------------------------------------------------------------------|
| Keep extension acceptance manual         | Add full browser automation immediately | Chrome extension automation can follow once the artifact is stable. |
| Run existing frontend and Go test gates  | Only manually load the extension        | Protects shared API and UI behavior while adding packaging.         |
| Avoid release-runbook changes in phase 1 | Publish extension artifact from day one | Showcase should prove value before changing public release process. |
