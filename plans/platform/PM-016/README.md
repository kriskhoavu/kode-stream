# PM-016: Remote Workspace Registration From Git URL

PM-016 adds a second workspace registration mode so users can register by remote Git/Bitbucket URL (HTTPS or SSH), clone it to a chosen local directory, and then use it exactly like existing local-path workspaces.

## Related Plans

| Item                          | Relationship        | Key Context                                                                 |
|-------------------------------|---------------------|-----------------------------------------------------------------------------|
| [PM-003](../PM-003/README.md) | Backend/API baseline | Current workspace create/update contract and app service layout             |
| [PM-008](../PM-008/README.md) | Workspace UX growth | Workspaces page already owns guarded path input, source setup, and actions  |
| [PM-013](../PM-013/README.md) | Git branch model    | Existing branch/load behavior must remain unchanged after clone registration |

## Glossary

| Term                    | Meaning                                                                 |
|-------------------------|-------------------------------------------------------------------------|
| Registration Mode       | How a workspace is created: `local_path` or `remote_clone`            |
| Remote URL              | Git repository URL using HTTPS or SSH                                 |
| Clone Root              | Local directory selected by user where the repository is cloned        |
| Managed Clone Workspace | Workspace cloned by Kode Stream but used as a normal local Git path   |

## Data Flow

```text
User opens Register Workspace
  -> selects Local Path or Remote Git URL mode
  -> fills shared fields (name, baseline branch, sources)
  -> if remote mode, provides remote URL + clone root
  -> backend validates input and clones repo on local machine
  -> backend validates baseline branch and sources in cloned repo
  -> workspace is saved in registry with local path (+ optional remote metadata)
  -> Kanban and Explorer use existing workspace path behavior
```

## Design Decisions

| Decision                                                           | Rationale                                                                 |
|--------------------------------------------------------------------|---------------------------------------------------------------------------|
| Keep existing local-path mode as default                           | Avoids breaking current users and scripts                                 |
| Add remote clone as creation-time option instead of separate flow  | Keeps one registration entry point and one success path                   |
| Persist local path as the canonical workspace path                 | Existing scan, file, and Git services continue to work                    |
| Validate/clone in backend using local machine `git` auth context   | Reuses current user SSH/HTTPS credential setup                            |
| Keep route shape stable (`POST /api/workspaces`) with expanded DTO | Minimizes frontend and test migration effort                              |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
