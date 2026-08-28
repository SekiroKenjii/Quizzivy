# Quizzivy — Agent Working Rules

Read this file at the start of every session. Read `quizzivy-spec-v0.3.md`
before touching any area you have not worked on before.

## Sources of truth, in order

1. `quizzivy-spec-v0.3.md` — product and engineering spec
2. `docs/plan/` — the implementation plan derived from it
3. Neon `postgres-best-practices` skill — all DDL and SQL
4. <https://www.postgresql.org/docs/18/index.html> — anything PG-version-specific

Where 1 and 2 disagree, the spec wins and the plan gets corrected.
Where 3 and 4 disagree, the PostgreSQL docs win.

## Non-negotiable

- Target PostgreSQL 18. Do not write DDL that silently degrades on 16/17.
- No `SELECT *` anywhere in application code.
- Student-facing payloads never contain `isCorrect`, `sampleAnswer`,
  `acceptedAnswers`, or `transcript`. There is a test asserting this;
  do not weaken it.
- Every unauthenticated endpoint is rate-limited and leak-reviewed in the
  same PR that adds it.
- Never remove or skip a failing test to make CI green. Fix it or flag it.
- No new dependency without a stated reason in the PR description.
- Never commit secrets. `.env.example` stays current; `.env` stays ignored.

## Design

Follow spec §12 exactly. Neutral zinc palette, charcoal primary buttons,
semantic color only where it carries meaning. No gradients, no
glassmorphism, no pulse rings, no glow, no emoji in UI chrome. If a
design choice feels like it needs a new color, it probably needs a
different layout instead.

## Language

Vietnamese first. Write the `vi` string, then `en`. No English-only
user-facing text ever reaches a commit. Code, comments, commit messages,
and docs are English.

## Per-PR checklist

- [ ] TypeScript strict passes; no `any` without a comment
- [ ] Lint clean
- [ ] Loading / error / empty states present
- [ ] Keyboard-operable; visible focus
- [ ] Tests added or updated at the right level (spec §14)
- [ ] DDL reviewed against spec §13 and the Neon skill
- [ ] `.env.example` updated if config changed

## High-risk areas — extra care

`features/take-test/`, `features/integrity/`, `features/media/`, and
anything touching `attempts` or `test_versions`. Run the relevant unit
tests before and after every change to these. Do not refactor them
opportunistically while doing something else.

## When you are unsure

Ask one precise question. Do not guess on §5 (auth), §6 (join codes),
§10 (integrity), §11 (audio), or §13 (data model). Guessing in these
areas is more expensive than waiting for an answer.
