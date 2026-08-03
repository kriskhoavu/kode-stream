# Frontend Design: Checkout-First Surfaces And Branch Review

## Overview

Operational pages share one checkout branch context. Workstream and Item Workspace replace snapshot selectors with a
checkout label and an explicit **Review branch** entry point. Canvas removes branch selection. Branch Review has a
distinct route, persistent read-only banner, plan list, plan detail, committed file reader, import action, and guarded
checkout action.

## Routes And State

| Route or State | Responsibility                                                                 |
|----------------|--------------------------------------------------------------------------------|
| `/workstream`  | Current checkout board and working-tree mutations.                             |
| `/items/{id}`  | Current checkout item only.                                                    |
| `/canvas`      | Checkout-derived branch layout and execution context.                          |
| `/knowledge`   | Checkout-derived Knowledge index.                                              |
| `/review`      | Route-local `workspaceId` and `branch`; commit comes from the server response. |

The review ref never updates active workspace state or `lastSelectedBranch`. Selecting the current checkout exits
review and opens Workstream.

## Branch Review UX

- Header: **Branch Review**, reviewed branch and short commit, plus **Checkout: {branch}**.
- Banner: **Read-only committed snapshot** with no working-tree status implication.
- Actions: **Refresh snapshot**, **Import plan into checkout**, **Switch workspace to this branch**, and **Exit review**.
- Files render through the existing safe Markdown/content viewer without editor controls.
- Import shows source, commit, destination checkout, and target directory before confirmation and sends that displayed
  checkout as the expected import destination.
- Dirty checkout switching uses the existing guarded confirmation flow.
- Disable **Switch workspace to this branch** while the initial review or a snapshot refresh is loading, preventing a
  competing switch request from being initiated by the same review surface.

## Synchronization

- The application owns one checkout context per active workspace.
- Successful checkout refreshes workspaces and increments the content refresh key once.
- Focus and visibility refresh reload the workspace branch inventory, compare the backend checkout with the stored
  context, and invalidate content when changed.
- Operational routes remove snapshot branch parameters and normalize legacy Canvas URLs.
- An item missing after checkout returns to Workstream with a concise message instead of showing another branch's item.
