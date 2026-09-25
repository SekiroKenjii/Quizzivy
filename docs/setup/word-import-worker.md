# Private Word processing worker

`make import-worker` runs a separate process; `make dev` also starts it when
`IMPORT_S3_BUCKET` is set. Teachers queue and cancel processing through
`POST /admin/imports/{id}/process` and `/cancel`; a finished run writes the
machine draft for review in the same transaction that marks it `needs_review`.

The API accepts processing only where a worker runs. Set
`IMPORT_PROCESSING_ENABLED=true` on the API beside the worker; `make dev` does this
when it starts one. Without it, `process` answers 503
`IMPORT_PROCESSING_UNAVAILABLE` and queues nothing. `GET /admin/imports/capabilities`
reports `intakeEnabled` (import storage configured) and `processingEnabled`, and the
web follows it:

- **Nothing configured:** no way into Word import from the tests list, and
  `/admin/imports` says it is not enabled.
- **Storage only:** the history stays, and finished imports can be reviewed and
  committed, but nothing new starts.
- **Both:** the whole flow.

Native `.docx` needs no converter: without `IMPORT_DOCKER_BINARY` and
`IMPORT_CONVERTER_IMAGE` the worker extracts DOCX directly and skips the page
rendition. Legacy `.doc` needs both, and the API must set `IMPORT_LEGACY_DOC=true`
so uploads are accepted; a `.doc` run on a worker without a converter fails with
`LEGACY_CONVERSION_UNAVAILABLE`.

Use PostgreSQL 18 with migrations through `00049`, the application database role,
and a separate private import bucket. The storage implementation must support
conditional immutable writes and SHA-256 checksums; no public bucket ACL or learner
media URL is used. Set the storage/import variables in `.env.example` in the
process environment. The worker does not need JWT signing or Google credentials.

Set `IMPORT_WORK_DIR` to an absolute, owner-only directory on disk. Keep source
staging, extraction and converter working files off memory-backed temporary
filesystems. The directory holds per-job scratch files only, so it may be emptied
between runs. Build `docker/word-converter/Dockerfile` explicitly, inspect
the resulting local image ID, and set `IMPORT_CONVERTER_IMAGE` to that immutable
`sha256:…` identity. Set `IMPORT_DOCKER_BINARY` to the trusted absolute Docker
client path. The worker never pulls an image automatically. Give Docker access
only to this dedicated worker; the document container never receives its socket,
storage credentials or application environment.

Default limits are one active global lease, one active lease per actor, a 512 MiB
Go soft-memory target and one synchronous job per process. The converter separately
enforces 512 MiB memory, one CPU, 64 processes and a 120-second deadline. Leave
headroom for the database, storage, Docker daemon and OS. Do not increase concurrency
without capacity measurements and approval of the supported envelope. Stage quotas
default to 512 MiB per creator, 2 GiB globally and 200 sets per import; pending
reservations count, including artifacts left by interrupted workers.

The worker does not poll on a timer. It claims until the queue is empty, then
sleeps until one of three things happens:

- The API wakes it. `process` sends `POST /wake` to `IMPORT_WORKER_WAKE_URL`, which
  the worker serves on `IMPORT_WORKER_WAKE_ADDR`. Wakes coalesce and are retried
  briefly, and a teacher watching a queued import re-sends one.
- Queued work falls due: a delayed retry, an expiring lease, or a retired
  pipeline version reaching its grace period.
- `IMPORT_WORKER_IDLE_POLL` passes (1h by default), as a safety net.

It never polls sooner than every two seconds. An idle worker therefore lets
Neon's compute suspend (#143). SIGTERM/SIGINT cancels the current job,
records a result that finished before the signal, and otherwise releases the run
to the queue without spending an attempt; the fenced write gets at most five
seconds. A heartbeat that fails transiently is retried until the lease is nearly
spent. A run is claimed only by a worker of the pipeline version it was scheduled
with; after a deploy, runs left waiting ten minutes for a retired version fail with
`PIPELINE_RETIRED` so the teacher can process them again. A hard crash recovers through lease expiry; completed stages remain reusable.
The converter's independent deadline still runs after the worker is killed, and
its physical slot prevents overlapping conversion. Another live converter causes
a retry after two minutes. Source/checksum mismatch and storage quota exhaustion
are terminal failures requiring investigation; retry does not repair corrupt bytes.

Operational logs include run/import/worker IDs, stage, attempt, safe failure code
and elapsed time. They must not include source text, answer keys, object keys,
signed URLs or provider responses. Run status and stage events are durable in
PostgreSQL. A completed run references private artifact sets; it is not a published
test and cannot bypass teacher review.

Retention/cleanup policy, monitored production supervision and release
acceptance are still pending. Byte quotas count only imports that are neither
committed nor cancelled.
Do not delete source or artifact rows/objects to make quota errors disappear.
Inspect and resolve the referenced run first; automatic retention is not enabled.

## Production

Word import is off in production. `fly.toml` sets no `IMPORT_*`, so the tests list
offers no way into it and `/admin/imports` says it is not enabled.
`platform/config/tests/deployment_test.go` pins two things:

- `IMPORT_PROCESSING_ENABLED` in `fly.toml` goes together with a worker process
  that the Dockerfile builds.
- `.doc` stays off. Its converter needs a Docker daemon, and the production
  image has none. Hosting one means a separate privileged Machine running
  `dockerd`, which this runbook does not cover.

Turning it on is a decision, not only a deploy: see O-24 in
`docs/plan/40-open-items.md`. When it is taken:

1. **Storage.** Create a private R2 bucket, for example `quizzivy-imports`, with no
   public access and no custom domain. Add it to the R2 API token's buckets: the
   import store reuses the `S3_*` credentials, and the token is scoped to
   `quizzivy-media` today (`docs/setup/r2.md`).

   Then check the bucket can hold import objects. The import store writes each
   object once, with `If-None-Match: *` and a full-object `x-amz-checksum-sha256`,
   then verifies that checksum with `HeadObject`
   (`server/internal/platform/storage/immutable.go`). `docs/setup/r2.md` records
   that R2 supports SHA-256 only as a composite multipart checksum. If R2 refuses
   that single-part write, every run fails at its first artifact. Try one
   conditional SHA-256 put and head against the new bucket before going further.
2. **Image.** Build `./cmd/import-worker` in the Dockerfile and copy it beside
   `/app/api`.
3. **Fly configuration.** In `fly.toml`:
   - add `[processes]` with `app = "/app/api"` and `worker = "/app/import-worker"`;
   - scope `[http_service]` to `processes = ["app"]`;
   - give the worker its own `[[vm]]`, sized for its 512 MiB Go memory target;
   - set `IMPORT_S3_BUCKET`, `IMPORT_WORK_DIR = "/home/nonroot/imports"` and
     `IMPORT_PROCESSING_ENABLED = "true"` in `[env]`.

   `IMPORT_WORK_DIR` works there because the image's nonroot user owns its home
   directory. A Machine's root filesystem is disk, not tmpfs. It is reset on
   every deploy, which is fine for scratch files.
4. **Acceptance.** Deploy, then take one small `.docx` through upload, review and
   draft.

**Cost.** Each running worker is one more Machine. The worker queries PostgreSQL
only when woken, when work falls due, and once per `IMPORT_WORKER_IDLE_POLL`. Set
that to `6h` in production, so an idle worker keeps Neon's compute awake about 20
minutes a day. On Fly the API reaches the worker at
`http://worker.process.quizzivy-api.internal:8091/wake`, and the worker listens on
`fly-local-6pn:8091`. Nothing is exposed publicly.
