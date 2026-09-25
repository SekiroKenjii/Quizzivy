# Backend architecture — modular monolith with DDD layers

Decided 2026-09-06 after the owner's IT review of the Go source. Supersedes
the "one package per feature" layout AGENTS.md described until then.

## Shape

- **One process, one database, nine bounded contexts** under
  `server/internal/modules/`: identity, classes, questions, media, tests,
  assignments, attempts, dashboard, imports. Each is one aggregate (dashboard is read
  models only) and exposes its operations through its own `http/` package,
  which core embeds into the generated strict server.
- **Four layers per module** — `domain/`, `application/`, `repositories/`,
  `http/` — with the dependency direction pinned by
  `core/tests/architecture_test.go`. `platform/` holds technical adapters and
  knows no module; `shared/` is the kernel and depends on nothing above it;
  `core/` is the composition root and the only place that knows everything.
- **Managers, not services, in the domain.** A domain rule that belongs to no
  single entity is a `*Manager` (GradingManager, DealManager,
  TimelineManager, InterventionManager, PublishManager, ScheduleManager,
  JoinCodeManager, PasswordManager, QuestionManager, AssetManager), reached
  as a package value. The application layer speaks in commands and queries.
- **CQRS in the application layer** (second review, 2026-09-06). Every
  module's `application/` is `Application{Commands, Queries}` built by `New`:
  `command/` and `query/` hold one struct and one handler per use case
  (`command.Create` / `command.CreateHandler.Handle(ctx, cmd)`), typed by the
  shared contracts in `shared/cqrs` (`CommandHandler[C, R]`,
  `QueryHandler[Q, R]`, `HandlerFunc` for stubs, `Nothing` for commands that
  only succeed or fail, `Err` for callers that keep only the error). `ports/`
  is what the module needs from outside, `model/` the results other modules
  read, `internal/support/` the ports, clock and helpers the handlers share.
  A command that acts for a signed-in user carries `shared/actor.Actor`.
  A query never writes: the attempts monitor screen runs the `ExpireDue`
  command and then the `Monitor` query.
- **Ports at the application boundary.** identity declares `GoogleProvider`;
  media declares `AudioProbe` and `ObjectStore`; questions declares
  `MediaKinds`; classes and identity read student figures through
  `shared/stats.Source`, which attempts implements; tests' repository takes
  `QuestionLocks` and `MediaLocks` for the row locks another module holds
  inside its transaction. A port one operation fills is typed as the other
  module's handler (identity's `SelfEnroller` is the classes `EnrolNewMember`
  command; attempts' `Students` is the identity `GetStudent` query); the
  rest `core/adapters` adapts.
- **One database context, one repository base.** `platform/db.Context` wraps
  a pool or a transaction (`Exec`, `Query`, `QueryRow`, `InTx`); every module
  Postgres type embeds `db.Repository` and uses the generic `QueryOne`,
  `QueryMany`, `Count`, `Exists`, `IsUniqueViolation`, `EscapeLike`. Optional
  values go through `shared/opt`, field errors through `shared/validation`.
- **core is four packages.** `core/wiring` builds each module (one file per
  module, in dependency order) and returns the `Assembly`; `core/router`
  fronts the transports with the generated strict server, the middleware
  order, `/livez`, `/healthz`, `/docs` and the rate limits; `core/adapters` translates
  platform errors into domain errors and one module's handlers into another's
  port; `core/jobs` runs background commands. `platform/httpserver` owns the
  listener and its shutdown.
- **Tests in three tiers** (unit, integration, e2e — see server/README.md),
  every test file in a `<layer>/tests/` directory as an external package,
  none reaching a private identifier.
- **Self-served API reference**: `/docs` is our page loading Scalar's pinned
  bundle against `/docs/openapi.json`, served by the API from the contract
  it was generated from.
- **Comments state contracts, code states the rest**: no comment inside a
  function body, no doc comment on an unexported identifier, one paragraph
  on an exported one, and a package comment naming the context's model.

## What moved where

| before | after |
|---|---|
| `internal/api` (one Server, every handler) | each module's `http/`; `core/router/server.go` embeds them |
| `internal/auth`, `internal/students` | `modules/identity` |
| `internal/join`, `internal/classes` | `modules/classes` (join code as a value object with a manager) |
| `internal/tests`, `internal/tests/publish` | `modules/tests` (publishing is one repository transaction handed the domain's validation) |
| `internal/attempts`, `grading`, `review`, `integrity` | `modules/attempts` |
| `internal/{db,storage,config,httpx,ratelimit}`, `auth/google`, `media/probe` | `platform/*` |
| `internal/{paging,audit}` | `shared/*`, plus `shared/stats` |
| `application.Service` methods (second review) | `application/command/*.go`, `application/query/*.go`, `application/{ports,model}`, `application/internal/support` |
| module-local `DB`/`Postgres`/`querier` types | `platform/db.Context` + `db.Repository`, embedded |
| `core/{modules,adapters,composite,router,server,jobs}.go` | `core/wiring`, `core/adapters`, `core/router`, `core/jobs`, `platform/httpserver` |

## Why

The reviewers named the costs: with SQL, rules and transport in one package
per feature, every change touched every concern, and nothing said which
module owned which table. The layout above makes ownership a directory, the
rules a named type, the boundaries a test, and the tiers of confidence a
build tag — which is what a maintainer inherits when the current team is
gone.
