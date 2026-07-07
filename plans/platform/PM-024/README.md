# PM-024: Import Existing Workspaces

PM-024 adds **Existing Workspaces** as a third Add Workspace mode. A user selects a predefined `workspaces.yaml`, reviews validation and configuration for every entry, selects the entries to keep, and confirms one import. Plan Manager writes valid selections into its effective OS-specific `workspaces.yaml`, then scans and indexes each new workspace.

## Scope

### Goals

- Select a YAML file through a native file picker or an entered absolute path.
- Parse only the current `workspaces.yaml` schema.
- Preview all candidates and their complete non-secret configuration before any write.
- Validate Git roots, branches, sources, duplicates, Jira settings, and Knowledge settings per candidate.
- Import selected valid candidates with one registry write.
- Scan and index every newly registered workspace and report scan failures independently.
- Persist imports in the effective application data directory for macOS, Linux, or Windows.

### Non-Goals

- No legacy `repositories.yaml`, `planDirectories`, or old item-index support.
- No remote clone during import and no ownership claim over imported directories.
- No path rewriting between operating systems.
- No automatic import at startup or live synchronization with the source file.
- No import of indexes, audit logs, saved filters, recent items, or secrets.
- No replacement or update of an already registered workspace.

## Glossary

| Term                | Meaning                                                               | Code                       |
|---------------------|-----------------------------------------------------------------------|----------------------------|
| Existing Workspaces | Add Workspace mode that imports registrations from a predefined file  | `existing_workspace`       |
| Import Source       | User-selected, read-only YAML file                                    | `sourcePath`               |
| Effective Registry  | Active OS-specific or overridden Plan Manager workspace configuration | `Paths.RegistryFile`       |
| Import Preview      | Read-only candidate list with normalized configuration and validation | `WorkspaceImportPreview`   |
| Candidate           | One source-file entry considered for import                           | `WorkspaceImportCandidate` |
| Candidate Key       | Stable digest used to select the same entry during confirmation       | `candidateKey`             |
| Import Result       | Per-candidate registration and indexing outcome                       | `WorkspaceImportResult`    |

## Components

| Layer     | Component                    | Purpose                                                                   |
|-----------|------------------------------|---------------------------------------------------------------------------|
| System    | File selection               | Selects a YAML file with the native OS dialog                             |
| Workspace | Import parser and validator  | Reads bounded current-schema input and validates every candidate          |
| Workspace | Registry batch operation     | Merges selected candidates and atomically persists the effective registry |
| Workspace | Scan orchestration           | Scans and indexes each imported registration after the registry write     |
| API       | Preview and import endpoints | Separates read-only review from confirmed mutation                        |
| Frontend  | Add Workspace dialog         | Selects the import mode, shows destination and previews candidates        |
| Frontend  | Import review                | Supports per-entry selection, validation details, and final outcomes      |

## Data Flow

```text
User selects Existing Workspaces
  -> native file picker returns an absolute YAML path
  -> preview API reads a bounded current-schema file
  -> backend validates every candidate against Git and the effective registry
  -> UI shows source, effective destination, configuration, warnings, and selection
  -> user confirms selected valid candidates
  -> import API rereads and revalidates the file
  -> registry atomically adds all still-valid, non-duplicate candidates
  -> workspace service scans and indexes each imported workspace
  -> UI reports registered, indexed, scan-failed, and skipped outcomes
```

## OS-Specific Destination

Imports always merge into `Paths.RegistryFile`. With no override this resolves from `os.UserConfigDir()`:

| OS      | Typical effective registry path                                                 |
|---------|---------------------------------------------------------------------------------|
| macOS   | `~/Library/Application Support/plan-manager/workspaces.yaml`                    |
| Linux   | `$XDG_CONFIG_HOME/plan-manager/workspaces.yaml` or `~/.config/plan-manager/...` |
| Windows | `%AppData%\plan-manager\workspaces.yaml`                                        |

`PLAN_MANAGER_DATA_DIR` or `bootstrap.yaml` may override the data directory. The preview must display the actual backend-resolved destination rather than constructing an OS path in the browser.

## Design Decisions

| Decision                                       | Alternatives Considered                  | Rationale                                                                      |
|------------------------------------------------|------------------------------------------|--------------------------------------------------------------------------------|
| Merge into the effective registry              | Use the selected file as a live registry | Produces one source of truth and respects existing data-directory behavior     |
| Preview and confirmation are separate requests | Import immediately after file selection  | Guarantees that no configuration changes before explicit review                |
| Reread and revalidate on confirmation          | Trust preview state                      | Detects source-file, filesystem, branch, and registry changes                  |
| Batch registry persistence before scans        | Save once per workspace                  | Avoids partial registry writes and repeated file rewrites                      |
| Keep registration after a scan failure         | Roll back registration                   | Registration is valid; indexing is retryable and its failure must stay visible |
| Mark imported entries `existing_workspace`     | Preserve remote-clone ownership          | Imported directories already exist and must never be deleted as managed clones |
| Use strict current-schema YAML                 | Silently accept old fields               | The project explicitly removed backward-compatibility behavior                 |
| Skip existing paths instead of updating them   | Merge or overwrite configuration         | Prevents an import from silently changing active workspace settings            |
| Never import timestamps or IDs as authority    | Copy persisted identity                  | IDs and timestamps belong to the destination registry                          |

## Related Plans

| Ticket                        | Relationship         | Key Context                                                             |
|-------------------------------|----------------------|-------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Workspace foundation | Defines registry, scanning, indexing, and OS-local app data             |
| [PM-014](../PM-014/README.md) | Source configuration | Defines source validation and `workspace-settings.yaml` behavior        |
| [PM-019](../PM-019/README.md) | Workspace manager    | Defines the current Add Workspace dialog and workspace details          |
| [PM-023](../PM-023/README.md) | Backend ownership    | Places import orchestration in Workspace and native selection in System |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
