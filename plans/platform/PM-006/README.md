# PM-006: Rich Content Viewer

PM-006 adds one secure viewer for Markdown, HTML, source code, JSON, YAML, and KaTeX. It replaces the duplicated Markdown preview logic in the Kanban drawer and item workspace. Existing editing, autosave, diff, file selection, layout, and navigation behavior stay unchanged.

## Implementation Status

Implemented. The backend classifies supported text files, rejects binary content, and bounds large responses. The frontend uses one lazy-loaded viewer in the Kanban drawer and item workspace. It supports sanitized GFM and KaTeX, sandboxed HTML, JSON and YAML trees, highlighted source, copy controls, wrapping, line numbers, parse errors, and large-file fallbacks.

Verification: 104 Go tests, 42 frontend tests, TypeScript typecheck, production build, runtime health check, and production dependency audit. Screenshot verification was unavailable because the in-app browser could not start.

## Related Plans

| Ticket                        | Relationship    | Key Context                                                                             |
|-------------------------------|-----------------|-----------------------------------------------------------------------------------------|
| [PM-001](../PM-001/README.md) | Parent feature  | Created the item workspace, file tree, document preview, and item APIs                  |
| [PM-002](../PM-002/README.md) | Parent feature  | Added Markdown editing, autosave, guarded writes, and diff workflows                    |
| [PM-003](../PM-003/README.md) | Foundation      | Established shared frontend modules and backend application service boundaries          |
| [PM-005](../PM-005/README.md) | Related feature | Added quick navigation that opens items in the same preview and item workspace surfaces |

### What Existing Plans Established

- **Item Workspace**: the full item page with file explorer, preview, raw editor, diff, metadata, and Git panels.
- **Preview Drawer**: the compact item view opened from the Kanban board.
- **File Content**: the existing `FileContent` API model with path, content, language, and content hash.
- **Raw Mode**: the existing text editing surface. Only Markdown remains editable in PM-006.
- **Write Guard**: backend path and symlink checks that protect all file reads and writes.

## Goals

- Render supported text formats with one shared component.
- Keep the current page layout and workflows.
- Prevent scripts and unsafe HTML from running in the application origin.
- Show useful errors for malformed and unsupported content.
- Keep large files responsive.
- Make renderer behavior testable without loading a full page.

## Out Of Scope

- Editing JSON, YAML, HTML, or source code.
- Rendering images, PDF, office files, or remote web pages.
- Running code blocks or HTML scripts.
- Changing item discovery, file paths, autosave, diff, or Git behavior.
- Adding a server-side rendering service.

## Glossary

| Term             | Meaning                                                                   | Code Target               |
|------------------|---------------------------------------------------------------------------|---------------------------|
| Content Viewer   | Shared component that selects and renders a supported file format         | `ContentViewer`           |
| File Kind        | Stable format group derived from a file extension                         | `FileKind`                |
| Render Mode      | Selected representation such as rendered, source, or tree                 | `ViewerMode`              |
| Render Adapter   | Format-specific parser and renderer used by the content viewer            | `ContentRenderer`         |
| Structured View  | Collapsible tree for valid JSON or YAML                                   | `StructuredDataView`      |
| HTML Preview     | Sanitized HTML shown inside a sandboxed iframe                            | `HtmlPreview`             |
| Markdown Preview | Sanitized GFM and KaTeX output                                            | `MarkdownPreview`         |
| Source View      | Read-only highlighted text with line numbers, copy, and wrapping controls | `SourceCodeView`          |
| Parse Error      | Non-fatal message produced when structured content cannot be parsed       | `ViewerParseError`        |
| Large File       | Text content above the configured preview threshold                       | `largeFileThresholdBytes` |
| Unsupported File | Binary or unknown content that the text viewer will not render            | `unsupported` file kind   |

## Components

| Layer    | Component                         | Purpose                                                                  |
|----------|-----------------------------------|--------------------------------------------------------------------------|
| Backend  | `internal/fileaccess/classify.go` | Classify file kinds, languages, binary content, and safe preview limits  |
| Backend  | `models.FileContent`              | Add backward-compatible viewer metadata to the existing response         |
| Frontend | `features/content-viewer`         | Own format selection, lazy renderer loading, errors, and shared controls |
| Frontend | `MarkdownPreview`                 | Render GFM and KaTeX through a sanitized pipeline                        |
| Frontend | `HtmlPreview`                     | Render sanitized HTML in a sandboxed iframe without script permission    |
| Frontend | `StructuredDataView`              | Parse and display JSON and YAML as a collapsible tree                    |
| Frontend | `SourceCodeView`                  | Highlight source text and provide line, copy, and wrap controls          |

## Data Flow

```text
User selects a file
  -> existing file API validates workspace and item paths
  -> file access classifies the file and enforces preview limits
  -> API returns content plus file kind and language
  -> ContentViewer selects a render adapter
  -> adapter parses and sanitizes untrusted text
  -> shared preview renders in the drawer or item workspace
```

## Recommended Package Structure

```text
internal/fileaccess/
├── classify.go
├── fileaccess.go
└── fileaccess_test.go

web/src/features/content-viewer/
├── ContentViewer.tsx
├── ContentViewer.test.tsx
├── types.ts
├── classify.ts
├── renderers/
│   ├── MarkdownPreview.tsx
│   ├── HtmlPreview.tsx
│   ├── StructuredDataView.tsx
│   └── SourceCodeView.tsx
├── components/
│   ├── ViewerToolbar.tsx
│   ├── ViewerError.tsx
│   └── TreeNode.tsx
└── content-viewer.css
```

## Security Boundaries

- Treat all workspace files as untrusted input.
- Do not enable raw HTML inside Markdown.
- Sanitize generated Markdown output with a strict schema.
- Sanitize standalone HTML before assigning iframe `srcDoc`.
- Use an iframe `sandbox` without `allow-scripts` or same-origin access.
- Block remote resource loading in HTML preview by default.
- Write rendered content only through reviewed viewer components.
- Keep the existing backend path and symlink guards.
- Reject binary content from text preview.

## Performance Strategy

- Lazy-load KaTeX, syntax highlighting, YAML parsing, and HTML preview code.
- Parse only the active view and selected file.
- Memoize output by file hash, content, kind, and mode.
- Collapse structured tree nodes by default after a bounded depth.
- Limit initial structured tree expansion and source highlighting.
- Show a large-file notice before expensive rendering.
- Keep plain source available when rich rendering is skipped.

## Design Decisions

| Decision                                       | Alternatives Considered                       | Rationale                                                                       |
|------------------------------------------------|-----------------------------------------------|---------------------------------------------------------------------------------|
| Use one shared viewer in both preview surfaces | Keep page-local rendering                     | One pipeline prevents security, behavior, and style drift                       |
| Keep format detection on the backend           | Detect only in React                          | The API can reject binary data and return one stable classification             |
| Add optional fields to `FileContent`           | Create a versioned viewer endpoint            | Additive fields preserve the current route and existing clients                 |
| Use a Markdown AST pipeline                    | Extend direct `marked.parse()` calls          | GFM, math, highlighting, and sanitization need explicit ordered processing      |
| Sandbox standalone HTML                        | Inject HTML into the application DOM          | The iframe isolates styles and prevents document-level access                   |
| Keep non-Markdown formats read-only            | Generalize the editor in this ticket          | PM-006 improves viewing without expanding write behavior or validation rules    |
| Use a custom bounded data tree                 | Add a large JSON editor dependency            | The required tree interactions are small and need predictable large-file limits |
| Preserve source and diff tabs                  | Replace all tabs with viewer-owned navigation | Existing editing and review workflows must remain stable                        |

## Acceptance Criteria

- Markdown tables, task lists, links, code fences, and KaTeX render correctly.
- Code blocks and source files support syntax highlighting, line numbers, copy, and wrapping.
- Valid JSON and YAML support source and collapsible tree views.
- Parse errors show a clear message and preserve source access.
- HTML renders without scripts, same-origin access, or unsafe remote resources.
- The Kanban drawer and item workspace use the same viewer implementation.
- Existing raw editing, autosave, diff, file selection, and Git behavior remain unchanged.
- Large or unsupported files do not freeze the page.
- Backend, frontend, security, and interaction tests pass.

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
- [Backend Design](design/design-01-backend.md)
- [Frontend Design](design/design-02-frontend.md)
- [Implementation Plan](implementation-plan.md)
