<!--
One task from docs/plan/1*.md = one branch = one PR.
Title: `T-<phase>.<n>: <imperative title>`
-->

## Task

Closes **T-_._** — <title>

## What changed

<!-- Two or three sentences. What a reviewer needs before reading the diff. -->

## Definition of done (spec §14)

- [ ] TypeScript strict passes; no `any` without a comment
- [ ] Lint clean
- [ ] Loading / error / empty states present
- [ ] Keyboard-operable; visible focus
- [ ] All strings via `t()`, keys in both `vi` and `en`
- [ ] Tests added or updated at the right level (spec §14)
- [ ] The task's own "Done when" boxes are all ticked

## New dependencies

<!--
Required by spec §14 and §18: no new dependency without a stated reason.
Write "None" if there are none. Otherwise one line each: package — why, and
what was considered instead.
-->

None

---

<!--
DELETE whichever of the two blocks below does not apply.
Leaving an unticked block is a request for changes.
-->

## This PR touches the database

- [ ] Migration file named here: `_____.sql`
- [ ] `-- +goose Down` written and verified (`up` → `down` → `up` is clean)
- [ ] DDL reviewed against spec §13, `docs/plan/20-data-model.md`, and the Neon
      `postgres-best-practices` skill
- [ ] Any deviation from `20-data-model.md` is recorded there with a `D-nn` entry
- [ ] `CREATE INDEX CONCURRENTLY` (if any) is in a `-- +goose NO TRANSACTION` file
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` pasted below for any query touching
      `attempts`, `attempt_answers` or `attempt_events` (spec §13.8)

## This PR adds a public (unauthenticated) endpoint

Spec §6.5 and §14. Both boxes, in this PR — not a follow-up.

- [ ] **Rate limited** — per-IP and, where a shared secret is involved, per-key.
      Returns `429` with `Retry-After`. Test names the limits it asserts.
- [ ] **Leak reviewed** — the response body was read field by field. It reveals
      nothing about which classes, students or tests exist. Distinct failure
      modes return distinct codes but identical detail. Paste the exact response
      shape below.

```json

```
