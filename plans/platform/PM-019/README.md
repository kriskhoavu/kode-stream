# PM-019: Read-Only Jira Integration and Workspace Settings Redesign

PM-019 connects a registered workspace to Jira Cloud or Jira Server/Data Center. It also redesigns the Workspaces page so Jira and future workspace settings do not turn the workspace list into a collection of inline forms. Plan Manager matches an item's identifier to its Jira issue, presents normalized issue details in the item workspace, and provides guarded attachment access without writing to Jira.

## Related Plans

| Item                          | Relationship                    | Key Context                                                     |
|-------------------------------|---------------------------------|-----------------------------------------------------------------|
| [PM-003](../PM-003/README.md) | Application architecture        | Add Jira behind application services and stable API DTOs        |
| [PM-013](../PM-013/README.md) | Content viewing                 | Reuse safe rendering and bounded-content practices              |
| [PM-015](../PM-015/README.md) | Current implementation baseline | Preserve local-only configuration and frontend boundaries       |
| [PM-016](../PM-016/README.md) | External provider behavior      | Reuse operation errors, audit discipline, and local credentials |
| [PM-014](../PM-014/README.md) | Source structure settings       | Move source configuration into the workspace detail experience  |

## Scope

### Goal

Display a matching Jira ticket, safely access its attachments, and provide a scalable workspace settings experience without copying Jira data into Git.

### Non-Goals

- No Jira field, status, comment, or attachment mutations; PM-021 owns writes.
- No Jira-driven Kanban synchronization.
- No automatic attachment loading or untrusted Jira HTML rendering.
- No token persistence by Plan Manager.
- No backend workspace or Jira contract changes solely for the page redesign.
- No redesign of Kanban, Explorer, or item workspace navigation.

## Glossary

| Term                       | Meaning                                                                    |
|----------------------------|----------------------------------------------------------------------------|
| Jira Connection            | Per-workspace deployment, base URL, project, identity, and token reference |
| Deployment Type            | `cloud` or `server`                                                        |
| Token Environment Variable | Environment variable read by the Plan Manager process                      |
| Issue Key                  | Uppercase project key and number, such as `DI-170`                         |
| Normalized Issue           | Common API model produced from Cloud or Server/Data Center responses       |
| Attachment Proxy           | Backend route that validates and streams Jira attachment content           |
| Workspace Manager          | Master-detail page for browsing and configuring registered workspaces      |
| Workspace Detail           | Selected workspace view with a sectioned Overview and Integrations tab     |
| Add Workspace Flow         | Focused dialog for local registration or remote cloning                    |

## Data Flow

```text
Workspace Jira settings -> connection test -> environment token
Item identifier -> project validation -> Jira adapter -> normalized issue cache
  -> item side panel -> explicit attachment request -> guarded backend proxy
Workspace list -> selected workspace -> detail tab -> scoped settings action
Add workspace -> local or remote mode -> required fields -> optional advanced settings -> register
```

## Design Decisions

| Decision                                | Alternatives Considered        | Rationale                                                        |
|-----------------------------------------|--------------------------------|------------------------------------------------------------------|
| Support Cloud and Server from the start | Single deployment first        | Required workspaces may use either API and authentication shape  |
| Reference token through environment     | Store token in YAML            | Keeps credentials out of app-owned persistence                   |
| Match exact normalized identifier       | Fuzzy title matching           | Prevents displaying the wrong issue                              |
| Normalize descriptions before rendering | Inject Jira HTML               | Treats all remote content as untrusted                           |
| Proxy attachments after explicit action | Embed authenticated URLs       | Prevents token leakage and broken side-panel content             |
| Five-minute in-memory cache             | Persist fetched issue data     | Reduces Jira traffic without creating a stale local source       |
| Use a master-detail workspace manager   | Keep expanding cards           | Preserves list context while isolating each settings domain      |
| Move global storage to app settings     | Keep it beside registration    | Data directory is application-wide and may require restart       |
| Use a focused add-workspace dialog      | Keep the permanent form        | Registration is occasional and should not consume the page       |
| Put Jira under Integrations             | Keep Jira inline in editing    | Makes optional provider settings discoverable without dominating |
| Keep completed Jira phases unchanged    | Rewrite implementation history | Accurately separates shipped Jira work from pending redesign     |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
