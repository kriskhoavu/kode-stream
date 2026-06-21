# PM-009: Scoped Content Search

## Status

Draft

## User Story

As a developer, I want to search text inside the documents related to my current item or Explorer scope so that I can find relevant content without opening every file.

## Problem

PM-008 searches file and directory names. It does not search file contents. Explorer also opens every workspace at its full root, which adds noise when most work happens inside configured sources such as `plans` and `docs`.

## Scope

- Add recursive literal content search to item details.
- Limit item details search to the selected item's directory.
- Add Configured Sources and All Files Explorer tree modes.
- Make Configured Sources the default mode.
- Scope Explorer content search to the active tree mode.
- Support all-workspace and one-workspace Explorer searches.
- Respect the existing ignored-file preference.
- Return bounded line numbers and snippets.
- Open results in the current item or Explorer file workspace.

## Acceptance Criteria

- [ ] Item details can search supported text files under the selected item directory.
- [ ] Item search cannot read sibling items or paths outside the item root.
- [ ] Explorer defaults to Configured Sources mode.
- [ ] Configured Sources mode shows only `WorkspaceConfig.sources` below each workspace.
- [ ] Explorer offers an explicit All Files mode that preserves the current full tree.
- [ ] Explorer content search uses only configured sources in Sources mode.
- [ ] Explorer content search uses the guarded workspace root in All Files mode.
- [ ] Explorer search supports all workspaces and one selected workspace.
- [ ] Ignored paths follow the Show ignored preference in tree and content search.
- [ ] `.git`, binary files, unsupported content, and outside symlinks never appear in results.
- [ ] Results show workspace or item context, relative path, line number, and a bounded snippet.
- [ ] Selecting a result opens the file and exposes the matched line context.
- [ ] Empty, stale, loading, truncated, and error states have stable layouts.
- [ ] Keyboard users can enter a query, navigate results, and open a match.
- [ ] Existing path-name search remains available and visually distinct.
- [ ] Search limits prevent one request from scanning unbounded files or bytes.
- [ ] Existing PM-007 and PM-008 browsing, editing, mutation, and Git behavior remains unchanged.
- [ ] Backend, frontend, and production-build checks pass.

## Default Limits

| Limit                | Value     |
|----------------------|-----------|
| Minimum query length | 2 chars   |
| Maximum query length | 200 chars |
| Results              | 100       |
| Files visited        | 10,000    |
| Bytes read           | 64 MiB    |
| File size            | 2 MiB     |
| Snippet length       | 240 chars |

## Dependencies

- PM-006 content classification and binary detection.
- PM-007 item file and Workspace Explorer selection behavior.
- PM-008 path search, ancestor expansion, ignored mode, and tree caches.

## Definition Of Done

- Search scope and safety rules have backend unit tests.
- Item and Explorer endpoints have API regression tests.
- Search budgets, binary files, ignored paths, and symlink escapes have tests.
- Tree mode, debounce, stale results, keyboard navigation, and result opening have frontend tests.
- Architecture and PM-009 documents reflect final behavior.
