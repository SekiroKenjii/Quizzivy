# Backend architecture — modular monolith with DDD layers

Decided 2026-09-06 after the owner's IT review of the Go source. Supersedes
the "one package per feature" layout AGENTS.md described until then.

## Shape

- **One process, one database, eleven bounded contexts** under
  `server/internal/modules/`: identity, classes, questions, media, tests,
  assignments, attempts, dashboard, imports, availability, access. Each is one
  aggregate (dashboard is read models only; availability reads the maintenance
  windows the operator writes; access reads roles and grants and resolves who a
  request acts as, with no transport of its own until R5's Roles page) and
  exposes its operations through its own `http/` package, which core embeds
  into the generated strict server.
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
  `shared/stats.Source`, which attempts implements, over only the
  assignments the reader reaches; tests' repository takes
  `QuestionLocks` and `MediaLocks` for the row locks another module holds
  inside its transaction, and for the ownership and readability checks that
  run under those locks (`NotOwnedBy`, `RequireReadable`). A port one operation fills is typed as the other
  module's handler (identity's `SelfEnroller` is the classes `EnrolNewMember`
  command; attempts' `Students` is the identity `StudentAccount` query); the
  rest `core/adapters` adapts.
- **Reach rules in the kernel.** `shared/visibility` holds the rules every
  teaching read shares, each defined once as SQL:
  - `TaughtClassIDs`: the classes a teacher teaches;
  - `TaughtMemberIDs`: the accounts in those classes;
  - `AuthoredAssignmentIDs`: the assignments a teacher created;
  - `StudentIDs`: the students a teacher reaches;
  - `AssignmentIDs`: the assignments a teacher reaches;
  - `Papers`: which attempts a list of papers shows.

  identity, classes, assignments, attempts and dashboard splice them into
  their own statements, and callers lift them under `scope.all`. Like
  `shared/audit`, it is SQL in the kernel, because a port cannot splice a
  predicate into another module's list or aggregate.
- **The answered rule in the kernel.** `shared/answered.SaysSomething` is the
  one SQL definition of a saved answer that says something. assignments (the
  student's card) and attempts (the teacher's monitor) splice it, so the two
  counts cannot drift.
- **Access in the kernel.** `shared/access` is the authorization model every
  layer may use, and it imports only the standard library: the catalogue as
  `Key`s, with the pseudo-keys `self`, `workspace.teacher` and
  `workspace.admin`; a role's grants as a `Set`; the any-of `Requirement` an
  operation declares in `x-permission`; `Workspaces`; the `Principal` a
  request acts as and the `Scope` it reaches (`All` only for `scope.all`;
  `Scope.Own` clears it, and the teacher workspace's six content lists pass
  that, T-R4.54);
  `IsStudentLike`, which is `app.student_like_roles` in Go; and `CanActOn`,
  the subset rule. The access module fills it: `core/adapters.Principals`
  puts its `ResolvePrincipal` behind `httpx.RequirePermission` and the docs
  gate, and identity's `Principals` port is the access application
  (`Resolve`, `Forget`). `shared/actor.Actor` carries the `Scope`, and every
  scoped repository filters by it (plan 70 §4.2).
- **One database context, one repository base.** `platform/db.Context` wraps
  a pool or a transaction (`Exec`, `Query`, `QueryRow`, `InTx`); every module
  Postgres type embeds `db.Repository` and uses the generic `QueryOne`,
  `QueryMany`, `Count`, `Exists`, `IsUniqueViolation`, `EscapeLike`. Optional
  values go through `shared/opt`, field errors through `shared/validation`.
- **core is five packages.** `core/wiring` builds each module (one file per
  module, in dependency order) and returns the `Assembly`; `core/router`
  fronts the transports with the generated strict server, the middleware
  order, `/livez`, `/healthz`, `/docs` and the rate limits; `core/adapters` translates
  platform errors into domain errors and one module's handlers into another's
  port; `core/jobs` runs background commands; `core/maintenance` holds the
  operator's commands that `cmd/maintenance` runs outside the API
  (retention, anonymisation, maintenance windows and, from R2,
  `rekey-join-codes`). `platform/httpserver` owns the listener and its
  shutdown.
- **Tests in three tiers** (unit, integration, e2e — see server/README.md),
  every test file in a `<layer>/tests/` directory as an external package,
  none reaching a private identifier.
- **Self-served API reference**: `/docs` is our page loading Scalar's pinned,
  SRI-checked bundle against `/docs/openapi.json`, served by the API from the
  contract it was generated from, behind an admin docs session (spec §5.5).
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
