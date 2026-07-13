# PM-031 Route Inventory

PM-031 completed the API route migration. Every `/api/` route is registered on Gin under `internal/server/api`; there are no fallback-owned API routes in `API.Routes()`.

## Route Family Status

| Family       | Representative Routes                                           | Owner | Status   |
|--------------|-----------------------------------------------------------------|-------|----------|
| Health/audit | `GET /api/health`, `GET /api/audit-events`                      | Gin   | Complete |
| Navigation   | `/api/saved-filters`, `/api/recent-items`                       | Gin   | Complete |
| System       | `/api/system/config-paths`, `/api/system/select-directory`      | Gin   | Complete |
| State/search | `/api/state`, `/api/search`, `/api/ai/settings`                 | Gin   | Complete |
| Workspace    | `/api/workspaces`, `/api/workspaces/:id/files`, source settings | Gin   | Complete |
| Item         | `/api/items`, `/api/items/:id/files/:fileID`, item metadata     | Gin   | Complete |
| Knowledge    | `/api/knowledge/wikis`, graph, rescan, sync, enrich             | Gin   | Complete |
| Verification | `/api/workspaces/:id/verification-jobs` and related job routes  | Gin   | Complete |
| Git          | `/api/workspaces/:id/git/*` operation routes                    | Gin   | Complete |
| Streaming    | `/api/workspaces/stream-create`, embedded AI session channel    | Gin   | Complete |

## Removed Fallback Surface

| Surface                         | Result                                                                                 |
|---------------------------------|----------------------------------------------------------------------------------------|
| `API.Routes()` `ServeMux` setup | Removed. The API entrypoint now returns the Gin transport directly.                    |
| Gin `NoRoute` fallback          | Removed. Missing API routes are not delegated to a legacy mux.                         |
| Route boundary test             | Updated to reject new `mux.HandleFunc` API registrations in `internal/server/api`.     |
| SPA serving                     | Unchanged. Embedded frontend assets are still served by `internal/server` outside Gin. |

## Compatibility Notes

- Public methods, paths, status codes, JSON envelopes, request IDs, SSE, and WebSocket behavior are preserved by transport tests.
- Gin route params are copied into `http.Request.PathValue` before existing transport handlers run.
- Gin imports remain inside the HTTP transport boundary under `internal/server/api`, including handlers, middleware, routing, response adapters, and transport-level tests.
