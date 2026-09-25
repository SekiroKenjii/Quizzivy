# Private Word processing worker

`make import-worker` runs a separate process; `make dev` also starts it when
`IMPORT_S3_BUCKET` is set. Teachers queue and cancel processing through
`POST /admin/imports/{id}/process` and `/cancel`; a finished run writes the
machine draft for review in the same transaction that marks it `needs_review`.

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

Set `IMPORT_WORK_DIR` to an absolute, owner-only directory on persistent disk.
Keep source staging, extraction and converter working files off memory-backed
temporary filesystems. Build `docker/word-converter/Dockerfile` explicitly, inspect
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

The worker polls every two seconds. SIGTERM/SIGINT cancels the current job,
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
