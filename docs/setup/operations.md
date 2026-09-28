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
upload previews on the deployed origin before closing #78.

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

Scheduling is a dry run until `-apply`. The dry run reports how many attempts and
assignments would move; read it before applying.

What moves when a window is scheduled, in the same transaction, audited as
System (no actor, `reason: "maintenance"`, the window id and the database role):
- every attempt still in progress whose deadline is after the window's start
  gets the window's length added to its deadline (`attempt.extended`);
- every open assignment that would close inside the window closes that much
  later (`assignment.extended`).

`window-cancel` cancels a window that has not ended; `window-end` ends an active
window now. Both keep the extensions already granted: a deadline that moved does
not move back. `window-list` prints the upcoming and active windows.

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
   - `legacy` counts the codes issued before v0.8.0. They hold no ciphertext
     and cannot be re-keyed; R4 rotates them.
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

## Evidence still required in production

- Neon project/branch and observed history window: pending access.
- PITR drill, application checks and private media restoration: pending access.
- Monitor recipient and delivered test notification: pending owner setup.
- Header/CSP behavior after deployment: pending deployment.
- Maintenance dry-run review and first authorized production batch: pending.
