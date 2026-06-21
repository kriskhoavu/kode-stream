# Backend Design: PM-009 Scoped Content Search

## Overview

Add a bounded, in-process literal content scanner. Reuse workspace path guards, Git ignore handling, PM-006 classification, binary detection, and file-size limits. Add item-scoped and Explorer-scoped application methods. No database or migration is required.

## Data Model

### `WorkspaceContentSearchRequest`

| Field            | Type     | Purpose                                          |
|------------------|----------|--------------------------------------------------|
| `query`          | `string` | Literal text query                               |
| `caseSensitive`  | `bool`   | Optional exact-case matching; false by default   |
| `includeIgnored` | `bool`   | Include ignored paths when Explorer enables them |

### `WorkspaceContentSearchResult`

| Field           | Type       | Purpose                                      |
|-----------------|------------|----------------------------------------------|
| `id`            | `string`   | Stable workspace, path, line, and column key |
| `workspaceId`   | `string`   | Registered workspace                         |
| `workspaceName` | `string`   | Result context                               |
| `itemId`        | `string?`  | Item context for item-scoped search          |
| `path`          | `string`   | Workspace-relative file path                 |
| `fileId`        | `string?`  | Existing item file ID when item-scoped       |
| `name`          | `string`   | File base name                               |
| `kind`          | `FileKind` | Shared PM-006 classification                 |
| `language`      | `string`   | Existing syntax language                     |
| `lineNumber`    | `int`      | One-based line number                        |
| `columnStart`   | `int`      | One-based match start                        |
| `columnEnd`     | `int`      | One-based exclusive match end                |
| `snippet`       | `string`   | Bounded matching line context                |
| `ignored`       | `bool`     | Git ignore state                             |

### `WorkspaceContentSearchResponse`

| Field          | Type                             | Purpose                               |
|----------------|----------------------------------|---------------------------------------|
| `results`      | `WorkspaceContentSearchResult[]` | Ordered line matches                  |
| `truncated`    | `bool`                           | A request budget stopped scanning     |
| `filesVisited` | `int`                            | Diagnostic work count                 |
| `bytesRead`    | `int64`                          | Diagnostic byte count                 |
| `skippedFiles` | `int`                            | Binary, large, unreadable, or changed |

## Search Root Resolution

| Surface / Mode     | Roots                                                      |
|--------------------|------------------------------------------------------------|
| Item details       | One canonical item directory                               |
| Explorer `sources` | Canonical configured source directories for selected scope |
| Explorer `all`     | Canonical registered workspace roots                       |

- Deduplicate nested and repeated canonical roots.
- Keep workspace-relative result paths even when a source root is nested.
- Skip missing configured roots with a warning counter instead of failing all workspaces.
- Reject unknown mode values.

## Scanner Rules

- Validate a trimmed query between 2 and 200 characters.
- Walk directories without following symlink directories.
- Exclude `.git` unconditionally.
- Batch Git ignore checks for each directory.
- Skip ignored files and directories unless `includeIgnored=true`.
- Use shared file classification before reading.
- Read regular files only.
- Skip files larger than 2 MiB.
- Read at most 64 MiB across one request.
- Stop after 10,000 visited files or 100 results.
- Detect binary content before matching.
- Match literal UTF-8 text line by line.
- Return at most one result per match occurrence until the result limit.
- Bound snippets to 240 characters while retaining the match.
- Check request context between directories and files for cancellation.

## Ordering

1. Preserve workspace registry order.
2. Preserve root order from `WorkspaceConfig.sources`.
3. Sort directories and files with existing natural path ordering.
4. Sort matches by path, line, then column.

## API Contract

| Method | Endpoint                               | Query                                                         | Response                         |
|--------|----------------------------------------|---------------------------------------------------------------|----------------------------------|
| `GET`  | `/api/items/{id}/content-search`       | `q`, `caseSensitive`                                          | `WorkspaceContentSearchResponse` |
| `GET`  | `/api/workspaces/files/content-search` | `q`, `mode`, `workspaceId`, `includeIgnored`, `caseSensitive` | `WorkspaceContentSearchResponse` |

`mode` accepts `sources` or `all`. Explorer defaults to `sources`. Item search does not accept mode or ignored overrides.

## Error Mapping

| Condition             | Status                                          |
|-----------------------|-------------------------------------------------|
| Unknown item          | 404                                             |
| Unknown workspace     | 404                                             |
| Invalid query or mode | 400                                             |
| Search canceled       | 499 when available, otherwise no response write |
| Root safety failure   | 400                                             |

## Safety And Performance Tests

- Item scope cannot reach sibling item directories.
- Sources mode cannot reach root files or unconfigured directories.
- All mode can reach guarded root files.
- `.git` is excluded in every mode.
- Outside symlinks are skipped.
- Ignored directory traversal follows `includeIgnored`.
- Binary, invalid UTF-8, large, unreadable, and changing files are skipped.
- Result, file, byte, and cancellation limits stop work predictably.
