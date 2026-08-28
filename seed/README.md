# Seed data

Never in a migration (spec §13.7). `make seed` applies every `.sql` here in
name order, against a database that has already been migrated.

Two seeds are planned:

- `01-dev.sql` — one admin, one class, a handful of questions. Enough to click
  through the app. Added with T-1.1.
- `02-volume.sql` — ~50 students, 10k attempts, 500k integrity events. Used by
  T-5.5's query review, which measures p95 latency at realistic volume rather
  than asserting plan shapes.
