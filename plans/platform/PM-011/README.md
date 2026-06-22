# PM-011: Consolidate Primary Navigation

PM-011 removes the top-level Items and Branches pages. Kanban owns item discovery and filtering. Explorer owns repository files and branch checkout.

## Related Plans

| Ticket                        | Relationship    | Key Context                                                   |
|-------------------------------|-----------------|---------------------------------------------------------------|
| [PM-003](../PM-003/README.md) | Parent feature  | Added the app shell and the original Items and Branches pages |
| [PM-009](../PM-009/README.md) | Related feature | Expanded Explorer search and multi-workspace file discovery   |
| [PM-010](../PM-010/README.md) | Related feature | Added per-workspace branch selection directly to Explorer     |

## Goals

- Keep Kanban, Explorer, and Workspaces as the top-level workspace navigation.
- Remove the redundant Items list page.
- Remove the redundant Branches summary page.
- Keep item detail routes used by Kanban, search, and recent items.
- Keep item and Git branch APIs used by supported surfaces.
- Redirect legacy top-level URLs to Kanban through the existing fallback route.

## Out Of Scope

- Removing item detail workspaces.
- Removing item search or recent-item navigation.
- Removing Kanban branch filters.
- Removing Explorer branch checkout.
- Removing backend item or Git branch endpoints.

## Glossary

| Term               | Meaning                                                    | Code Target         |
|--------------------|------------------------------------------------------------|---------------------|
| Primary Navigation | Supported top-level pages in desktop and mobile navigation | `App`               |
| Item Workspace     | Detail route for one item, retained at `/items/{id}`       | `ItemWorkspacePage` |
| Legacy List Route  | Removed `/items` or `/branches` top-level URL              | `routeFromLocation` |

## Design Decisions

| Decision                             | Alternatives Considered              | Rationale                                                   |
|--------------------------------------|--------------------------------------|-------------------------------------------------------------|
| Keep item detail routes              | Remove every `/items` route          | Kanban, search, and recent items still open item workspaces |
| Fall legacy list URLs back to Kanban | Add redirects or removed-page errors | Matches the existing unknown-route behavior                 |
| Keep backend APIs                    | Delete list and branch APIs          | Supported pages still depend on these contracts             |
| Remove page source and tests         | Hide navigation only                 | Avoids maintaining unreachable duplicate UI                 |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Frontend Design](design/design-01-frontend.md)
- [Implementation Plan](implementation-plan.md)

## Implementation Result

- Removed Items and Branches from desktop and mobile navigation.
- Removed their top-level route variants and page components.
- Kept `/items/{id}` for item workspaces opened by Kanban and search.
- Kept item and Git branch APIs used by Kanban and Explorer.
- Made legacy `/items` and `/branches` URLs fall back to Kanban.
- Verified 151 backend tests and 79 frontend tests.
