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
Flags must precede the command:

```sh
maintenance -batch 1000 retain-integrity
maintenance -apply -batch 1000 retain-integrity
maintenance -student STUDENT_UUID anonymize-student
maintenance -apply -student STUDENT_UUID anonymize-student
```

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

## Evidence still required in production

- Neon project/branch and observed history window: pending access.
- PITR drill, application checks and private media restoration: pending access.
- Monitor recipient and delivered test notification: pending owner setup.
- Header/CSP behavior after deployment: pending deployment.
- Maintenance dry-run review and first authorized production batch: pending.
