# Backend

Go module `quizzivy`: a modular monolith. One process, one database, eight
bounded contexts that talk to each other only through the ports they declare.
The decision record is `docs/plan/60-backend-architecture.md`; this file is the
map.

```
server/
  cmd/            api (the server), migrate, seedadmin — entry points only
  internal/
    core/         the composition root: wiring/ builds each module, router/ fronts them,
                  adapters/ bridge ports, jobs/ run background commands
    platform/     technical adapters: config, db (pgx; the Context and Repository every module embeds),
                  storage (S3), google (OIDC), probe (audio), httpx (middleware), httpserver,
                  ratelimit, httpapi (transport helpers), apidocs (Scalar)
    shared/       the kernel every layer may use: cqrs (handler contracts), actor, paging, audit,
                  stats, opt, validation
    modules/      the bounded contexts
      identity/    User aggregate, sessions, the teacher's roster of students, PasswordManager
      classes/     Class aggregate, members, join codes (JoinCodeManager, CodeState)
      questions/   Question aggregate with options and blanks, QuestionManager
      media/       Asset aggregate in object storage, AssetManager
      tests/       Test aggregate, drafts and published versions, PublishManager
      assignments/ Assignment aggregate, policies, ScheduleManager
      attempts/    Attempt aggregate: the deal, answers, grading, review, monitor, timeline
      dashboard/   read models for the teacher's home (no aggregate)
  tests/          end-to-end: the whole application in-process, over HTTP
  gen/openapi/    generated from api/openapi.yaml, committed, never hand-edited
```

## Layers of a module

| layer | holds | may import |
|---|---|---|
| `domain/` | aggregate root, entities, value objects, commands, errors, the repository interface, and the **managers** — the domain's rules that belong to no single entity | `shared`, other modules' `domain` |
| `application/` | the use cases as **commands and queries**: `command/` and `query/` hold one struct and one handler per use case, `ports/` what the module needs from outside, `model/` the results other modules read, `internal/support/` what the handlers share; `app.go` builds `Application{Commands, Queries}` | own `domain`, other modules' `domain` and `application`, `shared` |
| `repositories/` | Postgres, implementing the domain interface; SQL lives here and nowhere else | own `domain`, `platform`, `shared` |
| `http/` | the operations this module serves, embedded into core's composite strict server; presenters other transports reuse | own `domain`/`application`, `platform/httpx`, `platform/httpapi`, `gen/openapi`, other modules' `domain`, `application` and `http` |

`core/tests/architecture_test.go` fails the build on any import in the wrong
direction. Managers are reached as package values (`domain.Grading.Grade`,
`domain.JoinCodes.Normalize`); applications are constructed by `core/wiring`
with their repository and ports, and a transport reaches a use case as
`h.app.Commands.Create.Handle(ctx, command.Create{…})` or
`h.app.Queries.Get.Handle(ctx, query.Get{…})`. A port another module fills with
one operation is typed as that module's handler (identity's `SelfEnroller` is
the classes `EnrolNewMember` command; the attempts transport's `Students` is the
identity `GetStudent` query).

## Tests

Three tiers, three build tags, one directory convention: a layer's tests live
in `<layer>/tests/` as an external package and use only what the layer
exports. A test that needs a private function is a test of the wrong thing.

| tier | tag | where | needs | runs with |
|---|---|---|---|---|
| unit | none | `<layer>/tests/` | nothing | `make test-api-unit` |
| integration | `integration` | `repositories/tests/`, `application/tests/` | `TEST_DATABASE_URL` | `make test-api-integration` |
| end-to-end | `e2e` | `server/tests/` | `TEST_DATABASE_URL` | `make test-api-e2e` |

`make test-api` runs all three in that order; CI runs them as three steps.

## API reference

The server serves its own Scalar reference at `/docs`, loading the contract it
was built from at `/docs/openapi.json`. The page is `platform/apidocs`; no
package wraps it, and the Scalar version is pinned there.
