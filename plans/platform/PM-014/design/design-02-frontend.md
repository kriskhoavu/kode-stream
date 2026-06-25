# Frontend Design: Visual Source Items Dialog

## Overview

The Source Items dialog becomes a visual chooser. It still exposes the raw path pattern and field inputs, but it first shows backend proposals, live preview rows, and clickable segment roles so users can configure common docs layouts without writing template syntax from memory.

## Data Model

| Type                         | Fields                                                    | Purpose                                      |
|------------------------------|-----------------------------------------------------------|----------------------------------------------|
| `SourceStructureProposal`    | `id`, `label`, `summary`, `confidence`, `card`, `preview` | Render proposal cards and apply a rule       |
| `SourceStructurePreview`     | `path`, `scope`, `identifier`, `title`, `status`, `tags`  | Render example mappings before save          |
| `SourceStructureSegmentRole` | `segment`, `role`                                         | Represent click-labeled path segment mapping |

## State Management

| State              | Owner              | Responsibility                                            |
|--------------------|--------------------|-----------------------------------------------------------|
| `settingsEditor`   | `WorkspacesPage`   | Current workspace, source directory, warnings, draft rule |
| `selectedProposal` | Dialog state       | Highlights the proposal currently matching the draft      |
| `preview`          | API result + draft | Shows current proposal rows; fallback computes from draft |

## Components

```text
WorkspacesPage
  -> SourceStructureDialog
     -> SourceStructureProposalList
     -> SourceStructurePathBuilder
     -> SourceStructurePreviewTable
     -> SourceStructureAdvancedFields
```

For the first implementation, these can remain in `WorkspacesPage.tsx` with extracted helpers. PM-003 can later move them into `web/src/features/workspaces`.

## UX Behavior

- The top of the dialog shows proposal cards such as “Folder / feature / item”.
- Clicking a proposal replaces the draft `SourceStructureCard`.
- A sample path is shown as path-segment pills.
- Segment pills can toggle role choices for `scope`, `identifier`, or literal text.
- The preview table shows path, source, item, title, status, and tags.
- Warnings remain visible before the form.
- The raw path pattern and fields stay available under an advanced section.

## Design Decisions

| Decision                                       | Rationale                                                            |
|------------------------------------------------|----------------------------------------------------------------------|
| Keep Save and Scan as the only write action    | Previewing should be safe and read-only until the user confirms save |
| Prefer proposal cards over a blank form        | Most users need to choose a shape, not learn template syntax first   |
| Keep advanced inputs visible                   | Existing power-user workflow stays available                         |
| Use existing global CSS classes where possible | Avoid a broad styling refactor during this UX change                 |
