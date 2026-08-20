---
name: feature-brief
description: Render a plans/{service}/{ticket-id}/ plan as a single self-contained HTML page with hand-authored SVG figures, in the Claude-artifact visual style. Use when asked to produce a visual brief, an HTML brief, an architecture page or a shareable design summary for a ticket; to draw diagrams for a plan; to make a plan "look like the artifact"; or to backfill briefs for one or more existing tickets. Also invoked by feature-design as the final step of planning a new ticket.
---

# Feature Brief

Renders a plan that already exists. It does not plan. `feature-design` owns the markdown,
`plan.yaml`, scaffolding, and the implement loop; this skill produces one companion page,
`plans/{service}/{ticket-id}/brief.html`, for readers who will not open a plan directory.

Use these resources:

- [Page spine and components](references/DESIGN-SYSTEM.md)
- [SVG figure grammar](references/FIGURES.md) — read before writing any SVG
- [Voice and plan-to-page mapping](references/NARRATIVE.md)
- [Scaffold script](scripts/init_brief.sh)
- [Validator](scripts/check_brief.py)

## Rules

- The markdown in the plan directory is the source of truth. This page is regenerable output.
- Never invent a fact the plan does not contain. Unknowns become open decisions in section 7.
- Never restate a document verbatim. If it could be copied across unchanged, leave it in the plan.
- Three figures minimum, five maximum. A figure that a sentence already covers is deleted.
- Three accents, whose meaning is declared in the legend. No hardcoded colours, ever.
- Self-contained: no external assets, no scripts, no fonts, no images.
- Publish only when explicitly asked.

## Render a Brief

### 1. Locate the plan

Resolve the ticket ID to `plans/{service}/{ticket-id}/`. If there is no `plan.yaml` there, stop and
say the plan must be created with `feature-design` first. Do not create plan documents here.

### 2. Read everything

`plan.yaml`, `README.md`, every file under `scenario/`, every `design-0N-*.md`,
`implementation-plan.md`, and `automation/README.md` if present. Read the code a figure depends on —
a figure drawn from the plan's prose alone will be subtly wrong. Check `CLAUDE.md` for the ownership
and security rules a scope section has to respect.

### 3. Choose the figures first

Before writing any HTML, state in chat — one line each — the three to five figures and what each
one **proves**. Figures are the point of the page; the sections are scaffolding around them. See
the type table in `FIGURES.md`. If a figure's line reads "shows the components", it is a table.

### 4. Declare the palette

Assign a meaning to `accent-a`, `accent-b` and `accent-stop` for this page, and write the legend.
Do this before drawing, so no box gets a colour for aesthetic reasons.

### 5. Scaffold

```bash
bash .claude/skills/feature-brief/scripts/init_brief.sh {service} {ticket-id}
```

Writes `brief.html` with the stylesheet inlined and the masthead filled from `plan.yaml` and
`README.md`. It refuses to overwrite an existing brief without `--force`. Everything else stays a
`{{PLACEHOLDER}}` on purpose — the validator fails until each one is written.

### 6. Fill the page

Section by section, following `NARRATIVE.md`. Delete the starter figures you are not using and
renumber the remaining marker ids. Draw new figures against `FIGURES.md` — grid stops, character
budgets, and per-figure marker id prefixes are not optional.

### 7. Verify

```bash
python3 .claude/skills/feature-brief/scripts/check_brief.py plans/{service}/{ticket-id}/brief.html
```

Then open the page and look at it. Overflow detection is an estimate, not a measurement, so a green
script is necessary and not sufficient:

- Open `file://` + the absolute path in the browser pane.
- Screenshot at desktop 1280×800 and mobile 375×812, in both light and dark.
- Confirm: no horizontal scroll on the page body, figures scroll only inside `.frame`, no
  unreadable text in dark mode, phase cards collapse to one column under 620px.

Fix and re-check. Do not report the brief as done on the script alone.

### 8. Publish only if asked

When asked for a shareable link, use `Artifact` with `file_path` set to the brief, `description` set
to the standfirst, and a `favicon` that stays stable across re-renders. Publishing sends the plan's
contents to an external service — never do it by default, and never for a plan whose contents the
repository's security rules say must stay private.

## Backfill

Backfilling an older plan is the same eight steps, with two differences.

Older plans predate the brief, so their documents were not written with figures in mind. Expect
step 3 to be harder: the material for a figure is often spread across several documents, or sits in
the code rather than the plan. Read the code. If a figure cannot be drawn honestly from what exists,
say so and leave it out rather than inventing structure.

Add the `Visual Brief` line to the plan's `README.md` `## Documents` list as part of the backfill —
older READMEs do not have it.

**When asked to backfill several tickets, do them one at a time and report each before starting the
next.** Do not scaffold them all up front: a directory of placeholder pages fails the validator, and
a half-filled brief committed to the repo is worse than no brief. Ticket order does not matter, but
if the plans are related, do the one the others reference first so its terminology sets the pattern.

## Re-render

When the plan changes, re-render the affected sections from the markdown rather than hand-patching
the page. A brief and a plan that drift apart are the failure most briefs are written to document.
