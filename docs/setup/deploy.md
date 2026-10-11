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

- Before the merge, read the machine versions: `fly status -a quizzivy-api` lists each machine with
  its release, and `fly releases -a quizzivy-api` dates the releases. No machine may be older than
  the release that deployed v0.8.0 (2026-10-03), in either process group, for at least seven days.
  v0.8.0 and v0.9.0 never name the column, so they keep working on the new schema; v0.7.0 does name
  it.
- Rehearse on a Neon branch of production: `psql "$BRANCH_DSN" -v before=1 -f
  docs/setup/rehearsals/r4-legacy-role.sql > before.txt`, `goose up` as `quizzivy_migrate` (time
  it), then the script again without `-v before=1` into `after.txt`. Section A must read the same
  in both, and every "must be" line of section B must print what it says. Delete the branch.
- The migration holds ACCESS EXCLUSIVE on `app.users` for milliseconds and gives up after five
  seconds (`55P03`, schema unchanged). If it does, find the long query on `app.users` and redeploy.
- The API deploys first, so the v0.9.0 bundle meets an API whose responses carry no `role`. That
  bundle never reads it, and a tab left open keeps working.
- Roll forward, not back past v0.8.0: Down restores the column from what each role holds, but
  nothing in production runs it.

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
