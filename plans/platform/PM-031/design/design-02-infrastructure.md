# Infrastructure Design: PM-031

## Overview

PM-031 does not add a new production runtime. It hardens the current Gin dependency and test infrastructure until Gin is the only API router.

## Dependency Scope

| Dependency                     | Current Role                                     | PM-031 Rule                                                           |
|--------------------------------|--------------------------------------------------|-----------------------------------------------------------------------|
| `github.com/gin-gonic/gin`     | API transport in `internal/server/api`           | May stay only in transport package.                                   |
| `net/http`                     | Server, SPA, fallback, tests, response contracts | Still used for server and tests; no API route fallback after cutover. |
| `github.com/gorilla/websocket` | Embedded AI session channel                      | Keep until streaming migration decides adapter details.               |
| `gopkg.in/yaml.v3`             | Local persistence and config parsing             | Unchanged.                                                            |

## Governance Checks

| Check                    | Purpose                                                  | Timing                          |
|--------------------------|----------------------------------------------------------|---------------------------------|
| Gin import boundary      | Prevent framework leakage outside API transport.         | Every `rtk go test ./...` run   |
| Route inventory coverage | Ensure remaining legacy route registrations are tracked. | Every migration phase           |
| Unmigrated route count   | Show progress toward removing legacy fallback.           | Updated after each route family |
| Frontend typecheck       | Catch accidental API contract drift.                     | Write routes and final cutover  |
| Benchmark scorecard      | Record representative route overhead and cache effects.  | Before final fallback removal   |

## Documentation Updates

| Document          | Update Required                                                 |
|-------------------|-----------------------------------------------------------------|
| `ARCHITECTURE.md` | Mark Gin-only API final state and remove fallback language.     |
| `README.md`       | Update tech stack if Gin-only behavior changes developer notes. |
| PM-030 docs       | Keep as historical baseline, do not rewrite prior decisions.    |
| PM-031 docs       | Track route-family status and final cutover report.             |

## Verification Commands

| Command                 | Purpose                                                    |
|-------------------------|------------------------------------------------------------|
| `rtk go test ./...`     | Full backend, boundary, route inventory, and parity checks |
| `rtk npm run typecheck` | Frontend API contract and TypeScript compatibility         |
| `rtk npm test -- --run` | Frontend regression suite when route contracts change      |

## Design Decisions

| Decision                          | Rationale                                                                     |
|-----------------------------------|-------------------------------------------------------------------------------|
| No new framework dependencies     | Gin is already present; PM-031 should reduce duplicate routing, not add more. |
| Keep checks inside Go tests       | Developers already run `go test ./...`; no extra toolchain required.          |
| Document every cutover checkpoint | Full migration needs a visible audit trail for route family progress.         |
