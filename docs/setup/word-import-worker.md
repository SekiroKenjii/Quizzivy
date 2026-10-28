# Private Word processing worker

`make import-worker` runs a separate process; `make dev` also starts it when
`IMPORT_S3_BUCKET` is set. Teachers queue and cancel processing through
`POST /teacher/imports/{id}/process` and `/cancel`; a finished run writes the
machine draft for review in the same transaction that marks it `needs_review`.

The API accepts processing only where a worker runs. Set
`IMPORT_PROCESSING_ENABLED=true` on the API beside the worker; `make dev` does this
when it starts one. Without it, `process` answers 503
`IMPORT_PROCESSING_UNAVAILABLE` and queues nothing. `GET /teacher/imports/capabilities`
reports `intakeEnabled` (import storage configured) and `processingEnabled`, and the
web follows it:

- **Nothing configured:** no way into Word import from the tests list, and
  `/teacher/imports` says it is not enabled.
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

**Scans with AI.** An import created with `readWithAi` (T-D4.33, plan 17 §1.42) reads a PDF
that has no text layer with the AI assistant instead of failing it as `PDF_NO_TEXT`. The
worker loads `ANTHROPIC_API_KEY` and the tier settings as the API does
(`docs/setup/operations.md`, "AI provider key and tiers"); without them a scan is still
refused. For such a file the worker:

- draws every page in the same PDFium sandbox, as a JPEG of at most 1568 pixels and 1 MiB,
  in at most three minutes and the sandbox's 256 MiB. Forty pages of 300 dpi scans took
  about 30 s and 322 MiB resident, forty 12-megapixel photographs about 33 s and 530 MiB,
  so the worker's 1 GB machine holds either beside its 512 MiB Go target;
- refuses a file of more than 40 pages as `PDF_TOO_LONG_FOR_AI`, before it holds or sends
  anything;
- reserves 60 credits a page for the whole file from the teacher who started the run, and
  refuses a shortfall as `AI_CREDITS_EXHAUSTED` (or `AI_PAUSED`) with nothing sent;
- sends the pages to the capable tier four at a time, and settles in one ledger row what
  the finished pages billed, releasing the rest, whatever way the read ends;
- fails `PROVIDER_UNAVAILABLE` when the provider stays down or the read outlasts ten
  minutes, and `AI_UNREADABLE` when no page could be read. Both are terminal: the retry
  would pay for the pages again.

A run may now take up to 15 minutes while the assistant is on (5 otherwise), which the
one-minute lease covers through its heartbeat. The log has `import read with ai` (source,
role, pages, unreadable pages, credits) and `ai import settled` (the reservation and what
it charged); neither holds a page, a line or a response. If a worker dies mid-read, the
hourly reservation sweep of the API returns the credits held. Migration `00125` adds
`word_imports.read_with_ai` and must run before the API that sends it; a v0.10.x worker
that claims such an import refuses a scan as `PDF_NO_TEXT`, so deploy both together.

A pasted test (`POST /teacher/imports/{id}/sources/text`, source format `text`)
needs no converter, no sandbox and no Docker, whatever the worker's settings. The
API stores the NFC-normalised text as a private `pasted-text.txt` in the import
bucket, like any original. The worker reads it back and turns each non-blank line
into a paragraph block (evidence version `text-lines-v1`); it never normalises
or renders it. A text source is at most 100,000 Unicode code points and 20,000
non-blank lines, and the worker refuses to read more than 400,000 bytes of it.
A pasted import may still carry an answer-key file, which is read as its format
says.

`PipelineVersion` is `word-pipeline-v3`, and a run is claimed only by a worker of
the version it was scheduled with. Deploy the API and the worker together: a v2
worker leaves a text run unclaimed, and a run still queued under v2 when v3 is
deployed fails after ten minutes with `PIPELINE_RETIRED`, so its teacher processes
it again (the same release rule as at the PDF release).

Use PostgreSQL 18 with migrations through `00091`, the application database role,
and a separate private import bucket. `00090` lets `word_import_sources.format`
be `text`, `00091` adds the source's `characters` column, which every source
read names, and `00089` adds the draft's two review counters, which every draft
write names. A worker on an older schema fails on its first source or draft query.
(Later migrations, through `00106`, belong to the rest of the release and are
applied by the same deploy.) The storage implementation must support
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

The worker also builds the Admin's data export (T-R5.15). It drains the import queue first and
then the export queue, wakes through the same address, and needs nothing more than the import bucket and
`IMPORT_WORK_DIR`, where it writes the ZIP before it uploads it and removes it afterwards; one a killed worker
left there is removed the next time the worker finds nothing to claim. An
export is cut off after 25 minutes, and one still running 30 minutes after its claim, because its worker
died, is claimed again at most twice more and then failed with `WORKER_INTERRUPTED`. Without this process
the API answers the three `/admin/data-exports` operations with `501 EXPORT_OFF`.

The worker also creates the accounts of the Admin's user imports (T-R5.14). It drains the Word imports, then
the user imports, then the exports. It needs `SECRETS_KEY` (the same value as the API: it seals a sheet's
temporary passwords and the links sent with an email job), reads `MAX_CONCURRENT_PASSWORD_HASHES` to bound the
Argon2id hashing of a sheet's passwords, and sends the invitations of an email job through the same mail outbox
as the API. A job takes 25 accounts to a transaction, is cut off after 25 minutes, and one still running 30
minutes after its claim is claimed again at most twice more and then failed with `WORKER_INTERRUPTED`. Without
this process the API answers the four `/admin/user-imports` operations with `501`; without `SECRETS_KEY` it
answers the preview, the create and the sheet so. The worker reads the importer's rights from the database at the
start of every batch, so a revoked or disabled Admin stops a job within one batch (25 accounts) and not after the
API's 10-second cache. Size the process for a sheet job: with four concurrent Argon2id hashes of 64 MiB it
peaked at 553 to 612 MiB resident for 150 to 2,000 rows, and 2,000 rows took about 12 minutes under load, inside
the 25-minute run limit. A failed batch is logged with the class of its failure and never its text. The sealed
passwords of a sheet nobody downloads are erased 24 hours after the job finished by the API's daily housekeeping
(the `import_sheets` step), not by this process.

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

On the VPS (infra `vps/compose.yaml` and `vps/app.env`) the worker is the
`worker` service of the same image: `IMPORT_WORKER_WAKE_ADDR=0.0.0.0:8091` and
`IMPORT_WORKER_WAKE_URL=http://worker:8091/wake`. The port is published to no
one; only containers on the stack's private Docker network reach it, which is
what Fly's private network did. Each of the API and the worker has its own
volume at `/home/nonroot/imports`, owned by 65532 with mode 0700 from the image.
The worker's limit is 1.5 GB there, above the 1 GB below.

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
