# Phase 5 — Hardening

**Deliverable (§16):** perf budgets, a11y pass, empty/error audit, mobile QA at
360px, rate-limit verification, query review.

**Exit criteria — one corrected.** §16 asks for "no seq scans on hot paths".
That is not achievable and not desirable at this scale: with ~50 students the
attempts tables hold a few thousand rows across a handful of pages, and
PostgreSQL will correctly prefer a sequential scan. The Neon indexing reference
says the same — an index does not help when a query matches more than 10–20% of
a table. Forcing index usage here would mean adding indexes that cost writes and
return nothing. **Replaced by T-5.5:** a p95 latency budget measured against
seeded volume. Lighthouse a11y ≥ 95 and the perf budgets stand as written.

T-5.1 is the one feature task, deferred out of Phase 3.

---

### T-5.1 — Implement `auto_submit`
**Depends on:** T-3.14, T-4.9
**Touches:** `web/src/features/integrity/`, `server/internal/attempts/`
**Size:** M
**Done when:**
- [ ] `onLimitExceeded: 'auto_submit'` shows a 10-second countdown with a
      "Tôi vẫn đang làm bài" cancel that **grants one final strike**, then
      submits (§10.2)
- [ ] The countdown does not pause the timer
- [ ] Cancelling twice submits — the "one final strike" is one, not unlimited
- [ ] The submit goes through the same idempotent path as a manual submit, so a
      race between auto-submit and a manual tap yields one submission
- [ ] An `auto_submit` event is recorded so the timeline shows why (§10.4)
- [ ] The student is told plainly what happened; no shame copy (§12)
- [ ] Test: `integrity/auto-submit.test.tsx` — countdown, cancel, second
      violation submits
- [ ] Test: `attempts/submit_test.go` — concurrent auto and manual submit produce
      one graded attempt
- [ ] Both locales; keyboard-operable — the cancel button must be focusable and
      reachable without a mouse (§14)

---

### T-5.2 — Verify rate limits and re-run the leak review
**Depends on:** T-1.7, T-1.8
**Touches:** `server/internal/ratelimit/`, `web/e2e/`
**Size:** M
**Done when:**
- [ ] An automated suite drives every `public`-tagged operation past its limit
      and asserts `429` with a sane `Retry-After` (§6.5)
- [ ] Per-IP (10/min, 60/hour) and per-code (30/hour) limits are both exercised,
      including per-code across multiple IPs (§6.5)
- [ ] A second leak review of every public response body: `POST /join/preview`
      returns only class name and teacher name; the four join failure modes leak
      nothing about which classes exist; login does not distinguish unknown user
      from wrong password (§6.5, §9)
- [ ] The T-0.14 startup assertion still holds: no `public` route lacks a limiter
- [ ] Confirm the limiter's memory is bounded — a flood of distinct IPs must
      evict, not grow
- [ ] Test: `e2e/rate-limits.spec.ts` — one case per public endpoint
- [ ] **Public endpoint leak review recorded in the PR (§14)**

---

### T-5.3 — Accessibility pass
**Depends on:** T-4.11
**Touches:** `web/src/`
**Size:** L
**Done when:**
- [ ] Lighthouse accessibility ≥ 95 on `/login`, `/join/:code/confirm`, `/app`,
      `/app/attempts/:id`, `/app/attempts/:id/result`, `/admin`,
      `/admin/tests/:id/edit`, `/admin/attempts/:id` (§16)
- [ ] `axe` runs in CI against those routes and fails on serious/critical
- [ ] Every interactive element is a real element with a visible focus ring;
      icon-only buttons carry `aria-label`; icons are `aria-hidden` unless
      standalone (§12)
- [ ] The take-test flow is completable end to end with the keyboard alone,
      including the audio player and the fill-blank inputs
- [ ] `prefers-reduced-motion` is honoured; 150ms ease-out is the only motion and
      there are no entrance animations (§12)
- [ ] Colour is never the sole carrier of meaning — correct/incorrect states
      carry a text or icon cue as well
- [ ] Test: `e2e/a11y.spec.ts` — axe scan per route; `e2e/keyboard.spec.ts` —
      full attempt by keyboard

---

### T-5.4 — Performance budgets and bundle verification
**Depends on:** T-4.11
**Touches:** `web/`, CI
**Size:** M
**Done when:**
- [ ] Route-level code splitting is verified, not assumed: **a student's bundle
      contains no admin code and an anonymous visitor's contains neither** (§2).
      This is asserted against the built chunk graph in CI, not by inspection
- [ ] `@dnd-kit` is absent from the student and public entry graphs (§2 says
      builder-only, lazy-loaded)
- [ ] Budgets set and enforced in CI for initial JS on `/login`, `/app` and
      `/app/attempts/:id`; a regression fails the build
- [ ] The take-test route is measured on a throttled mobile profile, since §1.1
      says students are often on a phone
- [ ] Test: `e2e/bundle-budget.spec.ts`

---

### T-5.5 — Query review at seeded volume
**Depends on:** T-4.11
**Touches:** `seed/`, `server/internal/`
**Size:** M
**Done when:**
- [ ] A volume seed exists: 50 students, 20 classes' worth of membership, 200
      questions, 20 published versions, 100 assignments, ~10,000 attempts and
      ~500,000 `attempt_events` — roughly three years of this practice's data
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` captured for every query touching `attempts`,
      `attempt_answers` or `attempt_events`, and committed to
      `docs/perf/explain-<date>.md` (§13.8)
- [ ] **p95 latency budgets**, which replace §16's seq-scan criterion: monitor
      screen < 200ms, attempt payload < 150ms, integrity timeline < 300ms,
      autosave < 100ms, dashboard < 200ms
- [ ] No query is N+1: the monitor, grading and timeline screens each issue a
      fixed number of statements regardless of row count (§13.8)
- [ ] Any index added as a result gets `CREATE INDEX CONCURRENTLY` in a migration
      marked `-- +goose NO TRANSACTION` (§13.7), because these tables now have
      rows
- [ ] Unused indexes identified via `pg_stat_user_indexes` and either justified
      or dropped
- [ ] Test: `server/internal/db/perf_test.go` — the five budgets, run against the
      volume seed, skipped by default and run in a nightly job

---

### T-5.6 — Mobile QA at 360px
**Depends on:** T-5.3
**Touches:** `web/src/`
**Size:** M
**Done when:**
- [ ] Every student and public route is usable at 360px wide with no horizontal
      scroll (§16)
- [ ] Safe-area insets respected on notched devices (§9)
- [ ] The audio player, the fill-blank inputs and the integrity dialog are all
      operable at that width
- [ ] Verified on real iOS Safari, not only in a simulator — §11.3's gesture and
      single-element rules are the ones most likely to differ
- [ ] Admin routes are confirmed to degrade gracefully but are **not** made
      mobile-friendly; §1.1 says desktop/tablet only, minimum 768px
- [ ] Test: `e2e/mobile.spec.ts` at a 360×640 viewport across the student flow

---

### T-5.7 — Empty, loading and error state audit
**Depends on:** T-4.11
**Touches:** `web/src/`
**Size:** M
**Done when:**
- [ ] Every list and detail route has all three states implemented (§14)
- [ ] Empty states are one short sentence plus one primary action, with no
      illustrations (§12)
- [ ] Every error path surfaces the envelope's `requestId` as a copyable error ID
      (§9)
- [ ] Network failure during an attempt shows a calm, specific message and never
      implies the student's work is lost
- [ ] Test: `e2e/empty-states.spec.ts` — a fresh account sees three empty
      sections on `/app`, each with its own copy

---

### T-5.8 — Language review and release
**Depends on:** T-5.7, T-5.6, T-5.5, T-5.4, T-5.2
**Touches:** `web/src/lib/i18n/`, `docs/`
**Size:** M
**Done when:**
- [ ] A native review of every `vi` string, prioritising the join screens, the
      integrity dialogs and the test intro — the three places where tone matters
      most (§6.2, §10.2, §12)
- [ ] `en` parity: no key in one locale and missing in the other (T-0.11's test)
- [ ] No English-only user-facing text anywhere (AGENTS.md)
- [ ] Layouts hold with the longest Vietnamese strings; no fixed-width labels
      (§12)
- [ ] `docs/quizzivy-spec-v0.3.md` updated for every decision that changed during
      implementation, with the version bumped (§18)
- [ ] `40-open-items.md` reconciled: every item is answered, deferred with a
      reason, or promoted to the P1 list
- [ ] `release/phase-5` merges to `main`, tagged `v1.0.0`, and back to `develop`

---

## Not in v1

Carried from §16's P1 list and confirmed here: presigned direct-to-R2 upload,
CSV import for students and questions, password self-signup with email
verification, dark mode, per-student time accommodations, per-question analytics.

Added to that list by this plan:

- **R2 object lifecycle after soft delete** — a soft-deleted `media_assets` row
  leaves its object in the bucket. Harmless at this volume; a reconciliation job
  is the eventual answer (`20-data-model.md` §6).
- **Diacritic-insensitive full-text ranking** — T-2.6's trigram index handles
  matching; ranking Vietnamese results well would want `unaccent` with an
  immutable wrapper and a stored `tsvector` (D-11).
- **A global audit-log screen** — the table and its indexes exist; no UI reads
  them, so the `(occurred_at DESC)` index is deliberately not created
  (`20-data-model.md` §5).
