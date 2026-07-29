# Cloud Deployment Modes

Cloud is a hosted control plane, not a hosted Git checkout or terminal. Every Cloud deployment uses Postgres for
app-owned state. A workspace then chooses one of two access modes.

```mermaid
flowchart LR
  U[Authenticated browser] --> C[Cloud API + Postgres]
  C --> A[Agent-Backed workspace]
  A --> M[Outbound Cloud Agent on user machine]
  M --> R[Local repository and local tools]
  C --> S[Remote Snapshot workspace]
  S --> P[Read-only provider API]
  P --> I[Resolved immutable commit]
```

| Workspace access mode | Requires Cloud Agent | Repository source                              | Allowed capabilities                                                                   | Not available                                                                                             |
|-----------------------|----------------------|------------------------------------------------|----------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------|
| Agent-Backed          | Yes                  | User machine                                   | Reads plus role/Agent-gated file, Git, terminal, AI, runtime, and verification actions | Hosted repository execution                                                                               |
| Remote Snapshot       | No                   | Authorized Git provider at resolved commit SHA | Read, ref selection, and terminal handoff guidance                                     | Local path, dirty state, file writes, Git mutations, terminal, AI, runtime, verification, provider writes |

## Choosing A Mode

- Use Agent-Backed when a user needs to change a working tree, run a command, or use local credentials/tools.
- Use Remote Snapshot when a user only needs a safe, consistent view of provider-hosted content.
- Switching from Remote Snapshot to Agent-Backed is a new workspace access choice; it does not create or mutate a local checkout.

## Storage And Trust Boundary

Cloud stores workspace metadata, derived indexes, audit events, and settings in Postgres. The Cloud API never receives
an Agent-Backed repository checkout, SSH key, terminal transcript, or provider token. Remote Snapshot provider
credentials are user-scoped opaque connection material and are never returned by APIs or written to workspace records.

See [Cloud deployment](cloud-deployment.md), [Cloud Agent](cloud-agent.md), and [Remote Snapshot operations](remote-snapshot-operations.md).
