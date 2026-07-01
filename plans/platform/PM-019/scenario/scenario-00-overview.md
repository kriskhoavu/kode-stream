# Scenarios: PM-019 Read-Only Jira Integration

## Scenario List

| #   | Title                          | Expected Result                                                  |
|-----|--------------------------------|------------------------------------------------------------------|
| 1   | Configure Jira Cloud           | Connection test succeeds with email and token environment value  |
| 2   | Configure Jira Server          | Connection test succeeds with PAT environment value              |
| 3   | Open a matched item            | Normalized Jira details appear in the item side panel            |
| 4   | Issue does not exist           | Side panel shows `No Jira ticket` without failing item loading   |
| 5   | Authentication or outage       | Side panel distinguishes credentials from availability failures  |
| 6   | Open an attachment             | Backend validates ownership and streams bounded safe content     |
| 7   | Unsafe or oversized attachment | Request is rejected before unsafe content reaches the side panel |

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

## Acceptance Scenarios

- Workspace and item behavior remains unchanged when Jira is not configured.
- Token values never appear in persisted YAML, API responses, logs, or audit events.
- Cloud Atlassian Document Format and Server description variants normalize safely.
- A project mismatch or malformed identifier never performs an unrelated Jira lookup.
- Refresh bypasses cache; normal reads use a five-minute in-memory TTL.
- Attachments cannot execute active content inside the Plan Manager origin.
