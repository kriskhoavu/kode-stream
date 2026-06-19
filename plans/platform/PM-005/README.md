# PM-005: Search And Navigation

PM-005 improves how users find items across workspaces. It adds global search, saved filters, keyboard-friendly navigation, and better item discovery. It uses the existing item index first. It does not require a database server.

## Implementation Status

Implemented. The backend provides ranked item search, saved-filter and recent-item storage, APIs, and regression tests. The frontend provides the quick switcher, keyboard selection, recent items, and Kanban saved views.

Verification: `go test ./...`, `npm run typecheck`, `npm test -- --run`, and `npm run build`.

## Related Plans

| Ticket                        | Relationship   | Key Context                                                                             |
|-------------------------------|----------------|-----------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Parent feature | Created workspace scan, item index, Kanban, item list, and branch views                 |
| [PM-002](../PM-002/README.md) | Parent feature | Added item editing, source settings, and Git operations                                 |
| [PM-003](../PM-003/README.md) | Foundation     | Split frontend app state, shared API, feature helpers, and backend application services |
| [PM-004](../PM-004/README.md) | Related plan   | Adds reliability signals that search results can link to later                          |

### What Existing Plans Established

- **Workspace**: a registered local Git repository.
- **Item**: a scanned planning folder or docs card.
- **Item Index**: local YAML cache with item summaries and details.
- **Kanban Filter**: per-page filters for source, scope, status, branch, and author.
- **App Route**: browser path that opens Kanban, items, branches, workspaces, or item detail.

## Glossary

| Term           | Meaning                                                        | Code Target     |
|----------------|----------------------------------------------------------------|-----------------|
| Global Search  | Search across all indexed workspaces                           | `SearchQuery`   |
| Search Result  | One matched item, file, or workspace entry                     | `SearchResult`  |
| Search Scope   | Area included in search, such as active workspace or all items | `scope`         |
| Saved Filter   | Named filter set stored locally                                | `SavedFilter`   |
| Quick Switcher | Keyboard-first dialog for opening items and views              | `QuickSwitcher` |
| Recent Item    | Item opened recently by the user                               | `RecentItem`    |
| Result Context | Short matched text or metadata shown below a result            | `context`       |

## Data Flow

```text
User opens global search
  -> frontend reads query and scope
  -> backend queries item index
  -> backend ranks exact, prefix, and text matches
  -> frontend shows grouped results
  -> user opens item, workspace, branch, or saved filter
```

## Design Decisions

| Decision                    | Alternatives Considered       | Rationale                                                     |
|-----------------------------|-------------------------------|---------------------------------------------------------------|
| Use item index first        | Add SQLite or external search | Current data is small and already cached locally              |
| Add all-workspace search    | Only active workspace search  | Users often work across several local repositories            |
| Store saved filters locally | Store in Git workspace        | Saved filters are user preferences, not project content       |
| Add quick switcher          | Add only a search page        | Keyboard navigation improves daily use without changing pages |
| Keep search read-only       | Add edit actions in results   | Search should be safe and predictable first                   |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
