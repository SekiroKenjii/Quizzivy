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

## Media files that left a library

A media file leaves a teacher's library when it is replaced or deleted, but its object stays
in storage until seven days after nothing names it: no question, group, material or published
test version, of any owner. Then the API removes the object for good and marks the row
(`media_assets.purged_at`). Nothing restores it, and the teacher is told only by the answer
to the replace or the delete and by the figures on the media page (F-68, the owner's decision
of 2026-10-12; `docs/plan/20-data-model.md` §45).

The API runs the sweep (`jobs.SweepRetiredMedia`) on every machine, only where media storage is
configured, at start-up and then every hour. A row is taken under a lock the other machines
skip, so a second machine removes nothing twice. It logs one line a run, with counts and no
title or filename:

| Log line | Level | Meaning |
|---|---|---|
| `media retention swept` | INFO | `unreferenced` files dated, `cancelled` waits cleared, `removed` objects deleted |
| `media retention left files in place` (`MEDIA_RETENTION_INCOMPLETE`) | WARN | `failed` files stayed for the next run, ten minutes later; `err` is the first cause, an object key at most |
| `media retention sweep failed` (`MEDIA_RETENTION_FAILED`) | WARN | the run could not read or list the files; it retries in ten minutes |
| `media object removed but its row was not marked` (`MEDIA_RETENTION_UNMARKED`) | ERROR | the object of `asset_id` is gone and the database refused the mark, for the ten seconds the finish is given after the delete. The next run marks the row, because deleting a missing object succeeds. If the line repeats for one id, writes to `app.media_assets` are failing |

A run that keeps failing means storage is unreachable or refuses the delete: nothing is lost,
the objects stay and the rows are retried. Each removal writes an audit row `media.purged`, with
no actor. After the first start of the release that added the sweep, a file that left a library
earlier is dated by that run and its object goes seven days later; the first removals are
therefore seven days after the deploy. A file stays while a question, a group or a version names it,
and naming it again before its day clears the wait.

**The switch.** `MEDIA_RETENTION_SWEEP` is `on` (the default) or `off`, nothing else, and is read
when the API starts. Off means that machine never runs the sweep and logs `media retention sweep is
off` at WARN once; quota counting and the dating that replace and delete do go on, so files pile up
as pending and the usage figures still say so. Use it in an incident, to stop removals while
something is investigated, and turn it back on afterwards:

```sh
fly secrets set MEDIA_RETENTION_SWEEP=off -a quizzivy-api     # restarts the machines
fly secrets unset MEDIA_RETENTION_SWEEP -a quizzivy-api       # back to on
```

Keep it out of `fly.toml`, so that a secret is the only place it is set. A value other than `on`
or `off` stops the API at start-up, naming the setting.

### Rolling back past the sweep

The sweep removes objects for good, and the migrations 00111 to 00113 stay on a database whose
API goes back. If the API must go back to v0.10.0, the release before F-68, after the first
removal:

- v0.10.0 does not know `purged_at`. It still treats a purged file as a replaced or deleted one,
  so it signs a URL to an object that is gone wherever it signs one for that row, and a bind of
  that file (a stale media picker, an import review that names a replaced file) passes its checks
  and reaches the database, where the trigger of 00113 refuses it. v0.10.0 does not map that
  refusal, so the teacher gets a 500.
- Nothing is removed further, because v0.10.0 has no sweep; and nothing restores what was removed.
- Before rolling back, turn the switch off and count the damage:
  `SELECT count(*) FROM app.media_assets WHERE purged_at IS NOT NULL`. A teacher whose import review
  names a purged file re-attaches it from the library; a file a version holds is never purged.
- Run no Down migration: v0.10.0 starts on this schema, and Down would drop the trigger that
  keeps a writer from naming a removed file.

Roll forward when you can.

Never delete an object by hand to free space: a file the library still lists, or that a version
holds, would answer 404 to the students who sit that test. The usage on the media page
(`heldBytes`, `pendingBytes`) says what is counted and what waits.

## Organization settings and the sign-in policy

The centre's name, contact details, logos, sign-in methods, allowed Google domains and retention
choices are one row, edited by an Admin with "Manage system settings" through `GET` and `PUT /admin/settings`. The
API serves it from a copy each server keeps for at most 30 seconds. **A change of sign-in policy
therefore reaches every other instance within 30 seconds**, and the server that saved it applies
it at once. During those seconds another instance may still accept a method that was just turned
off, or refuse one that was just turned on; nothing is lost either way, and a retry after half a
minute settles it. An instance that nobody asks never queries the database.

A save is refused (`422 SIGN_IN_POLICY_LOCKOUT`) when it would leave the Admin saving it, or every
active built-in Admin, with no way to sign in. If an Admin is locked out by a change made outside
the API (a Google domain removed from the company, for example), the row can be corrected with the
migrate role and takes effect within 30 seconds:

```sql
UPDATE app.organization_settings SET password_sign_in = true, revision = revision + 1;
```

A logo is stored once under `branding/{sha256}.{png|svg}` in the media bucket and is never
replaced, so an unreferenced one is harmless; delete it by hand if it matters. Without object
storage the settings still save and the front door still reads them, and an upload answers `501`.

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

## Secrets key

`SECRETS_KEY` seals the few values the database holds for a short while and
must not hold in the clear: a set-password token waiting in the email outbox
(`email_outbox.secret_ciphertext`, T-R5.2) and, from T-R5.14, the temporary
passwords of an import's hand-out sheet. It is standard base64 of exactly 32
random bytes. Each purpose gets its own key from it (HKDF-SHA256), and each
value is sealed with AES-256-GCM, bound to the row it belongs to. The API and
the worker both read it, and neither logs it. Production refuses to start
without it.

- **Generate** it with `openssl rand -base64 32`.
- **Store** it as a Fly secret (`fly secrets set SECRETS_KEY=...`), with an
  offline copy in the owner's password manager. Fly secrets reach every process
  group, so the worker has it too. It is not `JOIN_CODE_KEY`: that key seals
  join codes and rotates on its own schedule.
- **Never** paste it into a log, a ticket, a chat or a workflow.

### Rotating the key

There is no previous-key slot, so rotation drains first:

1. Wait until `SELECT count(*) FROM app.email_outbox WHERE sent_at IS NULL AND
   failed_at IS NULL AND secret_ciphertext IS NOT NULL` is 0. Nothing pending
   then depends on the old key.
2. Let any pending import sheet expire (24 hours, T-R5.14), or download it.
3. Set the new key and deploy.
4. Re-issue any set-password link that was unsent at the swap.

### If the key is lost

Every sealed row fails closed: the dispatcher marks a message it cannot open
failed with `secret_unreadable`, and sends nothing. Nothing else is damaged.
Generate a new key, deploy it, and issue the unsent links again from the
console. Links already sent keep working, since the database holds only their
hash.

## Outgoing email

Mail is sent from `email_outbox` by a dispatcher in the API and one in the
worker, through Resend's HTTP API with `RESEND_API_KEY`. `EMAIL_FROM`,
`APP_URL` and the key are set together, and production refuses to start
without all three. Start-up refuses a malformed `EMAIL_FROM` or
`EMAIL_REPLY_TO` (one address, no line break) and an `APP_URL` that is not a
bare origin (no path, `?`, `#` or whitespace). `EMAIL_DAILY_LIMIT` (100, Resend's free tier) counts sends
per local day in `APP_TIMEZONE`; the excess moves to the next local midnight.
`EMAIL_SEND_RATE` (2 a second) spaces the sends. Outside production, without
the three, every message is written as a JSON file under `EMAIL_SINK_DIR`.

To see the queue:

```sql
SELECT template, count(*) FILTER (WHERE sent_at IS NULL AND failed_at IS NULL) AS pending,
       count(*) FILTER (WHERE failed_at IS NOT NULL) AS failed,
       max(last_error_code) AS last_error
  FROM app.email_outbox GROUP BY template;
```

A row of a template the running binary does not know stays pending until a
newer binary runs; if a deploy is rolled back, those rows wait for the next
release. A failed row keeps its `last_error_code` (`http_422:validation_error`,
`secret_unreadable`, ...) and nothing of the message. The logs carry the
outbox id, template, outcome and recipient domain, never an address, a subject
or a link.

## The plan the Admin console shows

The Admin console's Overview and its Plan & usage page set what the centre holds beside the plan
it states (T-R5.18). The plan is six optional variables, read at start-up: `PLAN_NAME` (up to 80
characters), `PLAN_RENEWS_ON` (`YYYY-MM-DD`, and it needs a name), `PLAN_SEAT_LIMIT` and
`PLAN_TEACHER_LIMIT` (whole numbers from 1 to 1,000,000), and `PLAN_MEDIA_BYTES` and
`PLAN_IMPORT_BYTES` (bytes, from 1 to 2^50). A number is plain digits: a sign, a leading zero, a
space or a separator stops the server at start-up. A variable that is unset states no limit, and a value
the server cannot read stops it at start-up, so a typo is found at deploy and not at the first
page load. Nothing is enforced (DG-55): a limit is drawn beside its figure, and a figure may pass
it.

Production sets none of them until the centre's plan is decided; the names are listed in
`fly.toml` and `.env.example`. Seats count the accounts that are not disabled against
`PLAN_SEAT_LIMIT`; teachers count the enabled accounts that are not a student target, Admins
included, against `PLAN_TEACHER_LIMIT`.
Media is the bytes of the files not yet purged from storage, and imports the bytes of the source
documents of the imports whose files have not been removed, against their limits. A meter at 80%
of its limit adds a storage row to the Overview's attention list. Days on the Overview are local
dates in `APP_TIMEZONE`.

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

## AI provider key and tiers

The AI assistant (D19, `docs/plan/74d-d4-deck-update.md` section G) calls
Anthropic's Messages API through `platform/anthropic`. It is off unless
`ANTHROPIC_API_KEY` is set: with the key unset nothing else below is read, the
assistant's adapter is nil, its operations answer 501 and the web draws no AI
entry point. The API never logs the key, and no error, log line or panic the
client produces carries it.

With the key set, both tiers and all six prices are required, and the API
refuses to start, naming the variable, when one is missing or malformed:

| Variable | Holds |
|---|---|
| `ANTHROPIC_API_KEY` | the provider's API key |
| `AI_TIER_FAST_MODEL`, `AI_TIER_CAPABLE_MODEL` | the provider's model identifier for each tier; the capable tier must accept images and PDF pages |
| `AI_TIER_FAST_PRICE_INPUT`, `AI_TIER_FAST_PRICE_OUTPUT`, `AI_TIER_FAST_PRICE_CACHE_READ` | the fast tier's list price in US dollars per million tokens, such as `0.10` |
| `AI_TIER_CAPABLE_PRICE_INPUT`, `AI_TIER_CAPABLE_PRICE_OUTPUT`, `AI_TIER_CAPABLE_PRICE_CACHE_READ` | the capable tier's |

A price is a plain decimal with at most six decimals between 0.001 and 1000. A
price per token or per thousand tokens would make a credit (0.001 US dollars)
undercount the pool, so it is refused. No model identifier is written in the
repository: take both from the provider's current model list.

- **Set** them as Fly secrets, all nine in one command so they reach the
  machines in the same release, and then deploy:
  ```sh
  fly secrets set --stage ANTHROPIC_API_KEY=<key> \
    AI_TIER_FAST_MODEL=<model> AI_TIER_CAPABLE_MODEL=<model> \
    AI_TIER_FAST_PRICE_INPUT=<usd> AI_TIER_FAST_PRICE_OUTPUT=<usd> AI_TIER_FAST_PRICE_CACHE_READ=<usd> \
    AI_TIER_CAPABLE_PRICE_INPUT=<usd> AI_TIER_CAPABLE_PRICE_OUTPUT=<usd> AI_TIER_CAPABLE_PRICE_CACHE_READ=<usd>
  ```
  The startup log line `ai assistant enabled` names both models; `ai
  assistant disabled` means no key was read.
- **Rotate** the key by creating the new one in the provider console, running
  `fly secrets set ANTHROPIC_API_KEY=<new key>` (which restarts the machines
  on it) and revoking the old one once the log line is back. A call in flight
  during the restart can fail once.
- **The import worker** reads the same nine variables when it starts: Fly secrets
  reach every process group, so nothing more is set. It meters the "Read with AI"
  of scanned PDFs (`docs/setup/word-import-worker.md`) through the same credits.
  With the key unset or misconfigured the worker refuses a scan as `PDF_NO_TEXT`
  or refuses to start, as the API does.
- **Change a model or a price** by setting its secret and deploying. Prices and
  models are read once, at start-up; a call is metered at the prices the
  process started with, so re-read them whenever the provider's list price
  changes.
- **Never** paste the key into a log, a ticket, a chat or a workflow. No
  GitHub workflow environment holds it, and it never appears in a test
  fixture: a task that calls the provider for real does so on the builder's
  machine, with the key in `.env`.
- **Before the key is set in production**, the provider account's data
  retention setting is the owner's to confirm
  (`docs/plan/70-redesign-overview.md` §10).

The client sends a 429 or a 5xx again up to three times with a doubling
back-off that honours `Retry-After`, never sends a 4xx again, and gives a call
60 seconds (120 with a PDF). A 401, 402 or 403 from the provider is the key or
the account, not a reason to retry: read the provider console.

## AI credits

The AI assistant is metered in credits (D21): one credit is $0.001 of the provider's list price,
so the $200 pool is 200,000 credits a month. Each teacher has an allowance under it (20,000 by
default), a call reserves its ceiling before it reaches the provider and settles what the provider
metered after, and the database refuses a reservation that would overspend either the pool or the
allowance (`docs/plan/20-data-model.md` §46). A month is the calendar month in `APP_TIMEZONE`
(`Asia/Ho_Chi_Minh` when unset), and the pool, the allowances and the used credits start over on
the 1st. Credits an Admin adds do not carry over; a teacher's custom allowance and pause do.

Until the Admin's AI credits page exists (R5, T-R5.35) the console is `cmd/maintenance`, the
operator's tool, run with `MAINTENANCE_DATABASE_URL` as the other commands are. Every write names
who is running it and is a dry run until `-apply`; a dry run runs the same code as the Admin
operation inside a transaction it rolls back, so it reports exactly what would happen, writes
nothing and tells nobody. The report is JSON.

```sh
maintenance ai-credits status
maintenance ai-credits set-allowance -user USER_UUID -credits 50000 -operator thuong
maintenance ai-credits set-allowance -user USER_UUID -default -operator thuong -apply
maintenance ai-credits pause  -user USER_UUID -operator thuong -apply
maintenance ai-credits resume -user USER_UUID -operator thuong -apply
maintenance ai-credits add-credits -credits 50000 -operator thuong -apply
maintenance ai-credits approve -request REQUEST_UUID -operator thuong -apply
maintenance ai-credits decline -request REQUEST_UUID -operator thuong -apply
maintenance ai-credits set-rules -default-allowance 50000 -warn-percent 90 -on-limit pause -operator thuong
maintenance ai-reservations-sweep
```

- **`status`** reads the current month: the pool (plan, added, used, held, left), the days to go and
  what the month is on course to use (`onPace`, credits used divided by the days elapsed times the
  days in the month) with whether that `withinPool`, the teachers with their allowance, use and
  standing (`on_track`, `near_limit` from the warning share, `at_limit`, `paused`), the requests
  waiting, and the rules. It lists the Admin and every user who holds `ai.use`, and anyone who has
  used AI this month.
- **`set-allowance`** takes `-credits N` (1,000 to 10,000,000), the teacher's own monthly figure,
  which a change of the default leaves alone, or `-default`. Credits an approved request added this
  month stay on top of either. It takes effect at once and the use so far still counts, so a figure
  below what the teacher has used is refused. **`pause`** and **`resume`** stop and restore a
  teacher; the buttons stay visible and say why.
- **`add-credits`** is the Admin's approval to spend beyond the plan: the pool is never overspent
  without it. The audit row names who approved. The product bills nobody; the extra is paid for
  outside Quizzivy (DG-196).
- **`approve`** raises the teacher's allowance for the request's month by the amount and
  **`decline`** changes nothing but the status; both tell the teacher. An earlier month's request
  can only be declined.
- **`set-rules`** takes any of `-default-allowance` (rewrites this month's allowances that are not
  custom, never below what a teacher has used), `-warn-percent` (50 to 99) and `-on-limit`
  (`pause`, or `request` to let a teacher ask for more).
- **`ai-reservations-sweep`** returns the credits of calls that never finished, which a process
  that died mid-call leaves held. The API does the same hourly when the provider is configured
  (`ReleaseStaleAiReservations`), so the command is for reading the count first or releasing at
  once. A reservation older than ten minutes is stale; a call lasts at most two minutes.

Every write is one `audit_log` row (`ai.allowance.updated`, `ai.credits.added`,
`ai.rules.updated`, `ai.credit_request.approved`, `ai.credit_request.declined`) with no actor,
`reason: "maintenance"` and the operator in `details`. The ledger (`app.ai_usage_ledger`) has a row
for every call the provider metered, with its token counts and the provider's request id; to
reconcile a month with the provider's invoice, sum its `credits` and compare the tokens. Neither the
ledger nor `audit_log` can be rewritten by the application role.

The credits tables exist whether or not `ANTHROPIC_API_KEY` is set, so allowances can be arranged
before the key is. Until R5's `grant_maintenance_role` (T-R5.17a) runs, use the owner or
maintenance credential; that migration must grant the role what these commands write
(`docs/plan/20-data-model.md` §46, "Privileges").

## What the assistant logs, and the live smoke

None of these lines carries the key, a prompt or a student's words; each names the feature and,
where the provider answered, its `request_id`, which is what the provider's console searches by.

| Line | Level | Means |
|---|---|---|
| `ai assistant enabled` / `ai assistant disabled (no ANTHROPIC_API_KEY)` | INFO | Start-up. The first names both models. |
| `anthropic_retry` | WARN | The provider answered 429 or 5xx, or a stream ended overloaded; the client waits and tries again, three times at most. |
| `ai call failed` | WARN | After the retries, or on a refusal or an answer that is not one complete JSON object. The teacher saw `503 AI_UNAVAILABLE`; what the provider billed was settled and the rest released. |
| `ai answer unusable` | WARN | The provider answered in the schema, but the operation's checks left nothing to return. `error` says why, with counts and never the answer's text. The teacher saw the same 503, and the call was billed. |
| `ai call rejected` | ERROR | The provider refused the request itself, which is the key, the account or a model name. Read the provider console. The teacher saw the same 503. |
| `ai call failed unexpectedly` | ERROR | An error that is none of the above. |
| `ai clean-up` | INFO | One bank clean-up: how many calls it made and how many gave a usable answer, whether the 45-second budget ran out, and every `request_id` of the pass. The ledger gets one row for the pass and keeps only the ids that fit 200 characters, so this line is where all of them are. |
| `ai clean-up: an answer had nothing usable` | WARN | One call of a clean-up answered something no decoder could use. It was billed and settled; the other calls' findings were returned with `truncated` set. |
| `ai clean-up: the smallest call can cost more than the reservation` | ERROR | At the configured prices not even one call of the clean-up fits its 200-credit ceiling, so none was made and nothing was reserved: the fast tier's prices are wrong. The teacher saw `503 AI_UNAVAILABLE`. |
| `ai reservation not ended` | ERROR | `Finish` could not settle or release a call. The sweep returns the credits once the reservation is ten minutes old. |
| `ai call cost more than its ceiling` | WARN | The provider metered more than the feature reserves. The teacher was charged the ceiling and the ledger keeps the real token counts. |

**Live smoke against the provider.** Before a release or a hotfix that touches the AI, run one real
call of each kind of feature with a key that is not production's:

```
QUIZZIVY_AI_TEST_KEY=… go test -tags aismoke ./internal/core/adapters/tests/ -run TestLive -v
```

It sends Generate, an explanation, wrong options, a score and a tagging (fast tier) with the models
in `QUIZZIVY_AI_TEST_CAPABLE_MODEL` and `QUIZZIVY_AI_TEST_FAST_MODEL` (default `claude-sonnet-5-5`
and `claude-haiku-5-5`), and fails unless every answer passes its decoder and Generate keeps all
five questions. It costs a few cents. Without the key it fails rather than skips, and CI never
builds the `aismoke` tag.
| `released stale ai reservations` / `ai reservation sweep failed` | INFO / WARN | The hourly sweep and its failure, which the next hour retries. |
| `import read with ai` / `ai import settled` | INFO | The worker's record of a scan it read: pages, pages it could not read, credits held and charged. |

A deploy while a bank clean-up runs (the server waits 15 seconds to shut down, and a pass can take
45) cuts the request before `Finish`, so the 200 credits it reserved stay held, with no line to say so.
The sweep returns them once the reservation is ten minutes old, at start-up and hourly, so for at most
an hour and ten minutes; nothing is charged, since nothing was settled.

**The live smoke** needs the owner's key and is not in CI. On the builder's machine, with the nine
variables in `.env` (`.env.example` lists them) and the API running against a database that holds
the seed: generate three questions from a test's passage, check the seeded test, suggest a score
for a seeded short answer that waits for a mark, run "Clean up" on a bank of a few hundred questions (the ledger gets one row for it, and its `request_id` lists the calls), and, for T-D4.33, create an import with "Read with
AI" on from a two-page scan of the corpus and let the worker read it. Then read the ledger back:
`maintenance ai-credits status` prints the pool's used credits and the teacher's use, and the rows
are `select feature, tier, credits, request_id from app.ai_usage_ledger order by created_at desc`
run as the owner. The pool's used credits must have moved by the sum of the rows. Paste the
credits and the ledger rows into the pull request, never the key and never a response, which may
hold a student's words.

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
