# Frontend Design: Chrome Extension Local Showcase

## Overview

The existing React app remains the only user interface. PM-034 adds a build/runtime surface that lets the same app run
from a Chrome extension page. The UI resolves all API paths through a shared origin adapter so extension-hosted pages
can call the local Kode Stream server.

## Runtime Surfaces

| Surface          | Page Origin              | API Origin Resolution                                         | Supported Scope                   |
|------------------|--------------------------|---------------------------------------------------------------|-----------------------------------|
| Local web        | `http://127.0.0.1:4317`  | Relative `/api/*` paths                                       | Existing full local mode          |
| Vite dev         | `http://127.0.0.1:5173`  | Vite proxy or relative dev behavior                           | Existing development behavior     |
| Chrome extension | `chrome-extension://...` | `localStorage.kodeStreamApiOrigin` or `http://127.0.0.1:4317` | Files + Git showcase, no terminal |

## API Origin Adapter

| Concern           | Behavior                                                                                  |
|-------------------|-------------------------------------------------------------------------------------------|
| Detection         | Use Vite build metadata and/or `location.protocol === "chrome-extension:"`.               |
| Path resolution   | Convert `/api/...` to absolute local API URLs only for extension surface.                 |
| Config override   | Read `localStorage.kodeStreamApiOrigin` and trim trailing slash when present.             |
| Default           | Use `http://127.0.0.1:4317`.                                                              |
| Shared coverage   | Apply to JSON requests, streaming workspace creation fetch, and Jira attachment URLs.     |
| Failure messaging | Surface health failure as “Kode Stream local server unavailable” with the configured URL. |

## UI Behavior

| Area                  | Extension Behavior                                                                 |
|-----------------------|------------------------------------------------------------------------------------|
| App shell             | Same navigation, workspace switcher, theme, search, and settings layout.           |
| Workspaces            | Add, import, scan, reveal path, and config operations continue through local API.  |
| Workstream Explorer   | File tree, preview, editor, diff, path mutations, Git status, and branch selector. |
| Item workspace        | Markdown edit/save, metadata, status updates, diff, and Git panels.                |
| Embedded terminal/AI  | Controls that require WebSocket streaming are hidden or marked unavailable.        |
| Unavailable local API | Show a compact recovery state before rendering workspace-dependent pages.          |

## Design Decisions

| Decision                                     | Rationale                                                                            |
|----------------------------------------------|--------------------------------------------------------------------------------------|
| Reuse the existing SPA                       | Keeps one application surface and avoids a parallel extension-only product.          |
| Centralize URL resolution in the API layer   | Prevents individual pages from branching on extension mode.                          |
| Do not use Chrome File System Access in v1   | Existing guarded file APIs already match Kode Stream’s safety model.                 |
| Do not implement JS Git in the extension     | Local Git behavior depends on the existing Git adapter, credentials, and safeguards. |
| Disable terminal streaming in extension mode | Current WebSocket origin policy is same-origin and should get separate review.       |

## Acceptance Criteria

| Area           | Criteria                                                                                   |
|----------------|--------------------------------------------------------------------------------------------|
| Packaging      | Extension build emits loadable MV3 assets with relative asset paths.                       |
| API calls      | Extension-hosted UI loads `/api/state`, workspaces, files, and Git endpoints successfully. |
| File editing   | Save and stale-content behavior matches localhost UI.                                      |
| Git operations | Status, branches, and clean branch switch work from the extension UI.                      |
| Recovery       | Stopped local server produces an actionable unavailable state.                             |
