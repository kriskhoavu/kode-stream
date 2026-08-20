#!/usr/bin/env bash
# init_plan.sh — Scaffold the plans/{service}/{ticket-id}/ directory structure
#
# Usage:
#   init_plan.sh <ticket-id> <short-description> <service> [tracks]
#
#   service: owning service (api, article, customer, api-worker, customer-worker,
#            user, gateway, aggregate, translation, mail, salesapi, esb, commons,
#            webapp, platform)
#   tracks:  comma-separated list of: backend,frontend,devops  (default: backend,frontend)
#
# Examples:
#   init_plan.sh DI-1234 custom-assortment api                       # api service, backend + frontend (default)
#   init_plan.sh DI-1234 my-feature        api    backend            # api service, backend only
#   init_plan.sh DI-1234 my-feature        webapp frontend           # webapp service, frontend only
#   init_plan.sh DI-1234 my-pipeline       platform devops           # platform service, devops only
#   init_plan.sh DI-1234 my-feature        api    backend,devops     # api service, backend + devops
#   init_plan.sh DI-1234 my-feature        api    backend,frontend,devops
#
# Creates:
#   plans/{service}/DI-1234/
#   ├── plan.yaml                        (minimal Plan Manager metadata)
#   ├── README.md                        (hub: overview, glossary, components, data flow, decisions, links)
#   ├── scenario/
#   │   └── scenario-00-overview.md
#   ├── design/
#   │   ├── design-0N-backend.md        (if backend track)
#   │   ├── design-0N-frontend.md       (if frontend track)
#   │   ├── design-0N-infrastructure.md (if devops track)
#   │   └── design-0N-pipeline.md       (if devops track)
#   ├── automation/
#   │   ├── README.md
#   │   └── results/latest.md
#   └── implementation-plan.md
#
# Design files are numbered sequentially from 01 based on which
# tracks are included and in order: backend → frontend → devops.

set -euo pipefail

TICKET="${1:-}"
DESC="${2:-feature}"
SERVICE="${3:-}"
TRACKS="${4:-backend,frontend}"

VALID_SERVICES="api api-worker article customer customer-worker user gateway aggregate translation mail salesapi esb commons webapp platform"

if [[ -z "$TICKET" || -z "$SERVICE" ]]; then
  echo "Usage: $0 <ticket-id> <short-description> <service> [tracks]"
  echo "  service: api | article | customer | api-worker | customer-worker | user |"
  echo "           gateway | aggregate | translation | mail | salesapi | esb |"
  echo "           commons | webapp | platform"
  echo "  tracks:  comma-separated list of: backend,frontend,devops  (default: backend,frontend)"
  echo ""
  echo "  e.g.: $0 DI-1234 custom-assortment api"
  echo "  e.g.: $0 DI-1234 my-pipeline platform devops"
  echo "  e.g.: $0 DI-1234 my-feature api backend,frontend,devops"
  exit 1
fi

# Validate service
SERVICE_VALID=false
for s in $VALID_SERVICES; do
  [[ "$SERVICE" == "$s" ]] && SERVICE_VALID=true && break
done
if [[ "$SERVICE_VALID" == "false" ]]; then
  echo "❌ Unknown service: '${SERVICE}'. Valid services: ${VALID_SERVICES}"
  exit 1
fi

# Parse tracks into flags
HAS_BACKEND=false
HAS_FRONTEND=false
HAS_DEVOPS=false

IFS=',' read -ra TRACK_LIST <<< "$TRACKS"
for track in "${TRACK_LIST[@]}"; do
  case "$track" in
    backend)  HAS_BACKEND=true ;;
    frontend) HAS_FRONTEND=true ;;
    devops)   HAS_DEVOPS=true ;;
    *)
      echo "❌ Unknown track: ${track}. Valid tracks: backend, frontend, devops"
      exit 1
      ;;
  esac
done

if [[ "$HAS_BACKEND" == "false" && "$HAS_FRONTEND" == "false" && "$HAS_DEVOPS" == "false" ]]; then
  echo "❌ No valid tracks specified. Use: backend, frontend, devops (comma-separated)"
  exit 1
fi

PLAN_DIR="plans/${SERVICE}/${TICKET}"

if [[ -d "$PLAN_DIR" ]]; then
  echo "⚠️  Directory ${PLAN_DIR}/ already exists. Aborting."
  exit 1
fi

mkdir -p "${PLAN_DIR}/scenario" "${PLAN_DIR}/design" "${PLAN_DIR}/automation/results" "${PLAN_DIR}/automation/artifacts"

# ── plan.yaml ────────────────────────────────────────────────────────────────
cat > "${PLAN_DIR}/plan.yaml" << EOF
plan:
  status: draft
  wiki_enriched: false
  e2e-runbook: false
automation-test:
  - path: "automation/README.md"
EOF

# ── Build dynamic lists for README design links ──────────────────────────
DESIGN_LINKS=""
IMPL_TRACK_LABELS=""
FILE_NUM=1

if [[ "$HAS_BACKEND" == "true" ]]; then
  BACKEND_FILE="design-0${FILE_NUM}-backend.md"
  DESIGN_LINKS+="- [Backend Design](design/${BACKEND_FILE})\n"
  IMPL_TRACK_LABELS+=" Backend"
  FILE_NUM=$((FILE_NUM + 1))
fi
if [[ "$HAS_FRONTEND" == "true" ]]; then
  FRONTEND_FILE="design-0${FILE_NUM}-frontend.md"
  DESIGN_LINKS+="- [Frontend Design](design/${FRONTEND_FILE})\n"
  IMPL_TRACK_LABELS+=" Frontend"
  FILE_NUM=$((FILE_NUM + 1))
fi
if [[ "$HAS_DEVOPS" == "true" ]]; then
  INFRA_FILE="design-0${FILE_NUM}-infrastructure.md"
  FILE_NUM=$((FILE_NUM + 1))
  PIPELINE_FILE="design-0${FILE_NUM}-pipeline.md"
  FILE_NUM=$((FILE_NUM + 1))
  DESIGN_LINKS+="- [Infrastructure Design](design/${INFRA_FILE})\n"
  DESIGN_LINKS+="- [Pipeline Design](design/${PIPELINE_FILE})\n"
  IMPL_TRACK_LABELS+=" DevOps"
fi

# ── README.md ────────────────────────────────────────────────────────────────
# Build components rows based on tracks
COMP_ROWS=""
[[ "$HAS_BACKEND" == "true" ]]  && COMP_ROWS+="| Domain     |           |         |\n| Service    |           |         |\n| Controller |           |         |\n"
[[ "$HAS_FRONTEND" == "true" ]] && COMP_ROWS+="| Frontend   |           |         |\n"
[[ "$HAS_DEVOPS" == "true" ]]   && COMP_ROWS+="| Build      |           |         |\n| Pipeline   |           |         |\n"

cat > "${PLAN_DIR}/README.md" << EOF
# ${TICKET}: {Feature Name}

## Overview

TODO: One-paragraph description of what this feature does and why.

## Glossary

| Term  | Meaning    | Maps To (code) |
| ----- | ---------- | -------------- |
| term1 | definition | field/class    |

## Components

| Layer      | Component | Purpose |
| ---------- | --------- | ------- |
$(printf '%b' "$COMP_ROWS")
## Data Flow

\`\`\`
TODO: Add data flow diagram (ASCII)
\`\`\`

## Design Decisions

| Decision | Alternatives Considered | Rationale |
| -------- | ----------------------- | --------- |
|          |                         |           |

## Documents

- [Scenario Overview](scenario/scenario-00-overview.md)
$(printf '%b' "$DESIGN_LINKS")- [UI Automation](automation/README.md)
- [Implementation Plan](implementation-plan.md)
EOF

cat > "${PLAN_DIR}/automation/README.md" << EOF
# UI Automation: ${TICKET}

## Applicability

$( [[ "$HAS_FRONTEND" == "true" ]] && printf '%s' 'Browser validation is required because this plan includes a frontend track.' || printf '%s' 'Not applicable: this plan has no frontend track or other user-visible workflow. Reassess if implementation adds one.' )

## Runtime Inputs

| Input | Source |
| ----- | ------ |
| Base URL | Supplied when executing |
| Authentication | Supplied when executing; do not commit secrets |
| Test data | Supplied when executing |

## Playbooks

| Playbook | Covers | Status |
| -------- | ------ | ------ |
$( [[ "$HAS_FRONTEND" == "true" ]] && printf '%s' '| TODO | TODO | Draft |' || printf '%s' '| None | No user-visible workflow planned | Not applicable |' )

## Latest Result

- [Latest execution result](results/latest.md)

## Reusable E2E Coverage

Not requested. Set plan.e2e-runbook: true only when this plan creates or changes reusable user-journey coverage.
EOF

cat > "${PLAN_DIR}/automation/results/latest.md" << EOF
# UI Automation Result: ${TICKET}

Status: Not run

Create and run provider-neutral browser playbooks with e2e-testing after implementation.
EOF

# ── scenario-00-overview.md ───────────────────────────────────────────────────
cat > "${PLAN_DIR}/scenario/scenario-00-overview.md" << EOF
# Scenarios: ${TICKET} Overview

## Scenario List

| # | Title              | Description                  |
| - | ------------------ | ---------------------------- |
| 0 | No customization   | Default state before feature |
| 1 | {Happy path}       | Main user flow               |

---

# Scenario 0: No Customization (Default State)

## Starting State

- TODO: Describe initial DB state
- TODO: Describe what the user sees

## Visual State (Before)

\`\`\`
┌────────────────────────────────────────┐
│ TODO: ASCII diagram                    │
└────────────────────────────────────────┘
\`\`\`

## Available Actions

| Action | Description | Flow |
| ------ | ----------- | ---- |
|        |             |      |

## Edge Cases

- TODO: What happens on empty state?
- TODO: What happens on reset?
EOF

# ── design-0N-backend.md ────────────────────────────────────────────────
if [[ "$HAS_BACKEND" == "true" ]]; then
cat > "${PLAN_DIR}/design/${BACKEND_FILE}" << EOF
# Backend Design: ${TICKET}

## Data Model

### Entity: {EntityName}

| Field | Type | Column | Purpose |
| ----- | ---- | ------ | ------- |
| id    | Long | id     | PK      |

### Database Schema

\`\`\`sql
-- TODO: Add CREATE TABLE / ALTER TABLE statements
\`\`\`

## API Endpoints

| Method | Endpoint | Request Body | Response |
| ------ | -------- | ------------ | -------- |
|        |          |              |          |

## Design Decisions

| Decision | Rationale |
| -------- | --------- |
|          |           |
EOF
fi

# ── design-0N-frontend.md ───────────────────────────────────────────────
if [[ "$HAS_FRONTEND" == "true" ]]; then
cat > "${PLAN_DIR}/design/${FRONTEND_FILE}" << EOF
# Frontend Design: ${TICKET}

## Service Layer

- \`application/service/{feature}/type.ts\` — request/response types
- \`application/service/{feature}/index.ts\` — service interface + impl

## State

- TODO: Zustand slice / standalone store

## Components

\`\`\`
{Feature}/
├── {Feature}.tsx             # Orchestrator
├── {Feature}.styled.ts       # Styled-components
├── types.ts                  # Feature-local types
├── component/                # Sub-components
└── hook/                     # Feature hooks
\`\`\`

## Query Keys

\`\`\`typescript
// application/config/queryKeys.ts
feature: (name: string) => ["offerDetail", "feature", name],
\`\`\`
EOF
fi

# ── design-0N-infrastructure.md ────────────────────────────────────────
if [[ "$HAS_DEVOPS" == "true" ]]; then
cat > "${PLAN_DIR}/design/${INFRA_FILE}" << EOF
# Infrastructure Design: ${TICKET}

## Gradle Tasks

| Task Name | Group      | Modules | Purpose |
| --------- | ---------- | ------- | ------- |
|           | publishing |         |         |

## Module Groups

| Group   | Modules | Gradle Task |
| ------- | ------- | ----------- |
| Group 1 |         |             |
| Group 2 |         |             |

## Docker / Registry

| Image | Registry | Tag Pattern |
| ----- | -------- | ----------- |
|       |          |             |

## Helm / Kubernetes (if applicable)

- HelmChart changes: TODO
- K8S resource changes (ConfigMap, Deployment, Service): TODO

## Design Decisions

| Decision | Rationale |
| -------- | --------- |
|          |           |
EOF

# ── design-0N-pipeline.md ──────────────────────────────────────────────
cat > "${PLAN_DIR}/design/${PIPELINE_FILE}" << EOF
# Pipeline Design: ${TICKET}

## Tag Patterns & Behavior

| Tag Pattern  | Build | Publish Internal | Publish External | Deploy |
| ------------ | ----- | ---------------- | ---------------- | ------ |
| nightly-*    | ✅    | ✅               | ❌               | ✅     |
| release-*    | ✅    | ✅               | ❌               | ✅     |
| cc-release-* | ❌    | ❌               | ✅ (re-tag only) | ❌     |

## Pipeline Stages

\`\`\`
Prepare → Build (parallel) → Deploy
            ├── Group 1
            └── Group 2
\`\`\`

## Options Considered

### Option A: {Description}

TODO: Describe option A.

### Option B: {Description}

TODO: Describe option B.

## Comparison

| Criterion | Option A | Option B |
| --------- | -------- | -------- |
|           |          |          |

## Design Decisions

| Decision | Alternatives Considered | Rationale |
| -------- | ----------------------- | --------- |
|          |                         |           |
EOF
fi

# ── implementation-plan.md ────────────────────────────────────────────────────
{
cat << EOF
# Implementation Plan: ${TICKET} - {Feature Name}

## Overview

TODO: Brief description of what will be implemented.

## Execution Flow

\`\`\`text
{Phase 1} Foundation ─────┐
                          ├─> {Phase 3} Integration ─> {Phase 4} Verification
{Phase 2} Parallel work ──┘
{External gate} ─────────────────────────────────────┘
\`\`\`

TODO: Replace the example with real phase IDs. Briefly explain the critical path, work that can run in parallel, where
branches join, and external gates that genuinely block progress. Use one left-to-right line when the plan is linear.

## Terminology Lock

All code, fields, API params, and TS types must use:

- \`TODO\` (not TODO_old_name)

> Lock this in before writing any code.

EOF

# Backend phases
if [[ "$HAS_BACKEND" == "true" ]]; then
cat << 'EOF'
## Backend Phases

### Phase B1: Domain Layer

**Deliverables:**

- [ ] `{Entity}.java`
- [ ] `{Entity}Repository.java`
- [ ] Liquibase changeset

**Verification:** `./gradlew :{service}:domain:build`

**Commit:** `{ticket-id}: Add {Entity} domain layer`

---

### Phase B2: Client Model

**Deliverables:**

- [ ] `{Feature}Model.java`
- [ ] `Create{Feature}Request.java`

**Verification:** `./gradlew :{service}:app:compileJava`

**Commit:** `{ticket-id}: Add {Feature} client model DTOs`

---

### Phase B3: Application Layer

**Deliverables:**

- [ ] `{Feature}Service.java`
- [ ] `{Feature}ServiceImpl.java`
- [ ] `{Feature}Controller.java`
- [ ] `{Feature}ModelBuilder.java`

**Verification:** `./gradlew :{service}:app:build`

**Commit:** `{ticket-id}: Add {Feature} service and controller`

---

EOF
fi

# Frontend phases
if [[ "$HAS_FRONTEND" == "true" ]]; then
cat << 'FEOF'
## Frontend Phases

> Follow `skills/frontend-feature/SKILL.md` for detailed templates.

### Phase F1: Types & Service

**Deliverables:**

- [ ] `service/{feature}/type.ts`
- [ ] `service/{feature}/index.ts`
- [ ] Registered in `serviceStore.ts`

**Verification:** `cd webapp && npx tsc --noEmit`

**Commit:** `{ticket-id}: Add {Feature} frontend service layer`

---

### Phase F2: State + Query Hooks

**Deliverables:**

- [ ] Zustand slice / store
- [ ] Query keys in `queryKeys.ts`
- [ ] `use{Feature}Query.ts`

**Verification:** `cd webapp && npx tsc --noEmit`

**Commit:** `{ticket-id}: Add {Feature} state and query hooks`

---

### Phase F3: Components

**Deliverables:**

- [ ] Main component
- [ ] Sub-components in `component/`
- [ ] Feature hooks in `hook/`

**Verification:** `cd webapp && npx jest --testPathPattern="{Feature}" --passWithNoTests`

**Commit:** `{ticket-id}: Add {Feature} components`

---

### Phase F4: Styling + Translations

**Deliverables:**

- [ ] `{Feature}.styled.ts`
- [ ] Translation keys in `localization/*.json`
- [ ] Visual parity check

**Verification:** Visual review in browser. Run full jest suite.

**Commit:** `{ticket-id}: Add {Feature} styling and translations`

---

FEOF
fi

# DevOps phases
if [[ "$HAS_DEVOPS" == "true" ]]; then
cat << 'CEOF'
## DevOps Phases

### Phase C1: Build / Infrastructure

**Deliverables:**

- [ ] Gradle task changes (`build.gradle`)
- [ ] HelmChart / K8S resource changes (if applicable)

**Verification:** `./gradlew tasks --group publishing`

**Commit:** `{ticket-id}: Add {description} Gradle tasks`

---

### Phase C2: Pipeline / CI

**Deliverables:**

- [ ] Jenkinsfile changes

**Verification:** Dry-run / lint Jenkinsfile locally

**Commit:** `{ticket-id}: Refactor Jenkinsfile for {description}`

---

CEOF
fi

cat << PEOF
## Post-Implementation Checklist

- [ ] Update \`plans/${SERVICE}/${TICKET}/\` docs to reflect any naming changes
- [ ] Update \`changelog.md\` with feature summary
- [ ] Verify no terminology drift: \`grep -r "{old-term}" --include="*.java" --include="*.ts"\`
- [ ] PR description references planning docs
PEOF
} > "${PLAN_DIR}/implementation-plan.md"

# ── Git branch ────────────────────────────────────────────────────────────────
BRANCH="feature/${TICKET}-${DESC}"
if git show-ref --quiet "refs/heads/${BRANCH}"; then
  echo "ℹ️  Branch ${BRANCH} already exists, skipping creation."
else
  git checkout -b "${BRANCH}"
  echo "✅ Created branch: ${BRANCH}"
fi

echo ""
echo "✅ Scaffolded: ${PLAN_DIR}/ (service: ${SERVICE}, tracks: ${TRACKS})"
echo "   ├── plan.yaml  (Plan Manager metadata)"
echo "   ├── README.md  (overview, glossary, components, data flow, decisions, links)"
echo "   ├── scenario/scenario-00-overview.md"
echo "   ├── automation/README.md"
[[ "$HAS_BACKEND" == "true" ]]  && echo "   ├── design/${BACKEND_FILE}"
[[ "$HAS_FRONTEND" == "true" ]] && echo "   ├── design/${FRONTEND_FILE}"
[[ "$HAS_DEVOPS" == "true" ]]   && echo "   ├── design/${INFRA_FILE}"
[[ "$HAS_DEVOPS" == "true" ]]   && echo "   ├── design/${PIPELINE_FILE}"
echo "   └── implementation-plan.md  (tracks:${IMPL_TRACK_LABELS})"
echo ""
echo "Next steps:"
echo "  1. Fill README.md: overview, components, data flow, glossary"
echo "  2. Complete scenario-00-overview.md with visual states"
echo "  3. Fill design docs with design details"
echo "  4. Add UI browser playbooks with e2e-testing when a user-visible workflow applies"
echo "  5. Run /md-formatting on ${PLAN_DIR}/ to fix table alignment"
echo "  6. Confirm implementation-plan.md phases, then say 'start implementing'"
