SHELL := /bin/sh

.DEFAULT_GOAL := help

API_DIR ?= apps/api
WEB_DIR ?= apps/web
ADMIN_DIR ?= apps/admin
CDN_DIR ?= apps/cdn
LOCAL_ARTIFACTS := $(API_DIR)/target $(WEB_DIR)/dist $(CDN_DIR)/bin

DOCKER ?= docker
COMPOSE ?= docker compose
NPM ?= npm
NODE ?= node
PHP ?= php
GO ?= go
PYTHON_BOOTSTRAP ?= python3
PYTHON ?= $(if $(wildcard .venv/bin/python),$(abspath .venv/bin/python),$(PYTHON_BOOTSTRAP))
DEBUG_AUTH_USERNAME ?= Zwuiix
TAIL ?= 200

.PHONY: help setup install python-venv build test lint audit \
	run run-d debug down logs ps \
	secrets format \
	admin-grant-email clean fclean reset-data require-env \
	api-image api-test web-build web-test admin-image admin-test cdn-build cdn-image

help:
	@printf '%s\n' \
		'Usage: make <target>' \
		'' \
		'Getting started:' \
		'  setup                 Create .env if missing and install dependencies' \
		'  install               Install frontend, CDN and Python tooling' \
		'  secrets               Print fresh local API/CDN/game-service secrets' \
		'' \
		'Build and test:' \
		'  build                 Build every local image and Go artifact' \
		'  test                  Run API, frontend, admin and CDN tests' \
		'  lint                  Run frontend and admin static checks' \
		'  audit                 Audit frontend dependencies' \
		'' \
		'Run the stack:' \
		'  run                   Build if needed and run in the foreground' \
		'  run-d                 Build if needed and run in the background' \
		'  debug                 Run the explicit local login-bypass stack' \
		'  down                  Stop and remove containers, keep volumes' \
		'' \
		'Operations:' \
		'  logs SERVICE=api      Follow service logs (also web, admin or cdn)' \
		'  ps                    Show container status' \
		'  admin-grant-email     Grant admin to EMAIL=you@learner.42.tech' \
		'' \
		'Cleanup:' \
		'  clean                 Remove containers and local build artifacts' \
		'  fclean                Also remove dependencies and local Compose images' \
		'  reset-data CONFIRM=1  Remove project volumes and persisted data' \
		'' \
		'Examples:' \
		'  make setup' \
		'  make run-d' \
		'  make logs SERVICE=api TAIL=100' \
		'  make admin-grant-email EMAIL=you@learner.42.tech' \
		'  make fclean'

setup:
	@test -f .env || cp .env.example .env
	$(MAKE) install

install: python-venv
	cd $(WEB_DIR) && $(NPM) ci
	cd $(CDN_DIR) && $(GO) mod download

python-venv:
	@test -x .venv/bin/python || $(PYTHON_BOOTSTRAP) -m venv .venv
	@echo 'Python virtual environment ready: .venv/bin/python'

secrets:
	$(PYTHON) scripts/generate-secrets.py

api-image:
	$(DOCKER) build -t transcendence-api:dev -f $(API_DIR)/Dockerfile .

api-test:
	$(DOCKER) build --target test -t transcendence-api-test:dev -f $(API_DIR)/Dockerfile .

web-build:
	cd $(WEB_DIR) && $(NPM) run build

web-test:
	cd $(WEB_DIR) && $(NPM) run typecheck

admin-image:
	$(DOCKER) build -t transcendence-admin:dev $(ADMIN_DIR)

admin-test:
	$(PHP) -l $(ADMIN_DIR)/src/ApiProxy.php
	$(PHP) -l $(ADMIN_DIR)/src/AdminSession.php
	$(PHP) -l $(ADMIN_DIR)/public/api.php
	$(PHP) -l $(ADMIN_DIR)/public/index.php
	$(PHP) -l $(ADMIN_DIR)/public/router.php
	$(NODE) --check $(ADMIN_DIR)/public/admin.js

cdn-build:
	@mkdir -p $(CDN_DIR)/bin
	cd $(CDN_DIR) && CGO_ENABLED=0 $(GO) build -trimpath -ldflags='-s -w' -o bin/cdn ./cmd/cdn

cdn-image:
	$(DOCKER) build -t transcendence-cdn:dev -f $(CDN_DIR)/Dockerfile .

build: api-image web-build admin-image cdn-build cdn-image

test: api-test web-test admin-test
	cd $(CDN_DIR) && $(GO) test ./...

format:
	cd $(API_DIR) && $(GO) fmt ./...
	cd $(CDN_DIR) && $(GO) fmt ./...

lint: web-test admin-test

audit:
	cd $(WEB_DIR) && $(NPM) audit --audit-level=high

admin-grant-email: require-env
	@test -n "$(EMAIL)" || { \
		echo 'Usage: make admin-grant-email EMAIL=you@learner.42.tech' >&2; \
		exit 1; \
	}
	@printf '%s\n' "$(EMAIL)" | grep -Eq '^[A-Za-z0-9._+-]+@learner\.42\.tech$$' || { \
		echo 'Only emails ending in @learner.42.tech are accepted.' >&2; \
		exit 1; \
	}
	@COMPOSE="$(COMPOSE)" $(PYTHON) scripts/admin-grant-email.py "$(EMAIL)"

require-env:
	@test -f .env || { \
		echo 'Missing .env. Run: make setup' >&2; \
		exit 1; \
	}

run: setup
	$(COMPOSE) up --build

run-d: setup
	$(COMPOSE) up -d --build

debug: require-env
	DEBUG_AUTH_ENABLED=true DEBUG_AUTH_USERNAME=$(DEBUG_AUTH_USERNAME) VITE_DEBUG_AUTH=true \
		$(COMPOSE) up -d --build --force-recreate --remove-orphans

down: require-env
	$(COMPOSE) down --remove-orphans

logs: require-env
	$(COMPOSE) logs -f --tail=$(TAIL) $(SERVICE)

ps: require-env
	$(COMPOSE) ps

clean: require-env
	$(COMPOSE) down --remove-orphans
	rm -rf $(LOCAL_ARTIFACTS)

fclean: require-env
	$(COMPOSE) down --remove-orphans --rmi local
	rm -rf $(LOCAL_ARTIFACTS) $(WEB_DIR)/node_modules .venv
	$(DOCKER) image rm -f transcendence-api:dev transcendence-api-test:dev \
		transcendence-admin:dev transcendence-cdn:dev 2>/dev/null || true

reset-data: require-env
	@test "$(CONFIRM)" = "1" || { \
		echo 'Refusing to remove database and CDN volumes. Re-run with: make reset-data CONFIRM=1' >&2; \
		exit 1; \
	}
	$(COMPOSE) down --remove-orphans --volumes
