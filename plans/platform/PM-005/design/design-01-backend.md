# Backend Design: Search And Navigation

## Overview

PM-005 adds read-only search APIs and local saved-filter storage. It uses the existing item index. It should not scan workspaces during search.

## Data Model

### Entity: SearchQuery

| Field         | Type       | Purpose                                      |
|---------------|------------|----------------------------------------------|
| `q`           | `string`   | User search text                             |
| `workspaceId` | `string`   | Optional workspace filter                    |
| `types`       | `string[]` | `item`, `workspace`, `branch`, `savedFilter` |
| `limit`       | `int`      | Max result count                             |

### Entity: SearchResult

| Field         | Type     | Purpose                           |
|---------------|----------|-----------------------------------|
| `id`          | `string` | Result ID                         |
| `type`        | `string` | Result type                       |
| `title`       | `string` | Main label                        |
| `subtitle`    | `string` | Workspace, scope, branch, or path |
| `context`     | `string` | Matched text or metadata          |
| `workspaceId` | `string` | Optional workspace ID             |
| `itemId`      | `string` | Optional item ID                  |
| `route`       | `string` | Frontend route target             |
| `score`       | `int`    | Ranking score                     |

### Entity: SavedFilter

| Field         | Type             | Purpose            |
|---------------|------------------|--------------------|
| `id`          | `string`         | Stable filter ID   |
| `name`        | `string`         | User-facing name   |
| `route`       | `string`         | Route to open      |
| `workspaceId` | `string`         | Optional workspace |
| `filters`     | `map[string]any` | Filter values      |
| `createdAt`   | `time.Time`      | Creation time      |
| `updatedAt`   | `time.Time`      | Last update time   |

## Storage

```text
<user-config-dir>/plan-manager/
  saved-filters.yaml
  recent-items.yaml
  item-index.yaml
```

## API Contract

| Method | Endpoint                  | Request       | Response         |
|--------|---------------------------|---------------|------------------|
| GET    | `/api/search`             | query params  | `SearchResult[]` |
| GET    | `/api/saved-filters`      | none          | `SavedFilter[]`  |
| POST   | `/api/saved-filters`      | `SavedFilter` | `SavedFilter`    |
| DELETE | `/api/saved-filters/{id}` | none          | `{ ok: true }`   |
| GET    | `/api/recent-items`       | none          | `RecentItem[]`   |
| POST   | `/api/recent-items`       | item target   | `{ ok: true }`   |

## Ranking

| Match Type          | Score |
|---------------------|-------|
| Exact identifier    | 100   |
| Item prefix   | 90    |
| Title exact words   | 80    |
| Source or workspace  | 60    |
| Description or tags | 40    |
| Branch              | 30    |

## Design Decisions

| Decision                        | Rationale                                       |
|---------------------------------|-------------------------------------------------|
| Search item index only          | Queries stay fast and read-only                 |
| Add saved filters as app config | Filters are user preferences                    |
| Keep ranking simple             | Current data size does not need a search engine |
| Add recent items separately     | Recents are navigation data, not item metadata  |

