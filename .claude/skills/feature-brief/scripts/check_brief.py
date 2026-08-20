#!/usr/bin/env python3
"""Validate a feature-brief page.

SVG text does not wrap. When a label is too long for its card it overflows
silently: nothing errors, nothing reflows, the page just looks wrong. Every
check here exists because its failure mode is invisible in the source.

Usage:
    python3 check_brief.py plans/{service}/{ticket-id}/brief.html
    python3 check_brief.py --quiet plans/platform/dap-005/brief.html

Exit codes: 0 clean (warnings allowed), 1 errors found, 2 could not read.
"""

from __future__ import annotations

import argparse
import re
import sys
import xml.etree.ElementTree as ET
from dataclasses import dataclass, field
from pathlib import Path

SVG_NS = "http://www.w3.org/2000/svg"

# Advance-width factors for the stack in --sans, as a fraction of font-size.
# Measured against system-ui/Helvetica at 11-14px; deliberately a little
# generous so the checker does not cry wolf on borderline labels.
WIDTH_REGULAR = 0.57
WIDTH_BOLD = 0.60

BOX_PADDING = 2.0  # px of breathing room required inside a rect
HARD_MARGIN = 4.0  # overflow beyond this is an error, below it a warning

PLACEHOLDER_RE = re.compile(r"\{\{[^}]*\}\}|\bTODO\b|\bTBD\b|\bFIXME\b")
# A colour literal, not an entity (&#215;) and not a url(#id) fragment reference.
HEX_RE = re.compile(r"(?<![&\w(])#[0-9A-Fa-f]{3,8}\b")
COMMENT_RE = re.compile(r"<!--.*?-->", re.DOTALL)
VAR_USE_RE = re.compile(r"var\(\s*(--[A-Za-z0-9-]+)\s*\)")
VAR_DEF_RE = re.compile(r"^\s*(--[A-Za-z0-9-]+)\s*:", re.MULTILINE)
EXTERNAL_RE = re.compile(r"""(?:src|href)\s*=\s*["']\s*(https?:)?//""", re.IGNORECASE)
CSS_URL_RE = re.compile(r"""url\(\s*["']?\s*(?:https?:)?//""", re.IGNORECASE)


@dataclass
class Report:
    errors: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)

    def error(self, msg: str) -> None:
        self.errors.append(msg)

    def warn(self, msg: str) -> None:
        self.warnings.append(msg)


@dataclass
class Rect:
    x: float
    y: float
    w: float
    h: float

    @property
    def area(self) -> float:
        return self.w * self.h

    def contains(self, px: float, py: float) -> bool:
        return self.x <= px <= self.x + self.w and self.y <= py <= self.y + self.h


def num(value: str | None, default: float = 0.0) -> float:
    if value is None:
        return default
    try:
        return float(re.sub(r"[a-z%]+$", "", value.strip()))
    except ValueError:
        return default


def local(tag: str) -> str:
    return tag.split("}", 1)[-1]


# --------------------------------------------------------------------------
# 1. no external references
# --------------------------------------------------------------------------

def check_self_contained(html: str, rep: Report) -> None:
    for match in EXTERNAL_RE.finditer(html):
        line = html.count("\n", 0, match.start()) + 1
        rep.error(f"line {line}: external src/href — the page must be self-contained")
    for match in CSS_URL_RE.finditer(html):
        line = html.count("\n", 0, match.start()) + 1
        rep.error(f"line {line}: external url() in CSS — inline the asset or drop it")
    if "<link" in html.lower():
        rep.error("<link> element present — inline the stylesheet into <style>")


# --------------------------------------------------------------------------
# 2. SVG text overflow  (the reason this script exists)
# --------------------------------------------------------------------------

def collect_rects(node: ET.Element, rects: list[Rect]) -> None:
    for child in node.iter():
        if local(child.tag) == "rect":
            w = num(child.get("width"))
            h = num(child.get("height"))
            if w > 0 and h > 0:
                rects.append(Rect(num(child.get("x")), num(child.get("y")), w, h))


def inherited(node: ET.Element, parents: dict, attr: str, default: str) -> str:
    seen = node
    while seen is not None:
        value = seen.get(attr)
        if value:
            return value
        seen = parents.get(id(seen))
    return default


def check_svg_text(svg: ET.Element, index: int, rep: Report) -> None:
    parents = {}
    for parent in svg.iter():
        for child in parent:
            parents[id(child)] = parent

    view_box = (svg.get("viewBox") or "0 0 1020 500").split()
    canvas_w = num(view_box[2]) if len(view_box) == 4 else 1020.0

    rects: list[Rect] = []
    collect_rects(svg, rects)

    for text in svg.iter():
        if local(text.tag) != "text":
            continue
        content = "".join(text.itertext()).strip()
        if not content:
            continue
        if text.get("transform"):
            continue  # rotated labels are not measurable this way

        fs = num(inherited(text, parents, "font-size", "12"), 12.0)
        weight = inherited(text, parents, "font-weight", "400")
        factor = WIDTH_BOLD if weight in ("bold", "600", "700", "800", "900") else WIDTH_REGULAR
        width = len(content) * fs * factor

        tx = num(text.get("x"))
        ty = num(text.get("y"))
        anchor = inherited(text, parents, "text-anchor", "start")
        if anchor == "middle":
            x0, x1 = tx - width / 2, tx + width / 2
        elif anchor == "end":
            x0, x1 = tx - width, tx
        else:
            x0, x1 = tx, tx + width

        label = content if len(content) <= 44 else content[:41] + "..."
        where = f"figure {index}, \"{label}\""

        # The innermost rect whose box contains the anchor point.
        holders = [r for r in rects if r.contains(tx, ty)]
        if holders:
            box = min(holders, key=lambda r: r.area)
            left = box.x + BOX_PADDING
            right = box.x + box.w - BOX_PADDING
            over = max(left - x0, x1 - right, 0.0)
            if x1 - right > HARD_MARGIN or left - x0 > HARD_MARGIN:
                budget = int((box.w - 2 * BOX_PADDING) / (fs * factor))
                rep.error(
                    f"{where} overflows its {box.w:.0f}px box by ~{over:.0f}px "
                    f"({len(content)} chars, budget ~{budget} at {fs:g}px)"
                )
            elif x1 > right or x0 < left:
                rep.warn(f"{where} sits within {HARD_MARGIN:g}px of its box edge")
        else:
            if x1 > canvas_w - HARD_MARGIN or x0 < 0:
                rep.error(
                    f"{where} is a free label running past the canvas "
                    f"(ends at ~{x1:.0f} of {canvas_w:.0f})"
                )


# --------------------------------------------------------------------------
# 3. marker ids
# --------------------------------------------------------------------------

def check_markers(svgs: list[tuple[int, ET.Element]], rep: Report) -> None:
    defined: dict[str, int] = {}
    referenced: set[str] = set()

    for index, svg in svgs:
        for node in svg.iter():
            node_id = node.get("id")
            if node_id:
                if node_id in defined:
                    rep.error(
                        f"id \"{node_id}\" defined in figure {index} and figure "
                        f"{defined[node_id]} — ids are global, prefix them per figure"
                    )
                else:
                    defined[node_id] = index
            for attr in ("marker-end", "marker-start", "marker-mid", "fill", "stroke"):
                value = node.get(attr) or ""
                ref = re.match(r"url\(#([^)]+)\)", value.strip())
                if ref:
                    referenced.add(ref.group(1))

    for ref in sorted(referenced - set(defined)):
        rep.error(f"url(#{ref}) referenced but never defined")


# --------------------------------------------------------------------------
# 4. token discipline
# --------------------------------------------------------------------------

def check_tokens(html: str, svg_blocks: list[str], rep: Report) -> None:
    style = re.search(r"<style[^>]*>(.*?)</style>", html, re.DOTALL)
    defined = set(VAR_DEF_RE.findall(style.group(1))) if style else set()
    if not defined:
        rep.error("no <style> block with custom properties found")

    for index, block in enumerate(svg_blocks, start=1):
        for match in HEX_RE.finditer(block):
            rep.error(
                f"figure {index}: hardcoded colour {match.group(0)} — "
                f"use a var(--token) so dark mode works"
            )

    for name in sorted(set(VAR_USE_RE.findall(html))):
        if name not in defined:
            rep.error(f"var({name}) used but not defined in :root")


# --------------------------------------------------------------------------
# 5. accessibility
# --------------------------------------------------------------------------

def check_accessibility(html: str, svgs: list[tuple[int, ET.Element]], rep: Report) -> None:
    for index, svg in svgs:
        if svg.get("role") != "img":
            rep.error(f"figure {index}: <svg> is missing role=\"img\"")
        label = (svg.get("aria-label") or "").strip()
        if not label:
            rep.error(f"figure {index}: <svg> is missing a non-empty aria-label")
        elif len(label) < 25:
            rep.warn(f"figure {index}: aria-label is very short — describe what it shows")

    figures = re.findall(r"<figure\b.*?</figure>", html, re.DOTALL)
    for index, figure in enumerate(figures, start=1):
        if "<figcaption" not in figure:
            rep.error(f"figure {index}: no <figcaption> — every figure needs a takeaway")


# --------------------------------------------------------------------------
# 6. page spine
# --------------------------------------------------------------------------

def check_spine(html: str, rep: Report) -> None:
    required = [
        ("masthead", 'class="masthead"', "section 1: masthead"),
        ("legend", 'class="legend"', "section 2: legend"),
        ("phases", 'class="phases"', "section 4: delivery phase cards"),
        ("foot", 'class="foot"', "section 8: provenance footer"),
    ]
    for _, needle, description in required:
        if needle not in html:
            rep.error(f"missing {description} ({needle})")

    keys = html.count('class="legend-key"')
    if keys and keys < 2:
        rep.error("the legend has fewer than 2 keys — state what each accent means")

    figure_count = len(re.findall(r"<figure\b", html))
    if figure_count < 3:
        rep.warn(f"only {figure_count} figures — the spine expects 3 to 5")
    elif figure_count > 5:
        rep.warn(f"{figure_count} figures — more than 5 stops being a brief")

    if not re.search(r"<title>\s*\S", html):
        rep.error("no <title> — it names the page in the tab and the artifact gallery")

    for match in PLACEHOLDER_RE.finditer(html):
        line = html.count("\n", 0, match.start()) + 1
        rep.error(f"line {line}: unfilled placeholder {match.group(0)!r}")


# --------------------------------------------------------------------------

def parse_svgs(html: str, rep: Report) -> tuple[list[tuple[int, ET.Element]], list[str]]:
    """Return (figure_number, element) pairs so a parse failure does not renumber
    every figure after it."""
    blocks = re.findall(r"<svg\b.*?</svg>", html, re.DOTALL)
    parsed: list[tuple[int, ET.Element]] = []
    for index, block in enumerate(blocks, start=1):
        # Authors write token names in comments, and "--" is illegal inside an
        # XML comment. Strip comments rather than making that an author's problem.
        xml = COMMENT_RE.sub("", block).replace("&nbsp;", "&#160;")
        try:
            parsed.append((index, ET.fromstring(xml)))
        except ET.ParseError as exc:
            rep.error(f"figure {index}: SVG is not well-formed XML ({exc})")
    return parsed, blocks


def main() -> int:
    ap = argparse.ArgumentParser(description="Validate a feature-brief page.")
    ap.add_argument("path", help="path to brief.html")
    ap.add_argument("--quiet", action="store_true", help="suppress warnings")
    args = ap.parse_args()

    path = Path(args.path)
    try:
        html = path.read_text(encoding="utf-8")
    except OSError as exc:
        print(f"cannot read {path}: {exc}", file=sys.stderr)
        return 2

    rep = Report()
    svgs, blocks = parse_svgs(html, rep)

    check_self_contained(html, rep)
    for index, svg in svgs:
        check_svg_text(svg, index, rep)
    check_markers(svgs, rep)
    check_tokens(html, blocks, rep)
    check_accessibility(html, svgs, rep)
    check_spine(html, rep)

    for message in rep.errors:
        print(f"ERROR  {message}")
    if not args.quiet:
        for message in rep.warnings:
            print(f"warn   {message}")

    print(
        f"\n{path}: {len(rep.errors)} error(s), {len(rep.warnings)} warning(s), "
        f"{len(svgs)} figure(s)"
    )
    if not rep.errors:
        print("Text overflow is estimated, not measured. Open the page and look at it.")
    return 1 if rep.errors else 0


if __name__ == "__main__":
    sys.exit(main())
