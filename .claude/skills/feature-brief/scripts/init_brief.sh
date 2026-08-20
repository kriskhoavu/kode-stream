#!/usr/bin/env bash
# Scaffold plans/{service}/{ticket-id}/brief.html from the feature-brief starter,
# with the stylesheet inlined so the page is self-contained.
#
# Usage: bash .claude/skills/feature-brief/scripts/init_brief.sh {service} {ticket-id} [--force]

set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "${SKILL_DIR}/../../.." && pwd)"

VALID_SERVICES="api api-worker article customer customer-worker user gateway aggregate translation mail salesapi esb commons webapp platform"

usage() {
  echo "usage: init_brief.sh {service} {ticket-id} [--force]" >&2
  echo "  services: ${VALID_SERVICES}" >&2
  exit 1
}

[ $# -ge 2 ] || usage

SERVICE="$1"
TICKET_ID="$2"
FORCE="no"
shift 2
while [ $# -gt 0 ]; do
  case "$1" in
    --force) FORCE="yes" ;;
    *) echo "unknown argument: $1" >&2; usage ;;
  esac
  shift
done

case " ${VALID_SERVICES} " in
  *" ${SERVICE} "*) ;;
  *) echo "error: unknown service '${SERVICE}'" >&2; usage ;;
esac

PLAN_DIR="${REPO_ROOT}/plans/${SERVICE}/${TICKET_ID}"
BRIEF="${PLAN_DIR}/brief.html"

if [ ! -f "${PLAN_DIR}/plan.yaml" ]; then
  echo "error: no plan at plans/${SERVICE}/${TICKET_ID}/" >&2
  echo "feature-brief renders an existing plan. Use feature-design to create one first." >&2
  exit 1
fi

if [ -f "${BRIEF}" ] && [ "${FORCE}" != "yes" ]; then
  echo "error: ${BRIEF#"${REPO_ROOT}/"} already exists." >&2
  echo "Edit it in place, or pass --force to overwrite it with a fresh scaffold." >&2
  exit 1
fi

# --- gather masthead facts from the plan ----------------------------------

STATUS="$(sed -n 's/^[[:space:]]*status:[[:space:]]*//p' "${PLAN_DIR}/plan.yaml" | head -1)"
[ -n "${STATUS}" ] || STATUS="draft"

FEATURE_NAME=""
if [ -f "${PLAN_DIR}/README.md" ]; then
  FEATURE_NAME="$(sed -n 's/^#[[:space:]]*//p' "${PLAN_DIR}/README.md" | head -1)"
  FEATURE_NAME="${FEATURE_NAME#*: }"
fi
[ -n "${FEATURE_NAME}" ] || FEATURE_NAME="${TICKET_ID}"

# Tracks come from the design document suffixes. Canonical track names first;
# plans that name their design docs by subject fall back to those subjects.
TRACKS=""
for track in backend frontend infrastructure pipeline; do
  if ls "${PLAN_DIR}"/design/design-*-"${track}".md >/dev/null 2>&1; then
    TRACKS="${TRACKS:+${TRACKS} · }${track}"
  fi
done
if [ -z "${TRACKS}" ]; then
  for doc in "${PLAN_DIR}"/design/design-*.md; do
    [ -e "${doc}" ] || continue
    name="$(basename "${doc}" .md)"
    TRACKS="${TRACKS:+${TRACKS} · }${name#design-*-}"
  done
fi
[ -n "${TRACKS}" ] || TRACKS="—"

DATE="$(date +%Y-%m-%d)"
COMMIT="$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || echo "uncommitted")"

# --- render ---------------------------------------------------------------

TMP="$(mktemp)"
trap 'rm -f "${TMP}" "${TMP}.css"' EXIT

# Indent the stylesheet by two spaces so it reads correctly inside <style>.
sed 's/^./  &/' "${SKILL_DIR}/assets/brief.css" > "${TMP}.css"

awk -v cssfile="${TMP}.css" '
  /^\/\* __BRIEF_CSS__ \*\/$/ {
    while ((getline line < cssfile) > 0) print line
    close(cssfile)
    next
  }
  { print }
' "${SKILL_DIR}/assets/starter.html" > "${TMP}"

# Substitute only the masthead and footer facts. Everything else stays a
# {{PLACEHOLDER}} on purpose, so check_brief.py fails until it is written.
python3 - "${TMP}" "${BRIEF}" <<PYEOF
import sys
src, dst = sys.argv[1], sys.argv[2]
values = {
    "{{TICKET_ID}}": """${TICKET_ID}""",
    "{{SERVICE}}": """${SERVICE}""",
    "{{TRACKS}}": """${TRACKS}""",
    "{{STATUS}}": """${STATUS}""",
    "{{FEATURE_NAME}}": """${FEATURE_NAME}""",
    "{{DATE}}": """${DATE}""",
    "{{COMMIT}}": """${COMMIT}""",
}
text = open(src, encoding="utf-8").read()
for key, value in values.items():
    text = text.replace(key, value)
open(dst, "w", encoding="utf-8").write(text)
PYEOF

REL="${BRIEF#"${REPO_ROOT}/"}"
echo "created ${REL}"
echo "open    file://${BRIEF}"
echo
echo "still to fill:"
grep -o '{{[A-Z_0-9]*}}' "${BRIEF}" | sort -u | sed 's/^/  /'
echo
echo "next: read references/NARRATIVE.md for the section-by-section mapping,"
echo "      and references/FIGURES.md before touching any SVG."
