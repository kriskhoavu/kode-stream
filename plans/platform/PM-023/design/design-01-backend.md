# Backend Design: Domain-Oriented Packaging

## Target Structure

```text
internal/
├── server/                      # composition root, API transport, embedded frontend
├── common/                      # shared errors, HTTP helpers, compatibility contracts
├── filesystem/                  # shared bounded content, path, and write access
├── workspace/                   # registry, scanner, files, sources, health
├── item/
│   ├── item_controller.go
│   ├── item_service.go
│   ├── item_repository.go
│   └── item_model.go
├── search/                      # item, content, and path search
├── ai/                          # settings, launch, embedded sessions
├── system/                      # paths, dialogs, application health
├── navigation/                  # filters and recent items
├── git/
│   ├── git_controller.go
│   ├── git_service.go
│   └── git_repository.go
├── jira/
├── audit/
└── knowledge/
    ├── knowledge_controller.go
    ├── knowledge_service.go
    ├── knowledge_repository.go
    └── knowledge_model.go
```

Health is owned by Workspace when it evaluates a workspace and by System when it evaluates the application. Configuration belongs to System. Registry, scanner, workspace files, item index, item writer, AI settings, and content search are implementation responsibilities inside their owning domains rather than independent domains. A role file exists only when the domain needs that role.

## Rules

- Controllers own route registration, request decoding, response encoding, and HTTP error mapping.
- Services own workflows and depend on interfaces rather than concrete adapters.
- Repository ports and domain-specific implementations are owned by the domain.
- Infrastructure packages exist only for concrete capabilities reused by multiple domains.
- Domain-owned types leave `internal/models`; temporary aliases are permitted only during migration.
- `internal/server` wires dependencies and cross-cutting middleware but contains no business decisions.
- Filenames use `<domain>_<role>.go`; exported types use `<Domain><Role>`.
- Do not create a domain package for a single technical class or persistence file.

## Compatibility

All HTTP paths, methods, payload fields, status codes, configuration schemas, and persisted application formats are fixed compatibility contracts for PM-023.
