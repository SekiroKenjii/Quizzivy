# Quizzivy — Agent Working Rules

Read this file at the start of every session. Read `docs/quizzivy-spec-v0.3.md`
before touching any area you have not worked on before, and
`docs/plan/00-overview.md` before touching architecture.

## Sources of truth, in order

1. `docs/quizzivy-spec-v0.3.md` — product and engineering spec
2. `docs/plan/` — the implementation plan derived from it
3. `api/openapi.yaml` — the API contract; spec §15 is documentation of it
4. Neon `postgres-best-practices` skill — all DDL and SQL
5. <https://www.postgresql.org/docs/18/index.html> — anything PG-version-specific

Where 1 and 2 disagree, the spec wins and the plan gets corrected.
Where 4 and 5 disagree, the PostgreSQL docs win.
Where 3 and anything disagree, fix `openapi.yaml` first, then regenerate.

Install the skill with `npx skills add neondatabase/postgres-skills`. One live
conflict to know: the skill teaches the pre-18
`ADD CHECK NOT VALID → VALIDATE → SET NOT NULL` pattern. PG18 has a native
`ALTER TABLE … ADD CONSTRAINT c NOT NULL col NOT VALID`; per rule 5, use it.

## Non-negotiable

- Target PostgreSQL 18. Do not write DDL that silently degrades on 16/17.
- No `SELECT *` anywhere in application code.
- Student-facing payloads never contain `isCorrect`, `sampleAnswer`,
  `acceptedAnswers`, or `transcript`. There is a test asserting this;
  do not weaken it.
- Every unauthenticated endpoint is rate-limited and leak-reviewed in the
  same PR that adds it.
- Never remove or skip a failing test to make CI green. Fix it or flag it.
- No new dependency without a stated reason in the PR description.
- Never commit secrets. `.env.example` stays current; `.env` stays ignored.
- **Never hand-edit generated code.** `server/gen/openapi/` and
  `web/src/lib/api/schema.d.ts` come from `api/openapi.yaml`. Change the
  contract and run `make gen`. CI fails on drift.
- **`attempt_events` and `audit_log` are append-only.** The app role has no
  `UPDATE` or `DELETE` on them. Do not grant it.
- **The refresh call is single-flight.** Concurrent 401s must await one shared
  promise. Parallel rotations trip reuse detection and log the user out on every
  cold load. See `docs/plan/30-risks.md` R-06.

## Verified platform facts (PG18)

Checked against the docs, not recalled. Do not re-derive; do not assume otherwise.

- `uuidv7()` is built-in, no extension. Signature `uuidv7([shift interval])`.
- **Virtual generated columns are the PG18 default** and `final_score` uses one.
  Verified against 18.6: they **cannot be indexed** ("indexes on virtual
  generated columns are not supported"), cannot carry `UNIQUE` or `PRIMARY KEY`,
  and cannot have extended statistics. They **can** take `NOT NULL` and **can**
  be referenced by a `CHECK`, and `sum()` over one works normally. Do not write
  `ORDER BY final_score` on a cross-attempt query.
- `OLD`/`NEW` in `RETURNING` work in all four DML statements, but capturing an
  audit diff in one statement requires a **data-modifying CTE** feeding the
  `INSERT`. A bare `UPDATE … RETURNING` then `INSERT` is a read-then-write race.
- **Google sign-in does NOT use the GIS SDK.** This is an approved deviation
  from §2, not an oversight: GIS `initCodeClient` cannot send a `code_challenge`,
  and §5.3 requires PKCE. We build the authorization request ourselves against
  Google's `authorization_endpoint` with S256. Do not "fix" this by reintroducing
  `accounts.google.com/gsi/client`.
- **`CLIENT_IP_HEADER` must never be `X-Forwarded-For`.** Proxies *append* to
  that header, so a client can prepend its own value and choose its own
  rate-limit bucket, defeating §6.5 on exactly the endpoints it protects. Name
  only a header the infrastructure overwrites: `CF-Connecting-IP` behind
  Cloudflare, `Fly-Client-IP` on Fly. The server refuses to start otherwise.
- **`unaccent()` is STABLE in PG18 — both the 1-arg and the 2-arg form.** It
  cannot go directly in an index expression. Use `app.immutable_unaccent()`,
  which pins the dictionary and asserts immutability. Changing the unaccent
  dictionary requires reindexing anything built on it.
- **`pg_trgm` is case-insensitive but not accent-insensitive.** `'nghé' ILIKE
  '%nghe%'` is false. Vietnamese search must go through
  `app.immutable_unaccent(lower(...))` on both the index and the query.
- `NOT NULL … NOT VALID` exists **only** as
  `ALTER TABLE … ADD CONSTRAINT c NOT NULL col NOT VALID`. It is not in the
  `CREATE TABLE` grammar and not available via `SET NOT NULL`. On a greenfield
  table, declare `NOT NULL` inline — it is free.

`server/internal/db/pg18_test.go` pins all four. If it fails, the docs changed
and the plan needs revisiting.

## Repository map

```
api/openapi.yaml   the contract — edit this first, then `make gen`
web/               quizzivy-web
  src/             spec §3 layout, unchanged -- source only, no tests
  tests/           units/ integration/ e2e/ support/ -- see web/tests/README.md
server/            Go module `quizzivy`; internal/ mirrors the feature folders
  gen/openapi/     generated, committed, never hand-edited
  media/probe/     pure-Go mp3 + mp4 duration; no ffprobe
migrations/        goose, forward-only, 00001…
seed/              seed data — never in a migration
docs/plan/         the plan; 20-data-model.md is the schema authority
```

A vertical slice is one `web/src/features/<name>/`, one
`server/internal/<name>/`, and one section of `api/openapi.yaml`.

## Authentication and authorization

- **`/admin/*` is gated on the path prefix**, in `httpx.RequireRole`. Adding an
  admin endpoint requires nothing: put it under `/admin/` and it is teacher-only.
  Putting a teacher-only endpoint anywhere else silently makes it student-
  reachable.
- **Everything the contract does not explicitly open requires a bearer token**,
  derived from `api/openapi.yaml`'s `security`. Five operations are open; the
  list is pinned by a test so a sixth takes an argument.

## `pnpm typecheck`, never `tsc --noEmit`

`web/tsconfig.json` is a solution file: `"files": []` plus project references.
Plain `tsc --noEmit` follows neither, so it checks **zero files and exits 0** --
it will happily "pass" on a file containing `const n: number = "a string"`.

Use `pnpm typecheck` (`tsc -b --noEmit`), which is what CI runs. This cost real
time once: a type-level contract assertion was silently never evaluated.

## Two things about the middleware chain

- **`oapi-codegen` applies the middleware slice in reverse**: the LAST entry
  wraps outermost and therefore runs FIRST. `NewRouter` passes the list through
  `inExecutionOrder`, so what is written top-to-bottom is what a request
  actually travels. Add new middleware to that list in the position you want it
  to RUN. `router_test.go` pins the direction.
- **The contract is enforced at runtime, once, in `httpx.ValidateRequests`.**
  Do not hand-write `required` / length / format checks in a handler; put the
  constraint in `api/openapi.yaml` and it is enforced everywhere. Handlers still
  own rules the schema cannot express — "the current password must be correct"
  is not a schema constraint.

## The API contract

`api/openapi.yaml` is hand-authored; everything else generates from it.
`oapi-codegen` → Go server interfaces. `openapi-typescript` → TS types. MSW
fixtures are validated against it with `ajv` so mocks cannot drift.

Zod schemas stay hand-written for form input, each carrying a type-level
`Expect<Equal<…>>` assertion against the generated request type. Drift fails
`tsc`, not review.

Adding an endpoint means: edit `openapi.yaml` → `make gen` → implement both
sides → commit the generated files.

## Local development

```
docker compose up -d     postgres:18 + minio
make migrate             goose up, as quizzivy_migrate
make seed
make gen                 regenerate from api/openapi.yaml
make dev                 web on :5173, api on :8080
```

Use `localhost` for both, never a `127.0.0.1`/`localhost` split — that is
cross-site and hides the cookie behaviour described in
`docs/plan/00-overview.md` §4.1.

Two local-dev facts worth not rediscovering:

- **The `postgres:18` image changed its data layout.** The volume mounts at
  `/var/lib/postgresql`, and the image puts data in a version subdirectory
  beneath it. Mounting at `/var/lib/postgresql/data` — correct for 17 and
  earlier — makes the container refuse to start.
- **Roles are created by `docker/initdb/`, not by a migration.** `CREATE ROLE`
  needs superuser and `quizzivy_migrate` deliberately is not one. `pg_trgm` and
  `unaccent` are *trusted* extensions, so the migrate role can install them
  itself — verified.

Two database roles: `quizzivy_migrate` owns the schema and runs goose;
`quizzivy_app` is what the API connects as and owns nothing.

## Migrations

- goose, SQL, forward-only, **one concern per file**, sequential zero-padded
  names (`00016_create_test_versions.sql`).
- Every file has a `-- +goose Down` that actually works. CI runs up/down/up.
- Every task that touches the DB names its migration file in the PR.
- `CREATE INDEX CONCURRENTLY` cannot run in a transaction — mark those files
  `-- +goose NO TRANSACTION`. It is only needed once a table has rows, so none
  of `00001`–`00022` uses it.
- Expand-contract for anything breaking; never in one migration.
- The inventory and the reasoning behind every constraint and index are in
  `docs/plan/20-data-model.md`. Read the deviation register in §12 before
  changing a table — twenty deviations from the spec sketch are deliberate.

## Tests

`web/tests/` sits beside `web/src/`, split by cost: `units/` (fast, no build, no
browser), `integration/` (several real layers at once), `e2e/` (Playwright
against a production build), `support/` (harness, not tests). `web/tests/README.md`
has the placement rule. `@/` is `src/`, `@tests/` is `tests/`.

Go tests stay beside the code they cover, as is idiomatic.

## Git workflow

Gitflow. `main` is released only and tagged; `develop` is integration.

- One task from a phase file = one branch = one PR: `feature/t-<phase>-<n>-<slug>`
  off `develop`.
- Phase completion is `release/phase-<n>` → `main` → back-merge to `develop`.
- `hotfix/<slug>` off `main`, merged to both.
- Never commit directly to `main` or `develop`.

## Design

Follow spec §12 exactly. Neutral zinc palette, charcoal primary buttons,
semantic color only where it carries meaning. No gradients, no
glassmorphism, no pulse rings, no glow, no emoji in UI chrome. If a
design choice feels like it needs a new color, it probably needs a
different layout instead.

Colors come from CSS variables / Tailwind tokens, never hard-coded. The one
exception is `GoogleMark` — a provider's identifying mark on a button that hands
the user to that provider. It is not decorative colour and not theme-aware on
purpose. Do not add a second exception without the same argument. Dark mode is
not in v1 but must remain addable without touching components.

Integrity UI is calm: plain dialogs, plain text, no alarm iconography, no red
banners, no shame. The teacher judges; the app reports.

## Language

Vietnamese first. Write the `vi` string, then `en`. No English-only
user-facing text ever reaches a commit. Code, comments, commit messages,
and docs are English.

Design for longer Vietnamese strings; avoid fixed-width labels.

## Per-PR checklist

- [ ] TypeScript strict passes; no `any` without a comment
- [ ] Lint clean
- [ ] Loading / error / empty states present
- [ ] Keyboard-operable; visible focus
- [ ] All strings via `t()`, keys in both `vi` and `en`
- [ ] Tests added or updated at the right level (spec §14)
- [ ] DDL reviewed against spec §13, `docs/plan/20-data-model.md`, and the Neon
      skill; migration file named
- [ ] New public (unauthenticated) endpoint: rate-limited **and** leak-reviewed
      in this PR (§6.5)
- [ ] `make gen` run and generated files committed if the contract changed
- [ ] `.env.example` updated if config changed
- [ ] New dependencies listed with reasons

## High-risk areas — extra care

`features/take-test/`, `features/integrity/`, `features/media/`, and
anything touching `attempts` or `test_versions`. Run the relevant unit
tests before and after every change to these. Do not refactor them
opportunistically while doing something else.

Four tests are canaries. If one starts failing, something load-bearing broke —
fix the cause, never the test:

- `publish/snapshot_test.go` — editing a bank question after publish must not
  change the published version. Without this, versioning is decorative.
- `AudioPlayer.test.tsx` — `.play()` must be called in the same synchronous tick
  as the click. Any `await` before it breaks iOS Safari silently.
- `integrity/events_test.go` — the same `client_seq` from two `session_id`s must
  both persist. This is what stops a resumed attempt's timeline vanishing.
- `client.refresh.test.ts` — five concurrent 401s must issue exactly one refresh.
- `tests/integration/router-chunks.test.ts` — the admin tree must stay out of
  the entry chunk. It runs a real build; reading the router and trusting `lazy`
  would not catch the regression that actually happens.

`docs/plan/30-risks.md` explains what each one is guarding.

## When you are unsure

Ask one precise question. Do not guess on §5 (auth), §6 (join codes),
§10 (integrity), §11 (audio), or §13 (data model). Guessing in these
areas is more expensive than waiting for an answer.

Check `docs/plan/40-open-items.md` first — the question may already be there with
a stated default, in which case build the default and move on.

## Keeping documents current

When a decision changes, edit the spec section and bump its version at the top
(§18), then correct the affected plan file. A plan that disagrees with what was
built is worse than no plan.
