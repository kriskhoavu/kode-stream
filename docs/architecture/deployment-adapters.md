# Deployment Adapter Class Diagrams

Kode Stream has no single `DeploymentMode` interface. Each deployment concern varies behind its own small abstraction.

## Runtime And Storage

`RuntimeConfig` supplies policy to storage configuration. `StorageProvider` is the persistence seam.

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

The API picks the command adapter from a workspace's `AccessMode`.

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

The extension only chooses the local API origin. It has no server-side runtime or workspace-access adapter.

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
