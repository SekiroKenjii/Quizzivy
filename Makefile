# Quizzivy -- developer entry points.
# See CONTRIBUTING.md for the branch model and docs/plan/ for the task list.

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Loaded from .env if present; every value has a dev default in docker-compose.
-include .env
export

# Generator versions are PINNED. A floating version makes the CI drift check
# meaningless: regenerated output would differ for reasons unrelated to the
# contract. oapi-codegen is pinned by the `tool` directive in server/go.mod.
SPECTRAL_VERSION ?= 6.16.3
OPENAPI_TS_VERSION ?= 7.13.0

MIGRATE_DSN ?= postgres://quizzivy_migrate:$(or $(QUIZZIVY_MIGRATE_PASSWORD),migrate)@localhost:5432/quizzivy?sslmode=disable
APP_DSN     ?= postgres://quizzivy_app:$(or $(QUIZZIVY_APP_PASSWORD),app)@localhost:5432/quizzivy?sslmode=disable

.PHONY: help doctor up down reset db-shell migrate migrate-down migrate-redo \
        seed gen contract verify-google verify-r2 dev dev-web dev-api test test-web test-api e2e lint

help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

doctor: ## Check prerequisites (T-0.2 .. T-0.4 blockers)
	@echo "== required =="
	@command -v docker  >/dev/null && echo "  docker   $$(docker --version | cut -d' ' -f3 | tr -d ,)" || echo "  docker   MISSING"
	@command -v node    >/dev/null && echo "  node     $$(node --version)"  || echo "  node     MISSING"
	@command -v pnpm    >/dev/null && echo "  pnpm     $$(pnpm --version)"  || echo "  pnpm     MISSING"
	@command -v go      >/dev/null && echo "  go       $$(go version | cut -d' ' -f3)" || echo "  go       MISSING"
	@command -v psql    >/dev/null && echo "  psql     $$(psql --version | cut -d' ' -f3)" || echo "  psql     MISSING  -> T-0.4 (install postgresql-client-18)"
	@command -v goose   >/dev/null && echo "  goose    $$(goose --version 2>&1 | head -1)" || echo "  goose    MISSING  -> T-0.4"
	@echo "== credentials =="
	@test -f .env && echo "  .env     present" || echo "  .env     MISSING  -> cp .env.example .env"
	@grep -q '^VITE_GOOGLE_CLIENT_ID=.\+' .env 2>/dev/null && echo "  google   set      -> verify with: make verify-google" || echo "  google   NOT SET  -> T-0.2  docs/setup/google-oauth.md"
	@grep -q '^R2_ACCESS_KEY_ID=.\+'      .env 2>/dev/null && echo "  r2       set      -> verify with: make verify-r2" || echo "  r2       NOT SET  -> T-0.3  docs/setup/r2.md (MinIO covers local dev)"

verify-google: ## T-0.2 -- check the Google OAuth client works
	@./scripts/verify-google.sh

verify-r2: ## T-0.3 -- check the R2 bucket, credentials and privacy
	@./scripts/verify-r2.sh

up: ## Start postgres:18 + MinIO
	docker compose up -d --wait db minio
	docker compose up minio-init

down: ## Stop the stack (keeps volumes)
	docker compose down

reset: ## Destroy the stack AND its data, then start clean
	docker compose down -v
	$(MAKE) up
	$(MAKE) migrate

db-shell: ## psql as the migrate role
	psql "$(MIGRATE_DSN)"

migrate: ## Apply all migrations
	goose -dir migrations postgres "$(MIGRATE_DSN)" up

migrate-down: ## Roll back one migration
	goose -dir migrations postgres "$(MIGRATE_DSN)" down

migrate-redo: ## up -> down -> up, proving every Down works (what CI runs)
	goose -dir migrations postgres "$(MIGRATE_DSN)" up
	goose -dir migrations postgres "$(MIGRATE_DSN)" reset
	goose -dir migrations postgres "$(MIGRATE_DSN)" up

seed: ## Load seed/ (never in a migration -- spec §13.7)
	@for f in seed/*.sql; do echo "  $$f"; psql -v ON_ERROR_STOP=1 "$(MIGRATE_DSN)" -f "$$f"; done

contract: ## Lint api/openapi.yaml and assert its invariants
	npx --yes @stoplight/spectral-cli@$(SPECTRAL_VERSION) lint api/openapi.yaml --ruleset api/.spectral.yaml
	# Structural assertions live in the Vitest suite (T-0.13 ported them out of
	# the interim Python checker). Run just those, not the whole suite.
	cd web && pnpm vitest run tests/units/contract

gen: contract ## Regenerate Go + TS from api/openapi.yaml
	cd server && go tool oapi-codegen --config ../api/oapi-codegen.yaml ../api/openapi.yaml
	npx --yes openapi-typescript@$(OPENAPI_TS_VERSION) api/openapi.yaml -o web/src/lib/api/schema.d.ts
	cd server && go build ./... && go test ./gen/...
	@echo "generated: server/gen/openapi/openapi.gen.go, web/src/lib/api/schema.d.ts"

gen-check: gen ## Fail if generated output drifts from the contract (what CI runs)
	@git diff --exit-code -- server/gen web/src/lib/api/schema.d.ts \
		|| (echo ""; \
		    echo "Generated code is out of date. Run 'make gen' and commit the result."; \
		    exit 1)
	@echo "generated code matches api/openapi.yaml"

dev: ## Run web and api together
	$(MAKE) -j2 dev-api dev-web

dev-api: ## Go API on :8080
	cd server && DATABASE_URL="$(APP_DSN)" go run ./cmd/api

dev-web: ## Vite on :5173
	cd web && pnpm dev

test: contract test-api test-web ## Run all tests

test-web:
	cd web && pnpm test

test-api: migrate ## Go tests, including the DB-backed ones
	# The DB tests skip themselves when TEST_DATABASE_URL is unset, so a bare
	# `go test ./...` passes without ever touching Postgres. The Makefile knows
	# the DSN, so wire it up -- a green run here means the DB tests really ran.
	# TEST_DESTRUCTIVE stays off: `make test` must not wipe a seeded database.
	#
	# Depends on `migrate` because only internal/db applies migrations, and
	# `go test ./...` runs packages in parallel -- so every other DB-backed
	# package was relying on winning a race it does not control. CI applies them
	# in its own step for the same reason.
	#
	# Packages run in parallel against one database. Tests that diff a global
	# aggregate (the dashboard) do so inside a REPEATABLE READ transaction, so
	# another package's inserts cannot move the number between two readings.
	#
	# -count=1, as CI runs it: a DB-backed test's result depends on the schema
	# and rows behind it, which the test cache cannot see, so a cached "ok" can
	# hide a test that would fail against the database as it is now.
	cd server && TEST_DATABASE_URL="$(MIGRATE_DSN)" go test ./... -count=1

e2e: ## Playwright, against a real production build
	cd web && pnpm e2e

lint: ## Lint both sides
	cd web && pnpm lint
	cd server && go vet ./...
	# staticcheck's SA1019 is what enforces "never use a deprecated identifier".
	cd server && go tool staticcheck ./...
