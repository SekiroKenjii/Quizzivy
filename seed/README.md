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
- `99-assert.sql` — runs last and refuses the seed if the rows above break a
  rule `publish` would have enforced, or if a user, class or owned row lacks
  the column R2's access model reads. The seed writes frozen version rows
  directly, which is the one path that skips publish's validation.

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

Still planned: a volume seed (~50 students, 10k attempts, 500k integrity
events) for the query review, which measures p95 latency at realistic volume
rather than asserting plan shapes.
