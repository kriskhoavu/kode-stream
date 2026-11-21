# Implementation Plan: PM-045 - Self-contained Knowledge Markdown preparation

## Overview

One phase, one commit, prefix `PM-045:`. A pure move with a rename of one private function. Verification is
`make verify`.

## Execution Flow

```text
R1 Move preparation into knowledgeMarkdown.ts
```

## Phases Summary

| Phase | Name                  | Track    | Status | Verification  |
|-------|-----------------------|----------|--------|---------------|
| R1    | Self-contained module | Frontend | Draft  | `make verify` |

## Terminology Lock

- Module `web/src/features/knowledge/knowledgeMarkdown.ts`.
- Exports `prepareKnowledgeMarkdown`, `normalizeKnowledgeTarget`, `knowledgeLinkPrefix`.
- The module has no `import` statement. A test proves it.

---

## Phase R1 — Self-contained module

### Deliverables

- [ ] `web/src/features/knowledge/knowledgeMarkdown.ts`: new. Holds `prepareKnowledgeMarkdown` moved unchanged from
      `KnowledgeReader.tsx`, and `normalizeTarget` moved and exported as `normalizeKnowledgeTarget`. Export the string
      `./__knowledge__/` as `knowledgeLinkPrefix` and use it in both functions. No `import` statement.
- [ ] `web/src/features/knowledge/KnowledgeReader.tsx`: delete both function bodies. Import them from
      `./knowledgeMarkdown`. Re-export `prepareKnowledgeMarkdown` so current importers keep working.
- [ ] `web/src/features/knowledge/KnowledgeReader.test.tsx`: move the case "does not convert Wiki syntax inside code"
      to the new test file. Keep the reader's link tests here.
- [ ] `web/src/features/knowledge/knowledgeMarkdown.test.ts`: new.
  - Front matter is removed. A file without front matter is unchanged apart from links.
  - `[[slug]]` and `[[slug|label]]` become `[label](./__knowledge__/slug)`. Inline and fenced code stay untouched.
  - `normalizeKnowledgeTarget('./__knowledge__/a%20b#x')` returns `a b`. A target ending in `.md` is kept whole.
  - The module source has no line that starts with `import` (read the file with `node:fs` in the test).

### Verification

`make verify` passes. `KnowledgeReader.test.tsx` passes unchanged apart from the moved case. The Knowledge page renders
the same pages with the same links.
