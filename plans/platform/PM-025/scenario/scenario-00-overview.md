# Scenarios: PM-025 Overview

## Scenario List

| #   | Title                     | Description                                                                  |
|-----|---------------------------|------------------------------------------------------------------------------|
| 0   | Current board intake      | User can create only a minimal structured item from the Kanban page          |
| 1   | Jira-first item creation  | User fetches a Jira ticket, reviews context, creates an item, and opens it   |
| 2   | AI-assisted plan drafting | User starts an AI session from the created item with a preset or free prompt |
| 3   | Lookup failures           | Workspace, Jira, auth, project, and missing-ticket errors do not write files |

---

# Scenario 0: Current Board Intake

## Starting State

- The main board route is named Kanban.
- New item creation asks for source, item name, and status.
- Jira lookup is available only after an indexed item exists.
- AI sessions can start from an existing item path or workspace root.

## Visual State

```text
Kanban Page
  -> New item modal
  -> create empty README.md
  -> open item workspace
  -> optional AI launch
```

## Available Actions

| Action               | Description                                             | Limitation                                      |
|----------------------|---------------------------------------------------------|-------------------------------------------------|
| Create blank item    | Creates a structured item folder and empty README       | User must manually bring Jira context afterward |
| Open Jira side panel | Reads Jira for the selected indexed item's identifier   | Requires item to exist first                    |
| Launch AI session    | Starts provider with workspace or selected card context | No intake-specific prompt preset                |

---

# Scenario 1: Jira-First Item Creation

## Goal

Create a structured implementation item from a Jira ticket before any local item exists.

## Execution Flow

```text
User opens Workspace
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
  -> Workspace shows Start AI option
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
