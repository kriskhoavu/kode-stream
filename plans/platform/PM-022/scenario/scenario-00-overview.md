# Scenarios: PM-022 Structured Knowledge Wiki

## Scenario List

| #   | Title                    | Description                                                              |
|-----|--------------------------|--------------------------------------------------------------------------|
| 1   | Discover and browse Wiki | Detect a compatible source and navigate domains and pages                |
| 2   | Read linked knowledge    | Render one page, follow links, and inspect backlinks                     |
| 3   | Explore the graph        | Inspect page relationships and open a selected graph node                |
| 4   | Rescan local files       | Rebuild index data without changing Git state                            |
| 5   | Sync remote changes      | Pull through the guarded Git flow and rebuild the index                  |
| 6   | Enrich the Wiki          | Run an explicitly configured enrichment process and then rebuild         |
| 7   | Recover from bad content | Keep valid pages available while reporting malformed or unresolved input |

## Scenario 1: Discover And Browse Wiki

### Goal

Find structured documentation by domain instead of browsing raw repository folders.

### Starting State

- A registered workspace has `docs` in `WorkspaceConfig.sources`.
- `docs/index.md` exists.
- One or more Markdown files below `docs` contain `slug` and `title` front matter.

### Execution Flow

```text
User opens Knowledge
  -> frontend requests detected Wikis
  -> backend validates registered sources
  -> detector confirms index.md and compatible pages
  -> index groups pages by relative parent directory
  -> frontend shows workspace, Wiki, domain, and page navigation
```

### Expected Result

- The Wiki appears under its workspace.
- Domains such as `offer` and `master-data/article` retain their hierarchy.
- Page title, type, roles, topics, summary, and source count are available for browsing.
- Unregistered directories and non-compatible docs sources do not appear.

## Scenario 2: Read Linked Knowledge

### Goal

Read a Wiki page and move through semantic relationships.

### Execution Flow

```text
User selects a page
  -> route stores workspace ID, Wiki root, and page slug
  -> backend resolves slug to one guarded relative path
  -> API returns classified Markdown content and page metadata
  -> shared ContentViewer renders sanitized Markdown
  -> outgoing links and backlinks are shown beside the content
  -> selecting a resolved link updates the Knowledge route
```

### Expected Result

- Browser Back and Forward restore the selected page.
- `[[slug]]` and `[[slug|label]]` links open the matching page.
- Relative Markdown links to indexed Markdown files open their matching page.
- External links retain the shared viewer's safe external-link behavior.
- “Open in Explorer” opens the same workspace and relative path.

## Scenario 3: Explore The Graph

### Goal

Understand how domains and pages relate without reading every index table.

### Execution Flow

```text
User selects Graph
  -> frontend loads graph nodes and resolved edges from the current Wiki index
  -> graph lays out nodes by domain and relationship
  -> user pans, zooms, focuses, or selects a node
  -> selected node opens its Knowledge reader route
```

### Expected Result

- Every valid page is represented once.
- Every resolved page-to-page link is represented once per source and target pair.
- Broken links are warnings, not graph nodes.
- Keyboard users can focus and select graph nodes.
- Large Wikis remain bounded and display a truncation notice if a configured graph budget is exceeded.

## Scenario 4: Rescan Local Files

### Goal

Refresh Knowledge after local documentation changes without pulling or generating content.

### Execution Flow

```text
User selects Rescan
  -> backend validates workspace and Wiki root
  -> parser rebuilds pages, links, backlinks, and warnings
  -> index atomically replaces the selected Wiki entry
  -> frontend reloads browse, reader, and graph data
```

### Expected Result

- Git is not invoked.
- Workspace content is not written.
- A deleted selected page falls back to the Wiki overview with a notice.
- Scan failures preserve the previous valid persisted index.

## Scenario 5: Sync Remote Changes

### Goal

Update the checked-out Wiki from its Git remote and then refresh Knowledge.

### Execution Flow

```text
User selects Sync
  -> frontend obtains current Git status
  -> dirty tree uses the existing pull confirmation contract
  -> backend runs guarded Git pull
  -> pull failure returns existing Git recovery guidance and stops
  -> successful pull triggers detection and rescan for workspace Wikis
  -> frontend reloads data and reports pull plus scan outcome
```

### Edge Cases

- Cancelled dirty-tree confirmation performs no pull or rescan.
- Merge conflict or authentication failure does not overwrite the previous Knowledge index.
- A successful pull that removes a Wiki root removes that Wiki from the workspace index.
- A successful pull with malformed pages indexes remaining valid pages and returns warnings.

## Scenario 6: Enrich The Wiki

### Goal

Explicitly run the workspace's Wiki generation or reconciliation tool.

### Starting State

- Workspace Knowledge settings contain a non-empty executable and argument array.
- The user accepts a confirmation showing executable, arguments, working directory, and mutation warning.

### Execution Flow

```text
User selects Enrich
  -> frontend shows explicit confirmation
  -> backend validates configuration and workspace root
  -> process runs without a shell from the workspace root
  -> timeout and combined-output limits are enforced
  -> backend records an audit event
  -> success triggers Wiki detection and rescan
  -> frontend shows bounded process output and refreshed Knowledge data
```

### Edge Cases

- Missing configuration keeps the action disabled and links to workspace setup.
- Missing executable returns a setup error without starting a process.
- Timeout terminates the process, records failure, and does not rescan.
- Non-zero exit records failure and preserves the previous index.
- Output beyond the limit is truncated with an explicit indicator.
- Workspace files changed by a failed process remain visible in Git status; Plan Manager never resets them.

## Scenario 7: Recover From Bad Content

### Goal

Keep useful documentation available when part of the Wiki is invalid.

| Condition                 | Result                                                               |
|---------------------------|----------------------------------------------------------------------|
| Missing `slug` or `title` | Skip page and add a path-specific warning                            |
| Duplicate slug            | Index first normalized path and warn about every conflicting path    |
| Missing Wiki-link target  | Keep source page, mark link unresolved, and add a warning            |
| Invalid optional metadata | Normalize when safe; otherwise omit the field and warn               |
| Invalid front matter YAML | Skip page and add a parse warning                                    |
| File exceeds read budget  | Skip page and add a size warning                                     |
| Symlink leaves workspace  | Reject target and add a safety warning without reading it            |
| Root no longer qualifies  | Remove it on successful rescan and return the user to Wiki selection |
