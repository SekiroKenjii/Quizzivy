# Quizzivy -- developer entry points.
# See CONTRIBUTING.md for the branch model and docs/plan/ for the task list.

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Loaded from .env if present; every value has a dev default in docker-compose.
-include .env
export

MIGRATE_DSN ?= postgres://quizzivy_migrate:$(or $(QUIZZIVY_MIGRATE_PASSWORD),migrate)@localhost:5432/quizzivy?sslmode=disable
APP_DSN     ?= postgres://quizzivy_app:$(or $(QUIZZIVY_APP_PASSWORD),app)@localhost:5432/quizzivy?sslmode=disable

.PHONY: help doctor up down reset db-shell migrate migrate-down migrate-redo \
        seed gen dev dev-web dev-api test test-web test-api lint

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
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
	@grep -q '^VITE_GOOGLE_CLIENT_ID=.\+' .env 2>/dev/null && echo "  google   set" || echo "  google   NOT SET  -> T-0.2 (blocks Phase 1 Google path)"
	@grep -q '^R2_ACCESS_KEY_ID=.\+'      .env 2>/dev/null && echo "  r2       set" || echo "  r2       NOT SET  -> T-0.3 (blocks Phase 2 deploy; MinIO covers local)"

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

gen: ## Regenerate Go + TS from api/openapi.yaml (T-0.8)
	@echo "not wired yet -- T-0.8"

dev: ## Run web and api together
	$(MAKE) -j2 dev-api dev-web

dev-api: ## Go API on :8080
	cd server && DATABASE_URL="$(APP_DSN)" go run ./cmd/api

dev-web: ## Vite on :5173
	cd web && pnpm dev

test: test-api test-web ## Run all tests

test-web:
	cd web && pnpm test

test-api:
	cd server && go test ./...

lint: ## Lint both sides
	cd web && pnpm lint
	cd server && go vet ./...
