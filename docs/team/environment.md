# Verification environment

How the team's cloud container reaches the state in which the gates in
[`verification.md`](verification.md) run. `AGENTS.md` and `docs/setup/ci.md` win
over this file. Written 2026-10-07 against `work/redesign-r4` @ `bb4d4000`;
re-date it when a step changes.

The container is Linux with 4 cores and 15 GB of memory. The Postgres and
MinIO services run in Docker. Everything else runs on the host. Outbound
traffic goes through the agent proxy, which allows npm, the Go module proxy and
GitHub but not every registry (see "What did not work").

## Reaching the state

Run from `/home/user/Quizzivy`. Each step is safe to repeat.

1. **Start the Docker daemon** if `docker info` fails. It inherits the proxy
   variables of the shell, which it needs to pull images.

   ```
   nohup dockerd > /tmp/dockerd.log 2>&1 &
   until docker info >/dev/null 2>&1; do sleep 1; done
   ```

2. **Create `.env`** if it is missing: `cp .env.example .env`, then set
   `JOIN_CODE_KEY` to the output of `openssl rand -base64 32`. `.env` is
   git-ignored; never commit it.

3. **Make the MinIO image exist.** `docker-compose.yml` names
   `quizzivy-minio:development` with `pull_policy: never`, so compose starts it
   only when the image is already local. Check with
   `docker image inspect quizzivy-minio:development`. If it is missing,
   `docker compose build minio` is the documented route (`docker/minio/README.md`)
   but fails here, so build the same pinned commits on the host:

   ```
   B=$(mktemp -d); cd "$B"
   git init minio && git -C minio fetch --depth=1 https://github.com/minio/minio.git 07c3a429bfed433e49018cb0f78a52145d4bedeb && git -C minio checkout --detach FETCH_HEAD
   git init mc    && git -C mc    fetch --depth=1 https://github.com/minio/mc.git    7394ce0dd2a80935aded936b09fa12cbb3cb8096 && git -C mc    checkout --detach FETCH_HEAD
   mkdir out
   (cd minio && CGO_ENABLED=0 GOTOOLCHAIN=go1.27.0 go build -trimpath -ldflags="$(GOTOOLCHAIN=go1.27.0 go run buildscripts/gen-ldflags.go)" -o ../out/minio .)
   (cd mc    && CGO_ENABLED=0 GOTOOLCHAIN=go1.27.0 go build -trimpath -ldflags="$(GOTOOLCHAIN=go1.27.0 go run buildscripts/gen-ldflags.go)" -o ../out/mc .)
   cat > Dockerfile <<'EOF'
   FROM golang:1.27-alpine AS certs
   FROM alpine:3.22
   COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
   COPY out/minio out/mc /usr/bin/
   ENTRYPOINT ["minio"]
   EOF
   docker build -t quizzivy-minio:development .
   ```

   The commits are the ones in `docker/minio/README.md`; keep them in step with
   it. The base images `golang:1.27-alpine` and `alpine:3.22` must already be
   local. The two compile steps take several minutes. This recipe was written
   from the build that produced the image in use on 2026-10-07 and has not been
   re-run end to end since; correct it the first time it is.

4. **Start the services.** `make up` also builds the image, which fails here, so
   start them directly:

   ```
   docker compose up -d --wait db minio
   docker compose run --rm minio-init
   ```

   `db` is `postgres:18` on 5432. On its first start `docker/initdb/` creates the
   roles `quizzivy_migrate` and `quizzivy_app` and the database `quizzivy` by
   calling `scripts/provision-db.sh`. The roles live in the `db-data` volume, so
   they survive a container restart. If the volume is gone, step 4 creates them
   again.

5. **Install the tools the repository expects on `PATH`.**
   - goose v3.27.3, the version of `.github/actions/db-tools`, in
     `/root/.local/bin` (`goose --version` should say v3.27.3). Take the release
     binary from GitHub as that action does, or
     `GOTOOLCHAIN=go1.27.0 GOBIN=/root/.local/bin go install github.com/pressly/goose/v3/cmd/goose@v3.27.3`,
     and `export PATH=/root/.local/bin:$PATH`.
   - `psql`, `createdb` and `dropdb` come from the container's `postgresql-client`
     16. They talk to the 18 server without trouble; CI installs client 18.
   - `cd web && pnpm install --frozen-lockfile`.

6. **Pin the Go toolchain.** The container has Go 1.24.7 and `server/go.mod`
   says `go 1.27`. Without a pin, `go` tries to fetch a toolchain per command
   and `make lint` fails (rc 2, 267 s). Export it in every shell that runs Go:

   ```
   export GOTOOLCHAIN=go1.27.0
   ```

7. **Migrate and seed the `quizzivy` database**:
   `make migrate` (goose up to version 91 at `bb4d4000`), then `make seed` if a
   gate needs the seed data.

8. **Provision what CI's `server-test` job provisions** before the Go integration
   tier. Without it, `attempts/repositories/tests` fails for want of `QUEUE_*`
   and `imports/repositories/tests` for want of `TEST_TEXT_IMPORT_*`. Use a
   scratch directory `T` for the journal. `TEST_DESTRUCTIVE=1`, as in CI, lets
   the migration round trip drop and recreate schema `app` in `quizzivy`, so run
   `make seed` again afterwards if a later step needs the seed data.

   ```
   export PATH=/root/.local/bin:$PATH GOTOOLCHAIN=go1.27.0
   export PGHOST=localhost PGPORT=5432 PGUSER=postgres PGPASSWORD=postgres
   export TEST_DATABASE_URL='postgres://quizzivy_migrate:migrate@localhost:5432/quizzivy?sslmode=disable'
   export TEST_DESTRUCTIVE=1 S3_ACCESS_KEY_ID=quizzivy S3_SECRET_ACCESS_KEY=quizzivy-dev-secret
   T=$(mktemp -d); J=$T/journal.json

   createdb --owner=quizzivy_migrate quizzivy_grading_queue
   goose -dir migrations postgres 'postgres://quizzivy_migrate:migrate@localhost:5432/quizzivy_grading_queue?sslmode=disable' up
   TEXT_IMPORT_APP_BASE_URL='postgres://quizzivy_app:app@localhost:5432/quizzivy?sslmode=disable' \
     GITHUB_OUTPUT=$T/output.txt python3 scripts/ci/text-import-purpose.py prepare $J
   ```

   `prepare` creates a database `qvtext_<token>` and a bucket `qv-text-<token>`
   and writes their names, with the two connection strings, to `$GITHUB_OUTPUT`
   as `database`, `bucket`, `app_url` and `migrate_url`.

9. **Run the tiers** from `server/`. The integration step's variables belong to
   that command only, as in CI, where they are the step's `env`:

   ```
   env QUEUE_COMMITTED_DATABASE_URL='postgres://quizzivy_app:app@localhost:5432/quizzivy_grading_queue?sslmode=disable' \
       TEST_TEXT_IMPORT_DATABASE_URL=<app_url> TEST_TEXT_IMPORT_MIGRATE_DATABASE_URL=<migrate_url> \
       TEST_TEXT_IMPORT_DATABASE_NAME=<database> TEST_TEXT_IMPORT_BUCKET=<bucket> \
       S3_ENDPOINT=http://localhost:9000 S3_REGION=us-east-1 S3_BUCKET=quizzivy-imports S3_FORCE_PATH_STYLE=true \
     python3 ../scripts/ci/text-import-purpose.py run $J -- go test -tags integration ./internal/... -count=1
   go test -tags e2e ./tests/... -count=1
   ```

   The end-to-end tier gets no `S3_*` at all: its harness defaults to the
   compose endpoint and to two buckets, `quizzivy-media` and `quizzivy-imports`
   (`server/tests/isolation_world_test.go`). Exporting `S3_BUCKET=quizzivy-imports`
   into its shell makes the two stores one, and
   `fixture_lifecycle_verification_test.go` then fails with
   `registered2 resources, want3`. That is the environment, not the code.

10. **Dispose of what step 8 created**, from the repository root. Skip nothing:
    a leftover journal makes the next `prepare` refuse.

    ```
    python3 scripts/ci/text-import-purpose.py cleanup $J
    dropdb quizzivy_grading_queue
    ```

    `psql -X -At -d postgres -c 'select datname from pg_database'` should then list
    only `postgres`, `quizzivy`, `template0` and `template1`. The end-to-end tier
    creates and removes its own `qve2e_*` database and bucket.

## Services and ports

| Service | How to start | Port | Stop |
| --- | --- | --- | --- |
| Docker daemon | `nohup dockerd > /tmp/dockerd.log 2>&1 &` | unix socket | `pkill dockerd` (stops every container) |
| PostgreSQL 18 (`quizzivy-db`) | `docker compose up -d --wait db` | 5432 | `docker compose stop db` |
| MinIO (`quizzivy-minio`) | `docker compose up -d --wait minio`, then `docker compose run --rm minio-init` | 9000 S3, 9001 console | `docker compose stop minio` |
| API | `make dev-api` (needs `.env` and the migrated database) | 8080 | stop the process |
| Vite dev server | `cd web && pnpm dev` | 5173 | stop the process |
| `vite preview` (live E2E) | started by Playwright | 4173 | `pkill -f 'vite preview'` before each run |

Leave Postgres and MinIO running between pieces of work; stop the API and Vite
when a verification run ends. Four sub-agents share one Postgres and one MinIO,
so give each test run its own database (`createdb --owner=quizzivy_migrate`).
`docker compose down` keeps the volumes; add `-v` only to start from nothing.

Credentials are the compose defaults: Postgres superuser `postgres`/`postgres`,
`quizzivy_migrate`/`migrate`, `quizzivy_app`/`app`; MinIO
`quizzivy`/`quizzivy-dev-secret`. Use `localhost` for both web and API.

## What did not work

- **`docker compose build minio`** fails: BuildKit cannot reach the Alpine
  mirror (`apk add git` answers HTTP 403 through the proxy). The host build in
  step 3 avoids it, because the host has Go, git and the proxy's CA bundle.
- **`make up`** runs the same build, so use the two `docker compose` commands in
  step 4.
- **Go without the pin.** `make lint` exits 2 after 267 s; with
  `GOTOOLCHAIN=go1.27.0` it passes in 184 s.
- **`make test-api` straight after `make migrate`.** The integration tier fails
  in two packages for want of the CI provisioning in step 8. That is the
  environment, not the code.
- **One shell for both Go tiers.** Carrying the integration step's `S3_*` into
  the e2e run fails two fixture-lifecycle tests (step 9).
- **The Word converter tests** (`TestDocker*`, `TestRealPipeline*`, the "Import
  converter tests" step in CI) are not run. They need the
  `docker/word-converter` image, which this piece of work did not try to build. With
  `TEST_WORD_CONVERTER_IMAGE` unset they skip and the packages still read `ok`,
  so a green local run says nothing about them. Only CI covers them.
- **`tsc --noEmit`** checks no files; use `pnpm typecheck`.

## Differences from CI

| | This container | CI |
| --- | --- | --- |
| Node | 22.22.0 | 24 |
| pnpm | 10.28.0 | 11.25.0 |
| Go | 1.24.7 host, 1.27.0 by `GOTOOLCHAIN` | 1.27 |
| `psql` client | 16.15 (server is 18.6) | 18 |
| Postgres | `postgres:18` in compose, shared by every run | a service container per job |
| MinIO image | built on the host from the pinned commits | pulled from `ghcr.io`, or built by compose |
| Browsers | Chromium only, in `/opt/pw-browsers` (`PLAYWRIGHT_BROWSERS_PATH`) | what the Playwright config installs |
| Parallelism | 4 cores, one shared Postgres | one job per runner |
| Word converter tests | not run | run |
| Retries, shards | none unless asked (`CI=1` for CI's Vitest defaults) | Playwright `retries: 2` |

A change to `pnpm-lock.yaml` was not exercised under pnpm 11.25.0 here; leave
its verification to CI.

## Baseline at `bb4d4000`

Durations are wall-clock in this container, with other gates sometimes running.

| Gate | rc | Duration | Notes |
| --- | --- | --- | --- |
| `node scripts/check-design-deck.mjs` | 0 | under 1 s | |
| goose up/down/up | 0 | 1 s | a first attempt failed rc 2 before the database was migrated |
| `pnpm lint` | 0 | 173 s | |
| `pnpm typecheck` | 0 | 28 s | |
| `pnpm format:check` | 0 | 20 s | |
| `pnpm test:unit` | 0 | 474 s | 320 files, 3936 tests |
| `pnpm test:integration` | 0 | 53 s | 19 files, 120 tests |
| `pnpm build` | 0 | 29 s | |
| `make gen-check` | 0 | 75 s | |
| `make lint` | 0 | 184 s | rc 2 in 267 s without the Go pin |
| `make test-api` | 2 | 113 s | unit tier passes; integration fails only for the missing CI provisioning |
| Go integration tier, CI-provisioned | 0 | 68 s | 66 packages ok, none skipped; converter tests excluded |
| Go e2e tier, CI's job env only | 0 | 44 s | `quizzivy/tests` ok in 42.7 s, nothing skipped |

No gate fails at `bb4d4000` once the environment matches CI. The first e2e run
here failed twice with `registered2 resources, want3` because it inherited the
integration step's `S3_BUCKET` (step 9 explains why); re-run with CI's job env
only, the same tests pass. Not run here: the Word converter tests, `pnpm e2e`,
`pnpm e2e:content` and `pnpm e2e:live`.
