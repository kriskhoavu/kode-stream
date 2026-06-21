# PM-009 Scenario Overview

## Scenario List

| #   | Scenario                                 | Expected Result                                                          |
|-----|------------------------------------------|--------------------------------------------------------------------------|
| 1   | Search inside a Kanban item              | Results come only from the selected item directory                       |
| 2   | Open an item content match               | The matching file opens with line context                                |
| 3   | Open Explorer in Configured Sources mode | Each workspace exposes only its configured source roots                  |
| 4   | Switch Explorer to All Files mode        | The current full workspace tree becomes available                        |
| 5   | Search Explorer configured sources       | Content results come only from configured source roots                   |
| 6   | Search Explorer all files                | Content results may come from any guarded workspace path                 |
| 7   | Change workspace search scope            | Results cover one workspace or all registered workspaces                 |
| 8   | Handle unsafe or expensive content       | Protected, ignored, binary, large, and outside-symlink files are skipped |
| 9   | Hit a search budget                      | Partial results return with a truncated state                            |

## Flow 1: Item Details Search

```text
User opens a card from Kanban
  -> item details load the item directory
  -> user enters a literal query
  -> frontend requests item-scoped content search
  -> backend resolves the item root and scans supported text files
  -> user selects a line match
  -> existing item file loader opens the result
```

## Flow 2: Explorer Sources Mode

```text
User opens Explorer
  -> Configured Sources mode is active by default
  -> each workspace root shows configured sources only
  -> content search resolves the same source roots
  -> result selection expands source ancestors
  -> existing Explorer file workspace opens the result
```

## Flow 3: Explorer All Files Mode

```text
User switches to All Files
  -> existing PM-008 full-root tree loads
  -> content search switches to guarded workspace roots
  -> ignored preference remains unchanged
  -> switching back restores Configured Sources tree state
```

## Edge Cases

- A workspace with no configured sources shows a source-mode empty state and an All Files action.
- Missing configured source directories show a non-fatal warning.
- Empty and one-character queries do not start a scan.
- Query changes ignore stale responses.
- Files changed during scanning may be skipped without failing the whole request.
- A result file removed before selection shows the existing file-not-found recovery state.
- Duplicate nested source roots search each physical file once.
