---
slug: platform-extension-reference
title: Browser Extension Reference
pageType: REFERENCE
topics: extension, browser, local, cors, packaging
summary: Reference for the unpacked Chromium extension surface and its Local API boundary.
sourceRef: |
  PM-034 (plan retired) | Extension surface
  PM-034 (plan retired) | Frontend build
  PM-034 (plan retired) | Local transport
  PM-034 (plan retired) | Packaging
  PM-034 (plan retired) | Delivery
lastVerified: 2026-09-12
---

## Distribution And Origin
<!-- chunkId: platform-extension-reference-distribution-origin -->
<!-- keywords: unpacked extension, manifest v3, api origin -->

[live] The Chromium surface builds as an unpacked Manifest V3 extension from the same frontend. It calls the Local server over HTTP at `http://127.0.0.1:4317` unless `localStorage.kodeStreamApiOrigin` names another origin. Evidence: `web/src/shared/api/http.ts:38`, `vite.config.ts:17`.

## Transport Boundary
<!-- chunkId: platform-extension-reference-transport-boundary -->
<!-- keywords: chrome extension origin, local cors, preflight -->

[live] Local mode allows `chrome-extension://` origins through extension CORS and answers preflight requests. Cloud mode does not. Evidence: `internal/server/api/gin_transport.go:35`.

## Not supported
<!-- chunkId: platform-extension-reference-not-supported -->
<!-- keywords: web store, native messaging, direct filesystem, embedded terminal -->

[live] Web Store distribution, native messaging, direct file or Git access from the browser, an embedded terminal and a Cloud transport. Evidence: `internal/server/api/gin_transport.go:35`.

## Known conflicts
<!-- chunkId: platform-extension-reference-known-conflicts -->
<!-- keywords: packaging smoke, clean profile, install evidence -->

[open] Has the current package been installed in a clean browser profile? Tests cover only the build and origin rules.

## Related pages

See [[platform-api-transport-reference]]. The build and load procedure is `deploy/chrome-extension.md`.
