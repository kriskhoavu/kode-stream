# PM-023: Domain-Oriented Backend Packaging

PM-023 reorganizes the Go backend around domain-owned controllers, services, models, and repository ports. Concrete operating-system, persistence, and external-system adapters move below `internal/infrastructure`. The refactor preserves every HTTP and persisted-data contract.

## Related Plans

| Ticket                        | Relationship        | Key Context                                                          |
|-------------------------------|---------------------|----------------------------------------------------------------------|
| [PM-015](../PM-015/README.md) | Architecture review | Identified centralized handlers and unclear backend ownership        |
| [PM-022](../PM-022/README.md) | Reference domain    | Supplies the newest service, index, process, and HTTP implementation |

## Glossary

| Term             | Meaning                                                      | Code                                 |
|------------------|--------------------------------------------------------------|--------------------------------------|
| Controller       | HTTP adapter owned by one domain                             | `knowledge_controller.go`            |
| Service          | Domain workflow and application orchestration                | `knowledge_service.go`               |
| Repository port  | Domain-owned interface for state or an external capability   | `knowledge_repository.go`            |
| Infrastructure   | Concrete capability shared by multiple domains               | `internal/infrastructure/filesystem` |
| Composition root | Construction and dependency wiring without business behavior | `internal/server/server.go`          |

## Dependency Direction

```text
HTTP -> domain Controller -> domain Service -> domain Repository interface
                                                   ^
                                                   |
                            infrastructure implementation
```

Domain-local repositories live beside their services, so Git has `git_service.go` and `git_repository.go` in one package. Shared infrastructure is reserved for capabilities used by multiple domains. `internal/server` constructs implementations and injects them. Cross-domain calls use explicit ports where direct ownership would otherwise be ambiguous.

## Domain Map

| Domain       | Owns                                                                               |
|--------------|------------------------------------------------------------------------------------|
| `workspace`  | Registry, scanning, source configuration, workspace files, and workspace health    |
| `item`       | Items, item index, item writer, metadata, and item-specific file operations        |
| `search`     | Item, content, and workspace-path search workflows                                 |
| `ai`         | AI settings, capabilities, launch, and embedded sessions                           |
| `system`     | Application paths, directory selection, path opening, and application-level health |
| `navigation` | Saved filters and recent items                                                     |
| `knowledge`  | Wiki detection, parsing, indexing, relationships, actions, and HTTP delivery       |
| `git`        | Guarded Git workflows and their HTTP delivery                                      |
| `jira`       | Jira workflows, HTTP delivery, and integration contracts                           |
| `audit`      | Append-only activity records and audit queries                                     |

Packages are domains only when they own a coherent capability. A helper, store, or adapter does not become a separate domain merely because it has a service or repository type.

## Legacy Package Ownership

| Current Package                  | Final Owner                                                               |
|----------------------------------|---------------------------------------------------------------------------|
| `aisettings`, `ptysession`       | `ai`                                                                      |
| `api`                            | `server/api` compatibility transport; handlers move to domain controllers |
| `application/*`                  | Matching owning domains                                                   |
| `doctor`                         | `system`                                                                  |
| `health`                         | `workspace` checks and `system` liveness                                  |
| `itemindex`, `itemwriter`        | `item`                                                                    |
| `registry`, `scanner`            | `workspace`                                                               |
| `workspacefiles`                 | `workspace`                                                               |
| `knowledge` plus app service     | One `knowledge` package                                                   |
| `infrastructure/git`             | `git` as `git_repository.go`                                              |
| `fileaccess`, `security`, guards | Shared filesystem infrastructure or owning domain policy                  |
| `models`                         | `common/models` compatibility contracts pending domain distribution       |

## Design Decisions

| Decision                                | Alternatives Considered       | Rationale                                                                |
|-----------------------------------------|-------------------------------|--------------------------------------------------------------------------|
| Use one package per domain              | Layer subpackages             | Keeps Go imports small while filenames expose responsibilities           |
| Use descriptive snake-case filenames    | Generic names, PascalCase     | Follows Go conventions and keeps domain ownership visible                |
| Register routes from domain controllers | Central handler object        | Keeps transport behavior with the owning domain                          |
| Co-locate domain-specific adapters      | Global adapter packages       | Removes duplicate domain/adapter trees and clarifies ownership           |
| Share only reused infrastructure        | Put every repository in infra | Keeps the infrastructure tree small and intentional                      |
| Preserve all external contracts         | Combine refactor and cleanup  | Makes structural regressions detectable                                  |
| Consolidate closely related packages    | Domain per technical object   | Avoids small packages that expose implementation details as architecture |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Implementation Plan](implementation-plan.md)
