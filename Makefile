# Front door for local work in kode-stream.
#
# Two lanes, the same shape as in agent-plane and context-cellar:
#
#   docker  the default. `make up` starts a stack from deploy/docker/local.
#   native  `make dev` builds the frontend and the Go binary and serves them
#           from this checkout, backgrounded, with no image rebuild in the
#           loop. This is what ./run.sh has always done; the Makefile is now
#           the way in, and run.sh is the implementation.
#
# The stack table below is the only part of the docker lane specific to this
# repository; the verbs live in the vendored docker-local.mk.

GO ?= go
NPM ?= npm

.DEFAULT_GOAL := help

# --- Docker lane ------------------------------------------------------------

STACKS := local cloud
STACK ?= local

# Local mode: one container, workspace bind-mounted, datadir or database storage.
DIR_local := deploy/docker/local/local-mode
FILE_local := compose.yaml
# Follows KODE_STREAM_LOCAL_PORT, the same variable the compose file publishes on.
URL_local := http://127.0.0.1:$(or $(KODE_STREAM_LOCAL_PORT),4317)
# The app root, not /api/health: that endpoint reports database status only, so
# under the default `datadir` storage option it answers 503 while the server is
# serving normally. (The image's own HEALTHCHECK has the same problem, so a
# datadir container never reports healthy. Pre-existing and product-side.)
PROBE_local := $(URL_local)/
# Not a plain `compose up`. The script validates KODE_STREAM_STORAGE_OPTION and
# the workspace mount before starting, so a typo fails with a sentence instead
# of a half-started container.
UP_local := deploy/docker/local/local-mode/run.sh

# Cloud mode: Keycloak and oauth2-proxy in front, Postgres behind, and a local
# agent registered against it.
DIR_cloud := deploy/docker/local/cloud-mode
FILE_cloud := compose.yaml
URL_cloud := $(or $(KODE_STREAM_CLOUD_URL),http://kode-stream.localhost:4318)
# Cloud mode runs on database storage, where /api/health is meaningful.
PROBE_cloud := $(URL_cloud)/api/health
# Start-up mints an agent token and gates on health; compose alone cannot.
UP_cloud := deploy/docker/local/cloud-mode/run.sh

# The two stacks are the same product in its two deployment shapes, not two
# halves of one system, and each wants port 4317/4318 and its own Postgres.
# `up-all` therefore starts Local mode only; reach for Cloud mode by name.
ALL_STACKS := local

include docker-local.mk

# --- Native lane ------------------------------------------------------------

DEV_START := ./run.sh start
DEV_STOP := ./run.sh stop
DEV_RESTART := ./run.sh restart
DEV_STATUS := ./run.sh status
DEV_LOGS := tail -f .run/kode-stream.log
DEV_BUILD := $(NPM) run build && $(GO) build -o bin/kode-stream ./cmd/kode-stream

include dev-local.mk

# --- Verification -----------------------------------------------------------

.PHONY: build binary typecheck test verify smoke-storage help

build:
	$(NPM) run build

binary:
	$(GO) build -o bin/kode-stream ./cmd/kode-stream

typecheck:
	$(NPM) run typecheck

test:
	$(NPM) test
	$(GO) test ./...

# Starts the native server once per storage option and health-checks each.
smoke-storage:
	./run.sh smoke-storage

verify: typecheck test build

help: docker-help
	@printf '\n'
	@$(MAKE) --no-print-directory dev-help
	@printf '\nVerification\n\n'
	@printf '  make verify           typecheck, test, build\n'
	@printf '  make build            frontend bundle\n'
	@printf '  make binary           go build ./cmd/kode-stream\n'
	@printf '  make test             vitest and go test\n'
	@printf '  make smoke-storage    native start under both storage options\n'
	@printf '\n'
	@$(MAKE) --no-print-directory stacks
