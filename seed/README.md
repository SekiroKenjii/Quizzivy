# Seed data

Never in a migration (spec §13.7). `make seed` applies every `.sql` here in
name order, against a database that has already been migrated.

What is here:

- `01-dev.sql` — one admin, one class, a handful of questions. Enough to click
  through the app.
- `02-dev-assignments.sql` — one published test, one open assignment, one
  submitted-and-flagged attempt, so `/admin` has something to count.
- `03-dev-students.sql` — a few more students, including a Google-only one.
- `99-assert.sql` — runs last and refuses the seed if the rows above break a
  rule `publish` would have enforced. The seed writes frozen version rows
  directly, which is the one path that skips that validation.

Still planned: a volume seed (~50 students, 10k attempts, 500k integrity
events) for T-5.5's query review, which measures p95 latency at realistic
volume rather than asserting plan shapes.
