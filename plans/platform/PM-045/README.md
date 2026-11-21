# PM-045: Self-contained Knowledge Markdown preparation

## Overview

The Knowledge reader prepares a page before `MarkdownPreview` renders it. `prepareKnowledgeMarkdown` drops the front
matter and turns `[[slug]]` and `[[slug|label]]` into links outside code. `normalizeTarget` turns such a link back into
a slug when a person clicks it.

Both functions live in `web/src/features/knowledge/KnowledgeReader.tsx`. That file imports `ContentViewer`,
`../../lib/types` and `KnowledgeWarnings`, so neither function can be copied alone. This ticket moves both into a
module that imports nothing. Another application can then copy that module verbatim beside
`content-viewer/renderers/MarkdownPreview.tsx` and read a wiki page exactly as the Knowledge page reads it.

## Motivating Gap

`MarkdownPreview.tsx` imports only libraries and is already copyable. The preparation step is not. A reader that copies
only `MarkdownPreview.tsx` shows the front matter as text and leaves `[[links]]` unresolved. A second copy written by
hand would drift from this one.

## Non-Goals

- No behaviour change. The prepared Markdown and the link handling stay byte-for-byte the same.
- No change to `KnowledgeBrowser.tsx`, `ContentViewer.tsx` or `content-viewer.css`.
- No new dependency.

## Glossary

| Term                 | Meaning                                                         | Maps To (code)                                    |
|----------------------|-----------------------------------------------------------------|---------------------------------------------------|
| Preparation          | Front matter removed, wiki links rewritten, code left untouched | `prepareKnowledgeMarkdown`                        |
| Knowledge link       | A rewritten wiki link, `./__knowledge__/<slug>`                 | `knowledgeLinkPrefix`                             |
| Target normalization | A clicked link's `href` turned back into the slug it names      | `normalizeKnowledgeTarget`                        |
| Self-contained       | A module with no `import` statement at all                      | `web/src/features/knowledge/knowledgeMarkdown.ts` |

## Documents

- [Implementation Plan](implementation-plan.md)
