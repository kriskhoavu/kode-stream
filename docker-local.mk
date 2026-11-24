# Docker lane. Source: ground-plane/make/docker-local.mk, vendored by
# ground-plane/scripts/sync-lane.sh. Never edit a vendored copy.
#
# The including Makefile declares STACKS, STACK, ALL_STACKS (up-all order) and,
# per stack, DIR_ and FILE_, optionally ENV_, URL_, PROBE_, PRE_UP_ and UP_.
# Declare PROBE_ only where an unauthenticated GET is meaningful (not mTLS).

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

# Sub-makes, not prerequisites, so `make -j` cannot run down and up at once.
restart:
	@$(MAKE) --no-print-directory down STACK=$(STACK)
	@$(MAKE) --no-print-directory up STACK=$(STACK)

logs:
	$(compose) logs --follow --tail 200 $(SVC)

ps:
	$(compose) ps

images:
	$(compose) build $(SVC)

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

clean:
	@test '$(FORCE)' = '1' || { printf 'clean deletes the volumes of stack %s. Re-run with FORCE=1.\n' '$(STACK)'; exit 2; }
	$(compose) down --volumes --remove-orphans

# Teardown walks ALL_STACKS backwards. MAKEOVERRIDES is cleared so a STACK= on
# the command line does not reach every sub-make.

ALL_STACKS ?= $(STACKS)

reverse = $(if $(1),$(call reverse,$(wordlist 2,$(words $(1)),$(1))) $(firstword $(1)),)
sub = $(MAKE) --no-print-directory MAKEOVERRIDES=
lane_each = @$(foreach s,$(2),$(sub) $(1) STACK=$(s) &&) true
lane_report = @$(foreach s,$(ALL_STACKS),printf '\n== %s\n' '$(s)'; $(sub) $(1) STACK=$(s);)

.PHONY: up-all down-all restart-all ps-all health-all clean-all

up-all:
	$(call lane_each,up,$(ALL_STACKS))

down-all:
	$(call lane_each,down,$(call reverse,$(ALL_STACKS)))

restart-all:
	@$(sub) down-all
	@$(sub) up-all

ps-all health-all: %-all:
	$(call lane_report,$*)

clean-all:
	@test '$(FORCE)' = '1' || { printf 'clean-all deletes the volumes of every stack (%s). Re-run with FORCE=1.\n' '$(ALL_STACKS)'; exit 2; }
	$(call lane_each,clean FORCE=1,$(call reverse,$(ALL_STACKS)))

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
