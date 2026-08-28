# Phase 0 — Scaffold & contract

**Exit criteria (§16):** `pnpm dev` / `pnpm test` / `pnpm build` green; `/login`
renders; migrations clean. Extended with: `api/openapi.yaml` exists and both
sides generate from it; the four PG18 constructs are proven by a passing test.

Tasks are ordered so the phase can stop at any boundary and still build.
T-0.2 – T-0.4 are **owned by Thuong** and block later phases, not this one; they
are listed first so they can be started in parallel with everything else.

Branch per task off `develop` as `feature/t-0-NN-<slug>`.

---

### T-0.1 — Install the Neon postgres-best-practices skill
**Depends on:** none
**Touches:** repo root, `AGENTS.md`
**Size:** S
**Done when:**
- [ ] `npx skills add neondatabase/postgres-skills` has been run and
      `skills/postgres-best-practices/SKILL.md` plus `references/` are readable
- [ ] `AGENTS.md` records the install path and that schema-design, indexing and
      migration guidance is binding (§13.1)
- [ ] `AGENTS.md` records the one live conflict with the PG18 docs: the skill
      teaches the pre-18 `ADD CHECK NOT VALID → VALIDATE → SET NOT NULL` pattern;
      PG18's native `ADD CONSTRAINT … NOT NULL … NOT VALID` supersedes it, and
      per §13.1 the PostgreSQL docs win
- [ ] `skills/` is gitignored if the tool vendors it; the pinned reference is the
      upstream repo, not a copy

---

### T-0.2 — [Thuong] Provision the Google OAuth client
**Depends on:** none
**Touches:** Google Cloud console, `.env.example`
**Size:** S
**Done when:**
- [ ] A Web application OAuth client exists with authorized JavaScript origins
      `http://localhost:5173` and `https://app.quizzivy.com`
- [ ] Authorized redirect URIs cover both origins (§5.3 uses Authorization Code
      + PKCE, so a redirect URI is required even though the code is exchanged
      server-side)
- [ ] Client ID is recorded in `.env.example` as `VITE_GOOGLE_CLIENT_ID` (public,
      §5.3); the client secret is handed over out-of-band and never committed
- [ ] The OAuth consent screen lists the `email`, `profile` and `openid` scopes
      only — these are Google's *non-sensitive* scopes, so publishing needs no
      verification review. Anything else triggers one
- [ ] **Publishing status is "In production", not "Testing".** Testing status
      only admits users on a hand-maintained test-user list, so every new student
      would be blocked until someone remembers to add them
- [ ] `make verify-google` passes — it probes Google's token endpoint with the
      real credentials and an invalid code, so `invalid_grant` proves the client
      works without any browser or consent screen
- [ ] Full instructions: `docs/setup/google-oauth.md`

**Blocks:** all of Phase 1's Google path (T-1.6 onward). No workaround — GIS
cannot be meaningfully mocked end-to-end without a real client ID.

---

### T-0.3 — [Thuong] Create the R2 bucket and scoped credentials
**Depends on:** none
**Touches:** Cloudflare dashboard, `.env.example`
**Size:** S
**Done when:**
- [ ] Bucket `quizzivy-media` exists, **private**: no public listing, no public
      read (§11.2)
- [ ] An API token scoped to that bucket only, with object read/write, exists
- [ ] The r2.dev **public development URL is disabled** and no custom domain is
      bound — Cloudflare offers public access in one click, and enabling it would
      make every listening file world-readable by URL
- [ ] The API token is **Object Read & Write scoped to `quizzivy-media` only**,
      not an Admin token
- [ ] `.env.example` lists `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`,
      `R2_SECRET_ACCESS_KEY` and `S3_BUCKET` with empty values; the endpoint is
      derived, not configured
- [ ] `make verify-r2` passes all six checks, including that an **unsigned
      request is refused** — the one that proves §11.2's private bucket
- [ ] Full instructions: `docs/setup/r2.md`

**Blocks:** Phase 2 deployment only. Local development is unblocked by the MinIO
service in T-0.6, which speaks the same S3 API.

---

### T-0.4 — [Thuong] Install the local database toolchain
**Depends on:** none
**Touches:** developer machine
**Size:** S
**Done when:**
- [ ] `psql --version` reports 18.x (`postgresql-client-18`)
- [ ] `goose --version` succeeds
- [ ] `docker pull postgres:18` succeeds
- [ ] `git flow version` succeeds, or a decision is recorded to drive gitflow
      with plain `git` branch conventions instead

---

### T-0.5 — Initialize the gitflow branch model and PR template
**Depends on:** none
**Touches:** repo settings, `.github/pull_request_template.md`
**Size:** S
**Done when:**
- [ ] `develop` exists and is the repository's default branch
- [ ] `main` and `develop` both require a passing CI check and one approving
      review before merge
- [ ] `.github/pull_request_template.md` carries the §14 definition of done as
      checkboxes, plus two conditional blocks that must be deleted or ticked:
      "DDL reviewed against §13 and the Neon skill" and "new public endpoint:
      rate-limited and leak-reviewed (§6.5)"
- [ ] The template has a **"New dependencies and why"** section (§14, §18)
- [ ] `CONTRIBUTING.md` records the branch naming: `feature/t-<phase>-<n>-<slug>`
      off `develop`, `release/phase-<n>`, `hotfix/<slug>`

---

### T-0.6 — Create the monorepo skeleton and local dev stack
**Depends on:** T-0.5
**Touches:** repo root, `docker-compose.yml`, `Makefile`, `.env.example`
**Size:** M
**Done when:**
- [ ] Directory layout matches `00-overview.md` §6: `api/`, `web/`, `server/`,
      `migrations/`, `seed/`, `docs/`
- [ ] `docs/quizzivy-spec-v0.3.md` is committed so `AGENTS.md` can reference it
      by a relative path
- [ ] `docker compose up -d` starts `postgres:18` on 5432 and MinIO on 9000/9001
- [ ] The postgres `initdb` script creates roles `quizzivy_migrate` and
      `quizzivy_app` and the `quizzivy` database owned by the former
      (`20-data-model.md` §1) — roles cannot be created by a migration
- [ ] The MinIO init step creates a private `quizzivy-media` bucket
- [ ] `make dev`, `make migrate`, `make seed`, `make gen`, `make test` exist and
      are documented in `README.md`
- [ ] `.env.example` is complete and `.env` is gitignored (AGENTS.md)
- [ ] `make dev` uses `localhost` for both web and API — never a
      `127.0.0.1`/`localhost` split, which is cross-site and would mask the
      cookie behaviour described in `00-overview.md` §4.1

---

### T-0.7 — Author the OpenAPI contract from §15
**Depends on:** T-0.6
**Touches:** `api/openapi.yaml`
**Size:** L
**Done when:**
- [ ] Every path in §15 is present with request and response schemas, including
      the endpoints §15 omits but §8 requires: admin dashboard aggregates,
      cross-assignment attempt lists (awaiting grading, flagged), test version
      history, and class member list
- [ ] The error envelope from `00-overview.md` §7 is a shared component, and
      every error `code` is enumerated (starting with `ACCOUNT_NOT_PROVISIONED`,
      `JOIN_CODE_INVALID`, `JOIN_CODE_EXPIRED`, `JOIN_CODE_EXHAUSTED`,
      `ATTEMPT_CLOSED`, `SESSION_SUPERSEDED`, `RATE_LIMITED`)
- [ ] The three join-code failure modes are **distinct codes but carry no
      class-identifying detail** (§6.5, §9)
- [ ] List responses share one `{ items, nextCursor }` envelope (§13.8)
- [ ] Student and admin question schemas are **separate components**;
      `isCorrect`, `sampleAnswer`, `acceptedAnswers` and `transcript` do not
      appear anywhere in a student schema (§13.5)
- [ ] Public endpoints are tagged `public` so T-0.14's middleware can assert
      every one of them is rate-limited
- [ ] `spectral lint api/openapi.yaml` passes in CI, with
      `operation-description` and `operation-operationId` raised to `error` —
      this file is read by people as much as by generators
- [ ] Contract assertions run mechanically, not by review: no dangling `$ref`;
      no `/app/*` 2xx response reaches a schema containing `isCorrect`,
      `sampleAnswer` or `acceptedAnswers` **at any depth**, and `transcript`
      only from the result endpoint; `AdminQuestion` still *has* the grading key;
      every public operation carries a rate limit and a 429; `operationId`s are
      unique; every paginated response uses the one `{items, nextCursor}`
      envelope
- [ ] The assertions are **mutation-tested** — deliberately leaking a key, or
      dropping a rate limit, must fail the check. A check that cannot fail is
      not a check
- [ ] These live in `api/contract_check.py` for now, because the web project
      does not exist until T-0.9. **T-0.13 ports them into the Vitest suite**
      so they run with everything else; delete the Python once it does

---

### T-0.8 — Wire code generation and the CI drift check
**Depends on:** T-0.7
**Touches:** `Makefile`, `server/go.mod`, `server/gen/openapi/`,
`web/src/lib/api/schema.d.ts`, `.gitattributes`, CI
**Size:** M

> **Ordering correction.** As planned, T-0.8 sat before T-0.14 (Go skeleton) —
> but codegen needs a module to generate *into*, so T-0.8 creates the minimal
> `server/go.mod` (`module quizzivy`) and T-0.14 builds the server on top of it.
> The plan had this dependency backwards.

**Done when:**
- [ ] `make gen` runs `oapi-codegen` (strict server mode) into
      `server/gen/openapi/` and `openapi-typescript` into
      `web/src/lib/api/schema.d.ts`
- [ ] **Generator versions are pinned** — `oapi-codegen` by the `tool` directive
      in `server/go.mod`, the npm tools by variables in the Makefile. A floating
      version makes the drift check meaningless: output would differ for reasons
      unrelated to the contract
- [ ] Go boundary tests assert the *generated types* honour §13.5:
      `StudentQuestion` and `AttemptSession` expose none of the four forbidden
      keys at any depth, `AdminQuestion` still carries all of them, and
      `ResultQuestion` carries `transcript` but not the other three
- [ ] Those tests are **mutation-tested** — leaking a key into the contract and
      regenerating must fail them
- [ ] Both outputs are committed, so the repo builds without the generators
- [ ] CI runs `make gen` then `git diff --exit-code` and fails on drift
- [ ] `server/gen/` and `schema.d.ts` carry a "generated — do not edit" header
      and are excluded from lint autofix
- [ ] PR description states the reason for `openapi-typescript`, `ajv` and
      `oapi-codegen` (§14)

---

### T-0.9 — Scaffold the web app: Vite, React 19, Tailwind v4, shadcn
**Depends on:** T-0.6
**Touches:** `web/`
**Size:** M
**Done when:**
- [ ] Vite + React 19 + TypeScript **strict** builds; `pnpm build` green
- [ ] Tailwind v4 configured with the §12 palette as CSS variables — zinc scale,
      `zinc-900` primary, semantic green/red/amber only. Theming goes through
      tokens so dark mode is addable without touching components (§12)
- [ ] shadcn/ui initialized on the neutral/zinc base; Button, Input, Dialog,
      Table, Card installed as a starting set
- [ ] ESLint (typescript-eslint, react-hooks, jsx-a11y) + Prettier; CI fails on
      lint errors (§2). **The Vite template now ships `oxlint` instead — it is
      removed.** §2 names ESLint and specifically `jsx-a11y`, and accessibility
      linting is load-bearing here (§14 keyboard-operable, §16 Lighthouse ≥ 95).
      Revisiting oxlint later is a §2 change needing approval, not a default
      to drift into
- [ ] Plugins are registered explicitly rather than composed through `extends` —
      the plugins disagree on how they export flat configs, and composing their
      presets produced an error naming the wrong plugin
- [ ] `lucide-react` is the only icon package in `package.json` (§2)
- [ ] Test: `web/src/styles/tokens.test.ts` guards §12's colour rules. Note the
      subtlety it has to get right: Tailwind's zinc sits at hue ~286, squarely in
      the blue band, so a naive hue check would reject the mandated palette. What
      separates zinc from indigo is **chroma** — 0.006 versus 0.23 — so the rule
      is "saturated **and** in the blue band"
- [ ] It also asserts `@theme` maps tokens to `var(--token)` and never to a
      literal colour, which is what makes §12's "dark mode addable without
      touching components" actually true
- [ ] **Mutation-tested**: swapping the primary for indigo, and inlining a
      literal into `@theme`, must each fail it
- [ ] TypeScript strict passes, no `any`; lint clean

---

### T-0.10 — Build the router, three route trees, and layouts
**Depends on:** T-0.9
**Touches:** `web/src/app/`, `web/src/layouts/`
**Size:** M
**Done when:**
- [ ] React Router v7 in SPA mode with three trees — `/admin/*`, `/app/*`, and
      public (`/login`, `/join/*`) — split with `lazy` so a student never
      downloads admin code and an anonymous visitor downloads neither (§2)
- [ ] `AdminLayout`, `StudentLayout`, `FocusLayout`, `PublicLayout` exist per
      §8/§9, with the collapsible sidebar ≤1280px and 768px minimum width
- [ ] `/login`, `/403`, `/404` render; `/login` is the §16 exit criterion
- [ ] Global error boundary with reload and a copyable error ID (§9)
- [ ] Test: `web/src/app/router.chunks.test.ts` runs a real Vite build in-memory
      and asserts against the actual chunk graph — not by reading the router and
      trusting `lazy`. It checks the admin tree, the student tree and the focus
      shell are all absent from the entry chunk, that the admin tree is still
      built somewhere (so a broken import path cannot pass vacuously), and that
      no chunk contains both the admin and student trees
- [ ] **Mutation-tested**: turning one `lazy` into a static import must fail it
- [ ] Note for whoever touches the router: React Router `lazy` supplies
      `Component`/`loader`/`action`/`ErrorBoundary` but **not `children`** — the
      route tree has to be statically matchable. A first version returned
      `children` from `lazy`; it type-checked, rendered the layout, and 404'd on
      every child route
- [ ] Loading / error / empty states present; keyboard-operable (§14)

---

### T-0.11 — Set up i18n with Vietnamese as default
**Depends on:** T-0.9

> **Ordering correction.** Done *before* T-0.10. §14 requires every string to go
> through `t()`, and T-0.10 writes the 403/404/nav/error-boundary copy — so the
> planned order meant writing every one of those strings twice.

**Touches:** `web/src/lib/i18n/`
**Size:** S
**Done when:**
- [ ] i18next + react-i18next configured, `vi` default and `en` secondary (§2)
- [ ] date-fns + date-fns-tz with `Asia/Ho_Chi_Minh` as the render timezone;
      all stored values are UTC (§13.2)
- [ ] `react/jsx-no-literals` fails on a string literal in JSX children.
      Configured with `ignoreProps`, because checking every attribute would flag
      `className`/`id`/`type` and be unusable
- [ ] A companion test covers what that rule cannot: literal text in the
      attributes that are actually user-facing — `aria-label`, `placeholder`,
      `title`, `alt`. An English `aria-label` is read aloud to a Vietnamese
      student, which is precisely what AGENTS.md forbids
- [ ] Both are **mutation-tested**
- [ ] `formatDateTime` and friends are the only place a timezone is applied, so
      "store UTC, render `Asia/Ho_Chi_Minh`" (§13.2) is a property of the code
      rather than a convention. Tested across the UTC+7 date boundary, so it
      holds wherever CI runs
- [ ] Test: `web/src/lib/i18n/parity.test.ts` fails if any key exists in one
      locale and not the other
- [ ] All strings via `t()`, keys in both `vi` and `en` (§14)

---

### T-0.12 — Build the typed API client with single-flight refresh
**Depends on:** T-0.8, T-0.11
**Touches:** `web/src/lib/api/client.ts`, `web/src/stores/auth.ts`
**Size:** M
**Done when:**
- [ ] Native `fetch` wrapper typed against `schema.d.ts`; no axios (§2)
- [ ] Access token read from the Zustand store in memory only — never
      `localStorage`, never `sessionStorage` (§5.2)
- [ ] `credentials: 'include'` on `/auth/*` so the host-only refresh cookie is
      sent (`00-overview.md` §4.1)
- [ ] On 401: one refresh, then retry; a second 401 logs out (§5.2)
- [ ] **Refresh is single-flight** — concurrent 401s await one shared promise.
      Without this, parallel query mounts trigger parallel rotations and §5.2's
      reuse detection revokes the family on every cold load
- [ ] Error envelope decoded into a typed `ApiError` carrying `code`, `message`
      and `requestId`
- [ ] Test: `client.refresh.test.ts` — "five concurrent 401s issue exactly one
      POST /auth/refresh and all five requests succeed". Asserted by **counting**
      calls: an implementation that refreshes per request passes every functional
      test and still logs everyone out in production
- [ ] Test: retries carry the **new** token, not the stale one
- [ ] Test: a later, separate 401 is allowed to refresh again — single-flight
      must not become refresh-once-ever
- [ ] Test: the login endpoint's own 401 does not trigger a refresh (that would
      loop, and would trip reuse detection)
- [ ] **Mutation-tested**: making refresh per-request must fail it
- [ ] `client.typecheck.ts` asserts the contract binds at the type level, using
      `@ts-expect-error` — which is self-verifying, since TypeScript fails the
      build if the directive stops being needed. It covers unknown paths, wrong
      methods, missing body fields, wrong path-param names, and **§13.5**: a
      student payload must not expose `sampleAnswer`, `transcript` or
      `isCorrect`. That is the third layer, after `contract_check.py` and the Go
      reflection tests
- [ ] Test: `client.refresh.test.ts` — "a second 401 after refresh clears the
      store and redirects to /login"
- [ ] TypeScript strict passes, no `any` (§14)

---

### T-0.13 — Set up the test harnesses
**Depends on:** T-0.9, T-0.8
**Touches:** `web/vitest.config.ts`, `web/src/test/`, `web/playwright.config.ts`
**Size:** M
**Done when:**
- [ ] Vitest + Testing Library run via `pnpm test`
- [ ] The T-0.7 contract assertions are ported from `api/contract_check.py` into
      `web/src/test/openapi-contract.test.ts`, mutation tests included, and the
      Python file is deleted
- [ ] MSW server configured; handlers are typed against `schema.d.ts`
- [ ] **Every MSW fixture is validated against `api/openapi.yaml` with `ajv`** at
      test setup, so a mock cannot drift from the contract (`00-overview.md` §3)
- [ ] Playwright configured against a `pnpm build && pnpm preview` target
- [ ] Test: `web/src/test/msw-contract.test.ts` fails when a handler returns a
      body the OpenAPI schema rejects
- [ ] `pnpm test` and `pnpm build` green (§16 exit criterion)

---

### T-0.14 — Scaffold the Go API server
**Depends on:** T-0.8, T-0.6
**Touches:** `server/cmd/api/`, `server/internal/httpx/`, `server/internal/ratelimit/`
**Size:** M
**Done when:**
- [ ] `go run ./cmd/api` serves on 8080. (`server/go.mod` already exists —
      T-0.8 created it because codegen needed a module to target.)
- [ ] Generated strict handlers from T-0.8 are mounted; routes come from the
      contract, not from hand-written mux entries
- [ ] pgx pool connects as `quizzivy_app` (never the owner, §13.5)
- [ ] Structured logging with a request ID that matches the error envelope's
      `requestId` (`00-overview.md` §7)
- [ ] CORS middleware: exact-origin allowlist, `Allow-Credentials: true`,
      `Vary: Origin`, preflight for `PATCH`/`DELETE` + `Authorization`. Never `*`
- [ ] Rate-limit middleware exists with per-IP and per-key buckets and emits
      `429` with `Retry-After` (§6.5)
- [ ] **A startup assertion fails the process if any route tagged `public` in
      `api/openapi.yaml` has no rate-limit rule registered** — this is what makes
      §14's public-endpoint checkbox structural rather than procedural
- [ ] `GET /healthz` returns 200 with database reachability
- [ ] Test: `httpx/cors_test.go` — a request from an unlisted origin gets no
      `Access-Control-Allow-Origin` header
- [ ] Test: `ratelimit/limiter_test.go` — 11th request in a minute returns 429
      with `Retry-After`
- [ ] Test: `httpx/public_routes_test.go` — every `public`-tagged operation has a
      registered limiter

---

### T-0.15 — Add the foundation migrations
**Depends on:** T-0.6
**Touches:** `migrations/`
**Size:** M
**Migrations:** `00001_create_schema_and_extensions.sql`,
`00002_create_enums.sql`, `00003_create_updated_at_trigger.sql`
**Done when:**
- [ ] Schema `app`; extensions `pg_trgm` and `unaccent`; the
      `app.immutable_unaccent()` wrapper; and the documented `REVOKE` from
      `20-data-model.md` §2
- [ ] Test: `db/unaccent_test.go` — `app.immutable_unaccent(lower('Đường'))` is
      `duong` and `lower('tiếng Việt')` folds to `tieng viet`, covering the `Đ`
      stroke that plain Unicode decomposition misses
- [ ] All seven enum types including `app.integrity_action`
- [ ] `app.set_updated_at()` trigger function
- [ ] Every file has a correct `-- +goose Down`
- [ ] `make migrate` runs clean as `quizzivy_migrate` (§16 exit criterion)
- [ ] Test: `server/internal/db/migrate_test.go` — `up`, `down`, `up` against a
      fresh `postgres:18` container leaves an identical schema dump
- [ ] DDL reviewed against §13 and the Neon skill (§14)

---

### T-0.16 — Prove the four PG18 constructs with a test
**Depends on:** T-0.15
**Touches:** `server/internal/db/pg18_test.go`
**Size:** S
**Done when:**
- [ ] Test asserts `uuidv7()` exists, needs no extension, and that
      `uuid_extract_timestamp` on two sequential values is monotonic
- [ ] Test asserts `VIRTUAL` is the kind chosen when neither keyword is given
      (`pg_attribute.attgenerated = 'v'`), and that the column reflects an update
      to its base columns on the next read
- [ ] Test pins the **exact** rejection set from `20-data-model.md` §10, matching
      on the error message: `CREATE INDEX`, `UNIQUE` and `PRIMARY KEY` on a
      virtual column all fail, `CREATE STATISTICS` fails, while `SET NOT NULL`,
      a referencing `CHECK`, and `sum()` all succeed. The first draft of this
      plan had two of these backwards; the test is what stops that recurring
- [ ] Test asserts `old.`/`new.` in `RETURNING` work **inside a data-modifying
      CTE** whose output feeds an `INSERT` (`00-overview.md` §4.4), not just in a
      bare statement
- [ ] Test asserts `ALTER TABLE … ADD CONSTRAINT c NOT NULL col NOT VALID`
      succeeds on a table containing a NULL, that `VALIDATE CONSTRAINT` then
      fails, and that it succeeds once the NULL is fixed
- [ ] Test asserts the same `NOT VALID` clause is **rejected** in `CREATE TABLE`,
      documenting why the migrations in `20-data-model.md` §13 declare `NOT NULL`
      inline instead
- [ ] Test asserts `unaccent()` is `STABLE` in **both** its 1-arg and 2-arg
      forms — the folklore that the 2-arg form is `IMMUTABLE` is false on 18.6 —
      and that a bare `unaccent(col)` in an index expression is rejected while
      `app.immutable_unaccent(col)` is accepted
- [ ] Each assertion carries a comment citing its PostgreSQL 18 docs section
      (§13.1)

---

### T-0.17 — Set up CI
**Depends on:** T-0.13, T-0.14, T-0.15
**Touches:** `.github/workflows/ci.yml`
**Size:** M
**Done when:**
- [ ] Jobs: web lint, web typecheck, web unit, web build, Go vet, Go test,
      migration up/down/up against a `postgres:18` service container, and the
      T-0.8 codegen drift check
- [ ] CI fails on lint errors (§2)
- [ ] The workflow runs on PRs into `develop` and `main`, matching the T-0.5
      branch protections
- [ ] A failing job blocks merge; no `continue-on-error` anywhere
- [ ] `pnpm dev` / `pnpm test` / `pnpm build` all green — Phase 0 exit criterion
      met (§16)
