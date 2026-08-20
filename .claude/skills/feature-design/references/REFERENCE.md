# Planning Skill — Reference

Detailed guidelines, conventions, and examples for the planning skill.
For the workflow, see [SKILL.md](SKILL.md). For document templates, see [FORMS.md](FORMS.md).

---

## Markdown Formatting

Planning docs must render correctly in both GitHub and IntelliJ. Two common issues and their fixes:

### Markdown Tables

**Rule:** Separator dashes must span the full cell width (content + 2 padding spaces). IntelliJ warns
*"Table is not correctly formatted"* when they don't match.

```markdown
✅ Correct — dashes fill the full column width
| Term  | Meaning    | Maps To (code) |
|-------|------------|----------------|
| term1 | definition | field/class    |

❌ Wrong — spaced separators trigger IntelliJ warning
| Term  | Meaning    | Maps To (code) |
| ----- | ---------- | -------------- |
| term1 | definition | field/class    |
```

**Formula:** separator width = `max(content_length_in_column) + 2`

### ASCII Box Diagrams

**Rule:** Every `│ content │` line must be padded so the right `│` aligns with the top `┌───┐` border.

```
✅ Correct — all lines same width
┌────────────────────────────────────┐
│ Short line                         │
│ A much longer line that fills more │
└────────────────────────────────────┘

❌ Wrong — short lines leave gap before │
┌────────────────────────────────────┐
│ Short line │
│ A much longer line that fills more │
└────────────────────────────────────┘
```

**Formula:** content padding = `inner_border_width - 2` (the two flanking spaces)

### Auto-fix Script

Run `fix_formatting.py` (available via `/project:md-formatting` skill) to fix both issues:

```bash
# Fix a single file
python3 .claude/skills/md-formatting/scripts/fix_formatting.py plans/DI-1234/README.md

# Fix all files in a plan directory
python3 .claude/skills/md-formatting/scripts/fix_formatting.py plans/DI-1234/**/*.md
```

The script handles:
- All markdown tables in the file (aligns widths, compacts separators)
- All ASCII boxes (pads `│` content lines to match box border width)
- Preserves content inside fenced code blocks (tables/boxes inside ` ``` ` are not modified)

---

## Branch & Commit Convention

| Item    | Format                             | Example                             |
|---------|------------------------------------|-------------------------------------|
| Branch  | `feature/{ticket-id}-{short-desc}` | `feature/DI-1234-custom-assortment` |
| Commit  | `{ticket-id}: {phase description}` | `DI-1234: Add domain entities`      |
| Trailer | `Change summary`                   | See commit template below           |

```
{ticket-id}: {phase description}

Change summary:
- {brief description of changes in this phase}
```

---

## ASCII Diagram Style

```
# Box style — for states and containers
┌────────────────────────────────────────┐
│ Content here                           │
└────────────────────────────────────────┘

# Flow/steps style — for multi-step processes
┌─────────────────────────────────────────────────────────────────────────┐
│  Step 1: Title                                                          │
│  ─────────────────────────────────────────────────────────────────────  │
│  Details and explanation                                                │
│                                                                         │
│  Step 2: Title                                                          │
│  ─────────────────────────────────────────────────────────────────────  │
│  More details                                                           │
└─────────────────────────────────────────────────────────────────────────┘

# Tree style — for hierarchy and directory structures
plans/{ticket-id}/
├── plan.yaml
├── README.md
├── scenario/
│   ├── scenario-00-overview.md
│   └── scenario-01-{description}.md
└── design/
    └── design-00-overview.md
```
