# R4 (v0.10.0) on a Neon branch of production

The release checklist of `docs/plan/74-r4.md` asks for these numbers from a branch of production.
v0.10.0 shipped without them, by the owner's decision of 2026-10-12 (`docs/setup/deploy.md`,
"Rolling out v0.10.0"); this is the run for the day it is done, or for the next release that
needs a production-sized measurement.

Rules:
- A **branch** of production, never the production endpoint.
- Connect as `quizzivy_migrate` on the branch's endpoint.
- Keep the connection string in the environment of the shell that runs this, never in a chat or a
  file.
- Delete the branch at the end.
- Everything below is read-only except the goose steps, which change the branch alone.
- Run it from a checkout of the release commit.

## 0. The branch

```bash
neon branches create --project-id "$NEON_PROJECT" --name r4-rehearsal-$(date +%Y%m%d) --parent <production branch>
export BRANCH_DSN='postgres://quizzivy_migrate:<password>@<the BRANCH endpoint host>/quizzivy?sslmode=require'
psql "$BRANCH_DSN" -Atc "SELECT current_database(); SELECT max(version_id) FROM goose_db_version"   # the version production is at
mkdir -p rehearsal
```

## 1. Before the migrations

```bash
psql "$BRANCH_DSN" -At > rehearsal/before-counts.txt <<'SQL'
-- questions with more than eight options (the cap applies to writes only)
SELECT 'questions_gt8', count(*) FROM (SELECT o.question_id FROM app.question_options o JOIN app.questions q ON q.id = o.question_id WHERE q.deleted_at IS NULL GROUP BY o.question_id HAVING count(*) > 8) x;
-- live join codes from before v0.8.0, which the API replaces at start-up
SELECT 'legacy_join_codes', count(*) FROM app.class_join_codes WHERE revoked_at IS NULL AND lookup_scheme = 1 AND expires_at > now();
-- what 00103 will rewrite, in its three heaviest columns, and the accepted answers it will fold
SELECT 'tests.title_nfd', count(*) FROM app.tests WHERE title IS NOT NFC NORMALIZED;
SELECT 'questions.prompt_nfd', count(*) FROM app.questions WHERE prompt IS NOT NFC NORMALIZED AND prompt_content IS NULL;
SELECT 'users.full_name_nfd', count(*) FROM app.users WHERE full_name IS NOT NFC NORMALIZED;
SELECT 'blank_answer_twins', count(*) - count(DISTINCT (blank_id, normalize(answer, NFC))) FROM app.question_blank_answers;
-- the teacher with the most attempts, for step 4
SELECT 'top_teacher', a.created_by, count(*) FROM app.attempts at JOIN app.assignments a ON a.id = at.assignment_id GROUP BY a.created_by ORDER BY 3 DESC LIMIT 1;
SQL
psql "$BRANCH_DSN" -v before=1 -f docs/setup/rehearsals/r4-legacy-role.sql > rehearsal/legacy-before.txt
psql "$BRANCH_DSN" -v before=1 -v scan=1 -f docs/setup/rehearsals/r4-ownership.sql > rehearsal/ownership-before.txt
grep -A6 "P1" rehearsal/ownership-before.txt      # every line 0, or 00106 will stop
```

## 2. 00103's notices

goose does not print a migration's NOTICEs, so run 00103's body once in a transaction that rolls
back:

```bash
( echo 'BEGIN;'; sed -n '/+goose Up/,/+goose Down/p' migrations/00103_compose_text_nfc.sql | sed '/+goose/d'; echo 'ROLLBACK;' ) \
  | psql -v ON_ERROR_STOP=1 "$BRANCH_DSN" 2>&1 | tee rehearsal/nfc-notices.txt
```

Expect "F-37: folded N …" and "every listed column is composed".

## 3. The migrations, timed

```bash
/usr/bin/time -v goose -dir migrations postgres "$BRANCH_DSN" up 2>&1 | tee rehearsal/goose-up.txt
psql "$BRANCH_DSN" -f docs/setup/rehearsals/r4-legacy-role.sql > rehearsal/legacy-after.txt
psql "$BRANCH_DSN" -f docs/setup/rehearsals/r4-ownership.sql  > rehearsal/ownership-after.txt
diff <(sed -n '/== A/,/== [BP]/p' rehearsal/legacy-before.txt)    <(sed -n '/== A/,/== [BP]/p' rehearsal/legacy-after.txt)      # empty
diff <(sed -n '/== A/,/== [BP]/p' rehearsal/ownership-before.txt) <(sed -n '/== A/,/== [BP]/p' rehearsal/ownership-after.txt)   # empty
grep -i "must_be" rehearsal/legacy-after.txt rehearsal/ownership-after.txt   # each prints what it says
```

The migrations do not rotate join codes; the API does at start-up, so `legacy_join_codes` is the
count the deploy's log should show as `legacy_join_codes_rotated`.

## 4. The dashboard's plans

`getTeacherSummary` and `getDashboard` for the teacher with the most attempts, on the migrated
schema. Take the statements from `server/internal/modules/dashboard/repositories/{nav,home,submissions,today,activity}.go`
and `server/internal/shared/visibility/visibility.go`, with the scope set to that teacher (`$1 =
false`, `$2 =` the `top_teacher` id), the zone `Asia/Ho_Chi_Minh` and 14 days, and run each under
`EXPLAIN (ANALYZE, BUFFERS)`. The plans should use the five indexes T-R4.19 names; a sequential
scan on `attempts` is a finding.

## 5. Optional: Down on production data

```bash
goose -dir migrations postgres "$BRANCH_DSN" down-to 79 && goose -dir migrations postgres "$BRANCH_DSN" up
```

## 6. Record and delete

Record in the release's pull request:
- the per-file timings from `goose-up.txt`;
- `questions_gt8`, `legacy_join_codes` and `blank_answer_twins` from `before-counts.txt`;
- the notices;
- the index names from the plans.

Then delete the branch:

```bash
neon branches delete --project-id "$NEON_PROJECT" r4-rehearsal-$(date +%Y%m%d)
```

The seven-day `legacy_admin_path` count (T-R4.49 A) is not a database number; it comes from the
API's logs.
