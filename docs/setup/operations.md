# Production operations

Owner: Thuong. These procedures accompany issues #78–#82. Repository tooling is
available; **production restore history, a completed Neon drill and delivered
alerts are not yet verified**. Never substitute a local test for that evidence.

## Deployment verification and alerts

Run `node scripts/verify-deployment.mjs --headers` after deploying both halves.
It checks `/healthz` (including the database), the SPA login document, and browser
security headers. The same script without `--headers` is the availability probe.
The deploy workflow checks that the built `_headers` matches the source file.
The CSP allows the actual PKCE flow and private R2 media, including blob previews;
it does not load Google's GIS SDK. Test login, image display, audio playback and
upload previews on the deployed origin before closing #78. The headers
themselves were observed on 2026-10-04; see the last section.

Fly's own check calls `/livez`, which never touches the database, so Neon's
compute can suspend after five idle minutes. `/healthz` is the database-aware
probe: every call is a query that wakes the compute and keeps it billed for at
least five more minutes. Point only deliberate probes at it. At three probes an
hour, the monitor below would keep the compute running for about a quarter of
otherwise idle hours; choose its cadence with that cost in mind. Both routes
are rate-limited per client address (`/livez` 30/min, `/healthz` 10/min).

The proposed initial monitor is `.github/workflows/production-monitor.yml`:
three checks per hour, with a three-minute job limit. To activate it:

1. Merge the workflow onto the repository's default branch.
2. The owner enables GitHub Actions failure notifications for their account.
3. Run `workflow_dispatch` with `test_alert=true`. This fails only the workflow;
   it never stops the application. Record the recipient and receipt timestamp.
4. Set repository variable `PRODUCTION_MONITOR_ENABLED=true` and verify a
   scheduled success. Record its run URL and the alert test's run URL here.

This is a best-effort starting point: GitHub schedules can be delayed, and
notification routing depends on the workflow actor/account settings. Upgrade to
an external uptime service if prompt detection is required. No service account
or billable subscription was created. Unexpected errors remain in structured
Fly logs with request IDs; an external error collector is deferred until a
service and data handling policy are chosen. Never send answers, tokens or
integrity payloads to an error collector by default.

Fly health checks remove unhealthy Machines from routing; they do not themselves
restart a failing Machine. During an incident, inspect `fly checks list` and
request-ID logs, then explicitly choose rollback or restart.

References: [GitHub scheduled-workflow limitations](https://docs.github.com/en/actions/how-tos/troubleshoot-workflows),
[notification recipients](https://docs.github.com/en/actions/concepts/workflows-and-actions/notifications-for-workflow-runs),
[Fly health checks](https://fly.io/docs/reference/health-checks/).

## Request logs and what counts as a server error

Every request logs one `http` line at INFO: `method`, `path`, `status`,
`duration_ms` and `request_id`. A failure a handler returns also logs one
`handler` line at ERROR with the same `request_id` and the error, and answers
`500 INTERNAL`; the permission gate's and the API-reference gate's 500 log only
the `http` line. Count 5xx from `status`.

A streamed upload (media, a replacement, an import source, a profile photo) is given
a 110-second read deadline in place of the server's 30 seconds. If the API cannot set
it, the request logs a `streaming_read_deadline` line at WARN with the error and
continues under the 30 seconds: a writer wrapped around the server's lost
`Unwrap`. Any such line is a fault to fix.

A browser that aborts a fetch (TanStack Query cancelling on navigation, a closed
tab, a dropped connection) is not a fault. A request that fails because its own
client went away answers and logs `status:499`, nginx's name for it, with no body
and no ERROR line; a `client_cancelled` line at DEBUG, off in production, names
the error. A 499 is in no 5xx count. The same holds for the permission gate, the
API-reference gate and `/healthz`.

The request's own context must be cancelled, and the error either `context
canceled` or a network timeout. The second is what pgx reports when the cancel
reaches it while it writes to Postgres: it expires the connection's deadline to
interrupt the write and returns `write failed: write tcp ...: i/o timeout`,
which does not name the cancel. The pool sets no write deadline of its own, so on
a request the client did not cancel the same line is a real fault.

The other case is a client that resets or closes the connection while the
response is being written (`write: broken pipe`, `write: connection reset by
peer`, `use of closed network connection`). The write meets the lost peer before
the server has seen the close, so the context may still be live and the error
names no cancel. It counts when it is the error of the write to the response
itself, as the access log middleware recorded it: the same text from the database
connection is a fault. It answers as a cancel does, `status:499` at INFO and the
`client_cancelled` line at DEBUG, though the connection may already carry the
first bytes of a 200; the access line reads 499, and the server writes no second
status to a socket that is gone.

What stays a server error: a `context deadline exceeded` (the pool's two-second
ping, an upload's two-minute budget), a context the server cancelled while the
client was still connected, an `i/o timeout` on a request nobody cancelled, a
write of the response that timed out, and any other failure, even on a request
the client also cancelled. The maintenance
gate and the rate limiters have no such path: the first reads its window under a
context the client cannot cancel, and the others decide from memory, a body they
cannot read only costing them its key.

## Backup and recovery drill

The proposed restore-history target is seven days, subject to the owner's Neon
plan and cost approval. **The actual production window is unknown.** Record the
project, parent branch, PostgreSQL version and configured window from the Neon
console before accepting #79. PITR history is separate from student-record
retention. Neon can create an isolated child branch using past data only within
its available history; do not restore over the production branch for a drill.
See [Neon branch creation and restoration](https://neon.com/docs/manage/branches).

1. Choose a known checkpoint in the available history. Record the UTC timestamp,
   migration version, expected assignment/attempt IDs and expected scores using
   a restricted incident record, not a public issue.
2. Create a separate recovery branch at that timestamp. Keep its credentials
   away from the production deployment and disable application writes/jobs.
3. Set `RESTORE_DATABASE_URL` to that branch's direct connection URL (PG18).
   Run `node scripts/verify-recovery.mjs > recovery.json`. The script uses a
   read-only repeatable-read transaction and never prints credentials or names.
4. If an evidence file was captured at the same immutable checkpoint, run
   `node scripts/verify-recovery.mjs checkpoint.json` to compare it. Digests cover
   the listed critical columns; they are a consistency signal, not a complete
   cryptographic backup verification. Independently inspect the expected
   records, score values, migration compatibility and one resumed/read-only
   student paper through an isolated API instance.
5. Check every object in the evidence's media manifest against private storage:
   object exists, downloaded bytes match the recorded SHA-256, and an unsigned
   request does not expose it. Database recovery cannot restore missing R2
   objects. Keep a separate private object backup and rehearse restoring it.
6. Record start/end time, branch ID, source timestamp, recovery evidence,
   application check, media check and operator. Remove the temporary branch
   after review so it does not accrue storage charges.

For a local logical-backup rehearsal, stop application writes, capture evidence,
use PG18 `pg_dump --format=custom`, restore into a new empty database owned by
`quizzivy_migrate`, and compare the evidence. Preserve roles/ACLs; never use
`--clean` against the source. This proves the local procedure, not Neon PITR.
A database-level privilege is restored only by `pg_restore --create`, which
this procedure does not use: after restoring into a new database, run
`REVOKE TEMPORARY ON DATABASE <database> FROM PUBLIC;` as its owner, or the
restored database holds goose's record of migration 00079 without its effect
(#194).

Local rehearsal on 2026-09-22: PG18 custom-format dump restored into a new
empty database, verifier matched the checkpoint before and after migration
00029 down/up, and the temporary database was removed. The non-application
`zz_review` scratch schema was excluded. See the
[PR #98 verification](https://github.com/SekiroKenjii/Quizzivy/pull/98) for scope and counts.
This does not change the pending production evidence below.

## Retention and manual anonymization

Approved on 2026-09-22: integrity events are retained for thirteen calendar
months after assignment closure and at least thirteen months from server `received_at`; the audit log is retained. Disabled accounts
are never automatically erased. Retention uses UTC calendar arithmetic, deletes
only when both dates are strictly before the cutoff, and appends an audit entry for each nonempty batch.
Migration `00029_index_integrity_retention.sql` adds the supporting index without
blocking normal writes. Application-role UPDATE/DELETE privileges on either log
remain forbidden.

Build `server/cmd/maintenance` and provide `MAINTENANCE_DATABASE_URL` only to that
manual process using an owner/maintenance credential. Never set it on the API.
The shape is `maintenance <command> [flags]`, each command with its own flags:

```sh
maintenance retain-integrity -batch 1000
maintenance retain-integrity -batch 1000 -apply
maintenance anonymize-student -student STUDENT_UUID
maintenance anonymize-student -student STUDENT_UUID -apply
```

The older order, flags before the command (`maintenance -apply -batch 1000
retain-integrity`), is still accepted for these two commands. Every command takes
`-timeout <duration>` (default `1m`); an unknown command or flag prints the usage
line and changes nothing.

Omitting `-apply` is a dry-run. Retention processes only one bounded batch per
invocation; repeat after review until it reports zero rows. Set a monthly
operator reminder after credentials and production authorization are arranged.
A student must have no active attempt before anonymization. The transaction
replaces email/name, removes password and Google identities, revokes refresh
credentials and disables the account. IDs, attempts and memberships survive;
existing access JWTs can remain valid until their configured `ACCESS_TOKEN_TTL` expires. Disable the account and allow that interval before a sensitive erasure operation; the operation is idempotent and records no original identity in its new audit
entry. It rejects admin accounts.

This is **structured-identity anonymization**, not a guarantee that all personal
information disappears: answers, teacher notes, existing audit diffs, IPs and
backups may retain identifying material. Review free text separately under an
explicit request. The approved policy retains audit entries unchanged. Restrict
access to backups and record how a restore will reapply later anonymizations
before allowing the restored system to serve users.

## Maintenance windows

A maintenance window is a period during which the API answers every request to a
path one of its routes serves, under any method, with 503 `MAINTENANCE`, except
GET/HEAD `/livez`, `/healthz` and `/public/status`, so a migration or a provider
change can run without users working against it. A path that no route serves gets
its 404 without the gate asking about a window, so a scanner probing for `/.env`
never makes the API read the database.
Plan one for anything that could fail half-way under live traffic. A window lasts at
most 12 hours, starts no earlier than a minute ago, and never overlaps another.

```sh
maintenance window-schedule -start 2026-10-01T15:00:00Z -end 2026-10-01T16:00:00Z
maintenance window-schedule -start 2026-10-01T15:00:00Z -end 2026-10-01T16:00:00Z -apply
maintenance window-list
maintenance window-cancel -window WINDOW_UUID -apply
maintenance window-end -window WINDOW_UUID -apply
```

Scheduling is a dry run until `-apply`. The dry run reports how many attempts,
assignments and student overrides would move (`attempts`, `assignments` and
`overrides` in its JSON); read it before applying.

What moves when a window is scheduled, in the same transaction, audited as
System (no actor, `reason: "maintenance"`, the window id and the database role):
- every attempt still in progress whose deadline is after the window's start
  gets the window's length added to its deadline (`attempt.extended`);
- every open assignment that would close inside the window closes that much
  later (`assignment.extended`);
- every per-student override of a published assignment whose `closes_at` falls
  inside the window closes that much later (`assignment.override_extended`, one
  audit row per override, naming the student). An assignment closed early is
  included, because the override is what keeps its student in.

An override with no `closes_at` (a longer time limit or more attempts only) has
no close to move, and the overrides of an assignment that was never published
are left alone.

`window-schedule`, `window-cancel` and `window-end` take the advisory lock
`(73819, 40)` exclusively. A student's start, a teacher's extension, update or
reopening of an assignment and the two override writes take it shared, so none
of them runs between a window's scheduling and the extensions it makes
(`docs/plan/20-data-model.md` §28).

The override extension updates `app.assignment_student_overrides.closes_at`, so
the database role behind `MAINTENANCE_DATABASE_URL` needs `SELECT` on the table
and `UPDATE (closes_at)` on that column. R5's `grant_maintenance_role` migration
grants both to `quizzivy_maintenance`; until it lands, run `cmd/maintenance` with
the owner or maintenance credential this document already asks for. A role
without the grant fails the apply with a permission error, and the transaction
writes nothing.

`window-cancel` cancels a window that has not ended; `window-end` ends an active
window now. Both keep the extensions already granted: a deadline or an override
close that moved does not move back. `window-list` prints the upcoming and
active windows.

A window takes effect within 30 seconds of being written, which is how long the
API may serve its cached answer to "is there a window?". Schedule the start at
least a minute ahead of the work. The import worker is not gated: it keeps
processing the imports it already holds (O-R1.1). If a window is needed for work
that must not race imports, stop the worker's process group for its length.

## Join-code key

`JOIN_CODE_KEY` seals every join code issued from v0.8.0 on and keys the hash
the API finds it by (D5). It is standard base64 of exactly 32 random bytes.
The API refuses to start without it, and never logs it.

- **Generate** it with `openssl rand -base64 32`.
- **Store** it as a Fly secret (`fly secrets set JOIN_CODE_KEY=...`), with an
  offline copy in the owner's password manager. The offline copy is the only
  way back from a lost Fly secret.
- **Never** paste the key into a log, a ticket, a chat or a workflow. No
  GitHub workflow environment holds it.

### Rotating the key

The rekey runs from the operator's machine, never from a scheduled workflow.
The operator supplies `JOIN_CODE_KEY` and `JOIN_CODE_KEY_PREVIOUS` to
`cmd/maintenance` from the offline copies, with `MAINTENANCE_DATABASE_URL`.

1. Stage both keys in **one** command, so they reach the machines in the
   same release:
   ```sh
   fly secrets set --stage JOIN_CODE_KEY_PREVIOUS=<old key> JOIN_CODE_KEY=<new key>
   ```
   Never set them one at a time. Without `--stage` every `fly secrets set`
   restarts the machines, and a half-changed pair either stops the API from
   starting (the two keys equal) or makes every code under the old key
   unredeemable until the second command lands.
2. Deploy. The API now issues codes under the new key and still finds and
   reads codes under the old one. Its startup log line `join code keys`
   names `current_key_id` and `previous_key_id`.
3. Run the dry run and read its report:
   ```sh
   maintenance rekey-join-codes
   ```
   - `currentKeyId` and `previousKeyId` must equal the ids in the API's log
     line. If they do not, the keys on the operator's machine are not the
     pair the API runs with: stop.
   - `unknown` must be 0: it counts active codes sealed under a key that is
     neither of the two. `-apply` refuses while it is not.
   - `pending` is the number of codes that will move.
   - `unopened` lists the codes under the old key whose ciphertext does not
     open, by row and class id, never by code.
   - `legacy` counts every code issued before v0.8.0, revoked or expired
     ones included. They hold no ciphertext and cannot be re-keyed. From
     v0.10.0 the API replaces the live ones at start-up (see "Legacy join
     codes" below), so what remains here is revoked or expired, and has
     nothing to re-key.
4. Apply it. It walks the codes in batches of 500 (`-batch`), one
   transaction each, and writes only `code_hash`, `code_ciphertext` and
   `key_id`. Joins to the codes in a batch wait until that batch commits, so
   run it at a quiet time:
   ```sh
   maintenance rekey-join-codes -apply
   ```
   A code that does not open is listed under `unopened` and left where it
   is; the others move. For each listed code with `revoked: false`, rotate
   that class's code (from the class page, or by `POST
   /teacher/classes/{id}/join-code`): once the old key is unset, it can
   neither be read nor redeemed. A revoked one needs nothing.
5. Run the dry run again and verify: `pending` is 0, `unknown` is 0, and
   every `unopened` code shows `revoked: true`.
6. Stage the removal of the old key:
   ```sh
   fly secrets unset --stage JOIN_CODE_KEY_PREVIOUS
   ```
7. Deploy.

The report is JSON, like every other maintenance command's.

### If the key is lost

Every code sealed under it becomes unreadable and unredeemable: its class's
code reads back as `code: null, legacy: false`, and joining with it answers
`JOIN_CODE_INVALID`. Generate a new key, deploy it, rotate every class's code,
and tell the teachers to share the new codes.

## Legacy join codes

A code issued before v0.8.0 is held only as its SHA-256 (lookup scheme 1): it
redeems, but cannot be read back. `jobs.RotateLegacyJoinCodes` replaces each live
one, once, when the API starts. No operator command is involved; `Serve` starts
the job beside the prunes, and the `worker` process does not run it.

For every class whose active code is scheme 1, not revoked and not yet expired,
one transaction revokes that code and issues a sealed one (scheme 2, under the
current `JOIN_CODE_KEY`). The new code keeps the old one's expiry, use cap (a
stored NULL stays NULL), use count and creator, and leaves the class's
self-join switch as it was. A class whose legacy code had reached its cap
therefore gets a code that is used up: its teacher issues a new one to admit
more students (F-67). The old code then answers as any replaced code does:
"revoked" where self-join is open, and as an unknown code where self-join is
closed or the class is archived. Archived classes are rotated too. Each rotation
is audited as `class.join_code_rotated` with no actor (the System), the diff
`{reason: "legacy_rotation", previousHint, hint}`, never a code.

- **Concurrency.** Each class is its own transaction and takes
  `pg_advisory_xact_lock(73819, 41)`, then the class row (`FOR NO KEY UPDATE`),
  then re-reads the class's active code. Two machines starting together
  therefore rotate a class once, and a second run finds nothing to do. A class
  whose transaction lost a deadlock or serialization conflict is tried once more
  after the other classes (`docs/plan/20-data-model.md` §28 and §33).
- **Budget.** Two minutes for the whole run. It never runs on a timer. A class it
  did not reach, or that failed, waits for the next start-up: restart the app
  machine or deploy again.
- **Teachers.** Each affected teacher gets one in-app notification,
  `join_codes.rotated`, with how many of their classes were rotated and the first
  five names; a later run adds its count to the same unread row. A failed
  notification undoes nothing, and a process that stops between a teacher's last
  rotation and their notification leaves them untold (the next run finds nothing
  to rotate), so the release note tells the teachers as well.
- **Logs** (`slog` JSON; none carries a code or a hint, only class and user ids):
  `legacy_join_codes_found {count}` before the first class when there is one;
  `legacy_join_codes_rotated {found, rotated, failed, teachers, notify_failed}`
  after the last; `legacy_join_codes_problem {err}` at Warn for each class left
  unrotated and each teacher left untold; `legacy_join_codes_failed {err}` when
  the listing itself failed. A run that finds nothing logs nothing.

Release check: run this with any credential that can read `app.class_join_codes`,
before and after the deploy. The count before is the number of classes the run will
rotate (record it for the release note); the count after must be 0, or the classes
that `legacy_join_codes_rotated` reports as `failed` are waiting for the next
start-up:

```sql
SELECT count(*) FROM app.class_join_codes
 WHERE revoked_at IS NULL AND lookup_scheme = 1 AND expires_at > now();
```

The lookup branch that still finds a scheme-1 row (so a revoked or expired
legacy code answers as revoked or expired, not as unknown) stays until R5 removes
it.

## Evidence still required in production

- Neon project/branch and observed history window: pending access.
- PITR drill, application checks and private media restoration: pending access.
- Monitor recipient and delivered test notification: pending owner setup.
  `PRODUCTION_MONITOR_ENABLED` is not set (checked 2026-10-04), so the monitor
  has never run on its schedule (#81).
- Response headers: observed on 2026-10-04 with `curl -sI`.
  `https://app.quizzivy.com/` sends HSTS, the CSP, `nosniff`, `Referrer-Policy`
  and `Permissions-Policy` exactly as `web/public/_headers` states them.
  `https://api.quizzivy.com/livez` sends HSTS, `nosniff`,
  `Referrer-Policy: no-referrer` and
  `Content-Security-Policy: frame-ancestors 'none'; base-uri 'none'; object-src 'none'`.
- Sign-in, image display, audio playback and upload preview under that CSP:
  pending the owner's check on the deployed site (#78).
- `/boot.js` is served with `cache-control: max-age=14400` (observed 2026-09-27
  and 2026-10-04) although `_headers` asks for `no-cache`: the zone's Browser
  Cache TTL overrides it. `https://quizzivy-web.pages.dev/boot.js` sends
  `no-cache` (2026-10-04), so the header is rewritten in the zone, not by Pages.
  A changed `boot.js` can reach a browser up to four hours late. Owner:
  Cloudflare dashboard, Caching, Configuration, Browser Cache TTL, "Respect
  Existing Headers"; then `curl -sI https://app.quizzivy.com/boot.js` must show
  `no-cache` (#191).
- `TEMPORARY` on the production database (#194): to be checked after the
  release that carries the migration revoking it from `PUBLIC`.
  `SELECT has_database_privilege('quizzivy_app', current_database(), 'TEMPORARY');`
  must answer false. If it answers true, the migration role does not own the
  database there, and its owner runs
  `REVOKE TEMPORARY ON DATABASE <database> FROM PUBLIC;` once; the query must
  then answer false. Pending.
- Maintenance dry-run review and first authorized production batch: pending.
