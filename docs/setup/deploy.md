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

It exists because `config.Load` has two all-or-nothing groups — Google sign-in
and object storage — and each exits 1 on a partial set. Both shipped partial,
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

Tabs still open on v0.7.0 keep working: the `/admin` alias serves the old
teaching paths and logs `legacy_admin_path`. v0.9.1 removes it
(`docs/plan/73-r3.md` T-R3.3).

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

## Rolling out v0.11.1 (hotfix)

v0.11.1 changes no schema and no contract. It fixes the AI assistant, which answered every
"Generate questions" in production with `503 AI_UNAVAILABLE` on the day v0.11.0 shipped.

- **What happened.** Each prompt ended with a literal JSON example full of `"..."`. Every call was
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

## Backups and operational verification

See [operations](operations.md) for the recovery rehearsal, retention commands,
post-deploy header check and proposed GitHub uptime probe. Neon history settings,
a production PITR drill and a delivered owner alert remain unverified as of
2026-09-22 because no Neon/monitor access was configured for this task.
