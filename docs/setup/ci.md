# CI

`.github/workflows/ci.yml` runs on every pull request and on every push to
`main`, `develop` and `work/**`. One check, **CI result**, is what the branch
rules require; it passes only when every job did what the plan asked of it.

## What runs

| Job | Reads | Does |
|---|---|---|
| Plan | everything | Tests `scripts/ci/`, then decides which jobs run |
| Deck | `deck` | `scripts/check-design-deck.mjs` |
| Contract | `code` | `make gen-check`: lint the contract, regenerate, fail on drift |
| Server lint | `server` | vet, staticcheck, golangci-lint, gofmt |
| Server tests | `server` | unit, integration and end-to-end tiers, the Word converter tests; migrations up, down, up |
| Production image | `image` | Builds the root `Dockerfile`; starts `/app/api` and `/app/import-worker` from it as `nonroot` |
| Web checks | `web` | `pnpm lint:ci`, typecheck, `pnpm format:ci`, integration tests, build (lint and format never read a cache) |
| Web unit | `web` | the unit suite, in shards |
| E2E | `web` | Playwright against a production build, API stubbed |
| E2E (live API) | `code` | Playwright against the real API, Postgres, MinIO and the Word import worker |
| CI result | — | Fails unless each job above did what the plan said |
| Sonar | everything | Pushes to `develop` only; not part of the result |

## How a job is skipped

A job is skipped only when there is proof that it passed on the same files and
has not failed on them since. Nothing is skipped because of what a diff looks
like.

1. `scripts/ci/plan.mjs` names five sets of files (`SETS`) and the set each job
   reads (`JOBS`). A set is a base (`all` or `none`) with directories and files
   included or excluded; the longest rule that matches a path decides.
2. Plan hashes the mode, blob and path of every file in a job's set. The
   workflow, `.github/actions/` and `scripts/ci/` are in every set, so a change
   to CI itself changes every hash. `scripts/dev/` holds the team's local
   scripts; no job reads it, `plan.test.mjs` fails if one does, and it is in no set.
3. A job's last step records how it ended, as an artifact kept for 30 days:
   `ci-pass-<job>-<hash>` after a pass, `ci-fail-<job>-<hash>` after a failure,
   nothing when it was cancelled. Plan skips a job when the newest pass for its
   hash is newer than the newest failure. Only artifacts uploaded by a run on a
   branch of this repository count; a fork's do not.
4. Each job checks out only its set (a sparse checkout built from the same
   rules). A test that reads a file outside its set fails with "no such file"
   instead of being skipped when that file changes.

So, while the proof stands, a pull request that changes only `docs/plan/` runs
Plan and CI result, a server-only change does not run the web jobs, and a
merge whose tree already passed as a pull request runs no job again. The proof
does not always stand: a marker expires after 30 days, and a tree nobody has
tested (the base moved under the pull request, for example) has none.

Three cases never use earlier proof:

- **A push to `main`.** The commit that deploys is tested in full.
- **"Re-run all jobs".** That is how to force a full run. "Re-run failed jobs"
  re-runs only what failed.
- **A job that failed on these files since it last passed.** A forced run that
  fails records the failure, and every later run on the same files runs the
  job until it passes again.

What the hash cannot see is anything outside the tree: the runner image, the
`postgres:18` image (which `mirror.gcr.io` serves), the Debian packages the Word converter image installs,
tools fetched at run time. The 30-day expiry and the full run on `main` are
what bring a job back in front of those.

A skipped job writes no toolchain cache. After a pull request changes
`server/go.sum` or `web/pnpm-lock.yaml`, its merge is skipped as proven, so
the new Go and pnpm caches exist only in that pull request's scope. Each later
pull request builds its own on its first run, until a push to the base branch
runs a job for real or the next push to `main` does. That costs the Go jobs
some tens of seconds and no result.

## How long it takes

Measured on GitHub's hosted runners: the first row on 2026-10-04, after the
server job began building the import converter's image and running its tests;
the other two on 2026-10-03, before it did. The old workflow took between
7 min 40 s and 10 min 33 s for every run.

| Run | Time |
|---|---|
| Every job runs | 4 min 25 s |
| Every job runs and MinIO is built from source, the first run after `docker/minio` changes | 8 min 43 s |
| No job has anything new to run | 18 s |

E2E (live API) is the longest job, a little over four minutes (4 min 3 s in
that run), and Server tests the next (3 min 40 s, the converter image and its
nine tests included); a faster run starts with those two.

Measured again on 2026-10-14 from the job timings of 702 of the 1,000 latest
runs (2026-10-03 to 2026-10-14; 653 of the 1,000 were pull request runs). A pull
request that passed took a median 6 min 11 s (p90 8 min 20 s); a push that
passed took a median 25 s, nearly every job having been proved already.

| Job | Median | p90 | Share of job-minutes |
|---|---|---|---|
| E2E | 5 min 55 s | 7 min 43 s | 18% |
| Web unit (both shards) | 5 min 21 s and 4 min 58 s | 7 min 28 s | 34% |
| E2E (live API) | 4 min 39 s | 6 min 10 s | 19% |
| Web checks | 4 min 3 s | 5 min 14 s | 13% |
| Server tests | 4 min 31 s | 6 min 44 s | 7% |
| Contract | 1 min 19 s | 1 min 38 s | 5% |
| Everything else | under 2 min | | 5% |

The web jobs are 83% of the 16,239 job-minutes. The hosted minutes of a public
repository are free, so what these numbers cost is the time a person or an agent
waits, not an invoice. Where the time goes inside a job: E2E spent a median
296 s in Playwright, on one worker; Web unit spent about 300 s per shard in
Vitest; E2E (live API) spent 177 s in the suite and about 100 s starting what the
suite needs.

## Tests in CI

- **Playwright workers.** The stubbed projects (`chromium`, `mobile-chromium`)
  run on three workers in CI: they stub the API per page and share no rows. The
  live project shares one seeded database, so `pnpm e2e:live` pins one worker
  in CI and locally. On a 4-core container the 271 stubbed tests took 649 s on
  one worker and 343 s on three.
- **A skipped test fails Plan.** `scripts/ci/tests/skipped-tests.test.mjs` fails
  on a web test that is skipped, todo, fixme, expected to fail or focused, and on
  a Go `t.Skip` whose message neither names the variable it waits for (`TEST_*`,
  `S3_*`, `IMPORT_*`, `APP_DATABASE_URL`) nor is listed in `ENVIRONMENT_SKIPS`.
  A new entry is a test that stops running somewhere, so it takes the owner's
  word.
- **Playwright retries twice.** A flaky test therefore passes: search the job log
  for `flaky` and treat each as a finding (`docs/team/verification.md`).

## When a job needs a file outside its set

Add the path to that set's `include` in `scripts/ci/plan.mjs`, in the same pull
request as the test that reads it. `scripts/ci/tests/plan.test.mjs` fails when
a rule no longer matches a tracked file, so a moved file is noticed.

Today's cross-tree reads:

| Reader | File |
|---|---|
| `web/tests/units/join/code.test.ts` | `server/internal/modules/classes/domain/joincode.go` |
| `server/tests/isolation_world_test.go` | `web/tests/e2e/fixtures/unit5-listening.mp3` |
| Server tests | `api/`, `migrations/`, `fly.toml`, `Dockerfile`, `docker/word-converter/`, `.github/workflows/deploy.yml` |
| Production image | `Dockerfile`, `.dockerignore`, `fly.toml`, `server/`, `migrations/`, `scripts/provision-db.sh` (its set, `image`, holds these and nothing else) |
| Web tests | `api/openapi.yaml`, `api/testdata/` |

A test must not treat a missing file as a reason to skip itself: in CI that
would hide the mistake this layout exists to show.

## Adding a job

Add it to `JOBS` with its set, give it the same `needs`, `if`, `env.JOB`,
sparse checkout and final `record` step as its neighbours, and add it to the
`needs` of **CI result**. `plan.test.mjs` reads the workflow and fails when a
job is missing from either list or lacks one of those parts; Plan runs that
test on every run. Sonar is the one job outside the result, by intent.

## Runs on a branch

A pull request's newer push cancels its run in progress. On `main`, `develop`
and `work/**` a run in progress is never cancelled, but runs go one at a time
and only the newest waiting one is kept, so a commit pushed while two others
wait may show a cancelled run. Its tree is covered by the run after it.

The result of a push run is named **CI result (push)**. The head commit of a
pull request from `main`, `develop` or a work branch has both runs, and only
the pull request's run tests the merge; the branch rules read that one.

## Pull requests from forks

A fork's pull request runs the workflow, the scripts and the tests from its
own merge commit, so its green **CI result** says only what its own files
chose to check. Read a fork's pull request before merging it, whatever CI
says. Its markers are never proof for another run, and its token cannot write
packages.

## The object store and the Word converter in Server tests

Server tests starts MinIO after the unit tier, so the integration tier runs
with a real object store: the storage client's tests and the import intake,
artifact and PDF pipeline tests store and read real objects. The storage
settings (`S3_*`) are set on the steps that use them, not on the job.
`server/internal/platform/storage/tests/s3_test.go` carries no build tag, so
the unit tier compiles it too, and there it has to skip. The bucket is
`quizzivy-imports`, the private one imports are kept in.

After the integration tier one step builds `docker/word-converter` and the next,
**Import converter tests**, runs the tests that need that image: the
converter's own (`internal/platform/wordconvert/tests`) and the test that takes
a synthetic paper through the real pipeline
(`internal/modules/imports/repositories/tests`). The tests are given the
image's id, because the converter refuses a tag. The step fails before
`go test` when it has no id, rather than skip every test in it and pass.

They run one package at a time (`-p 1`). A conversion takes one Docker-wide
slot, a container with a fixed name, and a conversion that finds the slot taken
answers busy. `go test` runs packages in parallel, the converter tests live in
two packages, and one of them occupies the slot on purpose, so run together
they would pass or fail by timing. In the integration tier these tests skip,
because `TEST_WORD_CONVERTER_IMAGE` is not set there.

`APP_DATABASE_URL`, the DSN of `quizzivy_app`, is set on the Server tests job
for `TestProfileAndPreferencesPersistWithTheActualAppRoleAndCheckedRollback`,
which proves profile writes through the role the API connects as and skips
without it. The step **App-role profile test ran** runs it once with `-v` and
fails unless it passed, so removing the variable cannot turn it back into a
silent skip. `make test-api-integration` sets it from `APP_DSN`.

A test that needs the object store or the image skips when its variable is
unset, and `go test` reports a package whose tests all skipped as `ok`. The
image id is the one such value that comes from another step, and an expression
that names a renamed step or a mistyped output is empty without an error; that
is why **Import converter tests** checks it first. The storage settings are
written on the steps themselves. The steps do not print each test; to see by
name which of these tests ran, add `-v` to the step's `go test` for one run, as
the pull request that added the steps did.

## The production image

Every other job runs the code on the runner's file system, so a start-up check
that depends on the image's own (INC-5: `/home/nonroot/imports` shipped at 0755
and the API refused it, on Fly only) passed every test. **Production image**
builds the root `Dockerfile` and runs `scripts/ci/smoke-production-image.sh`,
which does what a deploy does:

1. Checks that the image's user is `nonroot:nonroot`.
2. Runs `/app/migrate -dir /app/migrations up` from the image, as Fly's
   `release_command` does, against the job's `postgres:18` service.
3. Starts `/app/api` and `/app/import-worker` from the image, each in its own
   container on the host network and with no `--user`, so they run as the
   image's. The environment is `fly.toml`'s `[env]` read by the script, plus
   throwaway values for the secrets `docs/setup/dns.md` lists, and the worker's
   wake address and URL moved from `fly-local-6pn` to `localhost`.
4. Waits up to 60 seconds for `GET /livez` to answer and for the worker to log
   `import worker started`, and fails, printing both logs, when either
   container exits first or the time runs out.

MinIO is not started: neither process reaches the object store while it starts.
The job's set is `image`, the files above plus `scripts/ci/`; a change to the
web, the docs or the contract does not run it. It took 1 min 22 s on the first run, most
of it the Go build inside the image; the smoke test itself takes seconds. The
base image `golang:1.27-alpine` comes from the mirror, like the others.

To run it locally, with the services of `docs/team/environment.md`: build the
image, create a database for the run with
`QUIZZIVY_DB_NAME=smoke scripts/provision-db.sh`, then
`SMOKE_DATABASE=smoke scripts/ci/smoke-production-image.sh <image>`.

## The Word import worker in E2E (live API)

The live job builds the converter image with `.github/actions/word-converter`,
the same action Server tests uses, so one build recipe serves both. It then
starts `cmd/import-worker` beside the API, with the API's database, the
`quizzivy-imports` bucket that `minio-init` creates, an owner-only work
directory under the runner's temp directory, `IMPORT_DOCKER_BINARY` and the
image id as `IMPORT_CONVERTER_IMAGE`. The API runs with
`IMPORT_PROCESSING_ENABLED=true` and the wake URL of the worker's listener.
The worker keeps its default single global slot: `seed/08` seeds its
"processing" import as a queued run ten years out, so nothing holds the slot.
The job fails before the API starts when the build gives no image id, because
a worker without one would process `.docx` natively and skip the page rendition
the live spec is there to cover. A failed job prints the worker's log beside
the API's.

`web/tests/e2e/import.live.spec.ts` takes a synthetic `.docx` exam and key,
assembled in the test by `web/tests/e2e/support/docx.ts`, from the Imports page
through the worker to the review and to a draft test whose questions carry
their keys (#139). The live job's set is `code`, so a change to the worker,
the converter or the spec runs it; `plan.test.mjs` pins that.

## MinIO

`.github/workflows/minio-image.yml` builds `docker/minio` once per version of
that directory and pushes it to `ghcr.io/<owner>/quizzivy-minio:<hash>`. The
package is public, as the repository is; its labels name the licence and where
the source commits are listed. `.github/actions/minio` pulls that tag and
falls back to building from source when it is not published, which is what a
pull request that changes `docker/minio` does on its first run.

## Where CI pulls images from

Docker Hub limits anonymous pulls per address, and a hosted runner shares its
address with others, so a pull fails intermittently with `toomanyrequests` and
no test runs. CI therefore pulls every Docker Hub image from Google's mirror,
`mirror.gcr.io/library/<name>`, which takes no login and has no such limit:

- The `postgres:18` service of **Server tests** and **E2E (live API)** names
  `mirror.gcr.io/library/postgres:18`. It is the same official image, still
  PostgreSQL 18, and the tag still follows the 18.x minor release. It is not
  pinned by digest: a digest the mirror has not cached would fall through to
  Docker Hub, and the floating tag is what CI ran before. To pin one, append
  `@sha256:<digest>` to the image in both jobs and bump it deliberately.
- `scripts/ci/mirror-images.sh` pulls a base from the mirror and tags it under
  its Docker Hub name, so the build that follows finds it in the local store.
  `.github/actions/word-converter`, which Server tests and E2E (live API) use, runs it for
  `debian:trixie-slim` before building the Word converter; `.github/actions/minio` and `minio-image.yml` run it for
  `golang:1.27-alpine` and `alpine:3.22` before building MinIO. A base the
  mirror cannot serve prints a warning and is pulled from Docker Hub as before.
  A new `FROM` on a Docker Hub image adds its name to the call beside the build.
- MinIO itself comes from `ghcr.io`, and nothing else in the workflows pulls from
  Docker Hub.

If the mirror ever fails too, the alternative is a Docker Hub login from a
repository secret, which raises the limit for the account; it needs Thuong to
create a read-only access token and add it as a secret, and is not built.

## Branch rules

Three rulesets, under Settings › Rules:

| Ruleset | Branches | Rule |
|---|---|---|
| `pull requests only` | `main`, `develop`, `work/**` | No force push; changes arrive by pull request |
| `no deletion` | `main`, `develop` | The branch cannot be deleted |
| `ci result` | `develop`, `work/**` | **CI result** must pass, except when a branch is created, so `work/redesign-r<k>` can be cut from `develop` |

`main` joins `ci result` once the release that carries this workflow is on
`main`. Until then a `hotfix/*` cut from `main` runs the old workflow, which
never reports **CI result**.

A repository admin can merge a pull request past a failing check. That is for
the case where CI itself is broken, and the pull request should say so.

## The wiki

`.github/workflows/wiki.yml` publishes the documentation to the repository's
GitHub Wiki. It is a workflow of its own, outside CI, and its result is not
part of **CI result**. The planner counts `wiki.yml` and `scripts/wiki/` as
not code, so a change to them runs no code job.

| Event | Job | Does |
|---|---|---|
| Pull request that touches docs or `scripts/wiki/` | Check | Tests `scripts/wiki/`, then builds the pages without publishing |
| Push to `develop` that touches the same files | Publish | Builds the pages into a clone of the wiki and pushes the difference as `github-actions[bot]` |
| Manual run on `develop` | Publish | The same, for a resync after the wiki was edited by hand; GitHub offers it once the workflow is on `main` |

`scripts/wiki/build.py` reads the checkout and writes one page per document:
every `docs/**/*.md` outside `docs/design/deck/` (the deck is pinned by
`MANIFEST.sha256`), `AGENTS.md`, `CONTRIBUTING.md` and each directory's
`README.md`. The root `README.md` becomes Home. What it does to them:

- A document over 200 KB is split at heading boundaries into parts of about
  120 KB behind an index page that links every heading; the product spec is
  split into one page per section instead.
- A relative link to another document becomes a link to its wiki page, and to
  a heading in a split document the part that holds it. A link to anything
  else becomes a link to the file on `develop`. A code span that is exactly a
  document's path or file name is linked too. Fenced code is left as written.
- A document the table in the script does not name is still published, under a
  name derived from its path, and the run logs a warning. Add it to the table
  to name it and place it in the sidebar.
- Every page ends with the file it came from and carries a marker comment. A
  page with the marker that the build no longer writes is removed; a page
  without it, such as one written by hand in the wiki, is never touched. A
  generated page edited in the wiki is overwritten by the next run, so edit
  the file in the repository.

The build exits 1, and the run fails, when a link between wiki pages does not
resolve or a split loses text. Nothing is committed to the wiki when the
output matches what it holds. The `GITHUB_TOKEN` of the run writes to the
wiki, which needs the wiki to exist: GitHub creates its repository when
someone saves a first page in the browser.

## Deploy

`.github/workflows/deploy.yml` runs when CI succeeds for a **push** to `main`,
and deploys only when that commit is still the tip of `main`.

- The back-merge pull request from `main` to `develop` also has `main` as its
  head branch; its CI run does not deploy.
- Re-running an old CI run of `main` to green does not put that release back
  into production.
