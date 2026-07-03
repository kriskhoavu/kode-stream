# Scenarios: PM-021 Guarded Jira Editing

## Scenario List

| #   | Title                         | Expected Result                                                   |
|-----|-------------------------------|-------------------------------------------------------------------|
| 1   | Edit supported Jira fields    | Confirmed fresh update succeeds and normalized issue refreshes    |
| 2   | Transition Jira status        | UI offers only valid transitions and submits the transition ID    |
| 3   | Detect a stale Jira edit      | Mutation is rejected and current issue data is reloaded           |
| 4   | Upload Jira attachments       | Guarded operation reports per-file results and refreshes metadata |
| 5   | Delete a Jira attachment      | Named confirmation precedes deletion and refresh                  |
| 6   | Handle partial upload failure | Successful files remain visible and failures are shown separately |

## Flow 1: Jira Field or Status Update

```text
User opens dedicated Jira edit view
  -> backend returns fresh issue, editable fields, transitions, and version
  -> user changes supported values
  -> frontend displays exact mutation confirmation
  -> backend verifies version and policy
  -> adapter updates fields or executes transition
  -> backend invalidates cache and returns refreshed normalized issue
```

## Flow 2: Attachment Mutation

```text
User selects files or attachment deletion
  -> client and backend validate operation limits
  -> deletion requires explicit named confirmation
  -> backend streams each operation to Jira with bounded resources
  -> response identifies success or failure for each file
  -> issue attachment metadata refreshes
```

## Acceptance Scenarios

- Editing is unavailable without a successful PM-019 issue read and required permissions.
- Stale issue versions never silently overwrite newer Jira data.
- Only adapter-supported and policy-allowlisted fields are editable.
- Status changes use server-provided transition IDs.
- Attachment mutations reuse PM-019 filename, media-type, size, timeout, and audit controls.
- Jira field values and attachment content never enter logs or audit payloads.
