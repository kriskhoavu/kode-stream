# Frontend Design: Guarded Jira Editing

## Overview

Add a dedicated Jira edit view launched from the PM-019 issue panel. Separate field editing, transitions, and attachment management while sharing freshness and conflict state.

## State Management

| State                    | Owner             | Behavior                                         |
|--------------------------|-------------------|--------------------------------------------------|
| Fresh issue and metadata | Jira edit hook    | Required before fields become editable           |
| Draft fields             | Jira edit view    | Contains only allowlisted editable fields        |
| Transition selection     | Jira edit view    | Uses transition ID and server-provided label     |
| Attachment queue         | Attachment editor | Tracks validation, progress, and per-file result |
| Conflict state           | Jira edit hook    | Blocks retry until current issue is reviewed     |

## User Experience

- Open a full-width dedicated view from the Jira side panel.
- Separate field editing, status transition, and attachment management sections.
- Present exact changed field names and target transition in confirmation dialogs.
- On conflict, preserve the user's draft separately, show current Jira values, and require a new explicit submission.
- Show upload progress and independent results; successful files remain successful if another file fails.
- Require destructive confirmation naming the attachment before deletion.

## Accessibility

- Forms associate validation and conflict messages with affected controls.
- Upload progress and mutation results are announced without exposing file content.
- Confirmation dialogs move focus predictably and restore it to the initiating control.

## Design Decisions

| Decision                          | Rationale                                                    |
|-----------------------------------|--------------------------------------------------------------|
| Dedicated Jira edit route/view    | Editing, conflicts, and attachments exceed side-panel space  |
| Keep mutation drafts client-local | Jira remains authoritative until a confirmed API response    |
| Shared conflict state             | Fields, transitions, and attachments use one freshness model |
