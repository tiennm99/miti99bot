.PHONY: help test test-mongo mongo-local mongo-local-stop vet build run telegram-commands telegram-commands-info telegram-deletewebhook telegram-webhook-info clean

# Short git SHA baked into local binaries. Coolify sets SOURCE_COMMIT at
# runtime; this ldflags value is the fallback for local builds.
GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -s -w -X main.gitSHA=$(GIT_SHA)

TELEGRAM_COMMANDS_FILE ?= telegram-commands.json

help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS=":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

# ---- Test ------------------------------------------------------------------

test: ## Unit tests (no emulator required)
	go test -race -count=1 ./...

# Run MongoDB integration tests against a local Mongo container.
# Override MONGO_PORT if 27017 is taken on your host.
MONGO_PORT ?= 27017
test-mongo: mongo-local ## Run MongoDB tests against a local Mongo container
	MONGODB_TEST_URL=mongodb://127.0.0.1:$(MONGO_PORT) LOG_LEVEL=error \
		go test -race -count=1 ./internal/storage/...

# ---- Lint / Vet ------------------------------------------------------------

vet: ## go vet
	go vet ./...

# ---- Build -----------------------------------------------------------------

build: ## Build the local server binary (host arch)
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o ./bin/server ./cmd/server

# ---- Run -------------------------------------------------------------------

run: ## Run locally (in-memory storage unless MONGO_URL is set)
	go run ./cmd/server

# ---- MongoDB local for tests ------------------------------------------------

mongo-local: ## Start MongoDB container on :$(MONGO_PORT) (idempotent)
	@if ! docker ps --format '{{.Names}}' | grep -q '^miti99bot-mongo$$'; then \
		docker run -d --rm --name miti99bot-mongo -p $(MONGO_PORT):27017 mongo:7; \
		echo "MongoDB started on :$(MONGO_PORT)"; \
		sleep 2; \
	else \
		echo "MongoDB already running"; \
	fi

mongo-local-stop: ## Stop local MongoDB
	-docker stop miti99bot-mongo

# ---- Telegram operations ----------------------------------------------------

telegram-commands: ## Register command menu using TELEGRAM_BOT_TOKEN env
	@set -eu; \
	: "$${TELEGRAM_BOT_TOKEN:?set TELEGRAM_BOT_TOKEN}"; \
	echo "Registering Telegram commands from $(TELEGRAM_COMMANDS_FILE)"; \
	curl -sS -X POST "https://api.telegram.org/bot$${TELEGRAM_BOT_TOKEN}/setMyCommands" \
		-H 'Content-Type: application/json' \
		--data-binary "@$(TELEGRAM_COMMANDS_FILE)"; \
	echo

telegram-commands-info: ## Show Telegram getMyCommands using TELEGRAM_BOT_TOKEN env
	@set -eu; \
	: "$${TELEGRAM_BOT_TOKEN:?set TELEGRAM_BOT_TOKEN}"; \
	curl -sS "https://api.telegram.org/bot$${TELEGRAM_BOT_TOKEN}/getMyCommands"; \
	echo

telegram-deletewebhook: ## Delete webhook so the poller can run
	@set -eu; \
	: "$${TELEGRAM_BOT_TOKEN:?set TELEGRAM_BOT_TOKEN}"; \
	curl -sS -X POST "https://api.telegram.org/bot$${TELEGRAM_BOT_TOKEN}/deleteWebhook" \
		-d 'drop_pending_updates=false'; \
	echo

telegram-webhook-info: ## Show Telegram getWebhookInfo using TELEGRAM_BOT_TOKEN env
	@set -eu; \
	: "$${TELEGRAM_BOT_TOKEN:?set TELEGRAM_BOT_TOKEN}"; \
	curl -sS "https://api.telegram.org/bot$${TELEGRAM_BOT_TOKEN}/getWebhookInfo"; \
	echo

# ---- Clean -----------------------------------------------------------------

clean: ## Remove local build artifacts
	rm -rf build/ bin/ cov.out
