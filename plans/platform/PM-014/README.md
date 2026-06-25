# PM-014: Visual Source Items Proposals

PM-014 makes Source Items configuration easier by letting Plan Manager inspect a source directory, propose likely card rules, and preview how each rule maps real paths into Kanban items before the user saves `workspace-settings.yaml`.

## Related Plans

| Item                          | Relationship         | Key Context                                                                                    |
|-------------------------------|----------------------|------------------------------------------------------------------------------------------------|
| [PM-002](../PM-002/README.md) | Source settings base | Added source-owned settings for freestyle docs and guarded writes for configured source cards  |
| [PM-003](../PM-003/README.md) | Architecture guide   | Identified `WorkspacesPage` source settings as a feature area that should be easier to isolate |
| [PM-013](../PM-013/README.md) | Scanner input model  | Kept scanner behavior reader-based so source inspection can work for filesystem and snapshots  |

## Glossary

| Term                      | Meaning                                                                  | Code                          |
|---------------------------|--------------------------------------------------------------------------|-------------------------------|
| Source Items          | A saved rule set that maps a source directory into item cards            | `SourceStructureSettings`     |
| Source Items Proposal | A backend-generated candidate rule with examples and confidence          | `SourceStructureProposal`     |
| Source Items Preview  | Real sample paths mapped into source, item, title, status, and tags | `SourceStructurePreview`      |
| Segment Role              | A visual label applied to one path segment, such as folder or item  | `SourceStructureSegmentRole`  |
| Existing Settings         | A previously saved `workspace-settings.yaml` file                        | `SourceSettingsResult.exists` |

## Data Flow

```text
Open Source Items
  -> frontend requests GET /api/workspaces/{id}/source-structure?directory=docs
  -> backend reads existing settings and samples source paths
  -> backend returns settings, warnings, proposals, and preview rows
  -> user chooses a proposal or clicks path segment roles
  -> frontend updates the same SourceStructureCard draft
  -> preview rows update immediately
  -> user saves the draft as workspace-settings.yaml
  -> backend validates, writes, scans, and returns indexed item count
```

## Design Decisions

| Decision                                          | Alternatives Considered                    | Rationale                                                                                          |
|---------------------------------------------------|--------------------------------------------|----------------------------------------------------------------------------------------------------|
| Keep `workspace-settings.yaml` as the saved model | Add a new visual-only config file          | The scanner and write paths already trust this format; PM-014 should improve UX, not fork behavior |
| Return proposals from the source-structure API    | Infer only in the browser                  | Backend can inspect real files with the same path rules used by scanning                           |
| Show examples before saving                       | Save first, then scan to discover mistakes | Users need to know how many cards and which fields a rule creates before writing settings          |
| Support typed inputs and clickable roles          | Hide the raw pattern completely            | Power users keep precision while new users get a click/tick path                                   |
| Limit the first version to one card rule          | Add multi-rule editing immediately         | The existing UI edits one card rule; proposals can still prepare the rule users need most often    |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
