# Native lane — the project's own binaries and dev servers, no containers.
#
# Vendored copy, byte-identical across agent-plane, context-cellar, and
# kode-stream, for the same reason as docker-local.mk.
#
# Docker is the default way to run a stack; this lane exists for the edit loop,
# where an image rebuild per keystroke is not affordable. What a native process
# needs from its neighbours — a database, object storage, a model gateway —
# still comes from the docker lane, so a native run usually means: bring the
# stack up, then run the one component being edited from source.
#
# The including Makefile declares whichever of these it can honour:
#
#   DEV_APPS         optional list of separately runnable components
#   APP              the default one, overridable on the command line
#   DEV_RUN_<app>    foreground command for that component
#   DEV_DEPENDS_ON   optional note: which docker stack must be up first
#   DEV_START        foreground/daemon start when there is nothing to choose
#   DEV_STOP         stop a daemonised native run
#   DEV_RESTART      restart one; falls back to DEV_STOP followed by DEV_START
#   DEV_STATUS       report on a daemonised native run
#   DEV_LOGS         follow a daemonised native run's log
#   DEV_BUILD        compile everything natively
#
# A target whose variable is unset explains itself rather than failing
# obscurely. Those checks are `ifeq` rather than a shell `test -n`, because a
# DEV_RUN_ value legitimately contains quotes and a shell test would choke on
# them before the command ever ran.

APP ?= $(firstword $(DEV_APPS))

ifneq ($(DEV_APPS),)
ifeq ($(filter $(APP),$(DEV_APPS)),)
$(error Unknown APP '$(APP)'. This repository knows: $(DEV_APPS))
endif
endif

dev_run := $(if $(DEV_APPS),$(DEV_RUN_$(APP)),$(DEV_START))

.PHONY: dev dev-build dev-stop dev-restart dev-status dev-logs dev-apps dev-help

ifeq ($(strip $(dev_run)),)
dev:
	@printf 'No native run is wired in this repository$(if $(DEV_APPS), for APP=$(APP)). Use make up instead.\n'
	@exit 2
else
dev:
ifneq ($(DEV_DEPENDS_ON),)
	@printf 'Native run expects the %s stack to be up (make up STACK=%s).\n' '$(DEV_DEPENDS_ON)' '$(DEV_DEPENDS_ON)'
endif
	$(dev_run)
endif

ifeq ($(strip $(DEV_BUILD)),)
dev-build:
	@printf 'This repository declares no native build.\n'
	@exit 2
else
dev-build:
	$(DEV_BUILD)
endif

ifeq ($(strip $(DEV_STOP)),)
dev-stop:
	@printf 'The native run is a foreground process — stop it with Ctrl-C.\n'
	@exit 2
else
dev-stop:
	$(DEV_STOP)
endif

# DEV_RESTART when the launcher has its own restart; otherwise stop then start,
# which needs both halves — a stop with nothing to start again is not a restart.
ifeq ($(strip $(DEV_RESTART)),)
ifeq ($(strip $(DEV_STOP))$(strip $(DEV_START)),)
dev-restart:
	@printf 'The native run is a foreground process — restart it with Ctrl-C, then make dev.\n'
	@exit 2
else
ifeq ($(strip $(DEV_START)),)
dev-restart:
	@printf 'This repository can stop a native run but not start one back up. Use make dev.\n'
	@exit 2
else
dev-restart:
	$(DEV_STOP)
	$(DEV_START)
endif
endif
else
dev-restart:
	$(DEV_RESTART)
endif

ifeq ($(strip $(DEV_STATUS)),)
dev-status:
	@printf 'The native run is a foreground process — its state is your terminal.\n'
	@exit 2
else
dev-status:
	$(DEV_STATUS)
endif

ifeq ($(strip $(DEV_LOGS)),)
dev-logs:
	@printf 'The native run logs to its own terminal.\n'
	@exit 2
else
dev-logs:
	$(DEV_LOGS)
endif

dev-apps:
ifeq ($(DEV_APPS),)
	@printf 'This repository runs natively as a single unit; make dev takes no APP.\n'
else
	@printf 'Natively runnable components (default: %s)\n' '$(APP)'
	@$(foreach a,$(DEV_APPS),printf '  %s\n' '$(a)';)
endif

dev-help:
	@printf 'Native lane — source and local binaries, for the edit loop.\n\n'
	@printf '  make dev [APP=n]      run natively\n'
	@printf '  make dev-build        compile natively\n'
	@printf '  make dev-stop         stop a backgrounded native run\n'
	@printf '  make dev-restart      restart it\n'
	@printf '  make dev-status       is it running\n'
	@printf '  make dev-logs         follow its log\n'
	@printf '  make dev-apps         list runnable components\n'
