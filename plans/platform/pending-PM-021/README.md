# PM-021: Guarded Jira Editing

PM-021 builds on PM-019 by adding a dedicated Jira edit workflow for controlled field, transition, and attachment mutations. It keeps Jira authoritative through fresh metadata, conflict detection, explicit confirmation, cache invalidation, and normalized refreshes.

## Related Plans

| Item                          | Relationship             | Key Context                                                             |
|-------------------------------|--------------------------|-------------------------------------------------------------------------|
| [PM-019](../PM-019/README.md) | Required Jira foundation | Reuse connections, normalized issues, adapters, and attachment controls |
| [PM-016](../PM-016/README.md) | Local operation safety   | Reuse audit and explicit-confirmation patterns                          |
| [PM-020](../PM-020/README.md) | Separated terminal scope | Embedded AI terminal work remains independent                           |

## Scope

### Goal

Let users deliberately update supported Jira issue fields, execute valid transitions, and manage attachments with conflict and safety controls.

### Non-Goals

- No arbitrary Jira field editor, workflow administration, or bulk updates.
- No Jira-to-local-plan automatic synchronization.
- No unattended AI-initiated Jira writes.
- No comments, work logs, issue creation, or issue deletion.

## Glossary

| Term                  | Meaning                                                                      |
|-----------------------|------------------------------------------------------------------------------|
| Jira Edit View        | Dedicated interface for supported issue fields, transitions, and attachments |
| Editable Field Policy | Explicit allowlist derived from Jira metadata and Kode Stream rules         |
| Issue Version         | Freshness token used to reject stale updates                                 |
| Attachment Result     | Per-file success or failure returned after an attachment mutation            |

## Data Flow

```text
PM-019 normalized issue -> Jira edit view -> metadata and freshness validation
  -> confirmation -> adapter mutation -> normalized refreshed issue -> audit event
```

## Design Decisions

| Decision                                | Alternatives Considered       | Rationale                                                     |
|-----------------------------------------|-------------------------------|---------------------------------------------------------------|
| Dedicated Jira edit view                | Edit inside narrow side panel | Gives validation, attachments, and conflicts sufficient space |
| Allowlisted Jira fields and transitions | Generic JSON editor           | Avoids unsupported or unsafe mutations                        |
| Confirm mutations and refresh afterward | Optimistic local-only state   | Keeps Jira authoritative and makes side effects explicit      |
| Return per-file attachment results      | All-or-nothing batch          | Preserves successful uploads when another file fails          |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
