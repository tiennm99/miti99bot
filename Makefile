.PHONY: help test test-dynamodb test-mongo dynamodb-local dynamodb-local-stop mongo-local mongo-local-stop vet build build-lambda run sam-validate sam-build sam-deploy telegram-setup telegram-webhook telegram-webhook-info telegram-commands telegram-commands-info telegram-commands-selfhost telegram-deletewebhook-selfhost telegram-webhook-info-selfhost migrate-dynamo-to-mongo migrate-verify logs clean

# Lambda target architecture. Match Globals.Architectures in template.yaml.
LAMBDA_GOOS   ?= linux
LAMBDA_GOARCH ?= arm64
LAMBDA_OUT    := build/lambda/bootstrap

# Short git SHA baked into the binary at link time. Consumed by
# internal/deploynotify to DM the owner once per new version. Falls back to
# empty string outside a git checkout (tarball, fresh clone without history)
# — deploynotify treats empty as "stay silent".
GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -s -w -X main.gitSHA=$(GIT_SHA)

# AWS deploy defaults. Override as needed:
#   make telegram-webhook AWS_PROFILE=admin STACK_NAME=miti99bot STACK_ENV=prod
AWS_PROFILE ?= admin
STACK_NAME  ?= miti99bot
STACK_ENV   ?= prod
TELEGRAM_COMMANDS_FILE ?= aws/telegram-commands.json

help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS=":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

# ---- Test ------------------------------------------------------------------

# Default: run unit tests that don't require any emulator.
test: ## Unit tests (no emulator required)
	go test -race -count=1 ./...

# Run DynamoDB integration tests against DynamoDB Local.
# Override DDB_PORT if 8001 is taken on your host.
DDB_PORT ?= 8001
test-dynamodb: dynamodb-local mongo-local ## Run the DynamoDB→Mongo migrator e2e against local emulators
	DYNAMODB_LOCAL_URL=http://localhost:$(DDB_PORT) \
	MONGODB_TEST_URL=mongodb://127.0.0.1:$(MONGO_PORT) \
	MONGO_DATABASE=migrate_test LOG_LEVEL=error \
		go test -race -count=1 ./cmd/migrate-dynamo-to-mongo/...

# Run MongoDB integration tests against a local Mongo container.
# Override MONGO_PORT if 27017 is taken on your host.
MONGO_PORT ?= 27017
test-mongo: mongo-local ## Run MongoDB tests against a local Mongo container
	MONGODB_TEST_URL=mongodb://127.0.0.1:$(MONGO_PORT) LOG_LEVEL=error \
		go test -race -count=1 ./internal/storage/...

# ---- Lint / Vet -----------------------------------------------------------

vet: ## go vet
	go vet ./...

# ---- Build ----------------------------------------------------------------

build: ## Build the local server binary (host arch)
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o ./bin/server ./cmd/server

build-lambda: ## Cross-compile bootstrap for Lambda (linux/arm64)
	@mkdir -p $(dir $(LAMBDA_OUT))
	CGO_ENABLED=0 GOOS=$(LAMBDA_GOOS) GOARCH=$(LAMBDA_GOARCH) \
		go build -tags lambda.norpc -ldflags="$(LDFLAGS)" \
		-o $(LAMBDA_OUT) ./cmd/server
	@chmod +x $(LAMBDA_OUT)
	@ls -lh $(LAMBDA_OUT) | awk '{print "lambda binary:", $$5}'

# ---- Run ------------------------------------------------------------------

# Local dev run with an in-memory KV (no database needed).
run: ## Run locally (in-memory KV)
	go run ./cmd/server

# ---- DynamoDB Local for tests ---------------------------------------------

dynamodb-local: ## Start DynamoDB Local container on :$(DDB_PORT) (idempotent)
	@if ! docker ps --format '{{.Names}}' | grep -q '^miti99bot-ddb$$'; then \
		docker run -d --rm --name miti99bot-ddb -p $(DDB_PORT):8000 \
			amazon/dynamodb-local -jar DynamoDBLocal.jar -inMemory -sharedDb; \
		echo "DynamoDB Local started on :$(DDB_PORT)"; \
		sleep 1; \
	else \
		echo "DynamoDB Local already running"; \
	fi

dynamodb-local-stop: ## Stop DynamoDB Local
	-docker stop miti99bot-ddb

# ---- MongoDB local for tests ----------------------------------------------

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

# ---- Data migration (DynamoDB → MongoDB Atlas) ----------------------------

# Requires MONGO_URL + MONGO_DATABASE in the environment and AWS credentials
# for a READ-ONLY profile with dynamodb:Scan on the table. Use DRY_RUN=1 first.
#   make migrate-dynamo-to-mongo DRY_RUN=1 MONGO_URL=… MONGO_DATABASE=…
#   make migrate-dynamo-to-mongo        MONGO_URL=… MONGO_DATABASE=…
#   make migrate-verify                 MONGO_URL=… MONGO_DATABASE=…
MIGRATE_TABLE ?= miti99bot-data
migrate-dynamo-to-mongo: ## Copy DynamoDB → Mongo (DRY_RUN=1 for a dry run)
	go run ./cmd/migrate-dynamo-to-mongo --dynamodb-table $(MIGRATE_TABLE) $(if $(DRY_RUN),--dry-run,)

migrate-verify: ## Verify per-module counts DynamoDB vs Mongo (exit non-zero on mismatch)
	go run ./cmd/migrate-dynamo-to-mongo --dynamodb-table $(MIGRATE_TABLE) --verify

# ---- SAM (require AWS CLI + SAM CLI installed locally) -------------------

sam-validate: ## Validate template.yaml without contacting AWS
	sam validate --lint

sam-build: build-lambda ## Produce the Lambda artifact (alias for build-lambda; sam deploy uses raw template directly)
	@echo "Artifact ready at build/lambda/bootstrap; sam deploy --template-file template.yaml will zip it."

sam-deploy: build-lambda ## Deploy via SAM (uses samconfig.toml). Set ALERT_EMAIL=… optionally.
	@if [ -n "$$ALERT_EMAIL" ]; then \
		sam deploy --template-file template.yaml \
			--no-confirm-changeset --no-fail-on-empty-changeset \
			--parameter-overrides "AlertEmail=$$ALERT_EMAIL"; \
	else \
		sam deploy --template-file template.yaml \
			--no-confirm-changeset --no-fail-on-empty-changeset; \
	fi

telegram-setup: telegram-webhook telegram-commands ## Register Telegram webhook and command menu

telegram-webhook: ## Register Telegram webhook from stack FunctionUrl + SSM secrets
	@set -eu; \
	URL=$$(aws --profile "$(AWS_PROFILE)" cloudformation describe-stacks \
		--stack-name "$(STACK_NAME)" \
		--query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" \
		--output text); \
	TOKEN=$$(aws --profile "$(AWS_PROFILE)" ssm get-parameter \
		--name "/miti99bot/$(STACK_ENV)/telegram-bot-token" \
		--with-decryption --query Parameter.Value --output text); \
	SECRET=$$(aws --profile "$(AWS_PROFILE)" ssm get-parameter \
		--name "/miti99bot/$(STACK_ENV)/telegram-webhook-secret" \
		--with-decryption --query Parameter.Value --output text); \
	case "$$URL" in */) WEBHOOK_URL="$${URL}webhook" ;; *) WEBHOOK_URL="$${URL}/webhook" ;; esac; \
	echo "Setting Telegram webhook to $$WEBHOOK_URL"; \
	curl -sS -X POST "https://api.telegram.org/bot$${TOKEN}/setWebhook" \
		-d "url=$${WEBHOOK_URL}" \
		-d "secret_token=$${SECRET}" \
		-d 'allowed_updates=["message","callback_query"]'; \
	echo

telegram-webhook-info: ## Show Telegram getWebhookInfo using token from SSM
	@set -eu; \
	TOKEN=$$(aws --profile "$(AWS_PROFILE)" ssm get-parameter \
		--name "/miti99bot/$(STACK_ENV)/telegram-bot-token" \
		--with-decryption --query Parameter.Value --output text); \
	curl -sS "https://api.telegram.org/bot$${TOKEN}/getWebhookInfo"; \
	echo

telegram-commands: ## Register Telegram command menu from TELEGRAM_COMMANDS_FILE
	@set -eu; \
	TOKEN=$$(aws --profile "$(AWS_PROFILE)" ssm get-parameter \
		--name "/miti99bot/$(STACK_ENV)/telegram-bot-token" \
		--with-decryption --query Parameter.Value --output text); \
	echo "Registering Telegram commands from $(TELEGRAM_COMMANDS_FILE)"; \
	curl -sS -X POST "https://api.telegram.org/bot$${TOKEN}/setMyCommands" \
		-H 'Content-Type: application/json' \
		--data-binary "@$(TELEGRAM_COMMANDS_FILE)"; \
	echo

telegram-commands-info: ## Show Telegram getMyCommands using token from SSM
	@set -eu; \
	TOKEN=$$(aws --profile "$(AWS_PROFILE)" ssm get-parameter \
		--name "/miti99bot/$(STACK_ENV)/telegram-bot-token" \
		--with-decryption --query Parameter.Value --output text); \
	curl -sS "https://api.telegram.org/bot$${TOKEN}/getMyCommands"; \
	echo

# ---- Telegram (self-host: token from TELEGRAM_BOT_TOKEN env, no AWS/SSM) ---

telegram-commands-selfhost: ## Register command menu using TELEGRAM_BOT_TOKEN env
	@set -eu; \
	: "$${TELEGRAM_BOT_TOKEN:?set TELEGRAM_BOT_TOKEN}"; \
	echo "Registering Telegram commands from $(TELEGRAM_COMMANDS_FILE)"; \
	curl -sS -X POST "https://api.telegram.org/bot$${TELEGRAM_BOT_TOKEN}/setMyCommands" \
		-H 'Content-Type: application/json' \
		--data-binary "@$(TELEGRAM_COMMANDS_FILE)"; \
	echo

telegram-deletewebhook-selfhost: ## Cutover: delete the webhook so the poller can run (keeps buffered updates)
	@set -eu; \
	: "$${TELEGRAM_BOT_TOKEN:?set TELEGRAM_BOT_TOKEN}"; \
	curl -sS -X POST "https://api.telegram.org/bot$${TELEGRAM_BOT_TOKEN}/deleteWebhook" \
		-d 'drop_pending_updates=false'; \
	echo

telegram-webhook-info-selfhost: ## getWebhookInfo using TELEGRAM_BOT_TOKEN env (confirm url empty + pending draining)
	@set -eu; \
	: "$${TELEGRAM_BOT_TOKEN:?set TELEGRAM_BOT_TOKEN}"; \
	curl -sS "https://api.telegram.org/bot$${TELEGRAM_BOT_TOKEN}/getWebhookInfo"; \
	echo

logs: ## Tail Lambda logs (last 5m). Override with SINCE=10m.
	@sam logs --tail --stack-name miti99bot --start-time $${SINCE:-5m}ago

# ---- Clean ----------------------------------------------------------------

clean: ## Remove local build artifacts
	rm -rf build/ bin/ .aws-sam/ cov.out
