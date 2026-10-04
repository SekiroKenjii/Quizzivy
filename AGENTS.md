# Quizzivy — Agent Working Rules

Read this file at the start of every session. Read `docs/quizzivy-spec-v0.3.md`
before touching any area you have not worked on before, and
`docs/plan/00-overview.md` before touching architecture.

## Redesign in progress (Phase R)

The product is being rebuilt to a new design deck, release by release
(`docs/plan/70-redesign-overview.md`). Until the programme ends, some rules in
this file describe the code as it is and name the release that changes them.

- **In effect now:** `docs/design/deck/` is the single source of truth for the
  UI. The old mockups and their board ids (A-, F-, G-, S-, E-, B-) are gone; do
  not cite them. Deliberate departures from the deck are in
  `docs/design/gaps.md`.
- **Since R1** (v0.7.0) the deck's tokens, Be Vietnam Pro and the primitives
  are in. Deck geometry that would change an existing primitive (control
  heights, radii, badge and card shapes) applies only inside
  `data-scale="deck"`, which a rebuilt surface sets on its root. The student
  app and the take-test engine are such surfaces since R3 (v0.9.0) and follow
  the theme. The teacher console keeps its layout and forces light
  (`useForcedLightTheme`) until R4 rebuilds it. R5 makes the deck geometry the
  default and removes `data-scale`
  (T-R5.30). `web/public/boot.js` applies the theme and language before paint,
  since the CSP allows no inline script. Do not restyle an old console's screen
  ad hoc: its release rebuilds it.
- **Since R2** (v0.8.0) permissions, not paths, gate the API, repositories
  scope rows to their owner, and teaching operations are under `/teacher/*`
  ("Authentication and authorization" below). The v0.7.0 `/admin/*` API paths
  answer through an alias until v0.9.1 (T-R3.1 to T-R3.3).
- **v0.9.1** (T-R3.1 to T-R3.3, no earlier than 2026-10-10) removes that alias,
  drops the legacy `users.role` column with its sync trigger, and validates the
  owner constraints and drops their fill triggers. Until then all of them are
  in the code.
- **R4** moves the teacher web routes from `/admin/*` to `/teacher/*`.
- **The design team's fourth export (2026-10-04) is not the deck of record.**
  R4 builds from `docs/design/deck/` as it is, and D4 imports the export after
  v0.10.0 (`docs/plan/74d-d4-deck-update.md`).

## Sources of truth, in order

1. `docs/quizzivy-spec-v0.3.md` — product and engineering spec
2. `docs/plan/` — the implementation plan derived from it
   (`docs/design/deck/` is the UI; spec §12 records the rules drawn from it,
   and `docs/design/gaps.md` the decided departures)
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

## Code style

**Comments.** The code says what it does; a comment says only what the code
cannot: the contract of an exported identifier, in one paragraph, and a
package comment naming the context's model. No comment inside a function
body. No doc comment on an unexported identifier. Directives (`//go:`,
`//nolint`) are not comments. The reasoning behind a decision belongs in the
commit message or in `docs/plan/`. This applies to the frontend as well.

- **No comment on the `package` declaration** except the package comment.
- Doc comments follow Google's style: start with the identifier's name, state
  what it is and any non-obvious contract, and stop.
- **Never use an identifier marked `Deprecated`.** `make lint` runs staticcheck,
  whose SA1019 fails the build on one.

**Go.** Google's Go Style Guide, strictly. `gofmt`, `go vet` and `staticcheck`
all have to be clean — `make lint` runs all three.

**React.** Vercel's react-best-practices, strictly:
<https://github.com/vercel-labs/agent-skills/tree/main/skills/react-best-practices>
The ones this codebase keeps hitting are narrow effect dependencies (depend on
primitives and stable callbacks, not objects), subscribing to derived state
rather than raw objects, and Set/Map for repeated lookups in a keystroke path.
`web/src/components/ui/**` is vendored shadcn and is exempt.

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

`server/internal/platform/db/tests/pg18_test.go` pins all four. If it fails, the docs changed
and the plan needs revisiting.

## The composition root

`cmd/api/main.go` builds a logger, calls `core.Run`, sets the exit code.
`internal/core` is the composition root and the only package that knows every
module:

| package | holds |
|---|---|
| `core` (`core.go`) | `App`: config, signals, lifecycle, `Handler()`, `Serve()` |
| `core/wiring` | `Build`: one file per module, repository → `Application` → transport, in dependency order, starting with `access.go`, which refuses a database whose `app.permissions` lacks a key this binary knows; returns the `Assembly` (transports, the access application as `Principals`, token issuer, identity application) |
| `core/adapters` | platform clients behind module ports (`Google`, `AudioProbe`), one module's handlers behind another's port (`Media`, `MediaKinds`), and `Principals`: the access module's `ResolvePrincipal` as `httpx.PrincipalResolver` |
| `core/router` | `Deps`, whose `Principals` resolves who a request acts as for the permission and docs gates (`New` refuses a nil one), `Modules`, the `Server` composite embedding every module's `http` type, `New` (middleware order, `/livez`, `/healthz`, `/docs`, the `/admin` alias until v0.9.1 (T-R3.1 to T-R3.3)), `RateLimits` for contract operations and `ServiceRateLimits` for the routes beside it |
| `core/jobs` | background commands (`PruneRefreshTokens`) |
| `platform/httpserver` | the HTTP server, its timeouts and graceful shutdown |

A new module is wired in `wiring/<module>.go`. An optional dependency stays a
nil interface (see `adapters/media.go`) so its operations answer 501, never a
nil-pointer 500. A cross-module need is a port typed as the other module's
handler (`cqrs.CommandHandler[…]`/`cqrs.QueryHandler[…]`) where one operation
is enough, and an adapter in `core/adapters` where several are. The dependency
rules between `domain`, `application`, `repositories`, `http`, `platform`,
`shared` and `core` are in `docs/plan/60-backend-architecture.md` and enforced
by `core/tests/architecture_test.go`.

## Repository map

```
api/openapi.yaml   the contract — edit this first, then `make gen`
web/               quizzivy-web
  src/             spec §3 layout, unchanged -- source only, no tests
  tests/           units/ integration/ e2e/ support/ -- see web/tests/README.md
server/            Go module `quizzivy`: a modular monolith
  internal/core/     composition root: wiring/ router/ adapters/ jobs/
  internal/platform/ technical adapters (db context + repository base, storage, google, probe, httpx, httpserver, apidocs, ...)
  internal/shared/   kernel: cqrs, actor, paging, audit, stats, opt, validation, content,
                     access (permission keys, Principal, Scope, CanActOn), visibility (who a teacher reaches),
                     answered (when a saved answer counts as answered)
  internal/modules/  one directory per bounded context, four layers each:
                     domain/ application/{command,query,ports,model} repositories/ http/, tests in <layer>/tests/;
                     access (roles, grants, the principal cache) has no http/ until R5
  tests/             end-to-end tests (build tag e2e)
  gen/openapi/       generated, committed, never hand-edited
migrations/        goose, forward-only, 00001…
seed/              seed data — never in a migration
docs/plan/         the plan; 20-data-model.md is the schema authority; 70–81 are Phase R
docs/design/deck/  the design deck, pinned by MANIFEST.sha256 (scripts/check-design-deck.mjs)
```

A vertical slice is one `web/src/features/<name>/`, one
`server/internal/modules/<name>/`, and one section of `api/openapi.yaml`.

**Tests use the public surface.** Every Go test file lives in a `tests/`
directory under its layer as an external package; a test that needs a
private identifier is testing the wrong thing. Database-backed tests carry
the `integration` tag, end-to-end tests the `e2e` tag; `make test-api` runs
unit, integration and end-to-end in that order.

## Authentication and authorization

- **Every operation that requires a bearer token declares `x-permission`** in
  `api/openapi.yaml`, and `httpx.RequirePermission` enforces it on every
  request; the path is not the gate. The server refuses to start when an
  operation declares none, or a key its path's tree does not take
  (`httpx.PermissionRequirements`), and `permissions_test.go` pins the whole
  map in `testdata/permissions.golden`. The catalogue, the pseudo-keys and the
  guards that are not permissions (the subset rule, strict student targets,
  the last admin, sign-in lockout) are in `docs/plan/70-redesign-overview.md`
  §4.
- **A permission says what a caller may do; `access.Scope` says whose rows.**
  Repositories take an `access.Scope`, which only `scope.all` widens. Another
  teacher's id answers exactly as a missing one does: 404, or for a reference
  in a body the error an unknown id gets.
- **Student targets go through `app.student_like_roles` and nothing else.** No
  query reads `users.role` to decide who is a student, so an Admin with "Take
  tests" turned on is never a student target.
- **The subset rule lives in `access.CanActOn`.** Call it; do not restate it.
- **A new `/teacher/*`, `/app/*` or `/me/*` operation needs an entry in the
  isolation suite** (`server/tests/isolation_cases_test.go`), and its
  `x-resource` names a kind for every uuid it takes; a `/teacher/*` list also
  declares `x-resource-list`. `TestAnotherTeachersIdsAnswerAsMissingOnes`
  fails naming an operation the table lacks, and `resource_contract_test.go`
  a uuid without a kind.
- **The v0.7.0 `/admin/*` teaching paths answer until v0.9.1 (T-R3.1 to
  T-R3.3).** The router
  rewrites each to its new path (`LegacyAdminPaths`: a `/teacher/*` path, and
  `DELETE /admin/students/{id}` to `DELETE /admin/users/{id}`) and logs
  `legacy_admin_path`. A new operation never joins that table.
- **Everything the contract does not explicitly open requires a bearer token**,
  derived from `api/openapi.yaml`'s `security`. Seven operations are open —
  login, Google sign-in, refresh, logout, `POST /join/preview`, the integrity
  beacon `POST /app/attempts/{id}/events` and `GET /public/status` — and the
  list is pinned by `theOpenSeven` in `core/router/tests/auth_middleware_test.go`,
  so an eighth takes an argument.

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
  to RUN. `TestAuthenticationIsDecidedBeforeValidation` (`validate_test.go`) and
  `TestBodyLimitPrecedesJSONValidation` (`hardening_test.go`) pin the direction.
- **The maintenance gate is outside that list.** `httpx.Maintenance` wraps the
  mux inside CORS, `CORS(Maintenance(mux))`, so it runs before rate limiting and
  authentication: during a window every route answers `503 MAINTENANCE`, an
  expired token included, and only `GET`/`HEAD` `/livez`, `/healthz` and
  `/public/status` pass. It runs only when the path matches a route under some
  method (`routedOnly` in `router.go`): a path no route serves gets the
  envelope's `404 NOT_FOUND` without a read of the window snapshot, so scanners
  cannot wake Neon through it. Inside the gate, `servedMethodOnly` answers a
  known path under a method it does not serve with `405 METHOD_NOT_ALLOWED` and
  `Allow`, and serves `HEAD` on an open `GET` as that `GET` without a body;
  `HEAD` on a `GET` that needs a token is the same 405.
  `maintenance_gate_test.go` pins its position and `unrouted_test.go` these
  answers.
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

Four local-dev facts worth not rediscovering:

- **The `postgres:18` image changed its data layout.** The volume mounts at
  `/var/lib/postgresql`, and the image puts data in a version subdirectory
  beneath it. Mounting at `/var/lib/postgresql/data` — correct for 17 and
  earlier — makes the container refuse to start.
- **Roles are created by `docker/initdb/`, not by a migration.** `CREATE ROLE`
  needs superuser and `quizzivy_migrate` deliberately is not one. `pg_trgm` and
  `unaccent` are *trusted* extensions, so the migrate role can install them
  itself — verified.
- **The API refuses to start without `JOIN_CODE_KEY`** in `.env`: standard
  base64 of exactly 32 random bytes, generated with `openssl rand -base64 32`.
  A code sealed under a key the server no longer holds can be neither read nor
  redeemed. `JOIN_CODE_KEY_PREVIOUS` is set only while rotating
  (`docs/setup/operations.md`).
- **`make test-api`'s end-to-end tier needs MinIO** (`make up`): the isolation
  suite uploads real images, audio and Word sources, through the compose
  endpoint and buckets unless `S3_*` and `IMPORT_S3_BUCKET` say otherwise.

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
  changing a table — every deviation it lists from the spec sketch is
  deliberate.

## Tests

`web/tests/` sits beside `web/src/`, split by cost: `units/` (fast, no build, no
browser), `integration/` (several real layers at once), `e2e/` (Playwright
against a production build), `support/` (harness, not tests). `web/tests/README.md`
has the placement rule. `@/` is `src/`, `@tests/` is `tests/`.

Go tests stay beside the code they cover, as is idiomatic.

**A database test that commits rows owns their removal, and must not disturb
anyone else's.** Most integration tests commit through a pool and delete their
rows in `t.Cleanup`; a few roll back one transaction, the better choice when a
test needs only one (a race, or a start against a schedule, needs two and has
to commit). A test that commits:

- registers `t.Cleanup` before its first insert, gives each cleanup statement
  only the arguments it uses (pgx refuses one with unused ones), and fails the
  test on a cleanup error rather than dropping it (`_, _ =` leaked a whole run's
  fixtures once);
- keeps its rows out of global views while they exist: packages run
  concurrently, and a row started in the future sorts first in every
  "recent" list another test reads.

Verify on a database created for the purpose, as CI does, not on the dev
database.

**One E2E suite talks to a real API: `*.live.spec.ts`, the `live` Playwright
project, run with `pnpm e2e:live`.** Everything else is stubbed on purpose --
those tests are about what the browser does, and the server's behaviour is
covered by Go tests against a real Postgres. The live suite exists for claims
that are about the two halves meeting: E2E 1a uploads a real mp3 so §11.1's
sniff, size check and duration probe run rather than being mocked. Adding a case
there needs a reason of that kind; the default is `pnpm e2e`.

It needs `make up`, `make migrate`, `make seed`, and the API on :8080 with
`http://localhost:4173` in `CORS_ALLOWED_ORIGINS`. Kill any leftover `vite
preview` before re-running: `reuseExistingServer` is on outside CI, so
Playwright reuses it and serves the build that server started with -- which
looks exactly like your fix not working.

## CI

`docs/setup/ci.md` describes it. What to keep in mind while working:

- **A job is skipped only when the same files passed it and have not failed
  it since.** `scripts/ci/plan.mjs` names the set of files each job reads and
  hashes it; a job records its outcome for that hash, and Plan skips the job
  while the newest record is a pass. So a change under `docs/plan/` normally
  runs no code job, and a server-only change no web job.
- **Each job checks out only its set.** A test that reads a file outside its
  own tree (the web join-code test reads `joincode.go`; the isolation suite
  reads an mp3 under `web/tests/e2e/fixtures/`) needs that path in the set's
  `include`, in the same PR. Never make a test skip when a file is missing.
- **"Re-run all jobs" runs everything**, and so does a push to `main`. Use the
  first when a skipped job should be run again.
- **The required check is CI result**, not the individual jobs, and only a
  pull request's run reports it. A new job goes in `JOBS` and in that job's
  `needs`, or Plan's own test fails.

## Git workflow

Gitflow. `main` is released only and tagged; `develop` is integration.

- One task from a phase file = one branch = one PR: `feature/t-<phase>-<n>-<slug>`
  off `develop`.
- Phase R tasks are `T-R<k>.<n>` on `feature/t-r<k>-<nn>-<slug>`, off the
  release's integration branch `work/redesign-r<k>`, which is cut from `develop`
  and merged back `--no-ff` when the release is complete. D4 runs the same way
  under its own names: `T-D4.<n>` on `feature/t-d4-<nn>-<slug>`, off
  `work/deck-d4`.
- A release is `release/<version>` → `main` → back-merge to `develop`. Merging to
  `main` deploys to production.
- `hotfix/<slug>` off `main`, merged to both.
- Never commit directly to `main`, `develop` or a `work/**` branch. The branch
  rules refuse it: a change reaches them by pull request, merged with a merge
  commit, and `develop` and `work/**` also need **CI result** to pass
  (`docs/setup/ci.md`, "Branch rules").

**A behavioural change never rides along in a formatting sweep.** A commit that
reformats or restrips comments across many files must contain nothing else — the
diff a reviewer opens is thousands of lines, and a real change inside it is
invisible however carefully the commit message names it. Put it in its own
commit on the same branch, so the diff for it is the size of the change.

## Design

Build the UI from the deck in `docs/design/deck/`, and follow spec §12, which
records the rules drawn from it. Read a page's script, not only its markup: the
option sets, labels, defaults and breakpoints live there. The prototype chrome
never ships (the screen-switcher pills, demo accounts, "Try" hints, the canvas
theme buttons, the "Coming next" screen). Where the deck and a decision
disagree, `docs/design/gaps.md` says which wins and what to build meanwhile.

Colours come from CSS variables / Tailwind tokens, never hard-coded. The deck's
palette is a teal-grey neutral scale, a charcoal primary, the lime brand accent
(`--accent-c`, with soft and ink tones) for progress, counts and current
states, and semantic success, warning, danger and info tones. Dark mode is in
scope from R1: components read tokens and never branch on the theme. The
exceptions to "no hard-coded colour" are provider marks that identify the
provider: `GoogleMark` on a button that hands the user to Google, and the
deck's Google G that marks a Google sign-in in the admin Users table. Do not
add another without the same argument.

`tests/units/styles/no-raw-colours.test.ts` refuses a Tailwind palette class,
or a literal hex, `rgb()`/`rgba()` or `oklch()` colour, in every `.ts`/`.tsx`
file under `src/` except `GoogleMark.tsx`; the literal values behind the tokens
live in `src/index.css`, which it does not scan.

Every control has a visible `:focus-visible` ring, although the deck draws none.
Its colour is `--focus`, which `tokens.test.ts` holds at 3:1 or more on `--bg`,
`--card`, `--sidebar` and `--muted` in both themes.
Continuous motion (the marquee on overflowing titles, the live dot) pauses on
hover and focus and is static under `prefers-reduced-motion`.

Integrity UI follows the deck (decided 2026-09-26). On the student side: the
red timer in the last five minutes, and "Your teacher has been told" once a
student is past the allowance under `flag` or `auto_submit`; a `warn` policy
never names the teacher. Flags and "Flagged" in the teacher's roster follow in
R4 (T-R4.46). The teacher judges; the app reports —
the product never concludes that a student cheated.

The student app lives in `StudentLayout`, whose one outlet stays mounted at
every width, so forms survive resizing. The shell branches at 768px in code
(`useMediaQuery("(min-width: 768px)")` and `min-[768px]:`, never `md:`): the
destinations sit in the top bar from 768 and in a bottom tab bar below it. A
detail route declares `handle.detail`: below 768 the header swaps the logo for
a back arrow and the title and the tab bar hides, and from 768 Intro and
Result draw their own back link. Each page is one centred column with its own
maximum width: Home and Classes 960px, Test intro 720px, Result 820px,
Settings 760px. A new threshold inside a page that is not the shell's 768 is a
container query on the container named `student`, which the roots of both
student layouts declare; nothing queries it yet. The thresholds that exist
beside 768 are not on it: `min-[360px]:` on the Test intro's facts strip, `lg:`
(1024px, where the 44px floor ends) on the engine's blanks and answer field and
on the audio player's seek track, and the load-error card's own container,
`load-error`. `.student-surface`, on the
shell's `<main>` and on `FocusLayout`, puts a 44px floor on buttons below
1024px; a control the deck draws smaller opts out in its own classes. No `/app`
route loads `SideColumn`, directly or through `PageAside`
(`tests/units/student/side-column.test.ts`), and none imports
`useColumnWidth`. `DeckDialog`
(`components/shared/`) is the frame of the student's dialogs; the "You left
the test" alert (`StrikeDialog`, 420px) and the question sheet draw their own
on the dialog primitive.

The take-test engine runs in `FocusLayout` and branches at the same 768px.
From 768, a question whose group has something to read splits into a passage
pane and a question pane, and the question pane's footer is Previous, a strip
of numbered squares and Next. Below 768 a switcher shows one pane at a time,
and the footer's count button opens the squares in a bottom sheet. There is no
navigator column and no width preference. The timer draws each digit in a cell
one zero wide, because Be Vietnam Pro has no tabular figures. Unit tests of a
phone board pin `viewport("phone")` (`tests/support/viewport.ts`); jsdom
answers "wide" by default.

A paper's questions are dealt inside their section: `DealManager.Present`
keeps section order and shuffles within each, so the strip can leave a gap
between parts and the sheet can head each one. A single-section paper deals
exactly as it did before sections reached the payload.

## Language

Vietnamese first. Write the `vi` string, then `en`. No English-only
user-facing text ever reaches a commit. Code, comments, commit messages,
and docs are English. The deck is English except the Landing and Splash
pages: its English is the `en` string, and the `vi` string is ours to write.

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
- [ ] Screens compared with the deck at 360, 768, 1024, 1280 and 1440, light
      and dark, in the browser — not from code
- [ ] Every new or changed operation that requires the bearer token declares
      `x-permission` (an open one declares none), and a `/teacher/*`,
      `/app/*` or `/me/*` one has its isolation-suite entry

## High-risk areas — extra care

`features/take-test/`, `features/integrity/`, `features/media/`,
`server/internal/modules/access/`, and anything touching `attempts`,
`test_versions`, `users.role_id` or an owner column (`owner_id` on tests,
questions, question groups and media assets; `classes.teacher_id`;
`users.created_by`). Run the relevant unit tests before and after every change
to these. Do not refactor them opportunistically while doing something else.

**Access and ownership.** A write that ends someone's access revokes their
refresh families, bumps `session_epoch` and calls `Principals.Forget` in one
command (`docs/plan/70-redesign-overview.md` §4.2). Every insert names its
owner itself: the `BEFORE INSERT` fill triggers exist only for the v0.7.0
binary, and v0.9.1 (T-R3.1 to T-R3.3) drops them. A user write sets `role_id`,
never `role`: 00056's trigger derives `role` until v0.9.1 (T-R3.1 to T-R3.3)
drops the column. The `users_last_admin` trigger refuses any change that leaves
no active Admin.

**Soft delete and the reference check are two tables, so the lock must be taken
on both sides.** `SoftDelete` locks the row it is deleting and then counts
references in another table — and a `FOR UPDATE` on one table does not block an
`INSERT` into a different one, so on its own that only orders concurrent
deletes. The foreign key does not help either: `ON DELETE RESTRICT` fires on a
real `DELETE`, not a soft one.

Anything inserting into `app.test_section_questions` must call
`questions.LockForDraftUse` first, so both operations contend on the same row.
`TestLockForDraftUseSerialisesAgainstDelete` drives both halves concurrently and
fails within two attempts without it.

The same applies to `app.media_assets`: anything inserting into
`app.test_version_questions` must call `media.LockForVersionUse` first, or a
publish can freeze a reference to an asset a concurrent delete is removing.
`TestLockForVersionUseSerialisesAgainstDelete` covers it.

Five tests are canaries. If one starts failing, something load-bearing broke —
fix the cause, never the test:

- `server/internal/modules/tests/application/tests/publish_snapshot_test.go` —
  editing a bank question after publish must not change the published version.
  Without this, versioning is decorative.
- `web/tests/units/media/audio-player.test.tsx` — `.play()` must be called in
  the same synchronous tick as the click. Any `await` before it breaks iOS
  Safari silently.
- `TestTheSameClientSeqFromTwoSessionsBothPersist` in
  `server/internal/modules/attempts/application/tests/events_test.go` — the same
  `client_seq` from two `session_id`s must both persist. This is what stops a
  resumed attempt's timeline vanishing.
- `web/tests/units/api/client.refresh.test.ts` — five concurrent 401s must issue
  exactly one refresh.
- `web/tests/integration/router-chunks.test.ts` — the admin tree must stay out of
  the entry chunk. It runs a real build; reading the router and trusting `lazy`
  would not catch the regression that actually happens.

`docs/plan/30-risks.md` explains what each one is guarding.

**A debounced autosave owes the user two flushes it will not do by itself.**
`useAutosave` holds an edit for 1.5s. Unmounting has to save what is pending --
in the builder, "type, then click the next question" swaps the editor inside
that window, and clearing the timer loses the edit silently while the indicator
still reports the previous save. Publishing has to flush every autosave on the
screen first, because a version snapshots what is SAVED. Both were real bugs
that shipped past every unit test and were caught by E2E 1a; `builder/
autosave-unmount.test.tsx` pins the first.

## When you are unsure

Ask one precise question. Do not guess on §5 (auth), §6 (join codes),
§10 (integrity), §11 (audio), or §13 (data model). Guessing in these
areas is more expensive than waiting for an answer.

Check `docs/plan/40-open-items.md` first — the question may already be there with
a stated default, in which case build the default and move on.

## Keeping documents current

When a decision changes, edit the spec section and bump its version at the top
(§18), then correct the affected plan file. A plan that disagrees with what was
built is worse than no plan. In Phase R, the spec, this file and the plan
change in the release that changes the behaviour, never batched at the end.
