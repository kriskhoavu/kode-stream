# Deployment Adapter Class Diagrams

Kode Stream does not use a single `DeploymentMode` base interface. The diagrams below show the smaller abstractions that
vary by deployment concern.

## Runtime And Storage

`RuntimeConfig` supplies policy to storage configuration. `StorageProvider` is the concrete persistence seam.

```mermaid
classDiagram
  direction LR

  class RuntimeConfig {
    +RuntimeMode Mode
    +string BindAddress
    +map~Capability,bool~ Capabilities
  }
  class RuntimeMode {
    <<enumeration>>
    local
    cloud
  }
  class StorageConfigResolver {
    <<function>>
    +ResolveConfig(runtime, paths, getenv) Config
  }
  class StorageProvider {
    <<interface>>
    +Name() string
    +Repositories() RepositoryBundle
    +SQLStore() *SQLStore
    +Close() error
  }
  class DataDirProvider
  class SQLiteProvider
  class PostgresProvider

  RuntimeConfig --> RuntimeMode : Mode
  StorageConfigResolver --> RuntimeConfig : reads
  StorageConfigResolver --> StorageProvider : selects
  DataDirProvider ..|> StorageProvider
  SQLiteProvider ..|> StorageProvider
  PostgresProvider ..|> StorageProvider
```

## Cloud Workspace Access

The API selects the command adapter from a workspace's `AccessMode`. Agent-Backed and Remote Snapshot workspaces have
different command boundaries.

```mermaid
classDiagram
  direction LR

  class API {
    +workspaceAccessAdapter(workspace)
  }
  class WorkspaceConfig {
    +WorkspaceAccessMode AccessMode
    +WorkspaceLocation Location
  }
  class WorkspaceAccessMode {
    <<enumeration>>
    agent_backed
    remote_snapshot
  }
  class workspaceAccessAdapter {
    <<interface>>
    +Command(session, workspace, input) CommandResult
  }
  class agentAccessAdapter
  class remoteSnapshotAdapter {
    +Resolve(ctx, session, workspace) WorkspaceConfig
  }

  API --> WorkspaceConfig : routes
  API --> workspaceAccessAdapter : selects
  WorkspaceConfig --> WorkspaceAccessMode : AccessMode
  agentAccessAdapter ..|> workspaceAccessAdapter
  remoteSnapshotAdapter ..|> workspaceAccessAdapter
```

## Chrome Extension Surface

The extension is a frontend transport adaptation. It chooses the local API origin; it does not implement a server-side
runtime or workspace-access adapter.

```mermaid
classDiagram
  direction LR

  class ExtensionAPIOrigin {
    <<module>>
    +isExtensionSurface() bool
    +localAPIOrigin() string
    +apiURL(path) string
  }
  class LocalKodeStreamAPI {
    <<HTTP API>>
  }

  ExtensionAPIOrigin --> LocalKodeStreamAPI : HTTP requests
```
