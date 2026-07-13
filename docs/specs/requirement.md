# Kode Stream

## Vision

Kode Stream is a lightweight Git-native web application running locally on the developer machine.

Instead of browsing plans through Git branches and folders, users can manage them through a Kanban-style interface while still keeping everything in Git.

## Core Concepts

### Repository

A local Git repository registered with Kode Stream.

### Plan

A Plan is represented by a directory in machine under a configured location:

plans/{service}/{ticket-id}/
├── README.md # Overview, glossary, data flow, design decisions
├── scenario/ # Business context and overall workflow
│ ├── scenario-00-overview.md
│ └── scenario-NN-{description}.md
├── design/
│ ├── design-0N-backend.md
│ ├── design-0N-frontend.md
│ └── design-0N-infrastructure.md
└── implementation-plan.md # Phase-by-phase with draft commit per phase

Repositories may define their own plan structure through a repository configuration file. When no custom configuration is provided, Kode Stream falls back to the default plans/ structure described above.

Kode Stream automatically discovers, indexes, and visualizes plans across branches, enabling teams to browse, review, edit, and track planning artifacts in a unified interface.
⸻

## Functional Requirements

### Repository Management

#### Add Repository

User selects local folder.

Fields:

- Repository Name
- Local Path
- Baseline Branch
- Plan Directories

Example:

Name: Discovery
Path: ~/workspace/discovery
Baseline: master

Plan Directories:

- plans
- docs/plans

#### Repository Validation

System verifies:

- Git repository exists
- Baseline branch exists
- Plan directories exist

---

## Plan Discovery

System automatically:

- git fetch every 15 seconds
- Detect new branches
- Detect modified plans
- Detect deleted plans

Refresh methods:

- Auto Refresh
- Manual Sync

---

## Kanban Board

Columns:

- Draft
- In Progress
- Review
- Done

Card Information:

- Plan Name
- Plan Description
- Status
- Branch
- Author
- Updated Time

Actions:

- Open Details View
- Move Status
- Search
- Filter

---

## Plan Workspace (Details View)

### File Explorer

Supports:

- Folder tree
- Collapse/Expand
- Search

### Markdown Editor

Supports:

- Syntax highlighting
- Tables
- Mermaid
- Images
- Checklists

### Preview

Real-time rendering.

### Git Diff

View:

- Added lines
- Modified lines
- Deleted lines

---

## Git Operations

Supported:

- Commit
- Push
- Pull
- Fetch
- Branch Create
- Branch Switch

---

## Non Functional Requirements

### Performance

Initial Load:
< 2 seconds

Plan Open:
< 500ms

Git Fetch:
Background

## Scalability

Repositories:
100+

Plans:
10,000+

Files:
100,000+

## Security

- Local machine only
- Reuse Git credentials
- No credential storage
- Read-only mode available
- Embedded terminal channels remain loopback-only; grants and terminal content are never persisted or logged.
- Embedded provider processes have bounded concurrency, output, dimensions, reconnect leases, cancellation, and shutdown cleanup.

## Distribution Strategy

Preferred: Homebrew

Install:

```bash
brew install kode-stream
```

Update:

```bash
brew upgrade kode-stream
```
