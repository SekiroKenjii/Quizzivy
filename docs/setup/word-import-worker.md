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

PDF needs no converter either. The worker reads it with PDFium compiled to
WebAssembly, in a wazero sandbox with no filesystem and no network. Each read
gets a fresh sandbox, which is freed afterwards:

- up to 256 MiB of memory;
- one minute;
- 60 pages.

The compiled module is kept, so only the first PDF after start-up pays the
roughly 2.5 s compile. A scan, a locked file, a broken file and an oversized one
fail with `PDF_NO_TEXT`, `PDF_PROTECTED`, `PDF_INVALID` and `PDF_TOO_LARGE`.

Use PostgreSQL 18 with migrations through `00052`, the application database role,
and a separate private import bucket. The storage implementation must support
conditional `If-None-Match: *` writes, `Content-MD5` and round-tripped
`x-amz-meta-*` metadata; the SHA-256 digest travels as metadata and is checked by
the code itself. No public bucket ACL or learner media URL is used. Set the storage/import variables in `.env.example` in the
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
  briefly. The detail and history pages re-send one while they show a queued
  import, through the `Nudge` command.
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

Retention runs in the API, not in this worker, so it also works on a deployment
that has import storage but no worker. The API sweeps at start-up and then
daily. Each sweep:

- closes imports untouched for 60 days, which removes their files at once;
- removes the files and review drafts of imports committed more than 30 days
  ago or cancelled more than 7 days ago.

It logs only counts (`import retention swept`). It logs at Warn with
`IMPORT_RETENTION_FAILED` or `IMPORT_RETENTION_INCOMPLETE` when something went
wrong, and retries a failed sweep after an hour.

Byte quotas count only imports that are neither committed nor cancelled. Do not
delete source or artifact rows or objects by hand to make quota errors disappear.
Inspect and resolve the referenced run first. Monitored production supervision and
release acceptance are still pending.

## Production

Word and PDF import run in production (O-24, decided 2026-09-25). One image
carries both processes, and `fly.toml` declares them:

- `[processes]` runs `app = "/app/api"` and `worker = "/app/import-worker"`.
- `[http_service]` serves `processes = ["app"]` only. The worker has no public
  service.
- The app keeps its 512 MB `[[vm]]`. The worker gets its own with 1 GB: a 512 MiB
  Go memory target, plus up to 256 MiB of PDF sandbox.
- `[env]` holds the import settings:

  ```toml
  IMPORT_S3_BUCKET = "quizzivy-imports"
  IMPORT_WORK_DIR = "/home/nonroot/imports"
  IMPORT_PROCESSING_ENABLED = "true"
  IMPORT_WORKER_WAKE_URL = "http://worker.process.quizzivy-api.internal:8091/wake"
  IMPORT_WORKER_WAKE_ADDR = "fly-local-6pn:8091"
  IMPORT_WORKER_IDLE_POLL = "6h"
  ```

The import store reuses the `S3_*` secrets; no new secret is needed.

`platform/config/tests/deployment_test.go` boots both processes on these
values. It fails when:

- the processing switch and the worker process disagree;
- the Dockerfile does not build and copy `/app/import-worker`;
- the worker listens where the API machine cannot reach it, or the wake URL and
  the listener disagree;
- the idle poll is under an hour;
- `[http_service]` is not scoped to the app;
- the worker's `[[vm]]` has less than 1 GB;
- `.doc` is enabled. Its converter needs a Docker daemon, and the image has none.
  Hosting one means a separate privileged Machine, which this runbook does not
  cover.

`IMPORT_WORK_DIR` works because the image's nonroot user owns its home
directory. A Machine's root filesystem is disk, not tmpfs. It is reset on every
deploy, which is fine for scratch files.

**Before the first deploy with import on:**

1. **Storage.** The private bucket `quizzivy-imports` exists, with no public
   access and no custom domain. The R2 API token behind `S3_ACCESS_KEY_ID` must
   list it beside `quizzivy-media` (`docs/setup/r2.md`). Then run
   `make verify-r2-imports`. It drives the import store's own code against the
   bucket named by `R2_IMPORT_BUCKET` (default `quizzivy-imports`), with the
   `R2_*` values `make verify-r2` reads. It covers:
   - a create-only write, an identical retry and a refused changed retry;
   - the bytes read back;
   - a source upload and a signed download;
   - an unsigned request refused;
   - every verification object removed afterwards.

   Every line must pass. The S3 API cannot tell whether the bucket is public
   through r2.dev or a custom domain, so check both with wrangler:

   ```bash
   wrangler r2 bucket dev-url get quizzivy-imports    # "disabled"
   wrangler r2 bucket domain list quizzivy-imports    # no custom domains
   ```
2. **Deploy.** Deploy as usual. `flyctl deploy --ha=false` creates exactly one
   worker Machine the first time the `worker` group appears; without the flag it
   would add a standby that the deploy's start step turns into a second worker.
   The deploy fails unless every process group ends with a started machine.
   Check it with `flyctl machines list -a quizzivy-api`: one `app` and one
   `worker`, both `started`. The worker logs `import worker started` with the
   pipeline version.
3. **Acceptance.** Take one small `.docx` and one text-layer PDF through upload,
   review and draft. Then check `/healthz` and the Neon console: compute should
   suspend again after the worker's last query.

**Monitoring.** Only the deploy checks that the worker runs. The production
monitor probes `/healthz` and the web app, not the worker. A worker that later
exhausts its restart budget shows as imports that stay queued, so check
`flyctl machines list` when that happens.

**Turning it off.** Remove `IMPORT_PROCESSING_ENABLED` and the `worker` process
together; `deployment_test.go` refuses one without the other. Finished imports
stay reviewable while import storage stays configured. Removing
`IMPORT_S3_BUCKET` and `IMPORT_WORK_DIR` as well hides the feature, and
retention stops with it.

**Cost.** Each running worker is one more Machine. The worker queries PostgreSQL
only when woken, when work falls due, and once per `IMPORT_WORKER_IDLE_POLL`. At
`6h`, an idle worker keeps Neon's compute awake about 20 minutes a day. The wake
listener is on Fly's private network only, and wakes cannot make the worker poll
more than once every two seconds.
