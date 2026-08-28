# Quizzivy

Web app for a private English-teaching practice. Students take tests assigned by
their teacher; the teacher builds tests, assigns them, monitors attempts, grades
open answers, and reviews results.

v1 ships one capability end to end: **test-taking**.

Product language is Vietnamese. Code, comments and docs are English.

## Documents, in order of authority

| | |
|---|---|
| [`docs/quizzivy-spec-v0.3.md`](docs/quizzivy-spec-v0.3.md) | Product and engineering spec. Source of truth. |
| [`docs/plan/`](docs/plan/) | Implementation plan derived from it. Start at [`00-overview.md`](docs/plan/00-overview.md). |
| [`api/openapi.yaml`](api/) | The API contract. Spec §15 documents it. |
| [`AGENTS.md`](AGENTS.md) | Standing rules. Read before touching anything. |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Branch model and PR expectations. |

The schema lives in [`docs/plan/20-data-model.md`](docs/plan/20-data-model.md),
including a register of every deliberate deviation from the spec sketch.

## Stack

Vite + React 19 + TypeScript (strict), TanStack Query, Zustand, Tailwind v4 +
shadcn/ui, i18next. Go backend, PostgreSQL 18, Cloudflare R2. Fixed by spec §2 —
do not swap libraries without approval.

## Quick start

```bash
cp .env.example .env
make doctor      # what is installed, what is still blocked
make up          # postgres:18 + MinIO
make migrate
make seed
make dev         # web on :5173, api on :8080
```

`make help` lists everything.

Use `localhost` for both services, never a `127.0.0.1`/`localhost` split — that
is cross-site, and it hides the cookie behaviour the real deployment depends on
([`00-overview.md` §4.1](docs/plan/00-overview.md)).

## Local services

| | |
|---|---|
| PostgreSQL 18 | `localhost:5432`, database `quizzivy` |
| MinIO (S3 API) | `localhost:9000` — stands in for R2 |
| MinIO console | `localhost:9001` |

MinIO is why the app builds and tests without R2 credentials:
`aws-sdk-go-v2/s3` targets both and only the endpoint differs.

Two database roles, per spec §13.5. `quizzivy_migrate` owns the schema and runs
goose; `quizzivy_app` is what the API connects as and owns nothing.

## Status

Phase 0 of six. See [`docs/plan/10-phase-0.md`](docs/plan/10-phase-0.md).

Outstanding blockers, owned by Thuong: a Google OAuth client (**T-0.2**, blocks
the Phase 1 Google path with no workaround) and R2 credentials (**T-0.3**, blocks
Phase 2 deployment only). `make doctor` reports both.
