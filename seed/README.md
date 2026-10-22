# Seed data

Never in a migration (spec §13.7). `make seed` applies every `.sql` here in
name order, against a database that has already been migrated. Each file is
idempotent (fixed ids, `ON CONFLICT DO NOTHING`) and is numbered at merge, one
number per file in merge order; a file's header names the seeds it relies on.

What is here:

- `01-dev.sql` — the Admin, one student and one class with that student in it.
  Enough to sign in and click through the app.
- `02-dev-assignments.sql` — one published test, one open assignment, one
  submitted-and-flagged attempt, so the dashboard has something to count.
- `03-dev-students.sql` — a few more students, including a Google-only one.
- `04-dev-e2e.sql` — the take-test fixtures the live E2E suite uses.
- `05-dev-sections.sql` — a published paper with three parts.
- `06-dev-access.sql` — two Teachers, each with a class and a draft test, and
  an Assistant (R2).
- `07-dev-classroom.sql` — thirty students in no class, for the classroom join
  test (R2).
- `08-dev-teacher-workspace.sql` — the Teacher deck's fixtures for `giaovien@quizzivy.com`
  (R4, T-R4.51): seven tests, eight media files, four classes with members, assignments in
  every status, attempts, imports. It follows 06 and 07 and relies on 06's teacher. See below.
- `09-dev-admin-console.sql` — the Admin deck's fixtures (R5, T-R5.33): three Admins, nine
  Teachers (Lý Thanh Vân disabled, teaching the archived *Summer Camp 2026*), students in every
  status, *IELTS Writing Clinic* and *Business English B1*, the custom role *Tổ trưởng*, audit
  entries in every category and one by the system, fourteen days of activity, and this month's
  AI credits with Phan Minh Đức's request waiting. It relies on 01, 03, 06 and 08.
- `10-dev-question-types.sql` — a published paper with a question of every type (R6,
  T-R6.14): two parts, a reading-passage group holding a True / False / Not given and a
  matching question, a "Choose two", a short answer with a word limit, an essay with the four
  IELTS criteria, and an image in one prompt and in its explanation. Three assignments to the
  class: one E2E 11 runs, one E2E 9 reads the session of, and one that hides the score, the
  answers and the explanations for E2E 9's result case. It relies on 01.
- `99-assert.sql` — runs last and refuses the seed if the rows above break a
  rule `publish` would have enforced, or if a user, class or owned row lacks
  the column R2's access model reads. The seed writes frozen version rows
  directly, which is the one path that skips publish's validation. Since R6 it also
  checks the new types' frozen shape: True / False / Not given's three options, "Choose N",
  matching's items and choices, rubric criteria only on essays, and that the media references
  match the images and audio in the content.

## Logins

Every account's password is `quizzivy-dev`.

| Email | Role | From |
|---|---|---|
| `thuong@quizzivy.com` | Admin | `01` |
| `hocvien@quizzivy.com` | Student | `01` |
| `giaovien@quizzivy.com` | Teacher: a class, a draft test with one bank question, a bank group | `06` |
| `giaovien2@quizzivy.com` | Teacher: its own class and draft test | `06` |
| `trogiang@quizzivy.com` | Assistant | `06` |
| `dung.hoang@example.com`, `trang.le@example.com` | Student | `03` |
| `han.pham@example.com` | Student, Google only: no password | `03` |
| `hocvien.lop01@quizzivy.com` … `hocvien.lop30@quizzivy.com` | Student | `07` |
| `minhanh.tran@quizzivy.com`, `nam.le@quizzivy.com`, `thuha.pham@quizzivy.com`, `giabao.nguyen@quizzivy.com`, `khanhlinh.vo@quizzivy.com`, `phuongthao.do@quizzivy.com`, `ngocmai.bui@quizzivy.com`, `thanhtung.ngo@quizzivy.com` | Student, the deck's roster | `08` |
| `quochuy.dang@quizzivy.com`, `ducminh.hoang@quizzivy.com` | Student with a temporary password (the deck's "Password reset" and "Not signed in") | `08` |
| `yen.nguyen@quizzivy.com`, `quan.do@quizzivy.com` | Admin | `09` |
| `giau.bui@quizzivy.com`, `bao.dinh@quizzivy.com`, `trang.ngo@quizzivy.com`, `linh.ha@quizzivy.com` | Teacher (Linh has never signed in) | `09` |
| `khai.truong@quizzivy.com` | Teacher with a temporary password | `09` |
| `duc.phan@quizzivy.com` | Teacher, Google only: no password | `09` |
| `van.ly@quizzivy.com` | Teacher, disabled | `09` |
| `bich.hoang@quizzivy.com` (disabled), `son.vu@quizzivy.com` (temporary password), `tam.truong@quizzivy.com` (never signed in), `an.lam@quizzivy.com`, `khoa.dinh@quizzivy.com`, `nhi.ta@quizzivy.com`, `phuc.luu@quizzivy.com`, `chi.mai@quizzivy.com` (Google only) | Student | `09` |
| `hocvien.ielts65.01@quizzivy.com` … `hocvien.kids.59@quizzivy.com` | Student, fillers that bring the four classes to the deck's sizes | `08` |

## `08`: the teacher workspace

Everything belongs to Trần Thị Lan (`giaovien@quizzivy.com`), named in every `owner_id`,
`teacher_id` and `created_by`. Ids are fixed: `…00000080` + a kind + a number
(`pg_temp.sid`), or derived from a label (`pg_temp.did`); the header lists the kinds.

| Page | What it shows |
|---|---|
| Tests | Seven: *Mid-term Reading Mock* (published, three versions, a draft ahead of the third: a question added and one prompt corrected), *Unit 4 · Listening: Directions*, *Vocabulary Quiz: Travel* (two versions), *Present Perfect vs Past Simple*, *Writing Task 1 Practice*, and two drafts, *Speaking Part 2* and *TOEIC Part 5* (committed from an import) |
| Content kinds | *Unit 4 · Listening* holds a question each of: a prompt with a table and a link, a prompt with two gaps, a prompt still stored as Markdown, audio, and an image with its alt text. Its published version froze the same rows |
| Question bank | 37 questions with level, skill and tags; the deck's eight bank rows are in it, three of them (map, one-word completion, speaking) reworded to fit the test that uses them |
| Media | Eight files (five audio, three image) |
| Classes | *IELTS 6.5 Evening* 24, *IELTS Foundation A* 16, *TOEIC 600 Weekend* 22, *Kids Starters B* 12, with schedule and room, beside 06's class |
| Assignments | Open (3), scheduled (1), closed (1) and a draft |
| Grading | Fourteen attempts wait: six on the Reading Mock (one timed out) and eight on the closed Writing task |
| Integrity | One flagged attempt on the Reading Mock (Lê Hoàng Nam), with its fullscreen and tab events; two more on the Listening task |
| Imports | *Unit 5 Quick Check* pasted and in review (two open actions, one to confirm), *TOEIC Part 5* committed, *IELTS Reading Pack B* failed (`SOURCE_INVALID`), *Grade 9 Mock Test 4* processing |

A published version is written the way the publish command writes it (`group_v1`, the
draft's rows copied, the test moved to published), and the file is one transaction.

### Media objects

The seed has no object-store access, so media rows are metadata only: the library, the
tests and the question editor load and list them, but an image shows as broken and an audio
file does not play, because the signed URL points at a key with no object. Nothing
here commits a binary. To give the audio rows sound for a look at the player, copy the one
small fixture over their keys:

```
keys=$(psql "$MIGRATE_DSN" -Atc "select storage_key from app.media_assets where kind = 'audio' and id::text like '01935000-0000-7000-8000-00000080%'")
docker compose run --rm --no-deps -v "$PWD/web/tests/e2e/fixtures:/fx:ro" --entrypoint /bin/sh minio-init -c \
  'mc alias set local http://minio:9000 "$S3_ACCESS_KEY_ID" "$S3_SECRET_ACCESS_KEY" >/dev/null &&
   for key in "$@"; do mc cp /fx/unit5-listening.mp3 "local/$S3_BUCKET/$key"; done' sh $keys
```

`MIGRATE_DSN` is the Makefile's. Images stay broken: the repository holds no image fixture.

### Imports

Rows only again: the import bucket holds no source file, artifact set or evidence. The
history, the detail and the review (its questions, findings and counts) read the database and
load; the review's *source* pane reads evidence from storage and answers with an error for
these four. The *processing* import's run is queued at the extraction stage and not due for
ten years, so no worker claims it and it takes none of the worker's slots; cancel it from the
page to clear it. A database seeded before 2026-10-12 holds the old row, a run "running" on a
ten-year lease that takes the worker's only slot; cancel that import or seed again.

Still planned: a volume seed (~50 students, 10k attempts, 500k integrity
events) for the query review, which measures p95 latency at realistic volume
rather than asserting plan shapes.
