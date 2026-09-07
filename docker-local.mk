# Docker lane — every local stack in this repository runs as containers.
#
# Vendored copy. The source of truth is ground-plane/make/docker-local.mk; the
# same bytes sit at the root of every kompas repository, and
# `ground-plane/scripts/sync-lane.sh --check` proves they still do. Change it
# there and run the sync; never edit a vendored copy in place.
#
# The including Makefile declares the stack table:
#
#   STACKS          every stack name this repository can run
#   STACK           the default stack, overridable on the command line
#   ALL_STACKS      the stacks `up-all` starts, in start order; defaults to
#                   STACKS. Narrow it where a stack is an alternative to
#                   another rather than a companion to it, and order it where
#                   one stack joins a network or volume another one publishes
#   DIR_<stack>     compose project directory, relative to the repository root
#   FILE_<stack>    compose file name inside that directory
#   ENV_<stack>     optional --env-file, relative to DIR_<stack>
#   URL_<stack>     optional address printed once the stack is up
#   PROBE_<stack>   optional URL for `health` to GET. Declare it only where an
#                   unauthenticated GET is meaningful: a mutual-TLS endpoint
#                   rejects a plain curl by design, and reporting that as a
#                   failed health check would be a lie about the stack
#   PRE_UP_<stack>  optional shell command run from the repository root before up
#   UP_<stack>      optional shell command replacing the whole up recipe, for a
#                   stack whose start-up needs more than compose (bootstrap,
#                   token minting, health gating)
#
# Compose is always invoked from DIR_<stack>, so a relative build context or
# bind mount inside the compose file resolves the way its author wrote it.

DOCKER ?= docker
COMPOSE ?= $(DOCKER) compose

STACK ?= $(firstword $(STACKS))

ifeq ($(filter $(STACK),$(STACKS)),)
$(error Unknown STACK '$(STACK)'. This repository knows: $(STACKS))
endif

stack_dir := $(DIR_$(STACK))
stack_file := $(FILE_$(STACK))
stack_env := $(ENV_$(STACK))
stack_url := $(URL_$(STACK))
stack_probe := $(PROBE_$(STACK))
stack_pre := $(PRE_UP_$(STACK))
stack_up := $(UP_$(STACK))

ifeq ($(stack_dir),)
$(error Stack '$(STACK)' declares no DIR_$(STACK))
endif
ifeq ($(stack_file),)
$(error Stack '$(STACK)' declares no FILE_$(STACK))
endif

compose = cd $(stack_dir) && $(COMPOSE) $(if $(stack_env),--env-file $(stack_env),) -f $(stack_file)

# A stack whose env file is missing fails inside compose with a bare path. Say
# what to do about it instead: the ignored env file always has a committed
# .example beside it and a README in the same directory.
ifeq ($(stack_env),)
env_check := @true
else
env_check := @test -f '$(stack_dir)/$(stack_env)' \
	|| { printf 'Stack %s needs %s, which is not there.\nCopy the .example beside it and fill it in — see %s.\n' \
	     '$(STACK)' '$(stack_dir)/$(stack_env)' '$(stack_dir)'; exit 2; }
endif

.PHONY: up down restart logs ps images config health shell clean stacks docker-help

up:
	$(env_check)
ifneq ($(stack_pre),)
	$(stack_pre)
endif
ifneq ($(stack_up),)
	$(stack_up)
else
	$(compose) up --detach --build --remove-orphans
endif
	@printf '\n%s is up.%s\n' '$(STACK)' '$(if $(stack_url), Open $(stack_url).,)'
	@printf 'Logs: make logs STACK=%s\n' '$(STACK)'

down:
	$(compose) down --remove-orphans

# Sequenced through sub-makes, not as two prerequisites: `make -j restart` would
# be free to run the down and the up at the same time.
restart:
	@$(MAKE) --no-print-directory down STACK=$(STACK)
	@$(MAKE) --no-print-directory up STACK=$(STACK)

# SVC narrows logs, images, and shell to a single compose service:
# make logs SVC=query-api. The other verbs act on the whole stack.
logs:
	$(compose) logs --follow --tail 200 $(SVC)

ps:
	$(compose) ps

# Named `images`, not `build`: in every one of these repositories `make build`
# already means "compile the project".
images:
	$(compose) build $(SVC)

# Renders the merged compose file. The first thing to run when a stack fails to
# start: an unset variable in the env file surfaces here, not in a container log.
config:
	$(env_check)
	$(compose) config

health:
	$(compose) ps --format 'table {{.Service}}\t{{.State}}\t{{.Status}}'
ifneq ($(stack_probe),)
	@curl -fsS -o /dev/null -w 'endpoint $(stack_probe) -> HTTP %{http_code}\n' '$(stack_probe)' \
	  || printf 'endpoint $(stack_probe) is not answering yet.\n'
endif

shell:
	@test -n '$(SVC)' || { printf 'shell needs a service: make shell STACK=%s SVC=<service>\n' '$(STACK)'; exit 2; }
	$(compose) exec $(SVC) sh

# Destructive: drops the stack's named volumes with it. Databases, object
# storage, and vector indexes in this stack are gone afterwards.
clean:
	@test '$(FORCE)' = '1' || { printf 'clean deletes the volumes of stack %s. Re-run with FORCE=1.\n' '$(STACK)'; exit 2; }
	$(compose) down --volumes --remove-orphans

# --- Whole-repository verbs -------------------------------------------------
#
# One gesture for every stack this repository runs together. ALL_STACKS is in
# start order, so teardown walks it backwards: a stack that joined a network
# another one publishes has to be gone before its owner is.
#
# MAKEOVERRIDES is cleared on the way in. Without that, `make up-all STACK=mcp`
# would propagate STACK=mcp from the outer command line into every sub-make and
# start the same stack N times.

ALL_STACKS ?= $(STACKS)

reverse = $(if $(1),$(call reverse,$(wordlist 2,$(words $(1)),$(1))) $(firstword $(1)),)
sub = $(MAKE) --no-print-directory MAKEOVERRIDES=

.PHONY: up-all down-all restart-all ps-all health-all clean-all

up-all:
	@$(foreach s,$(ALL_STACKS),$(sub) up STACK=$(s) &&) true

down-all:
	@$(foreach s,$(call reverse,$(ALL_STACKS)),$(sub) down STACK=$(s) &&) true

restart-all:
	@$(sub) down-all
	@$(sub) up-all

ps-all:
	@$(foreach s,$(ALL_STACKS),printf '\n== %s\n' '$(s)'; $(sub) ps STACK=$(s);)

health-all:
	@$(foreach s,$(ALL_STACKS),printf '\n== %s\n' '$(s)'; $(sub) health STACK=$(s);)

# Destructive across every stack at once: all of their volumes go.
clean-all:
	@test '$(FORCE)' = '1' || { printf 'clean-all deletes the volumes of every stack (%s). Re-run with FORCE=1.\n' '$(ALL_STACKS)'; exit 2; }
	@$(foreach s,$(call reverse,$(ALL_STACKS)),$(sub) clean FORCE=1 STACK=$(s) &&) true

stacks:
	@printf 'Stacks in this repository (default: %s)\n\n' '$(STACK)'
	@$(foreach s,$(STACKS),printf '  %-16s %-52s %s%s\n' '$(s)' '$(DIR_$(s))/$(FILE_$(s))' '$(if $(filter $(s),$(ALL_STACKS)),[up-all],        )' '$(if $(URL_$(s)), $(URL_$(s)),)';)
	@printf '\nmake up-all starts, in this order: $(ALL_STACKS)\n'

docker-help:
	@printf 'Docker lane — containers, the default way to run this repository.\n\n'
	@printf '  make up [STACK=n]          start a stack, building images first\n'
	@printf '  make down [STACK=n]        stop it and remove its containers\n'
	@printf '  make restart [STACK=n]     down, then up\n'
	@printf '  make logs [STACK=n] [SVC=] follow logs\n'
	@printf '  make ps [STACK=n]          container states\n'
	@printf '  make health [STACK=n]      per-service health, and probe the endpoint where that is meaningful\n'
	@printf '  make images [STACK=n] [SVC=] build images without starting anything\n'
	@printf '  make config [STACK=n]      render the merged compose file (debug a start-up failure)\n'
	@printf '  make shell STACK=n SVC=s   shell into a running service\n'
	@printf '  make clean [STACK=n] FORCE=1  stop and delete the volumes\n'
	@printf '  make stacks                list the stacks\n'
	@printf '\n  make up-all                start every stack this repository runs together\n'
	@printf '  make down-all              stop them, in reverse order\n'
	@printf '  make restart-all           down-all, then up-all\n'
	@printf '  make ps-all / health-all   the same report for each of them\n'
	@printf '  make clean-all FORCE=1     stop them and delete every volume\n'
