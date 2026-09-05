# Backend architecture — modular monolith with DDD layers

Decided 2026-09-06 after the owner's IT review of the Go source. Supersedes
the "one package per feature" layout AGENTS.md described until then.

## Shape

- **One process, one database, eight bounded contexts** under
  `server/internal/modules/`: identity, classes, questions, media, tests,
  assignments, attempts, dashboard. Each is one aggregate (dashboard is read
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
  as a package value. `Service` is the application layer's word.
- **Ports at the application boundary.** identity declares `GoogleProvider`;
  media declares `AudioProbe` and `ObjectStore`; questions declares
  `MediaKinds`; classes and identity read student figures through
  `shared/stats.Source`, which attempts implements; tests' repository takes
  `QuestionLocks` and `MediaLocks` for the row locks another module holds
  inside its transaction. core adapts.
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
| `internal/api` (one Server, every handler) | each module's `http/`; core's `composite.go` embeds them |
| `internal/auth`, `internal/students` | `modules/identity` |
| `internal/join`, `internal/classes` | `modules/classes` (join code as a value object with a manager) |
| `internal/tests`, `internal/tests/publish` | `modules/tests` (publishing is one repository transaction handed the domain's validation) |
| `internal/attempts`, `grading`, `review`, `integrity` | `modules/attempts` |
| `internal/{db,storage,config,httpx,ratelimit}`, `auth/google`, `media/probe` | `platform/*` |
| `internal/{paging,audit}` | `shared/*`, plus `shared/stats` |

## Why

The reviewers named the costs: with SQL, rules and transport in one package
per feature, every change touched every concern, and nothing said which
module owned which table. The layout above makes ownership a directory, the
rules a named type, the boundaries a test, and the tiers of confidence a
build tag — which is what a maintainer inherits when the current team is
gone.
