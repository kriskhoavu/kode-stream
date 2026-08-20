# Narrative

A brief is a reading document. Someone who will never open the plan directory should finish it
knowing what is being built, why, and what is still undecided.

That makes it different from the markdown it comes from. The markdown is a working artifact for
whoever implements the ticket. The brief is an argument.

## Voice

- Short, clear English. Same rule as `feature-design`.
- Reuse the plan's terminology exactly. If the plan says "bundle", the brief says "bundle". Never
  introduce a synonym — a new word reads as a new concept.
- Lead every section with its conclusion. The figure supports the sentence; the sentence does not
  caption the figure.
- Prefer "Today X. After the plan Y. Because Z." over restating headings.
- Say what is true, including what is weak. "The security model is already strong; what is weak is
  how configuration reaches the cluster" is worth ten paragraphs of neutral description.
- No filler transitions, no "it is important to note", no summarising what the reader just read.

## Never

- Restate a markdown document verbatim. If a paragraph could be copied across unchanged, it belongs
  in the plan, not the brief.
- Invent a fact the plan does not contain. If something is genuinely unknown, it belongs in
  section 7 as an open decision.
- Draw a figure for something a sentence already handles.
- Include a phase checklist. Phase cards carry the intent of each phase and its status; the
  deliverable checkboxes stay in `implementation-plan.md`.

## Section by section

**1. Masthead.** `h1` is a claim, not a title: "agent-platform after the runtime plan" beats
"DAP-007 Implementation". The standfirst says what the page covers in two or three sentences and
becomes the published `description`.

**2. The short version.** Two paragraphs. The first says what is true today and why it does not
hold. The second says what is true after the plan and what stays unchanged. Then the legend.
A reader who stops here should still have the argument.

**3. Figures.** Choose these before writing any HTML — see `FIGURES.md`. Each section is an
`.eyebrow` reading `Figure N`, an `h2` that is a claim, one short intro paragraph, the figure, and
a `figcaption` giving the takeaway.

**4. Delivery.** One card per phase from `implementation-plan.md`. The card says what the phase
*establishes* — the reason it exists and what becomes possible once it lands — plus its status from
the plan. Two or three bullets at most, and only where the deliverables are not obvious from the
sentence. Keep phase ids (`B1`, `F2`, `C1`) and order identical to the plan, and do not invent
dependencies to make the sequence look tidier.

**5. Reference.** One table, for the thing readers will come back to look up: ownership boundaries,
the glossary, or the API surface. Not all three. If two tables feel necessary, one of them is
detail that belongs in the plan.

**6. Scope.** What was deliberately left out, and why. Each item names the thing and the condition
under which it would come back in. This section prevents the most expensive kind of review comment.

**7. Open decisions.** One `.note.stop` per decision. Phrase the heading as a question. In the
body, give the options, what each costs, and who decides. An open decision with no named owner is
not open, it is forgotten.

**8. Provenance.** Source directory, date, commit, and the reminder that the markdown is the source
of truth and the page should be re-rendered rather than hand-patched.

## Code in a brief

Inherited from `feature-design`'s code block policy, tightened for a page with no code fences:

- Allowed inline in `<code>`: identifiers, file paths, config keys, tool names, short values.
- Not allowed: class bodies, YAML or JSON blocks, SQL, framework annotations, or anything that
  duplicates a source file. Use the table, a figure, or a sentence.

## Keeping it in sync

`brief.html` is regenerable output. When the plan changes, re-render the affected sections from the
markdown — do not hand-patch the page and the plan separately, or they will drift, which is exactly
the failure most briefs end up documenting.
