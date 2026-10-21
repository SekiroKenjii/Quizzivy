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

## Retention and anonymization

Approved on 2026-09-22 (integrity events) and made the organization's choice in R5 (D16): the
Admin's Data settings hold three retention choices, and a nightly sweep applies them. The audit
log is never swept. Retention uses UTC calendar arithmetic and a strict `<` against its cutoff:
`(now() at UTC − N months)`, so a row exactly at the cutoff stays until the next day.

| Setting | Choices | Default | What the sweep does |
|---|---|---|---|
| `keep_answers_months` | 12, 24, or never | never | An attempt that has ended (submitted, timed out, graded or voided) and whose assignment has closed for that student before the cutoff has each answer's `payload` replaced by `{"removed": true}`. For a student, the assignment closes at the later of its own close and the extension a teacher gave them, so an extended student's answers stay until that date is past the cutoff too. An attempt voided before it was submitted ended, for this purpose, at its deadline. Scores, comments and the grader stay. An attempt still in progress is never touched, however overdue (below). One `retention.answers_removed` entry per batch. |
| `keep_focus_events_months` | 6, 12, 13, 24 | 13 | Deletes `attempt_events` whose `received_at` is before the cutoff and whose assignment has closed for that attempt's student before it, with the same close as the answers rule: a later close a teacher gave that student holds their events back until it passes the cutoff too. The command it replaces read the assignment's own close alone, which predates the per-student close. One `integrity.retained` entry per batch. |
| `anonymise_disabled_after_months` | 12, 24, or never | never | Anonymises accounts disabled before the cutoff, of any role. One `user.anonymized` entry per account. |

An account that owns classes or content (a class it teaches, or a test, question, group, media file or
assignment it owns) is skipped and counted (`skippedOwners`): transfer the content, then it is picked up on a
later run. Having created an account is not ownership: an Admin reaches it through the Users directory.
An account with an attempt in progress is skipped for now (`skippedInProgress`). The last active Admin can never be anonymized. The three counts split the accounts that are due: `anonymise`, `skippedOwners` and `skippedInProgress` add up to them, an owner who also has an attempt in progress is counted once, as an owner, and a dry run's `anonymise` is what the apply anonymises. If a batch fails, its accounts are tried one by one, the others are anonymised, `failedAccounts` counts the ones that could not be, and the step ends with an error, so the job fails every night until someone looks at that account. Before the name is
overwritten, in-app notifications that carry the account's full or display name are deleted
(`notices`), so no name outlives the anonymization there. Anonymization replaces the email with
`anonymous-<id>@anonymous.invalid`, the name with a placeholder ("Học viên đã ẩn danh" for a
student, "Người dùng đã ẩn danh" for anyone else), clears the display name, phone, avatar key and
password, removes the Google link and the refresh tokens, and revokes unused set-password links. It
clears the avatar *key* only: the photo object under `avatars/<id>/` is not deleted by this step, because
the maintenance role holds no storage credential. The daily housekeeping of the API removes every object
under that prefix at its next run (where media storage is configured), so until then the object stays in
storage with nothing pointing at it. A
revoked link holds a hash of a random token, its purpose, its dates and the account's id, which is no
personal data, and is as unusable as a deleted one; the daily housekeeping removes the row 30 days later.
Attempts, memberships and the audit trail stay, which is why this is **structured-identity
anonymization**, not a guarantee that all personal information disappears: teacher notes, free
text in answers kept under the settings, existing audit diffs, IPs and backups may retain
identifying material. Review free text separately under an explicit request. Restrict access to
backups and record how a restore will reapply later anonymizations before allowing the restored
system to serve users. Existing access JWTs can stay valid until `ACCESS_TOKEN_TTL` expires.

Every sweep audit entry has no actor and carries `databaseRole` in its diff.

### The maintenance role

`cmd/maintenance` connects as `quizzivy_maintenance`, never as the owner and never as the API's
role. It is created by `scripts/provision-db.sh` (`LOGIN`, `CONNECTION LIMIT 2`, not a
superuser, no `CREATE` anywhere) with the password in `QUIZZIVY_MAINTENANCE_PASSWORD`
(`maintenance` locally). The script is idempotent, and on a database that is not local it touches
only this role (step 1 below). Migration `00139_grant_maintenance_role.sql` gives it
exactly what the commands use, column by column, and fails when the role does not exist. It can
read what the commands' predicates read, delete from `attempt_events`, `user_identities`,
`refresh_tokens`, `notifications` and `ai_reservations`, write `payload` on `attempt_answers`,
the identity columns on `users` (only to anonymise them, below), and insert into `audit_log`. It cannot `UPDATE` or `DELETE` the
audit log, cannot touch `ai_usage_ledger`, and owns nothing.
`server/internal/platform/db/tests/privileges_test.go` pins the whole list; a command that needs
more adds the grant in its own migration.

**Never set `MAINTENANCE_DATABASE_URL` on the API or on Fly.** The credential lives in two
places: the GitHub environment `production-maintenance` (as the secret
`MAINTENANCE_DATABASE_URL`, read by the retention workflow) and the operator's own machine.

**What a leaked `MAINTENANCE_DATABASE_URL` costs.** The role is the operator role for every
`cmd/maintenance` command, not only the sweep, so its reach is the sum of them. Whoever holds the
credential can read every answer payload and every account's name and email; move exam windows
(`closes_at`, `deadline_at`); change the AI allowance and credit counters; rewrite join-code
ciphertexts; delete focus events and sessions (refresh tokens); and anonymise any account that
is not the last active Admin, a teacher who owns classes or content included, which cannot be
undone for that account's sign-in. It cannot sign in as anyone and cannot read a password or a token. A trigger on
`app.users` (migration `00139`) refuses, with `42501`, every update the role makes that does not
leave the account anonymised (the reserved address, no password hash, no phone, photo or display
name, and disabled), so the role can set neither an email nor a password hash nor enable an
account again, alone or together; and it holds no `INSERT` on `user_identities` or
`set_password_tokens`, so it cannot reopen an anonymised row either. The database refuses to
anonymise only the last active Admin; skipping an owner of classes or content, or an account with
an attempt in progress, is the sweep's and `anonymize-user`'s rule, which direct SQL with the
credential does not follow. It also cannot change a `role_id`, update or delete the
audit log or the AI ledger, or create or own a table, schema or function. It can change its own
password and create large objects, because PostgreSQL gives every role both.

The credential lives only in the environment secret and on the operator's machine. The repository's
deploy credential reaches the application's own database URL, so it is a larger secret than this
one and is the owner's too; a second, retention-only role for the workflow would shrink what is
left here, and is open item O-R5.9 (`docs/plan/40-open-items.md`), to be done if the workflow ever
runs another command or a second consumer holds the connection string. To revoke the credential,
run `ALTER ROLE quizzivy_maintenance NOLOGIN` as the owner. To rotate it, run
`ALTER ROLE quizzivy_maintenance PASSWORD '<new random password>'` as the owner (step 1 below does
the same), then replace the secret `MAINTENANCE_DATABASE_URL` with the new connection string.

### The nightly workflow

`.github/workflows/retention-sweep.yml` runs at `0 20 * * *` UTC (03:00 in Ho Chi Minh City)
and on `workflow_dispatch` with an `apply` input. It runs in the environment
`production-maintenance`, builds `cmd/maintenance`, runs `maintenance retention-sweep -budget 5m`
(`timeout-minutes: 15`, one run at a time) and prints the report into the job summary. A
schedule applies only when the repository variable `RETENTION_SWEEP_ENABLED` is `true`; otherwise
it is a dry run, as is a manual run whose `apply` box is off. GitHub runs a schedule, and lists
a manual workflow, only from the default branch, so the workflow starts to run when the release
that carries it reaches `main`.

The owner sets this up once, in this order:

1. Create the role, before the deploy: the release command runs migration `00139`, which fails,
   and so stops the release, while the role does not exist. Choose a new random password and run
   this from a shell that has not sourced `.env` (its development passwords would otherwise be in
   the environment):

   ```sh
   QUIZZIVY_MAINTENANCE_PASSWORD='<new random password>' scripts/provision-db.sh "$NEON_ADMIN_URL"
   ```

   Against a database that is not local, the script creates `quizzivy_maintenance` and sets its
   password, and nothing else about the other two roles changes. It never sets the password of an
   existing `quizzivy_migrate` or `quizzivy_app` (an exported `QUIZZIVY_MIGRATE_PASSWORD` or
   `QUIZZIVY_APP_PASSWORD` is ignored, with a note), never grants `CREATEDB`, refuses a
   development default such as `maintenance`, and refuses to create a role whose password
   variable is not set. The admin need not be a superuser: the script names no attribute that only a
   superuser may change, and alters `quizzivy_maintenance` only where it differs from `LOGIN`, no
   `CREATEDB`, no `CREATEROLE` and `CONNECTION LIMIT 2` (a managed provider's admin has `CREATEROLE`
   and `CREATEDB` and is not a superuser; the script once failed there after creating the roles,
   QC-RS-16). Use the provider's own host. A tunnel to a production database on `localhost`
   (`fly proxy`, an SSH forward) looks local to the script, which says so on stderr before it runs a
   statement and then goes on with the local rules, so it cannot be stopped in time; run it with
   `QUIZZIVY_MANAGED=1` and it applies the rules above. It repeats the grants of the original setup (`CONNECT` on the database,
   `public` denied to everyone but the migrate and app roles), which change nothing on a database
   already provisioned. Its last lines name the roles whose password it set: read them, and only
   `quizzivy_maintenance` should be listed. `scripts/ci/tests/provision-db.test.mjs` pins all of
   this. To rotate the maintenance password later, run it again with the new value and replace the
   secret; rotate the other two roles' passwords in the provider's console. On a new managed
   database, set all three variables to new random values.
2. Create the GitHub environment `production-maintenance`. Restrict its deployment branches to
   the default branch **before** adding the secret, and check the restriction in Settings,
   Environments before the secret goes in: a manual run can be started on any branch, and only
   that restriction keeps a pushed branch from running its own code with the credential. Do not
   add required reviewers. A reviewer rule holds the scheduled run as well as the manual one, so
   the nightly sweep would wait for a person every night; the branch restriction is the control,
   and it can be verified. Then add the secret `MAINTENANCE_DATABASE_URL` for
   `quizzivy_maintenance`: the direct connection string, not the pooled one, with
   `sslmode=require` kept, because the credential crosses the internet from a GitHub runner. The
   workflow runs only GitHub's own `actions/checkout` and `actions/setup-go`, each pinned to a
   commit SHA with its tag in a trailing comment (stricter than `docs/setup/ci.md` asks of the
   other workflows), because this job holds a database credential. The checkout leaves no
   repository token in `.git/config` (`persist-credentials: false`) and `setup-go` restores no cache
   (`cache: false`). Any action added to it is pinned the same way, and
   `scripts/ci/tests/retention-sweep.test.mjs` fails otherwise.
3. Run the workflow by hand with `apply` off and read the report (below).
4. When the dry run counts what you expect, set the repository variable
   `RETENTION_SWEEP_ENABLED` to `true`.

To pause the sweep, unset the variable or set it to anything but `true`: the next night is a dry
run. Disabling the workflow in the Actions tab also stops it.

### Reading the report

The report is JSON; with `-apply` off it counts what the sweep would do and writes nothing (it
runs in a read-only transaction).

```json
{
  "applied": false,
  "databaseRole": "quizzivy_maintenance",
  "retention": {"keepAnswersMonths": 24, "keepFocusEventsMonths": 13, "anonymiseDisabledAfterMonths": null},
  "answers": {"enabled": true, "cutoff": "2024-10-14T20:00:07Z", "attempts": 12, "answers": 310, "batches": 1, "overdueInProgress": 0},
  "focusEvents": {"cutoff": "2025-09-14T20:00:07Z", "events": 4200, "batches": 9},
  "accounts": {"enabled": false, "anonymise": 0, "skippedOwners": 0, "skippedInProgress": 0, "notices": 0, "batches": 0, "failedAccounts": 0},
  "complete": true
}
```

- `databaseRole` must be `quizzivy_maintenance`. Anything else means the wrong credential is in
  the secret: stop.
- `retention` is the setting the sweep read; check it against the Admin's Data settings page
  before the first apply. A step with `enabled: false` is switched off by its setting and does
  nothing.
- `complete: false` means the 5-minute budget ended a step or a step failed; the next night goes
  on from where it stopped, because every batch commits on its own. A step that failed is listed
  in `failed` with its error, the other steps still ran, and the job fails (exit 1, with the report in the
  job summary). GitHub emails only the person who last changed the workflow's cron line, and only if their
  notification settings allow it, and it disables a scheduled workflow in a repository with no activity for 60
  days, which looks the same as a sweep with nothing to do. Open the workflow's job summary every week, or add
  a failure notification to the repository, and check that the schedule still shows as enabled.
  A backlog the first night does not clear is normal.
- `overdueInProgress` counts attempts still in progress past their deadline on an assignment that
  closed before the answers cutoff. The sweep never removes their answers: an overdue attempt is
  closed, and scored from its stored answers, only when a teacher's monitor reads its assignment,
  and an answer already removed would be scored blank for good. Until it is closed, such an
  attempt keeps its answers and holds its student back from anonymization (`skippedInProgress`).
  A figure above zero is for the owner to decide on; nothing closes them by itself
  (`docs/plan/40-open-items.md`, O-R5.8).
- On a dry run, `batches` is the number of batches an apply would take.

### By hand

The same binary runs the sweep from the operator's machine, with `MAINTENANCE_DATABASE_URL` set
to the role's connection string and nothing else changed:

```sh
maintenance retention-sweep -budget 5m
maintenance retention-sweep -budget 5m -apply
maintenance retention-sweep -batch 200 -apply
maintenance retain-integrity -batch 1000 -apply
maintenance anonymize-user -user USER_UUID
maintenance anonymize-user -user USER_UUID -apply
```

`-batch` (default 500, 1 to 10000) is the rows in one batch. `retain-integrity` is the focus-events step alone, with the configured months and one batch per
run; repeat until it reports zero rows. `anonymize-user` handles one account of any role on an
explicit request (`anonymize-student -student UUID` is still accepted): it refuses an account
that owns classes or content, an account with an attempt in progress and the last active Admin,
and is idempotent. Every command takes `-timeout <duration>` (default `1m`; `retention-sweep`
takes `-budget` instead, which is also its timeout), and writes are dry runs until `-apply`. An
unknown command or flag prints the usage line and changes nothing. The older order, flags before
the command, is still accepted for `retain-integrity` and `anonymize-student`.

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

A logo is stored once under `branding/{sha256}.png` in the media bucket and is never
replaced. One that neither the light nor the dark logo names is removed by the daily
housekeeping once it is 24 hours old ("Daily housekeeping"). Without object
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

The window commands run as `quizzivy_maintenance` ("Retention and anonymization" above),
which migration `00139_grant_maintenance_role.sql` gives `INSERT` and `UPDATE` on
`maintenance_windows`, `UPDATE (deadline_at)` on `attempts`, `UPDATE (closes_at)` on
`assignments` and `SELECT` with `UPDATE (closes_at)` on
`app.assignment_student_overrides`. A credential without a grant fails the apply with a
permission error, and the transaction writes nothing. Run them from the operator's machine
with `MAINTENANCE_DATABASE_URL`; no workflow runs them.

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

The rekey runs from the operator's machine, never from a scheduled workflow, and as
`quizzivy_maintenance`: the operator supplies `MAINTENANCE_DATABASE_URL` for that role and
`JOIN_CODE_KEY` and `JOIN_CODE_KEY_PREVIOUS` from the offline copies to `cmd/maintenance`.
The role can read `class_join_codes` and write only `code_hash`, `code_ciphertext` and
`key_id`. The GitHub environment `production-maintenance` holds the role's connection string
for the retention workflow and no join-code key.

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
   - `legacy` counts every code issued before v0.8.0. They hold no
     ciphertext and cannot be re-keyed. From v0.12.0 every one is revoked
     (migration `00141`, see "Legacy join codes" below), so it has nothing to
     re-key.
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
bare origin (no path, `?`, `#` or whitespace). The emails-per-day and emails-per-month limits ("The centre's usage limits",
below) count sends per local day and month in `APP_TIMEZONE`; the excess moves to
the next local midnight or the 1st, and a held message whose link has expired is
failed with code `expired` instead of sent.
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

## Daily housekeeping

Each API machine runs the housekeeping when it starts and then every 24 hours,
on the tick of the refresh-token prune, so it adds no timer and no database
wake of its own. It removes, in this order: `user_activity_days` rows older
than 400 days; outbox rows sent or failed more than 30 days ago (a pending row
never); set-password links that were used, revoked or expired more than 30 days
ago (a link that still works never); the sealed passwords of a hand-out sheet
nobody downloaded in its 24 hours (where the import worker is configured); user
imports, with their rows, 30 days after they finished (one still waiting or
running never; this step runs without the import worker); the files of data
exports past their 24 hours, which are then marked `expired` (this step needs `IMPORT_S3_BUCKET`); the
profile photo objects under `avatars/<id>/` of every anonymised account, once per account (this step
needs media storage); and
the `branding/` logo objects that neither the light nor the dark logo names, once
they are 24 hours old (this step needs media storage). Two machines running it
at once only repeat each other's deletes, and one can end a step early, as done,
when the other has taken the rows it was about to; nothing is lost and the next
run goes on.

Every step handles 500 rows or objects at a time, at most 20 times and for at
most a minute, so a run clears 10,000 rows or objects of a step at most and a
large backlog goes over several days rather than in one long transaction. The
exceptions are the import step, which counts jobs: a full batch of 500 jobs with
2,000 rows each is one statement that measured about three seconds and 54 MB of
write-ahead log; and the photo step, which counts accounts and lists and deletes
each account's objects one by one, so it is the step most likely to report
"left work for the next run" after a large anonymisation sweep. The logs say how each went and carry counts, never a key, id, address, endpoint or
bucket:

- `housekeeping step done` with `step` and `removed`: the last batch came back
  short. It does not promise nothing is left; the next run goes on;
- `housekeeping step left work for the next run`, when the backlog outlasted the
  step's rounds or budget;
- `housekeeping step failed` with `code=HOUSEKEEPING_STEP_FAILED`, the `step` and
  a `failure` class: `timeout`, `canceled`, `database {SQLSTATE}`,
  `storage http {status}`, `network` or `error`. The error's own text is
  withheld because it names keys and the bucket; reproduce it from the class and
  the step. The other steps still ran. A step that fails every day (a bucket
  that denies deletes, say) is the one to alert on.

A download link signed in the last minute before an export expires stops
working once the sweep has deleted the file.

There is no switch. A logo the settings still name is never deleted. An import
that never finishes is never removed either: on a server whose import worker
has been switched off with jobs still waiting, the names and addresses of their
rows stay until the worker is switched on and the jobs finish, or until someone
deletes those jobs by hand. The import step can wait for rows that a class or a
user deleted at the same moment holds, and be chosen as a deadlock victim; the
step then logs `database 40P01` and the next run goes on (D-044).

The photo step marks an account in `app.avatar_sweeps` once its prefix is empty
and never lists it again (D-050). An account whose objects could not be listed
or deleted is not marked and is tried again at the next run. A server without
media storage runs no photo step and marks nothing, so turning storage on later
still finds every anonymised account. A photo written under a marked account's
prefix afterwards (which only a restored backup or a manual upload could do) is
not removed; delete it by hand.

## The centre's usage limits

The limits live in the settings document and an Admin edits them in Settings → Plan & usage (CR-002,
D-060). Each is resolved, in order, as the value stored there, else a deploy variable when one is set
(`MEDIA_OWNER_QUOTA_MIB`, `IMPORT_GLOBAL_MIB`, `IMPORT_ACTOR_MIB`, `EMAIL_DAILY_LIMIT`), else the hosting
plan's figure in `server/internal/shared/limits`, read on 2026-10-21:

| Limit | Default | Source | What happens at it |
|---|---|---|---|
| Accounts (not disabled) | none | — | a new, re-enabled or joining account is refused (`SEAT_LIMIT`) |
| Teachers (enabled, role opens the teacher workspace) | none | — | a new teacher or a role change to one is refused (`TEACHER_LIMIT`) |
| Total file storage | 10 GB | Cloudflare R2 free | an upload or an import is refused |
| Media library per teacher | 5 GB | the app | that teacher's upload is refused |
| Import documents, centre / per teacher | 1 GB / 256 MB | the app | a new import is refused |
| Emails per day / per month | 100 / 3,000 | Resend free | the rest waits for the next local day or the 1st |
| Database size | 10 GB | warning line for Neon Launch | the Overview warns; nothing is refused |
| Warn at | 80% | the app | the share at which the Overview's storage row appears |

Leave the deploy variables unset in production so the page is the one place a limit is changed. If
Resend or R2 moves off its free plan, raise the matching limits on the page; if Neon's plan changes,
move the database line. The guards for accounts and teachers count under advisory lock `(73819, 45)`
and the storage guard under `(73819, 46)`, each only while its limit is set. Database size is
`pg_database_size`, which is close to but not what Neon bills. Days and months are local to
`APP_TIMEZONE`. The `PLAN_*` variables of v0.12.0 are retired and ignored.

## Legacy join codes

A code issued before v0.8.0 was held only as its SHA-256 (lookup scheme 1).
v0.10.0 to v0.11.x replaced each live one, once, at API start-up
(`jobs.RotateLegacyJoinCodes`, T-R4.22). v0.12.0 removes that job and the
scheme-1 lookup (T-R5.1): a code is found only by its keyed hash, so an old
revoked or expired legacy code answers as an unknown code, and migration
`00141` adds `class_join_codes_live_encrypted`, which refuses any unrevoked code
without a ciphertext.

`00141` stops with an error if a live legacy code is still there. That cannot
happen after a v0.10.0 start-up; if it does, start a v0.11.x API once before
deploying v0.12.0. The check it runs, on a Neon branch before the release:

```sql
SELECT count(*) AS live_legacy_codes FROM app.class_join_codes
 WHERE revoked_at IS NULL AND expires_at > now() AND code_ciphertext IS NULL;
```

## Stored emails the API cannot print

The contract's `email` format used to accept `"qc tp"@example.com`: the request
passed, the generated type stored the unquoted `qc tp@example.com`, and the same
type could not print that value back, so the write committed and answered 500
and every list holding the row answered 500 afterwards. The `email` format now
accepts an address only if the bare address it yields parses to itself
(`validation.ParseEmail`), so the row can no longer be created. Nothing in the
release rewrites a stored value.

Release check: run this read-only query with any credential that can read
`app.users`, before and after the deploy. It lists accounts whose local part (the
text before the last `@`) holds an ASCII control character, a space, any of
`"(),:;<>@[\]`, DEL, or that begins or ends with a dot or holds two in a row.
Nothing non-ASCII is refused by Go's address parser, so it has no Unicode clause.
It finds every row the old API could have written. It reads only the local part,
so a row inserted by hand with a defect in its domain, or with an empty local part,
is not listed; the API cannot create one. `stored_email_test.go` runs this text
against `mail.ParseAddress`:

```sql
SELECT id, email
  FROM app.users
 WHERE regexp_replace(email, '@[^@]*$', '') ~ '[\x01-\x20"(),:;<>@\[\\\]\x7f]|^\.|\.$|\.\.'
 ORDER BY created_at, id;
```

No rows is the expected answer. If it returns rows, nothing in the release repairs
them; the repair is per row and the owner's.

- **A student.** An Admin, or the teacher who owns the student, corrects the address
  with `PATCH /teacher/students/{id}`. A teacher who only shares the student gets
  `403 STUDENT_SHARED`.
- **A staff account (a teacher or an Admin).** `PATCH /teacher/students/{id}` answers
  404 for it, and the API has no operation that disables an account, so the repair is
  one statement run by the owner with a credential that may write `app.users`:
  `UPDATE app.users SET email = '<corrected address>' WHERE id = '<id from the query>';`
  Write the id and the change in the release record.

Until a row is repaired, these keep answering `500 INTERNAL`: `GET /teacher/students`
for a page holding the row (also with a `q` that matches it) and `GET
/teacher/students/{id}`, `POST /teacher/students/reset-passwords`, `GET
/teacher/classes/{id}/members` for a class holding the student, `GET
/teacher/attempts/{id}` for that student's attempt, and, for the account's own
session, `GET /auth/me` and `PATCH /auth/me`. The account cannot sign in again: the
quoted form is refused, and the stored form fails the sign-in request's own check
with `400`. A session it already holds keeps refreshing.

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

Since v0.12.0 the Admin's AI credits page (T-R5.35) does all of this in the console;
`cmd/maintenance ai-credits` stays as the operator's tool, run with `MAINTENANCE_DATABASE_URL` as the other commands are. Every write names
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
before the key is. These commands run as `quizzivy_maintenance`; migration
`00139_grant_maintenance_role.sql` grants it what they write and never the ledger
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
| `ai variant` | INFO | One "Make a variant": how many calls it made (8 questions to a call), how many gave a usable answer, whether the 45-second budget ran out, and every `request_id` of the pass. The ledger gets one row for the pass. |
| `ai clean-up: an answer had nothing usable` | WARN | One call of a clean-up answered something no decoder could use. It was billed and settled; the other calls' findings were returned with `truncated` set. |
| `ai clean-up: the smallest call can cost more than the reservation` | ERROR | At the configured prices not even one call of the clean-up fits its 200-credit ceiling, so none was made and nothing was reserved: the fast tier's prices are wrong. The teacher saw `503 AI_UNAVAILABLE`. |
| `ai reservation not ended` | ERROR | `Finish` could not settle or release a call. The sweep returns the credits once the reservation is ten minutes old. |
| `ai call cost more than its ceiling` | WARN | The provider metered more than the feature reserves. The teacher was charged the ceiling and the ledger keeps the real token counts. |

**Live smoke against the provider.** Before a release or a hotfix that touches the AI, run one real
call of each kind of feature with a key that is not production's:

```
QUIZZIVY_AI_TEST_KEY=… go test -tags aismoke ./internal/core/adapters/tests/ -run TestLive -v
```

It sends Generate (from a passage, and standalone for a section), an explanation, wrong options, a score and a tagging (fast tier) with the models
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
- Email (v0.12.0): one real message from `quizzivy.com` whose headers show SPF, DKIM
  and DMARC passing (`dns.md`, "Email (Resend)"): pending the release.
- Retention sweep (v0.12.0): the first manual dry run of `retention-sweep.yml` on
  production, read by the owner before `RETENTION_SWEEP_ENABLED` is set ("The
  nightly workflow"): pending the release.
