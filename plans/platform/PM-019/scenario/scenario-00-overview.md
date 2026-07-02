# Scenarios: PM-019 Jira Integration and Workspace Settings

## Scenario List

| #   | Title                          | Expected Result                                                     |
|-----|--------------------------------|---------------------------------------------------------------------|
| 1   | Configure Jira Cloud           | Connection test succeeds with email and token environment value     |
| 2   | Configure Jira Server          | Connection test succeeds with PAT environment value                 |
| 3   | Open a matched item            | Normalized Jira details appear in the item side panel               |
| 4   | Issue does not exist           | Side panel shows `No Jira ticket` without failing item loading      |
| 5   | Authentication or outage       | Side panel distinguishes credentials from availability failures     |
| 6   | Open an attachment             | Backend validates ownership and streams bounded safe content        |
| 7   | Unsafe or oversized attachment | Request is rejected before unsafe content reaches the side panel    |
| 8   | Browse workspace settings      | Selecting a workspace opens its overview without expanding the list |
| 9   | Add a local workspace          | Focused flow registers a folder with inferred defaults              |
| 10  | Clone a remote workspace       | Focused flow clones and registers a Git repository                  |
| 11  | Configure sources and Jira     | Settings are separated into Sources and Integrations tabs           |
| 12  | Diagnose workspace health      | Summary remains compact and detailed checks open on demand          |
| 13  | Change the data directory      | Global storage is configured outside workspace-specific settings    |

## Flow 1: Configure and Test Jira

```text
User edits workspace Jira settings
  -> frontend validates required fields
  -> backend reads the named environment variable
  -> selected Jira adapter requests current-user/server metadata
  -> backend validates project availability
  -> settings persist without token value
```

## Flow 2: Load an Issue

```text
Item side panel opens Jira section
  -> backend loads item and workspace connection
  -> identifier normalizes to an exact issue key
  -> cache hit returns normalized issue, or adapter fetches Jira
  -> description becomes safe normalized content
  -> UI renders issue details and attachment metadata
```

## Flow 3: Access an Attachment

```text
User selects attachment
  -> backend reloads or validates cached issue metadata
  -> attachment ID must belong to matched issue
  -> Jira response headers and size are checked
  -> backend streams with safe content disposition
  -> browser opens or downloads after explicit action
```

## Flow 4: Browse and Configure a Workspace

```text
User selects a workspace in the workspace list
  -> workspace detail opens on Overview
  -> user selects Overview, Sources, or Integrations
  -> only the selected settings domain is rendered
  -> edits remain scoped to that domain
  -> save refreshes workspace data without losing list context
```

## Flow 5: Add a Workspace

```text
User selects Add workspace
  -> dialog asks for Local folder or Remote Git URL
  -> frontend infers name and sensible branch/source defaults
  -> optional settings remain collapsed under Advanced settings
  -> registration runs with focused progress and error feedback
  -> new workspace becomes selected in the workspace manager
```

## Flow 6: Manage Application Storage

```text
User opens application settings
  -> Storage displays data and managed clone directories
  -> user selects and saves a new data directory
  -> UI explains restart requirements and affected paths
  -> workspace manager remains focused on workspace-scoped concerns
```

## Acceptance Scenarios

- Workspace and item behavior remains unchanged when Jira is not configured.
- Token values never appear in persisted YAML, API responses, logs, or audit events.
- Cloud Atlassian Document Format and Server description variants normalize safely.
- A project mismatch or malformed identifier never performs an unrelated Jira lookup.
- Refresh bypasses cache; normal reads use a five-minute in-memory TTL.
- Attachments cannot execute active content inside the Plan Manager origin.
- The workspace list never expands into an edit, Jira, source, or health form.
- Selecting tabs does not discard unsaved changes without warning.
- Jira is optional and appears under the selected workspace's Integrations tab.
- Source rows expose names, index state, and a labeled Configure action.
- Health problems are visible in the list and Overview without rendering every check by default.
- Registration supports keyboard use, progress feedback, retry, and remote-clone logs.
- Destructive workspace removal remains separated from routine editing and requires confirmation.
- Bulk selection is hidden until explicitly activated and is usable by keyboard.
