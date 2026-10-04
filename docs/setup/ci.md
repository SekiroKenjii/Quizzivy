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
| Web checks | `web` | eslint, typecheck, prettier, integration tests, build |
| Web unit | `web` | the unit suite, in shards |
| E2E | `web` | Playwright against a production build, API stubbed |
| E2E (live API) | `code` | Playwright against the real API, Postgres and MinIO |
| CI result | — | Fails unless each job above did what the plan said |
| Sonar | everything | Pushes to `develop` only; not part of the result |

## How a job is skipped

A job is skipped only when there is proof that it passed on the same files and
has not failed on them since. Nothing is skipped because of what a diff looks
like.

1. `scripts/ci/plan.mjs` names four sets of files (`SETS`) and the set each job
   reads (`JOBS`). A set is a base (`all` or `none`) with directories and files
   included or excluded; the longest rule that matches a path decides.
2. Plan hashes the mode, blob and path of every file in a job's set. The
   workflow, `.github/actions/` and `scripts/ci/` are in every set, so a change
   to CI itself changes every hash.
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
`postgres:18` image, the Debian packages the Word converter image installs,
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

A test that needs the object store or the image skips when its variable is
unset, and `go test` reports a package whose tests all skipped as `ok`. The
image id is the one such value that comes from another step, and an expression
that names a renamed step or a mistyped output is empty without an error; that
is why **Import converter tests** checks it first. The storage settings are
written on the steps themselves. The steps do not print each test; to see by
name which of these tests ran, add `-v` to the step's `go test` for one run, as
the pull request that added the steps did.

## MinIO

`.github/workflows/minio-image.yml` builds `docker/minio` once per version of
that directory and pushes it to `ghcr.io/<owner>/quizzivy-minio:<hash>`. The
package is public, as the repository is; its labels name the licence and where
the source commits are listed. `.github/actions/minio` pulls that tag and
falls back to building from source when it is not published, which is what a
pull request that changes `docker/minio` does on its first run.

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

## Deploy

`.github/workflows/deploy.yml` runs when CI succeeds for a **push** to `main`,
and deploys only when that commit is still the tip of `main`.

- The back-merge pull request from `main` to `develop` also has `main` as its
  head branch; its CI run does not deploy.
- Re-running an old CI run of `main` to green does not put that release back
  into production.
