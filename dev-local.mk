# Native lane, for the edit loop. Source: ground-plane/make/dev-local.mk.
#
# The including Makefile declares what it can honour: DEV_APPS, APP, DEV_RUN_<app>,
# DEV_DEPENDS_ON, DEV_START, DEV_STOP, DEV_RESTART, DEV_PS, DEV_LOGS, DEV_BUILD.
# Unset checks use ifeq, not a shell test, because a DEV_RUN_ value holds quotes.

APP ?= $(firstword $(DEV_APPS))

ifneq ($(DEV_APPS),)
ifeq ($(filter $(APP),$(DEV_APPS)),)
$(error Unknown APP '$(APP)'. This repository knows: $(DEV_APPS))
endif
endif

dev_run := $(if $(DEV_APPS),$(DEV_RUN_$(APP)),$(DEV_START))

ifeq ($(strip $(dev_run)),)
dev_absent := This repository has no native lane$(if $(DEV_APPS), for APP=$(APP)). Use make up instead.
else
dev_absent := The native run here is a foreground process. Ctrl-C stops it and its own terminal is its status and log.
endif

.PHONY: dev-up dev-down dev-restart dev-logs dev-ps dev-build dev-apps dev-help

ifeq ($(strip $(dev_run)),)
dev-up:
	@printf '%s\n' '$(dev_absent)'
	@exit 2
else
dev-up:
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
dev-down:
	@printf '%s\n' '$(dev_absent)'
	@exit 2
else
dev-down:
	$(DEV_STOP)
endif

ifeq ($(strip $(DEV_RESTART)),)
ifeq ($(strip $(DEV_STOP))$(strip $(DEV_START)),)
dev-restart:
	@printf '%s\n' '$(dev_absent)'
	@exit 2
else
ifeq ($(strip $(DEV_START)),)
dev-restart:
	@printf 'This repository can stop a native run but not start one back up. Use make dev-up.\n'
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

ifeq ($(strip $(DEV_PS)),)
dev-ps:
	@printf '%s\n' '$(dev_absent)'
	@exit 2
else
dev-ps:
	$(DEV_PS)
endif

ifeq ($(strip $(DEV_LOGS)),)
dev-logs:
	@printf '%s\n' '$(dev_absent)'
	@exit 2
else
dev-logs:
	$(DEV_LOGS)
endif

dev-apps:
ifeq ($(DEV_APPS),)
	@printf 'This repository runs natively as a single unit; make dev-up takes no APP.\n'
else
	@printf 'Natively runnable components (default: %s)\n' '$(APP)'
	@$(foreach a,$(DEV_APPS),printf '  %s\n' '$(a)';)
endif

dev-help:
	@printf 'Native lane — source and local binaries, for the edit loop.\n'
	@printf 'Every verb is the docker one with a dev- prefix.\n\n'
	@printf '  make dev-up [APP=n]        run natively\n'
	@printf '  make dev-down              stop a backgrounded native run\n'
	@printf '  make dev-restart           down, then up\n'
	@printf '  make dev-logs              follow its log\n'
	@printf '  make dev-ps                is it running\n'
	@printf '  make dev-build             compile what dev-up runs\n'
	@printf '  make dev-apps              list runnable components\n'
