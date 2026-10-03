# CI

`.github/workflows/ci.yml` runs on every pull request and on every push to
`main`, `develop` and `work/**`. One check, **CI result**, is required by the
branch rules; it passes only when every job did what the plan asked of it.

## What runs

| Job | Reads | Does |
|---|---|---|
| Plan | everything | Tests `scripts/ci/`, then decides which jobs run |
| Deck | `deck` | `scripts/check-design-deck.mjs` |
| Contract | `code` | `make gen-check`: lint the contract, regenerate, fail on drift |
| Server lint | `server` | vet, staticcheck, golangci-lint, gofmt |
| Server tests | `server` | unit, integration and end-to-end tiers; migrations up, down, up |
| Web checks | `web` | eslint, typecheck, prettier, integration tests, build |
| Web unit | `web` | the unit suite, in shards |
| E2E | `web` | Playwright against a production build, API stubbed |
| E2E (live API) | `code` | Playwright against the real API, Postgres and MinIO |
| CI result | — | Fails unless each job above did what the plan said |
| Sonar | everything | Pushes to `develop` only; not part of the result |

## How a job is skipped

A job is skipped only when there is proof that it already passed on the same
files. Nothing is skipped because of what a diff looks like.

1. `scripts/ci/plan.mjs` names four sets of files (`SETS`) and the set each job
   reads (`JOBS`). A set is a base (`all` or `none`) with directories and files
   included or excluded; the longest rule that matches a path decides.
2. Plan hashes the mode, blob and path of every file in a job's set. The
   workflow, `.github/actions/` and `scripts/ci/` are in every set, so a change
   to CI itself changes every hash.
3. A job's last step uploads an artifact named `ci-pass-<job>-<hash>`, kept for
   30 days. Plan skips a job when that artifact exists, has not expired and was
   uploaded by a run on a branch of this repository.
4. Each job checks out only its set (a sparse checkout built from the same
   rules). A test that reads a file outside its set fails with "no such file"
   instead of being skipped when that file changes.

So a pull request that changes only `docs/plan/` runs Plan and CI result. A
merge whose tree already passed as a pull request finishes in under a minute,
and a server-only change does not run the web jobs.

Two cases never use earlier proof:

- **A push to `main`.** The commit that deploys is tested in full.
- **"Re-run all jobs".** That is how to force a full run. "Re-run failed jobs"
  re-runs only what failed.

A marker expires after 30 days, so every job runs for real at least that often
on files that never change.

## When a job needs a file outside its set

Add the path to that set's `include` in `scripts/ci/plan.mjs`, in the same pull
request as the test that reads it. `scripts/ci/tests/plan.test.mjs` fails when
a rule no longer matches a tracked file, so a moved file is noticed.

Today's cross-tree reads:

| Reader | File |
|---|---|
| `web/tests/units/join/code.test.ts` | `server/internal/modules/classes/domain/joincode.go` |
| `server/tests/isolation_world_test.go` | `web/tests/e2e/fixtures/unit5-listening.mp3` |
| Server tests | `api/`, `migrations/`, `fly.toml`, `Dockerfile`, `.github/workflows/deploy.yml` |
| Web tests | `api/openapi.yaml`, `api/testdata/` |

A test must not treat a missing file as a reason to skip itself: in CI that
would hide the mistake this layout exists to show.

## Adding a job

Add it to `JOBS` with its set, give it the same `needs`, `if`, `env.JOB`,
sparse checkout and final `passed` step as its neighbours, and add it to the
`needs` of **CI result**. The result job fails when the workflow and `JOBS`
name different jobs.

## MinIO

`.github/workflows/minio-image.yml` builds `docker/minio` once per version of
that directory and pushes it to `ghcr.io/<owner>/quizzivy-minio:<hash>`, a
private package of this repository. `.github/actions/minio` pulls that tag and
falls back to building from source when it is not published, which is what a
pull request that changes `docker/minio` does on its first run.

## Branch rules

The ruleset `protected branches` covers `main`, `develop` and `work/**`:

- no deletion and no force push;
- changes arrive by pull request;
- **CI result** must pass.

A repository admin can merge a pull request past a failing check. That is for
the case where CI itself is broken, and the pull request should say so.

## Deploy

`.github/workflows/deploy.yml` runs when CI succeeds for a **push** to `main`.
The back-merge pull request from `main` to `develop` also has `main` as its
head branch; its CI run does not deploy.
