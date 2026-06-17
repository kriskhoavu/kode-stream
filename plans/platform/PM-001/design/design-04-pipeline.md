# Pipeline Design: PM-001

## Goals

- Make the MVP safe to implement in phases.
- Keep backend, frontend, and browser verification visible.
- Use AI-agent-managed Playwright MCP as a required development gate.
- Prepare for later Homebrew release automation.

## Local Pipeline

```text
Prepare
  -> Backend tests
  -> Frontend typecheck
  -> Frontend unit tests
  -> Production build
  -> Binary build
  -> Playwright MCP acceptance flow
```

## Verification Commands

| Stage               | Command                                        |
|---------------------|------------------------------------------------|
| Backend tests       | `go test ./...`                                |
| Frontend typecheck  | `npm run typecheck`                            |
| Frontend unit tests | `npm test`                                     |
| Frontend build      | `npm run build`                                |
| Binary build        | `go build ./cmd/plan-manager`                  |
| App smoke           | `plan-manager serve`                           |
| Browser acceptance  | AI agent runs Playwright MCP against localhost |

## Playwright MCP Acceptance Flow

The AI agent must run this flow during UI and integration phases.

1. Start the local app server.
2. Open the app in the in-app browser.
3. Register this repository.
4. Set plan directories to `plans` and `docs` when both exist.
5. Run Scan.
6. Verify the repository can be edited without changing managed repository files.
7. Verify the repository path can be revealed in the native file manager.
8. Verify all five Kanban columns render.
9. Verify known sample cards:
   - `PM-001` under `platform`.
   - `DI-202602` in `In Progress`.
   - `DI-170` in `Done`.
10. Verify docs cards show the appropriate badge when a configured docs root has Markdown without plan structure.
11. Filter by multiple repositories, statuses, authors, branches, and text.
12. Verify selected options use OR matching within a facet and AND matching across facets.
13. Verify filter menus show chevrons and close on outside click.
14. Open `PM-001`.
15. Verify file tree, raw Markdown, preview, metadata, and diff tabs.
16. Verify file explorer and plan info panels collapse, expand, and resize smoothly.
17. Capture desktop screenshot.
18. Set mobile viewport.
19. Verify mobile board follows `specs/design.png`.
20. Capture mobile screenshot.
21. Register a disposable repository entry and delete it.
22. Verify deleted repository plans disappear from the board.

## Phase Gate

| Gate           | Rule                                                            |
|----------------|-----------------------------------------------------------------|
| Backend phase  | Backend tests must pass                                         |
| Frontend phase | Typecheck, unit tests, and Playwright MCP flow must pass        |
| DevOps phase   | Production build, binary build, and app smoke must pass         |
| Exception      | A phase may stop only with a concrete blocker and captured logs |

## Future Release Pipeline

| Step              | Purpose                                         |
|-------------------|-------------------------------------------------|
| Tag release       | Select version                                  |
| Build matrix      | Build binaries for supported OS and CPU targets |
| Package archive   | Create release artifacts                        |
| Generate checksum | Support Homebrew formula                        |
| Publish formula   | Enable `brew install plan-manager`              |

## Design Decisions

| Decision                                          | Rationale                                                          |
|---------------------------------------------------|--------------------------------------------------------------------|
| Run Playwright MCP during development             | Layout and workflow regressions are easier to catch in the browser |
| Keep release automation out of MVP implementation | The MVP first needs stable runtime behavior                        |
| Require local binary smoke test                   | Embedded frontend serving must be verified, not assumed            |
| Include docs roots in acceptance                  | PM-001 now supports plan-like and freestyle documentation roots    |
