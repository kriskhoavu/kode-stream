# Scenario 0: Visual Source Items Setup

## Goal

Help a user configure a docs source into item cards by choosing a proposed structure and verifying real preview rows before saving.

## Starting State

| #   | Title               | Summary                                                       |
|-----|---------------------|---------------------------------------------------------------|
| 1   | Workspace exists    | The workspace has a `docs` source registered                  |
| 2   | No settings file    | `docs/workspace-settings.yaml` does not exist                 |
| 3   | Docs tree has shape | Paths such as `docs/api/feature/DI-101/README.md` are present |

## Visual State (Before)

```text
Source Items
  No settings file yet
  Path Pattern: {folder}/feature/{item}
  Title: readme_heading
  Default Status: Draft
```

## Execution Flows

### Flow 0.1: Apply A Proposed Structure

```text
User opens Source Items
    ↓
Backend samples docs paths
    ↓
Backend returns proposed rules and preview rows
    ↓
User clicks "Folder / feature / item"
    ↓
Frontend applies the proposal to the draft card
    ↓
Preview table shows API Search, DI-101, scope api
    ↓
User clicks Save and Scan
    ↓
Backend writes workspace-settings.yaml and refreshes items
```

### Flow 0.2: Adjust A Segment Role

```text
User sees sample path api / feature / DI-101
    ↓
User clicks a segment pill
    ↓
Frontend updates the draft path pattern
    ↓
Preview rows update to show the new source and item mapping
```

## Visual State (After)

```text
Proposal: Folder / feature / item

Preview
docs/api/feature/DI-101 -> scope api, identifier DI-101, title API Search
docs/web/feature/DI-202 -> scope web, identifier DI-202, title Web UI
```

## Edge Cases

- Empty sources show the default proposal with an empty preview.
- Sources with no matching paths still allow manual advanced editing.
- Invalid saved settings keep showing warnings and propose safer alternatives.
- Large sources cap preview rows to keep the dialog fast.
