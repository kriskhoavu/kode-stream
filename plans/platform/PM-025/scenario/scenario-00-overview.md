# Scenarios: PM-025 Overview

## Scenario List

| #   | Title                     | Description                                                                  |
|-----|---------------------------|------------------------------------------------------------------------------|
| 0   | Workstream branch context | User opens Workstream and loads board context for the selected branch        |
| 1   | Jira-first item creation  | User fetches a Jira ticket, reviews context, creates an item, and opens it   |
| 2   | AI-assisted plan drafting | User starts an AI session from the created item with a preset or free prompt |
| 3   | Lookup failures           | Workspace, Jira, auth, project, and missing-ticket errors do not write files |

---

# Scenario 0: Workstream Branch Context

## Starting State

- At least one registered workspace exists.
- The workspace has one or more configured item sources.
- The user has an active branch selection or a baseline branch.

## Visual State

```text
Workstream
  -> select workspace
  -> load selected branch context
  -> show board columns and item cards
  -> open item workspace or create a new work item
```

## Available Actions

| Action           | Description                                                        |
|------------------|--------------------------------------------------------------------|
| Change branch    | Loads snapshot or working-tree context for the selected branch     |
| Filter board     | Narrows cards by source, status, author, branch, and free text     |
| Create work item | Opens blank or Jira-first intake                                   |
| Open item        | Navigates to the item workspace with files, preview, diff, and Git |

---

# Scenario 1: Jira-First Item Creation

## Goal

Create a structured implementation item from a Jira ticket before any local item exists.

## Execution Flow

```text
User opens Workstream
  -> clicks New Work Item
  -> chooses From Jira
  -> enters Jira key
  -> frontend validates key shape
  -> API fetches issue by workspace and key
  -> user reviews summary, status, assignee, labels, and description
  -> user confirms source, identifier, title, owner, tags, and status
  -> backend creates item folder and README context
  -> backend rescans workspace index
  -> frontend opens the created item
```

## Expected Result

- The item identifier defaults to the Jira key.
- The title defaults to Jira summary.
- Owner and tags use available Jira fields.
- README contains a compact Jira context section for humans and AI tools.
- Jira attachments are visible as references but are not copied into Git.

---

# Scenario 2: AI-Assisted Plan Drafting

## Goal

Start implementation planning immediately after creating the Jira-backed item.

## Execution Flow

```text
Created item opens
  -> Workstream shows Start AI option
  -> user chooses preset or free prompt
  -> frontend launches embedded or external AI session
  -> provider receives the item path and selected prompt
  -> user continues interactively in the terminal
```

## Expected Result

- Presets include implementation plan, technical design, and test scenarios.
- Free prompt remains available for custom workflows.
- Existing AI provider, terminal, and embedded dock behavior remains unchanged.
- The AI starts against the real item path, not a temporary intake draft.

---

# Scenario 3: Lookup Failures

## Goal

Prevent remote Jira failures from creating partial local files.

## Edge Cases

| Case                      | Expected Behavior                                                |
|---------------------------|------------------------------------------------------------------|
| Jira not configured       | Show setup message and do not create an item                     |
| Invalid key               | Show validation error before calling Jira                        |
| Project mismatch          | Reject before fetching an unrelated Jira project issue           |
| Authentication failed     | Show PM-019 recovery hint and do not write files                 |
| Forbidden or unavailable  | Show remote error state and preserve the intake draft            |
| Issue not found           | Let user switch to blank item creation or correct the key        |
| Duplicate local item path | Keep Jira context loaded and ask user to choose another location |
