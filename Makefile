# Two lanes: `make up` runs a Docker stack, `make dev-up` runs the native server
# through run.sh. The lane verbs live in the vendored docker-local.mk and dev-local.mk.

GO ?= go
NPM ?= npm

.DEFAULT_GOAL := help

# Docker lane

STACKS := local cloud
STACK ?= local

DIR_local := deploy/docker/local/local-mode
FILE_local := compose.yaml
URL_local := http://127.0.0.1:$(or $(KODE_STREAM_LOCAL_PORT),4317)
# /api/health reports database status only, so it answers 503 under datadir storage.
PROBE_local := $(URL_local)/
# The script checks the storage option and the workspace mount before compose up.
UP_local := deploy/docker/local/local-mode/run.sh

DIR_cloud := deploy/docker/local/cloud-mode
FILE_cloud := compose.yaml
URL_cloud := $(or $(KODE_STREAM_CLOUD_URL),http://kode-stream.localhost:4318)
PROBE_cloud := $(URL_cloud)/api/health
# The script mints an agent token and waits for health.
UP_cloud := deploy/docker/local/cloud-mode/run.sh

# Both stacks are the same product, so `up-all` starts Local mode only.
ALL_STACKS := local

include docker-local.mk

# Native lane

DEV_START := ./run.sh start
DEV_STOP := ./run.sh stop
DEV_RESTART := ./run.sh restart
DEV_PS := ./run.sh status
DEV_LOGS := tail -f .run/kode-stream.log
DEV_BUILD := $(NPM) run build && $(GO) build -o bin/kode-stream ./cmd/kode-stream

include dev-local.mk

# Verification

.PHONY: build binary typecheck test race verify smoke-storage help

build:
	$(NPM) run build

binary:
	$(GO) build -o bin/kode-stream ./cmd/kode-stream

typecheck:
	$(NPM) run typecheck

test:
	$(NPM) test
	$(GO) test ./...

race:
	$(GO) test -race ./...

smoke-storage:
	./run.sh smoke-storage

verify: typecheck test race build

help: docker-help
	@printf '\n'
	@$(MAKE) --no-print-directory dev-help
	@printf '\nVerification\n\n'
	@printf '  make verify           typecheck, test, race, build\n'
	@printf '  make build            frontend bundle\n'
	@printf '  make binary           go build ./cmd/kode-stream\n'
	@printf '  make test             vitest and go test\n'
	@printf '  make smoke-storage    native start under both storage options\n'
	@printf '\n'
	@$(MAKE) --no-print-directory stacks
