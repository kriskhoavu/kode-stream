# Implementation Plan: PM-034 - Chrome Extension Local Showcase

## Overview

Implement a local unpacked Chrome extension showcase for Kode Stream. The extension bundles the existing React app,
routes API calls to the local server, and verifies Files + Git workflows against registered local workspaces. The Go
server stays responsible for filesystem and Git operations.

## Terminology Lock

All code, fields, API params, and docs must use:

- `Extension Surface`
- `Local API Origin`
- `API Origin Adapter`
- `Unpacked Extension`
- `Chrome Extension Showcase`

Avoid:

- `pure extension`
- `direct Git in Chrome`
- `file URL mode`
- `native messaging`
- `Chrome app`

## Phases Summary

| Phase | Name                                    | Track    | Status |
|-------|-----------------------------------------|----------|--------|
| F1    | API origin adapter                      | Frontend | Done   |
| F2    | Extension surface behavior              | Frontend | Done   |
| C1    | Extension build artifact                | DevOps   | Done   |
| C2    | Showcase verification and documentation | DevOps   | Draft  |

## Frontend Phases

### Phase F1: API Origin Adapter

**Deliverables:**

- [x] Add a shared API URL resolver used by all frontend API requests.
- [x] Keep relative `/api/*` behavior for localhost and Vite dev surfaces.
- [x] Resolve extension-surface API calls to `localStorage.kodeStreamApiOrigin` or `http://127.0.0.1:4317`.
- [x] Update direct fetch helpers and attachment URLs to use the resolver.
- [x] Add focused tests for URL resolution and representative Files + Git endpoints.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run web/src/shared/api/index.test.ts`

**Commit:** `PM-034: Add extension API origin adapter`

---

### Phase F2: Extension Surface Behavior

**Deliverables:**

- [x] Detect the extension surface without affecting normal local web mode.
- [x] Add a local-server health check state for extension startup.
- [x] Show a clear unavailable state when the configured local API origin is not reachable.
- [x] Hide or disable embedded terminal/AI streaming controls in extension mode.
- [x] Add component tests for unavailable state and unsupported streaming controls.

**Verification:** `rtk npm run typecheck && rtk npm test -- --run`

**Commit:** `PM-034: Add extension surface guards`

---

## DevOps Phases

### Phase C1: Extension Build Artifact

**Deliverables:**

- [x] Add an extension Vite build mode with relative asset paths and separate output directory.
- [x] Add an MV3 manifest for local unpacked loading.
- [x] Add `npm run build:extension`.
- [x] Ensure the normal `npm run build` output for Go embed remains unchanged.
- [x] Confirm the extension artifact can be loaded with Chrome Developer Mode.

**Verification:** `rtk npm run build && rtk npm run build:extension`

**Commit:** `PM-034: Add unpacked extension build`

---

### Phase C2: Showcase Verification And Documentation

**Deliverables:**

- [ ] Document manual load steps for `dist/chrome-extension`.
- [ ] Document local server startup and configurable API origin.
- [ ] Document Files + Git acceptance scenarios.
- [ ] Document v1 limits: no direct file URL permission, no downloads permission, no native messaging, no embedded terminal streaming.
- [ ] Add troubleshooting for stopped server, wrong port, and localhost permission issues.

**Verification:** `rtk npm run build:extension && rtk go test ./...`

**Commit:** `PM-034: Document Chrome extension showcase`

## Post-Implementation Checklist

- [ ] Update `plans/platform/PM-034/` docs to reflect final file names and commands.
- [ ] Run Markdown formatting on all PM-034 Markdown files.
- [ ] Run `rtk npm run typecheck`.
- [ ] Run `rtk npm test -- --run`.
- [ ] Run `rtk go test ./...`.
- [ ] PR description references PM-034 planning docs.
