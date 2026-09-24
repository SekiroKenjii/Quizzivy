# Private Word processing worker

`make import-worker` runs a separate process. It never runs as part of `make dev`
or the HTTP API. The queue currently uses the internal application interface;
public processing controls and production rollout are separate milestones.

Use PostgreSQL 18 with migrations through `00044`, the application database role,
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

The worker polls every two seconds. SIGTERM/SIGINT cancels the current job and
allows at most five seconds for the fenced retry-state write after processing
stops. A hard crash recovers through lease expiry; completed stages remain reusable.
The converter's independent deadline still runs after the worker is killed, and
its physical slot prevents overlapping conversion. Another live converter causes
a retry after two minutes. Source/checksum mismatch and storage quota exhaustion
are terminal failures requiring investigation; retry does not repair corrupt bytes.

Operational logs include run/import/worker IDs, stage, attempt, safe failure code
and elapsed time. They must not include source text, answer keys, object keys,
signed URLs or provider responses. Run status and stage events are durable in
PostgreSQL. A completed run references private artifact sets; it is not a published
test and cannot bypass teacher review.

Retention/cleanup policy, created-state orphan recovery, monitored production
supervision, full domain validation and release acceptance are still pending.
Do not delete source or artifact rows/objects to make quota errors disappear.
Inspect and resolve the referenced run first; automatic retention is not enabled.
