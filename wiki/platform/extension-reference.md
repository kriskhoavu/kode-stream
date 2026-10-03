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
<!-- keywords: unpacked extension, manifest v3, api origin, local host -->

[live] The Chromium surface is built as an unpacked Manifest V3 extension and stores a configurable API origin that defaults to `http://127.0.0.1:4317`. Evidence: `web/src/shared/api/index.ts:118`, `vite.config.ts:17`.

The extension reuses the application frontend while producing a separate output directory and manifest. It talks to the Local application over HTTP.

## Transport Boundary
<!-- chunkId: platform-extension-reference-transport-boundary -->
<!-- keywords: chrome extension origin, local cors, standard verbs, options -->

[live] Local mode permits `chrome-extension://` origins through extension-specific CORS and answers preflight requests; Cloud does not enable this exception. Evidence: `internal/server/api/gin_transport.go:35`, `internal/server/api/gin_transport.go:58`.

## Not supported
<!-- chunkId: platform-extension-reference-not-supported -->
<!-- keywords: web store, native messaging, direct filesystem, cloud extension -->

[live] Web Store distribution, native messaging, direct browser filesystem or Git access, and a Cloud extension transport are not supported. Evidence: `deploy/chrome-extension.md:1`, `internal/server/api/gin_transport.go:35`.

## Known conflicts
<!-- chunkId: platform-extension-reference-known-conflicts -->
<!-- keywords: packaging smoke, browser installation, release evidence -->

[open] Has the current unpacked package been installed and exercised in a clean browser profile? Repository tests cover the build and origin rules but do not establish current browser installation evidence.

## Related pages

See [[platform-cloud-runtime-reference]], [[platform-api-transport-reference]], and [[workspace-index]].
