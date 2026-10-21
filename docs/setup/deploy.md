# Deploying

Merging `release/*` into `main` ships. Nothing else does, and nothing deploys
from `develop`.

```
feature/*  →  develop  →  release/x.y  →  main  →  CI  →  Deploy
                                                    │
                                          only if CI is green
```

`.github/workflows/deploy.yml` runs on `workflow_run` after **CI** concludes on
`main`, not on the push itself. That ordering is the point: a merge that breaks
something is not deployed while its own test run is still red. CI runs every
job on a push to `main`, whatever passed before (`docs/setup/ci.md`).

Only the CI run of a **push** deploys, and only while its commit is still the
tip of `main`. The back-merge pull request from `main` to `develop` has `main`
as its head branch too, and through v0.8.0 its CI run deployed the same commit
a second time. Re-running an older CI run of `main` to green completes it again;
the tip check is what keeps that from putting an earlier release back.

## What it does

| Job | Target | How |
|---|---|---|
| `api` | Fly.io `quizzivy-api` | `flyctl deploy --remote-only` |
| `api-vps` | the VPS, instead of `api` | build and push the image to `ghcr.io`, then `ssh deploy@host deploy <sha>` |
| `web` | Cloudflare Pages `quizzivy-web` | `pnpm build` then `wrangler pages deploy` |

The API goes first. The SPA is the half that calls the other, so the window
between the two deploys is old-SPA-against-new-API rather than the reverse. That
is safe while a release only adds to the contract. A release that tightens a
request or adds a refusal (v0.7.0 did both) needs a server message an older tab
can show as it is, and a line in the release notes for tabs loaded before the
deploy. The reverse is never safe: a new SPA calling an endpoint that has not
shipped yet is a broken screen.

Migrations ride along with the API. `fly.toml`'s `release_command` applies them
before the new version takes traffic and rolls the deploy back if they fail,
which is why a schema change and the code that needs it must ship as one merge.

## What you have to set up once

The workflow fails with a named error before touching production if any of these
is missing, rather than half way through.

```bash
# Fly — a deploy-scoped token, not your personal one
fly tokens create deploy -a quizzivy-api
gh secret set FLY_API_TOKEN

# Cloudflare — an API token with "Cloudflare Pages: Edit" on this account
gh secret set CLOUDFLARE_API_TOKEN
gh secret set CLOUDFLARE_ACCOUNT_ID

# Not a secret: it travels in the authorization URL the browser follows.
# A variable, so it is readable in logs when a build looks wrong.
gh variable set VITE_GOOGLE_CLIENT_ID --body '<client-id>.apps.googleusercontent.com'
```

`VITE_GOOGLE_CLIENT_ID` is worth the fuss because a build without it does not
fail — it just hides the Google button, which is the kind of breakage nobody
reports and everybody works around.

Check what is set:

```bash
gh secret list && gh variable list
```

## Choosing where the API runs

The repository variable `API_HOST` picks one of the two API jobs: `fly` (the
default when it is unset) or `vps`. Any other value stops the run before
anything deploys. `web` waits for whichever job ran. Switching is therefore a
variable, not a code change, and so is switching back:

```bash
gh variable set API_HOST --body vps    # or: fly
```

`api-vps` needs three secrets and nothing else on the GitHub side:

```bash
gh secret set VPS_HOST          # the box's address, as DNS-only or its IP
gh secret set VPS_SSH_KEY       # private half of the deploy key (ed25519)
gh secret set VPS_KNOWN_HOSTS   # ssh-keyscan -t ed25519 <host>, checked against the box's console
```

The public half of the key goes to the box as `QUIZZIVY_DEPLOY_PUBKEY` when
`bootstrap.sh` runs; the box then forces that key's command to `deploy.sh`, so
the key can deploy a commit and do nothing else. The job tags the image
`ghcr.io/<owner>/quizzivy-api:<sha>`; the box's `compose.env` must name that
repository in `API_IMAGE_REPO`, and `docker login ghcr.io` there needs a token
with `read:packages` only. Everything on the box side (PostgreSQL, Caddy,
backups, rollback) is in `github.com/Quizzivy/infra`, `docs/vps.md`.

Migrations run on the box before the new API starts, as `fly.toml`'s
`release_command` does on Fly, and a failed health check puts the previous image
back. The `Fly-Client-IP` setting in `fly.toml` does not apply there: the box
reads `CF-Connecting-IP`, which only Cloudflare can set.

The preflight below is Fly-only. The box checks its own `app.secrets.env` when
the API starts, and the log says which name is wrong.

## The preflight

Before anything is built, the API job unions `fly.toml`'s `[env]` with
`flyctl secrets list` and checks that the result is a configuration the app
accepts. It reports missing names in seconds instead of after a build, a
migration, a rollout and a health check.

It exists because `config.Load` has all-or-nothing groups — Google sign-in,
object storage and, from v0.12.0, email (`RESEND_API_KEY`, `EMAIL_FROM`,
`APP_URL`, all required in production, with `SECRETS_KEY`) — and each exits 1
on a partial set. Both shipped partial,
one after the other, and the cause was legible only in the app's own stderr:
Fly reports it as "the app appears to be crashing" and prints an empty log tail,
because the machine is gone before it attaches.

Names only. A secret set to the wrong *value* passes here; that is a different
failure with a different symptom.

## The build-time trap this guards against

`VITE_*` values are inlined into the bundle at build time; there is no runtime
override. `src/lib/api/client.ts` falls back to `http://localhost:8080` when
`VITE_API_BASE_URL` is unset, so a mis-scoped variable produces a bundle that
builds, uploads, serves, and cannot reach anything — with no error anywhere.

The `web` job greps the built bundle for `https://api.quizzivy.com` and fails if
it is absent. Do not remove that step.

## Running it by hand

Actions → **Deploy** → *Run workflow*, with a target of `all`, `api` or `web`.

Two reasons this exists:

- **A failed deploy can be retried on its own.** An expired token or a Fly
  region hiccup should not cost twenty minutes of tests that already passed on
  the same commit.
- **The very first merge may not trigger it.** `workflow_run` reads the workflow
  definition from the default branch, so `deploy.yml` has to already be on
  `main` for a run to fire. It arrives there in the same merge it would fire on.
  If nothing happens after the first release, dispatch it by hand; subsequent
  merges are automatic.

## Cutting a release

```bash
git checkout -b release/0.9.0 develop
# only fixes on this branch -- no new features
git push -u origin release/0.9.0 && gh pr create --base main   # merging it pushes main: CI runs, then Deploy
git fetch origin && git tag v0.9.0 origin/main && git push origin v0.9.0
gh pr create --base develop --head main                        # back-merge; its CI run does not deploy
```

Merge back into `develop` too, or fixes made on the release branch are lost from
the next one. `main` and `develop` take changes only by pull request
(`ci.md`, "Branch rules").

## Verifying afterwards

The workflow checks `/healthz` reports `database:ok` before it calls the API
deploy done. Fly's routing check uses `/livez` instead, which does not query the
database (see `operations.md`). The rest is by hand:

```bash
curl -s https://api.quizzivy.com/healthz
curl -o /dev/null -w '%{http_code}\n' https://app.quizzivy.com/join/K7M3-P9QR   # 200, the SPA shell
GOOGLE_REDIRECT_URI=https://app.quizzivy.com/auth/google/callback make verify-google
```

The deep-link check is not decoration: `web/public/_redirects` is what makes a
QR-code link work on a cold load, and it fails silently at the CDN if it is ever
dropped from the build output.

## Still manual

Both need dashboard access the project's tokens do not have. See `dns.md`:

1. `api` — add the A/AAAA records **DNS only**, wait for
   `fly certs check api.quizzivy.com`, then switch to proxied.
2. `app` — Pages project → Custom domains → add `app.quizzivy.com`. Cloudflare
   creates the record itself; making it by hand returns 522.

## Word and PDF import

The deploy runs two process groups from one image: `app` (the API, the only one
with an HTTP service) and `worker` (`/app/import-worker`, on its own 1 GB
Machine). The API wakes the worker over Fly's private network. The first deploy
that declares the group creates the worker Machine, so `flyctl machines list`
shows one of each afterwards. Before that deploy, the R2 token must reach
`quizzivy-imports` and `make verify-r2-imports` must pass. The steps, and how to
turn import off again, are in `word-import-worker.md` § Production.

The deploy runs `flyctl deploy --ha=false`. Without the flag, flyctl gives a
group that has no machine yet a stopped standby, and the next step would start
it as a second worker. That step now starts only non-standby machines, and fails
unless every group in `[processes]` has a started machine.

## Rolling back from v0.6.0: roll forward instead

Migrations 00032–00052 only add columns, tables and constraints, so the v0.5.0
image still boots on a v0.6.0 database, and goose reports nothing to apply.
Once v0.6.0 has written shared-context groups, gap bindings or rich content,
for example by committing a Word import, v0.5.0 no longer handles them:

- Its question edit rewrites blanks without `gap_id`. On a grouped fill-blank
  question the deferred gap-binding foreign key then fails with a 500, and on a
  standalone one the bindings are dropped.
- Its soft delete of a group member violates `questions_context_complete`.
- Its readers skip group passages and shared recordings.
- Its media delete does not see group references.

So recover from a v0.6.0 problem with a hotfix, not by redeploying v0.5.0. The
down migrations for the group graph refuse to run once group data exists.

Two migrations, `00036` and `00038`, build indexes `CONCURRENTLY` outside a
transaction. If one is interrupted, drop the index it left behind before
redeploying, as `quizzivy_migrate`:

```sql
DROP INDEX CONCURRENTLY IF EXISTS app.questions_context_ordinal_key;
DROP INDEX CONCURRENTLY IF EXISTS app.questions_context_identity_key;
DROP INDEX CONCURRENTLY IF EXISTS app.tvq_section_identity_key;
```

Deploy outside active assignment windows. The other new migrations hold brief
ACCESS EXCLUSIVE locks on test-version tables while they validate new CHECKs.

## Rolling out v0.8.0 (R2): forward only

Before merging `release/0.8.0` into `main`:

- Set `JOIN_CODE_KEY` as a Fly secret and keep its offline copy
  (`operations.md` § Join-code key). The preflight fails the deploy without it,
  and v0.8.0 refuses to start without it. Leave `JOIN_CODE_KEY_PREVIOUS`
  unset.
- Rehearse the migrations on a Neon branch of production
  (`docs/plan/72-r2.md` § Release checklist).

Deploy only when no assignment window has attempts in progress: scoping
changes the attempt queries, and the files that add and backfill a column
(`00056`, `00060`–`00063`, `00065`) hold ACCESS EXCLUSIVE on their table until
they commit.

While v0.7.0 and v0.8.0 machines overlap:

- **Do not rotate join codes.** A code v0.8.0 issues is sealed and found by a
  keyed hash, which a v0.7.0 machine cannot look up. Codes issued before the
  deploy redeem on both.
- **Do not reset student passwords or disable students.** Either moves the
  student's session epoch, and v0.8.0 refuses a token older than it. A v0.7.0
  machine issues tokens with no epoch, so a student it signs in stays signed
  out until the old machine is gone.

Tabs still open on v0.7.0 kept working through an `/admin` alias that served
the old teaching paths and logged `legacy_admin_path`. v0.12.0 removes it
(`docs/plan/75-r5.md` T-R5.36), and a former teaching path under `/admin/` now
answers `404`.

Roll forward, never back. The migrations are the expand half, so v0.7.0 would
still boot on the new schema, but it cannot redeem a code v0.8.0 issued, and
`00078`'s Down refuses while one is live. Recover from a v0.8.0 problem with a
hotfix.

## Rolling out v0.9.0 (R3)

Nothing to set before the merge. v0.9.0 needs no new secret, variable or bucket.
Its one migration, `00079`, revokes `TEMPORARY` on the database from `PUBLIC`
where the migration role owns the database, and changes nothing where it does
not. After the deploy, run the check in `operations.md`: the application role
must not hold `TEMPORARY`; if it does, the database's owner revokes it once.

The API deploys first and the web second, so the v0.8.0 bundle meets the v0.9.0
API for a few minutes, and for longer in tabs left open. The contract's changes
are additive, and a student in the middle of a test is not reloaded. A tab
opened on v0.8.0 is asked to reload when it leaves the engine, or when a lazy
chunk it asks for is gone.

If the API is ever rolled back to v0.8.0, roll Pages back to its previous
deployment with it: the v0.9.0 result page reads `sections`, which v0.8.0 does
not send, and a graded result would not open. Prefer a hotfix.

## Rolling out v0.10.0 (R4)

R2's contract steps ship inside this release (`docs/plan/74-r4.md` T-R4.49). `release_command` runs
`migrate up` before the new version takes traffic, so v0.9.0 keeps serving while a migration
applies, and keeps serving if the deploy stops after one.

**`00105_drop_users_legacy_role.sql`** drops `users.role`, its trigger, its index and
`app.user_role`, and the API stops naming a role (`User.role`, `CurrentUser.role`, the access
token's claim).

**`00106_validate_ownership.sql`** validates the five owner constraints (`classes.teacher_id`, and
`owner_id` on `tests`, `questions`, `question_groups` and `media_assets`) and drops the five
`BEFORE INSERT` fill triggers with their three functions (`docs/plan/20-data-model.md` §43).

- Before the merge, read the machine versions: `fly status -a quizzivy-api` lists each machine with
  its release, and `fly releases -a quizzivy-api` dates the releases. No machine may be older than
  the release that deployed v0.8.0 (2026-10-03), in either process group, for at least seven days.
  v0.8.0 and v0.9.0 never name `users.role` and name the owner in every insert, so they keep working
  on the new schema; v0.7.0 does neither, and its inserts into the four content tables and `classes`
  would fail.
- Rehearse on a Neon branch of production, both scripts in one run. Before the migrations, `psql
  "$BRANCH_DSN" -v before=1 -f docs/setup/rehearsals/r4-legacy-role.sql > legacy-before.txt` and the
  same with `r4-ownership.sql > ownership-before.txt`; then `goose up` as `quizzivy_migrate` (time
  each migration); then both scripts again without `-v before=1` into `legacy-after.txt` and
  `ownership-after.txt`. In each script, section A must read the same before and after, and every
  "must be" line of section B must print what it says. Section P of `ownership-before.txt` must
  show 0 rows without an owner on every line. Add `-v scan=1` to the first `r4-ownership.sql` run
  for the cost of each table's validation scan, which the PR records per table. Delete the branch.
- 00105 holds ACCESS EXCLUSIVE on `app.users` for milliseconds. 00106's validations hold SHARE UPDATE
  EXCLUSIVE and block neither a read nor a write; its drops hold ACCESS EXCLUSIVE on the five owner
  tables for milliseconds, last, to commit. Each migration gives up on a lock after five seconds.
- The API deploys first, so the v0.9.0 bundle meets an API whose responses carry no `role`. That
  bundle never reads it, and a tab left open keeps working.
- If a migration stops, its transaction rolled back whole: the schema, the version row and v0.9.0 are
  as they were, and a redeploy retries it. The codes, by what to do:
  - `55P03` (a lock did not arrive within five seconds): find the long query or open transaction on
    `app.users` (00105) or on the five owner tables (00106) in `pg_stat_activity`, and redeploy.
  - `40P01` (the deadlock detector ended the migration): 00106 only. A transaction of v0.9.0 that
    touches two of the five tables in the opposite order, and is open at that moment, deadlocks with
    the migration, and the detector ends one of the two after a second. If it ends the migration,
    redeploy as for `55P03`. If it ends the v0.9.0 transaction instead, the migration proceeds and
    that one request fails with a server error; repeating it works. Either way nothing else is lost.
  - `23502` (a row without what the migration validates): the message names the table. For 00105 a
    user has no `role_id`; for 00106 a row has no owner, which the constraint has refused since R2, so
    someone dropped it. Set the owner from `created_by` (`uploaded_by` for media) and redeploy.
- Roll forward, not back past v0.8.0: Down restores the column from what each role holds and the
  triggers for a v0.7.0 that nothing in production runs, and nothing runs either in production.
- **`00103_compose_text_nfc.sql`** rewrites decomposed text to NFC (F-37). It disables the
  `updated_at` trigger on eight tables by name (`tests`, `questions`, `question_groups`, `classes`,
  `assignments`, `assignment_student_overrides`, `users`, `word_imports`), which takes SHARE ROW
  EXCLUSIVE on each until it commits: every write to those tables waits for it, and a v0.9.0
  transaction holding a write on one of them for more than five seconds stops it with `55P03`. Do as
  above: find the transaction in `pg_stat_activity` and redeploy. The migration is idempotent; a
  second run changes nothing.
- **At start-up** the API replaces every live join code issued before v0.8.0 (`jobs.RotateLegacyJoinCodes`,
  spec §6.1, `operations.md` "Legacy join codes"). The log shows `legacy_join_codes_rotated` with the
  count only when there was one to rotate; a production with none logs nothing. A class not reached
  within the two-minute budget waits for the next start-up.
- **No import run should be queued or running at the deploy.** The v3 worker retires a v2 run that is
  ten minutes past due (`PIPELINE_RETIRED`), and its teacher starts it again.
- **v0.10.0 shipped without the production-branch rehearsal**, by the owner's decision of 2026-10-12,
  as v0.9.0 did: every migration passed up, down to 00079 and up again on a seeded scratch database
  (0.17 s in all), and none rewrites a table. The runbook for the rehearsal is
  `docs/setup/rehearsals/r4-neon-rehearsal.md`, for the day it is run.

### Rolling v0.10.0 back

Prefer a hotfix. If the API must go back to v0.9.0, redeploy the v0.9.0 image (`fly releases -a
quizzivy-api`, then `fly deploy --image <v0.9.0 image> -a quizzivy-api`) and roll Pages back to its
previous deployment **in the same hour**: the v0.10.0 bundle calls 25 operations v0.9.0 does not
serve (`getTeacherSummary`, `listNotifications`, `listSessions`, `setStudentOverrides`,
`pasteImportSource`, `replaceMedia` and others), so the teacher dashboard, the bell, Settings →
Devices, per-student extensions, text import, media rename and replace and the version diff would
fail. Run no Down migration: v0.9.0's migrate binary sees a database at 00106 and has nothing to run,
and v0.9.0 starts on this schema, because every change through 00104 is additive and 00105 and 00106
remove only what v0.9.0 never names.

What v0.9.0 then does with what v0.10.0 wrote:
- Per-student overrides are ignored, so an extended student loses the extension; a review released
  "after close" shows on submit.
- Notifications, preferences, photos, display names and device locations are ignored; the rows stay.
- A replaced media file is listed in the library again; audio above 10 MiB stays served.
- Questions keep `level`, `skill` and `media_alt`, but a version published under v0.9.0 is
  snapshotted without them.
- A pasted-text import queued since the deploy is retired `PIPELINE_RETIRED` by the v2 worker ten
  minutes after it falls due; its teacher runs it again after rolling forward.
- Access tokens minted by v0.10.0 carry no `role` claim; v0.9.0 does not require one, so sessions
  survive.
- **Legacy join codes do not come back.** The start-up job revoked every pre-v0.8.0 code and issued
  sealed ones, which v0.9.0 redeems. The codes teachers shared after the deploy stay valid; the old
  printed ones stay revoked.
- `00084`'s Down leaves its constraint `NOT VALID` for the same reason. Never roll back past v0.8.0
  (the join-code key).

## Rolling out v0.11.11 (hotfix)

v0.11.11 finishes change request CR-001 to the design team's drawing: "Generate questions" in the builder
fills one section or the whole test, from a plan of up to 40 questions and 5 calls, each call reserving its
own ceiling (spec 0.115, "Changes since v0.114"). It changes no schema and no configuration; the contract's
`count` on `generateAiQuestions` becomes any whole number from 1 to 8 (it was 3, 5 or 8).

- **No migration.** `release_command` runs `migrate up` and finds nothing to do.
- **Deploy the API first.** The old API refuses a count of 1, 2, 4, 6 or 7 (`400`), which the new web sends
  whenever the teacher chooses one; the old web on the new API sends 3, 5 or 8, as before.
- **Live smoke** with the key before the merge to `main` (`aismoke`, `docs/setup/operations.md`): a
  standalone run of 2 questions, and a whole-test plan of two sections, one standalone and one per passage.
- **What to watch.** A whole-test run is several ordinary generate calls, each logged and metered as
  before; nothing new is logged.
- **Rolling back to v0.11.10** needs nothing; the panel loses the whole-test scope and the stepper.

## Rolling out v0.11.10 (hotfix)

v0.11.10 lets "Generate questions" write standalone questions, about a topic or fitted to a section of the
open test (spec 0.114, "Changes since v0.113"; change request CR-001). It changes no schema and no
configuration; the contract gains two optional fields on `generateAiQuestions` (`standalone`, `sectionId`)
and lowers the schema's minimum for `source` to 3 characters.

- **No migration.** `release_command` runs `migrate up` and finds nothing to do.
- **Deploy the API first.** The old API refuses the new web's standalone request (`400`, an unknown field),
  while the old web on the new API sends what it always did and is answered as before.
- **Live smoke** with the key before the merge to `main` (`aismoke`, `docs/setup/operations.md`): one
  standalone run from a topic, one fitted to a section with questions in it.
- **What to watch.** Nothing new is logged; a run that keeps no usable question logs `ai answer unusable`,
  as before.
- **Rolling back to v0.11.9** needs nothing; the panel loses the style control.

## Rolling out v0.11.9 (hotfix)

v0.11.9 carries two hotfixes and changes no schema, no configuration and no contract (spec 0.113, "Changes
since v0.112"): "Make a variant" (#728, API and web) and the Questions tab of Assignment detail, which now
draws only the item analysis and its AI insights (#727, web only). Merge #727 into `main` before this
release. A variant now covers the members of a test's groups: a test made only of group members used to
answer "There is nothing for the assistant to read yet." It also asks about 8 questions a call, at most five
at a time, within 45 seconds, under one reservation, where one call for a long test timed out with
`503 AI_UNAVAILABLE`.

- **No migration.** `release_command` runs `migrate up` and finds nothing to do. The web build and the API
  can deploy in either order. The new web on the old API varies the standalone questions, as before; the old
  web on the new API stops on its first read of a member's change (`404`) before it writes anything.
- **What to watch.** Each variant logs one `ai variant` line (`docs/setup/operations.md`) with its calls, how
  many were usable and whether the budget ran out. A pass whose calls all fail still logs `ai call failed` or
  `ai answer unusable` for each, as before.
- **Rolling back to v0.11.8** needs nothing, and brings back both the group-member limit and the timeout.

## Rolling out v0.11.8 (hotfix)

v0.11.8 carries three hotfixes and changes no schema, no configuration and no response (spec 0.112, "Changes
since v0.111" and "Changes since v0.110"). An AI panel keeps its session when its drawer closes, and says
in a toast when a run finishes behind a closed drawer (web only). The contract's `email` format refuses an
address the API cannot print back, such as `"qc tp"@example.com`. And `POST /auth/login` counts one account
once, however its address is written.

- **No migration.** `release_command` runs `migrate up` and finds nothing to do. The web build and the API
  can deploy in either order: no contract shape changed.
- **Run the email query before and after.** `docs/setup/operations.md`, "Stored emails the API cannot
  print", has a read-only query that lists accounts the previous API could have stored with a quoted
  local part. No rows is the expected answer. If it returns rows, nothing in the release rewrites them; the
  repair is per row and the owner's, and the section says how. Until a row is repaired, the lists that hold
  it keep answering `500`, as they did before this release.
- **What a user sees.** `POST /auth/login`, `createStudent`, `updateStudent` and any other `format: email`
  parameter answer `400 VALIDATION_FAILED` naming `email` for a quoted local part. `Name <addr>` and a
  quoted atom that needs no quotes are unchanged, and no stored value changes.
- **The login limiter's key changes, its limits do not.** The buckets live in each API process's memory,
  so a deploy starts them empty, and a pod still on v0.11.7 keys on the address as written until it is
  replaced.
- **Rolling back to v0.11.7** needs nothing: there is no schema and no stored value to undo. Rolling the API
  back brings back the quoted-address `500`, so run the query again if it was out for long. Rolling Pages back
  brings back the drawer that discards its results.
- **Afterwards**, in the builder run "AI check", open a finding's question and open "AI check" again: the
  findings must still be there, with no new call. Create a student with the address `"a b"@example.com`: it
  must answer `400` and write no row. The query above must return no rows.

## Rolling out v0.11.7 (hotfix)

v0.11.7 adds one nullable column and one index and changes no configuration
(`docs/plan/74e-e1-group-move.md`, "After release"). A copy that a question's drag into a shared-context
group makes now remembers the bank question it came from, so dragging it straight back out puts the
original back instead of leaving a second bank question. `moveTestQuestion` answers two more booleans,
`restored` and `editedCopy`; nothing else in the contract changes.

- **Two migrations, both before traffic.** `release_command` runs `migrate up`, and the new version takes
  traffic only after it. **00126** adds `app.questions.copied_from_id`, nullable with no default and
  no backfill, referencing `app.questions (id)` with `ON DELETE SET NULL`. The foreign key is part of
  the `ADD COLUMN`, and PostgreSQL does not scan the table to validate the column it is adding: on a
  table of 1M rows and 320 MB that statement took 1.6 ms, against 6.7 s for a foreign key added in a
  statement of its own. **00127** builds the partial index `questions_copied_from_idx` on it with
  `CREATE INDEX CONCURRENTLY`, so writes carry on while it builds; the file is `NO TRANSACTION` and holds
  nothing else. If a build is interrupted, the half-built index is left invalid: drop it
  (`DROP INDEX CONCURRENTLY app.questions_copied_from_idx`) and run the migration again.
- **v0.11.6 pods that are still running** write copies with a NULL `copied_from_id`: their insert names
  no such column. That is harmless. Such a copy has no origin to restore, so dragged out it keeps its
  id and becomes a bank question, as in v0.11.6; every copy written by v0.11.7 restores. The previous
  release also reads a v0.11.7 row unchanged, since the new column is one it never selects.
- **Rolling back to v0.11.6** needs nothing. The column and the index stay and nothing reads them;
  a copy written by v0.11.7 is an ordinary member there. Down (`migrate down`) drops the index and then the
  column, and loses only the memory of origins.
- **Afterwards**, drag a bank question into a group and straight back out: the bank must hold the
  question once. The live E2E `builder-groups.live.spec.ts` covers the same round trip on CI.

## Rolling out v0.11.6 (E1)

v0.11.6 adds one operation and changes no schema and no configuration
(`docs/plan/74e-e1-group-move.md`). A teacher can drag a question into a shared-context group, out
of one, between two, or within one; each drag has a menu equivalent.

- **No migration.** The deploy is a rolling one. Until every pod runs v0.11.6, a pod still on
  v0.11.5 answers `POST /teacher/tests/{id}/questions/move` with 404, and the builder reports the
  move as failed; the teacher tries again.
- **Rolling back to v0.11.5** needs nothing. A moved question is an ordinary row: a copy is a member
  like any other, and a question moved out is a bank question, so v0.11.5 reads both unchanged.
  Only the drag goes away.
- **Afterwards**, check that a teacher can drag a bank question into a group and publish. The live
  E2E `builder-groups.live.spec.ts` covers the same path on CI.

## Rolling out v0.11.2 (hotfix)

v0.11.2 changes no schema and no contract. It is the second fix for "Generate questions" answering
`503 AI_UNAVAILABLE`, after v0.11.1 did not cure it.

- **What v0.11.1's log showed.** With the template gone, `ai answer unusable` still read "none of the
  1 questions in the answer is usable": 46 to 59 output tokens, one question where five were asked,
  and that one failing the checks.
- **The cause.** The client decoded each schema into a Go map before sending it, and a map marshals
  its keys in alphabetical order. Structured output writes an object's properties in schema order,
  so the model had to write `answers` and `explanation` before `options`, `prompt` and `type`: the
  answer before the question. The schema now reaches the provider as written
  (`internal/platform/anthropic`), and a test pins the order on the wire. Every AI feature was
  affected, the page reader included.


v0.11.1 changes no schema and no contract. It fixes the AI assistant, which answered every
"Generate questions" in production with `503 AI_UNAVAILABLE` on the day v0.11.0 shipped.

- **What happened, as first read** (v0.11.2 found the main cause). Each prompt ended with a literal
  JSON example full of `"..."`. Every call was
  billed for 46 output tokens, whatever the passage, which is the size of that example copied back,
  and the decoder found nothing usable in the answer. The teacher saw the 503. Nothing was logged,
  because only a failed call was, so the answer itself could not be read back.
- **What changed.** The prompts describe the fields in words and name the count. Structured output
  already holds the answer to the schema. An answer the checks reject now logs `ai answer unusable`
  (`docs/setup/operations.md`).
- **The start-up lesson from v0.11.0.** With `ANTHROPIC_API_KEY` set and the tier variables missing,
  the API and the worker refuse to start. Set the key and all eight `AI_TIER_*` variables in one
  `fly secrets set`, before the deploy, or set none of them.

## Rolling out v0.11.0 (D4)

D4 adds twelve migrations and changes no existing column (`docs/plan/74d-d4-deck-update.md`, the
release checklist). `release_command` runs `migrate up` before the new version takes traffic, so
v0.10.1 keeps serving while they apply.

- **00114 to 00117** add `allowPause` ("Students can pause") to a question's audio and a group's
  recording, live and frozen in a version. The question columns are nullable with no default and no
  backfill, and a reader takes NULL as true, the value every recording had before the control
  existed. The recording columns are `NOT NULL DEFAULT true`, a constant default, so nothing is
  rewritten.
- **00118 to 00123** create the AI assistant's tables: the settings row (a 200,000-credit monthly pool
  and a 20,000-credit allowance per teacher), the month's pool, each teacher's allowance,
  reservations, the append-only usage ledger and credit requests. The pool and the allowance are CHECK
  constraints, so the database refuses a call that would overspend.
- **00124** adds `ai.use` (to the Teacher role) and `ai.credits.manage`. **00125** adds
  `word_imports.read_with_ai`.
- **Rehearsed on a Neon branch of production on 2026-10-13.** The branch was at 00113, with 205
  questions, 257 frozen questions, 12 attempts and 7 assignments. Every migration went up, down to
  00113 and up again, each in one transaction with its version row, in about 0.8 s, which was almost
  all the round trip from where it ran. `ai_settings` read 200000 and 20000. A reservation past the
  pool was refused with `23514` (`ai_credit_periods_within_pool`), and nothing was written. The app
  role holds only INSERT and SELECT on the ledger. The branch was deleted.
- **The AI is off until `ANTHROPIC_API_KEY` and the tier variables are set** as Fly secrets
  (`.env.example`, `docs/setup/operations.md`). Without them every AI operation answers `501 AI_OFF`,
  no AI control is drawn, and the release is still complete.
- The API deploys first, then Pages. The v0.10.1 bundle meets an API whose student payloads carry
  `allowPause`, which that bundle ignores; a teacher's tab of v0.10.1 that saves an audio policy
  without the field keeps the stored value.
- Attempts in progress keep their answers, flags and play counts. The draft and flag formats and the
  open-question key did not change. A recording playing at the reload costs one play, as does leaving
  the test page mid-play and coming back (spec §11.3).

### Rolling v0.11.0 back

Prefer a hotfix. Run no Down migration. If the API must go back to v0.10.1, roll Pages back to its
previous deployment **in the same hour**: the v0.11.0 bundle sends `allowPause` and calls the AI
operations, which v0.10.1 does not serve.

While v0.10.1 runs on the v0.11.0 schema, either during the rolling deploy or after a rollback:
- **A teacher who removes or replaces the audio of a question last saved by v0.11.0 gets a server
  error.** v0.11.0 stores `audio_allow_pause` on every audio question. v0.10.1 does not clear it when
  the audio goes, and `questions_allow_pause_only_with_audio` refuses the row. It affects only that
  edit; roll forward. To unblock one question by hand, as `quizzivy_migrate`:
  `UPDATE app.questions SET audio_allow_pause = NULL WHERE id = '<id>';`
- A group saved by v0.10.1 writes its recordings back with the default, so a stored "Students can
  pause: off" turns on again. Check the group after rolling forward.
- AI usage, allowances, requests and the ledger stay in their tables. v0.10.1 ignores them.

## Interrupted index builds in v0.8.0 (R2)

R2 (v0.8.0) builds eight indexes `CONCURRENTLY`, one per file: `00057` and
`00071`–`00077`. An interrupted build leaves an INVALID index that the rerun's
`CREATE INDEX CONCURRENTLY` would trip over. Find it, as `quizzivy_migrate`:

```sql
SELECT indexrelid::regclass FROM pg_index WHERE NOT indisvalid;
```

Drop only the index that query names, then redeploy. The files before it are
already recorded as applied, so dropping their valid indexes would lose them
for good:

```sql
DROP INDEX CONCURRENTLY IF EXISTS app.users_role_id_active_idx;        -- 00057
DROP INDEX CONCURRENTLY IF EXISTS app.classes_teacher_idx;             -- 00071
DROP INDEX CONCURRENTLY IF EXISTS app.tests_owner_idx;                 -- 00072
DROP INDEX CONCURRENTLY IF EXISTS app.questions_owner_bank_idx;        -- 00073
DROP INDEX CONCURRENTLY IF EXISTS app.question_groups_owner_bank_idx;  -- 00074
DROP INDEX CONCURRENTLY IF EXISTS app.media_assets_owner_idx;          -- 00075
DROP INDEX CONCURRENTLY IF EXISTS app.users_created_by_idx;            -- 00076
DROP INDEX CONCURRENTLY IF EXISTS app.assignments_creator_idx;         -- 00077
```

## Rolling out v0.12.1 (hotfix)

v0.12.1 puts every Admin console page in the teacher workspace's page frame (up to 1600px, centred
beyond it; DG-205, spec 0.117). It is web only: no schema, no contract and no configuration change,
so the API need not be redeployed and rolling the web back to v0.12.0 is safe.

## Rolling out v0.12.0 (R5)

v0.12.0 adds the Admin console, outgoing email and the nightly retention sweep, with
migrations `00128` to `00141` (`docs/plan/75-r5.md`, the release checklist).

**The order.** Merging `release/0.12.0` to `main` deploys the API first: Fly's
`release_command` (`fly.toml`, `/app/migrate -dir /app/migrations up`) runs `00128` to
`00141` before the new version takes traffic, and v0.11.11 keeps serving while they
apply, except while `00131` holds `app.users` (below). Each migration commits on its own, so a
stop at `00139` or `00141` leaves the earlier ones applied, which v0.11.11 tolerates. The rolling strategy then replaces the `app` and `worker` machines. Pages deploys
second. Every new column on a populated table (`00131`, four on `app.users`) is nullable
with no default, so v0.11.11's inserts keep working during the roll.

- `00128`, `00130`, `00132`, `00134` to `00137` and `00140` create tables v0.11.11 never
  reads. `00130` writes the one `organization_settings` row and revokes `INSERT` and
  `DELETE` on it from the app role.
- `00129` grants the app role the role writes (`INSERT` and some `UPDATE` columns on
  `roles`, `INSERT` and `DELETE` on `role_permissions`).
- `00131` backfills `last_sign_in_at` and `last_active_at` once, from refresh tokens,
  attempts and the audit log, with `users_set_updated_at` disabled for the update. The
  backfill runs in the transaction that adds the columns, so `app.users` is held
  `ACCESS EXCLUSIVE` until it ends, and every request that reads a user (sign-in, refresh,
  any authenticated call) waits for it. `refresh_tokens` has only a partial index on its user,
  so the backfill scans it per user. Its time on the Neon-branch rehearsal is a go/no-go: if it
  takes more than a few seconds, deploy outside teaching hours.
- `00133` and `00138` build their indexes `CONCURRENTLY` (`-- +goose NO TRANSACTION`); see
  "Interrupted index build in v0.12.0" below.
- `00139` stops unless `quizzivy_maintenance` exists, then grants it column by column and adds
  `users_maintenance_guard`, which fires for that role only.
- `00141` stops on a live legacy join code, revokes every unrevoked legacy code at its expiry
  and validates `class_join_codes_live_encrypted`.

Three things must exist before the merge to `main`. A missing secret or a missing
maintenance role stops the deploy on its own; the sending domain has to be checked by hand:

- **Fly secrets `SECRETS_KEY` and `RESEND_API_KEY`** (`dns.md`, "Secrets on Fly").
  `EMAIL_FROM`, `APP_URL`, `EMAIL_DAILY_LIMIT`, `EMAIL_SEND_RATE` and `APP_TIMEZONE`
  are in `fly.toml`'s `[env]`. The preflight fails without any of `SECRETS_KEY`,
  `RESEND_API_KEY`, `EMAIL_FROM` and `APP_URL`, and fails if `EMAIL_SINK_DIR` is set, before anything is built.
- **The `quizzivy.com` sending domain verified in Resend**, open and click tracking
  off (`dns.md`, "Email (Resend)"). Without it the API starts and every message
  fails at Resend.
- **The `quizzivy_maintenance` role**, created by `scripts/provision-db.sh`
  (`operations.md`, "The nightly workflow", step 1). `00139_grant_maintenance_role.sql`
  fails while the role is missing, so the release command stops and the previous
  version keeps serving. Create the `production-maintenance` environment as step 2
  says, and leave `RETENTION_SWEEP_ENABLED` unset until the first dry run on
  production has been read.

Two changes need a check rather than a setting:

- **`00141_require_encrypted_join_codes.sql` stops on a live legacy join code.** Run the
  count in `operations.md`, "Legacy join codes", on a Neon branch of production before the
  merge; it should be 0, since every v0.10.0 or later API replaces live legacy codes at
  start-up. If it is not, start a v0.11.x API once, then count again on a new Neon branch. If the migration stops, the release command fails and the previous
  version keeps serving. It holds `ACCESS EXCLUSIVE` on `class_join_codes`, a small
  table, so a join waits for it briefly.
- **The v0.7.0 `/admin` alias is gone** (T-R5.36). A script or bookmark that still calls a
  teaching operation at its v0.7.0 path under `/admin/` gets `404 NOT_FOUND`, except
  `GET` and `POST /admin/classes`, which are now the console's `listAllClasses` and
  `createClassForTeacher` (D-053) and answer a teacher's token `403`; it moves to the
  operation's `/teacher/*` path in `api/openapi.yaml`. The release notes say so.
  Production logged no aliased request in the seven days to 2026-10-19
  (`docs/plan/40-open-items.md` O-R4.22). A web address under `/admin/` that is not a
  console route still redirects to the same path under `/teacher`.

The `PLAN_*` variables stay unset until the centre's plan is decided
(`operations.md`, "The plan the Admin console shows"). A tab still signed in with a
temporary password gets `403 PASSWORD_CHANGE_REQUIRED` once and is sent to change it;
the release notes say so. Roll forward rather than back
(`docs/plan/70-redesign-overview.md` §8).

**Where the prerequisites stand at the release commit (2026-10-20).** `fly.toml`'s `[env]`
already holds `EMAIL_FROM`, `APP_URL`, `EMAIL_DAILY_LIMIT` (`100`), `EMAIL_SEND_RATE` and
`APP_TIMEZONE`; no `PLAN_*` value is set. Thuong confirmed the Resend DNS records and
`SECRETS_KEY` on 2026-10-19, and counted the live legacy join codes on a Neon branch of
production: 0. Still owed before the merge, by the owner: Resend's open and click tracking off
and the `RESEND_API_KEY` secret; `provision-db.sh` for `quizzivy_maintenance` and the
`production-maintenance` environment's `MAINTENANCE_DATABASE_URL`; the Neon-branch rehearsal of
`goose up` with its timings (`00131`'s above all), `audit_log_recent_idx` valid and T-R5.12's
`EXPLAIN` of the audit log's first page using it, `has_schema_privilege('quizzivy_maintenance',
'public', 'USAGE')` true after `00139` with no `WARNING` from its grants in goose's output, and
the retention sweep's dry run; and the exam-window check.

### Rolling v0.12.0 back

Rollback is roll forward (`docs/plan/70-redesign-overview.md` §8): prefer a hotfix and run no
Down migration. Every Down of `00128` to `00141` drops what its Up created (`00141`'s keeps the
revocations of expired legacy codes), but none is needed for v0.11.11 to run, and the Downs of
`00128`, `00130` to `00132`, `00134` to `00137` and `00140` delete the outbox, the settings, the
users' activity columns and daily activity, the exports, the set-password links, the imports and
the avatar sweeps with their rows. If the API must go
back to v0.11.11, roll Pages back to its previous deployment **in the same hour**: the v0.12.0
bundle calls the Admin console's operations, which v0.11.11 does not serve (`404`), and two of
them reach the wrong handler (below).

While v0.11.11 runs on the v0.12.0 schema, either during the rolling deploy or after a rollback:
- **The `/admin` alias comes back.** v0.11.11's `legacyAdminPatterns` still holds `GET` and
  `POST /admin/classes`, so the v0.12.0 Admin Classes page reads the teacher workspace's
  `listClasses` through it, and its create is refused with `400` (`teacherId` is not a field of
  `createClass`). Scripts on the old teaching paths work again until the roll forward, after
  which they get `404` once more.
- **The centre's settings are not enforced.** v0.11.11 never reads `organization_settings`:
  password sign-in turned off, the allowed Google domains and joining by code turned off all stop
  applying, and the front door shows the `VITE_ORG_*` build values.
- **A temporary password is not enforced on the server.** v0.11.11 has no
  `PASSWORD_CHANGE_REQUIRED`; its web still sends an account with `mustChangePassword` to Change
  password (`RequireSession`), but its API answers every operation.
- **Join codes are unaffected.** v0.11.11 seals every code it issues (`lookup_scheme` 2 with a
  ciphertext, `classes/repositories/joincodes.go`), so it never writes a row
  `class_join_codes_live_encrypted` refuses, and its start-up rotation finds no live legacy code
  to replace. Its legacy SHA-256 lookup finds only revoked codes.
- **`getDashboard` returns its four old fields again**; the v0.12.0 bundle does not read them,
  and the v0.11.11 bundle has not read them since R4 (T-R4.23).
- **The grants stay.** `00129`'s role writes and `00139`'s maintenance grants are unused by
  v0.11.11, and `users_maintenance_guard` fires only for `quizzivy_maintenance`.
- **Nothing drains or stamps.** No dispatcher sends the outbox, so pending messages wait and a
  set-password link already emailed opens no page until v0.12.0 is back (the link lives 24
  hours). Queued imports and exports wait for the v0.12.0 worker. `last_active_at` and
  `user_activity_days` stop moving, so the Overview's chart has a gap for those days, and the
  daily housekeeping does not run.
- The nightly retention workflow builds `cmd/maintenance` from the repository in GitHub
  Actions (`.github/workflows/retention-sweep.yml`), not from the API's image, so a rollback of
  the API does not change it; it applies only once `RETENTION_SWEEP_ENABLED` is set.

## Interrupted index build in v0.12.0 (R5)

`00133` builds `audit_log_recent_idx` on `app.audit_log (occurred_at DESC, id DESC)`
`CONCURRENTLY`, so writes to the log carry on while it builds. `00138` does the same
for `users_search_trgm_idx`, a gin trigram index on the Users search expression of
`app.users`: about 4 seconds at 100,000 accounts and 12 to 17 seconds at 500,000
(more with a `maintenance_work_mem` below 64 MB, 107 seconds at 64 kB), and it ends
with `ANALYZE app.users`, which takes about a second. Both drop any index of their
name before they build, so an interrupted build needs nothing from you: redeploy,
and the rerun removes the INVALID leftover and builds the index again. A rerun over
a valid index drops it and builds it again, which leaves the search unindexed for
the length of the build. Check afterwards, as
`quizzivy_migrate`; the query returns no row when every index is valid:

```sql
SELECT indexrelid::regclass FROM pg_index WHERE NOT indisvalid;
```

The Admin audit log reads through its index. Until it is valid the list still
answers, by sorting the range it was asked for, which on a large log is slow. The
Users search reads through `users_search_trgm_idx` in the same way: until it is
valid the directory answers every search by reading the whole table.

A database that ran the first form of `00133` (`CREATE INDEX CONCURRENTLY IF NOT
EXISTS`, merged in #641) and was interrupted has the file recorded over an INVALID
index, and the query above names it. Repair it by hand, as `quizzivy_migrate`, in
a session that is not inside a transaction, running the two statements one after
the other. Neither can run inside a transaction block, so `psql -c` with both in
one string, or a script that wraps them in `BEGIN`, fails with "cannot run inside
a transaction block" and repairs nothing; give each its own `-c`, or paste them
into an interactive `psql`:

```sql
DROP INDEX CONCURRENTLY IF EXISTS app.audit_log_recent_idx;
```

```sql
CREATE INDEX CONCURRENTLY audit_log_recent_idx ON app.audit_log (occurred_at DESC, id DESC);
```

Then run the query above again: it returns no row.

## Backups and operational verification

See [operations](operations.md) for the recovery rehearsal, retention commands,
post-deploy header check and proposed GitHub uptime probe. Neon history settings,
a production PITR drill and a delivered owner alert remain unverified as of
2026-09-22 because no Neon/monitor access was configured for this task.
