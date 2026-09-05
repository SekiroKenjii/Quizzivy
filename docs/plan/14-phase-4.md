# Phase 4 — Grade & results

**Deliverable (§16):** monitor, attempt review + integrity timeline, grading with
sample answers, result page honoring review + transcript flags.
**Exit criteria (§16):** full E2E suite green, including 9 — plus E2E 1 and E2E 2
in full, which Phases 2 and 1 could not run (see those files).

Backend first so each screen has real data the day it is built.

**Where the build departed from the text above, and why.**

- T-4.1's "two queries" is the store's promise, pinned by a tracer on the
  pool. The service adds a sweep before it: in-progress attempts past their
  deadline are closed (and auto-graded) first, so the monitor never shows a
  live row beside a deadline in the past and a timed-out essay reaches the
  grading queue even if its student never came back. That is one query plus
  one per expired attempt, on the read path rather than a scheduler.
- T-4.3's DB half lives in `server/internal/review/`, not `grading/`.
  `attempts` imports `grading` for the pure rules, so a grading store that
  reads attempts would be an import cycle; `review` is the teacher's side of
  an attempt and the one package that hands the key to a screen.
- T-4.2's audit tests are in `attempts/`, beside the code, as the other
  packages do; the plan named a separate `audit/` touch that was not needed.
- Grade also folds `SUM(final_score)` onto `attempts.score_earned` on every
  call, not only on finish, so the monitor's "26/30 · chờ chấm 2" is live.
- The monitor response carries `questionCount`, and each row `startedAt` and
  `answeredCount`; the review and result responses carry `testTitle` and
  `maxAttempts`. G-02, G-03 and S-09 draw all five and §15 had no source.
- G-04 (grade by question), G-05's "Ghi chú của bạn" and the manual
  flag/unflag were left out of the phase for want of a contract and filed as
  #75–#77; all three were built on 2026-09-05 (`GET
  /admin/assignments/{id}/answers`, `PATCH /admin/attempts/{id}/note`,
  `POST /admin/attempts/{id}/flag`) in the post-Đợt-2 re-evaluation.
- "Chờ chấm" is a nav item with a route, `/admin/grading`, as A-00 argued;
  the sheet gained board G-10 for it. The dashboard's two queue cards land
  there instead of on the assignments list.
- A closed assignment keeps the per-student table under G-09's results
  strip, without polling, because it is the way into grading once the window
  has shut.
- Student home: a completed card's title links to the result (S-03 draws no
  affordance; S-09 is otherwise unreachable).
- E2E 9 has its own seed assignment (`…ee09`), for the same reason E2E 2 has
  its own: two specs on one assignment is a session takeover.

---

### T-4.1 — Implement the assignment monitor endpoint
**Depends on:** T-3.9
**Touches:** `server/internal/attempts/`
**Size:** M
**Done when:**
- [x] `GET /admin/assignments/:id/attempts` returns one row per targeted student:
      not started / in progress / submitted / graded, with live remaining time,
      focus-loss count, flagged, and an audio summary (§8, §15)
- [x] **Two queries total** — one for the roster, one for the attempts — not one
      per student. N+1 is the named default failure mode of this screen (§13.8)
- [x] Students with no attempt appear as "not started"; the roster is the left
      side of the join, not the attempts table
- [x] Remaining time is derived from `deadline_at` and `serverTime`, never from
      the caller's clock
- [x] Test: `attempts/monitor_test.go` — a roster of 50 with 30 attempts issues
      exactly two queries
- [x] `EXPLAIN (ANALYZE, BUFFERS)` recorded in the PR (§13.8)

---

### T-4.2 — Implement extend, reset and void
**Depends on:** T-4.1
**Touches:** `server/internal/attempts/`, `server/internal/audit/`
**Size:** M
**Done when:**
- [x] `POST /admin/attempts/:id/extend` `{ minutes, reason }`,
      `/reset` `{ reason }`, `/void` `{ reason }` per §15
- [x] **Each writes its audit row in the same statement as the mutation**, using
      the `OLD`/`NEW` data-modifying CTE from `00-overview.md` §4.4 — not a
      read-then-write (§13.4)
- [x] `reason` is required and non-empty; the API rejects a blank one rather than
      storing it
- [x] `void` sets `status = 'voided'` and `void_reason`, satisfying the
      `attempts_void_has_reason` constraint
- [x] `reset` voids the existing attempt and permits a new one; it never deletes,
      so §6.4's retention holds
- [x] `extend` may push `deadline_at` past `closes_at` — that is the point of an
      accommodation, and the constraint only requires it exceed `started_at`
- [x] Test: `attempts/admin_actions_test.go` — one audit row per action with a
      `diff` containing both old and new values
- [x] Test: `attempts/admin_actions_test.go` — an empty reason is rejected
- [x] Test: `attempts/reset_test.go` — after reset the old attempt is still
      readable and the new one starts at `attempt_no + 1`

---

### T-4.3 — Implement grading
**Depends on:** T-3.9
**Touches:** `server/internal/grading/`
**Size:** M
**Done when:**
- [x] `POST /admin/attempts/:id/grade` `{ items: [{questionId, points, comment}] }`
      writes `manual_score`, `grader_comment`, `graded_by`, `graded_at` (§15,
      D-19)
- [x] `points` is validated against the question's `points` ceiling and rejected
      above it
- [x] `POST /admin/attempts/:id/finish-grading` recomputes `score_earned` from
      `SUM(final_score)` and sets `status = 'graded'`, `graded_at`
- [x] Finish-grading is rejected while any `requires_manual AND manual_score IS
      NULL` row remains, using `attempt_answers_pending_idx`
- [x] Grading is re-enterable: a graded attempt can be re-graded and the score
      recomputed
- [x] Test: `grading/grade_test.go` — `SUM(final_score)` uses the manual score
      where present and the auto score otherwise, exercising the VIRTUAL column
- [x] Test: `grading/finish_test.go` — finishing with an ungraded short answer is
      rejected
- [x] Test: `grading/grade_test.go` — points above the question ceiling rejected

---

### T-4.4 — Implement the integrity events endpoint
**Depends on:** T-3.8
**Touches:** `server/internal/integrity/`
**Size:** S
**Done when:**
- [x] `GET /admin/attempts/:id/events` returns the ordered timeline with
      `occurred_at`, offset from `started_at`, kind, question, and paired
      durations (§10.4, §15)
- [x] Pairing (`tab_hidden`/`tab_visible`, `window_blur`/`window_focus`,
      `fullscreen_enter`/`fullscreen_exit`) is computed server-side so the client
      does not reimplement it
- [x] An unpaired opening event at the end of the log (the student never came
      back) renders as open-ended rather than being dropped
- [x] Summary strip data: total away-time, episodes ≥ `minAwayMs`, paste count,
      resume count, audio replays (§10.4)
- [x] Ordering uses `client_seq` within a session and `occurred_at` across
      sessions, so clock skew cannot scramble a session's internal order (§10.6)
- [x] Test: `integrity/timeline_test.go` — pairing across a resume boundary, and
      an unclosed final episode
- [x] `EXPLAIN (ANALYZE, BUFFERS)` recorded (§13.8)

---

### T-4.5 — Implement the result endpoint
**Depends on:** T-4.3
**Touches:** `server/internal/attempts/`
**Size:** M
**Done when:**
- [x] `GET /app/attempts/:id/result` honours the assignment's review policy:
      `showScore`, `showCorrectAnswers`, `showExplanations` (§7, §9)
- [x] `transcript` is returned **only** when `audio_show_transcript_after` is
      true, and only from this endpoint — never from `GET /app/attempts/:id`
      (§13.5)
- [x] `sampleAnswer` and accepted blank answers are **never** returned here,
      regardless of review policy — §13.5 lists them without exception
- [x] `pendingManual > 0` is reported so the client can show the "chờ chấm" banner
      (§9)
- [x] Test: `attempts/result_test.go` — the eight combinations of the three
      review flags, each asserting exactly which keys are present
- [x] Test: `attempts/result_test.go` — `sampleAnswer` is absent even with all
      review flags on
- [x] Test: `attempts/result_test.go` — transcript present iff the policy allows

---

### T-4.6 — Implement the admin dashboard aggregates
**Depends on:** T-4.1
**Touches:** `server/internal/attempts/`
**Size:** S
**Done when:**
- [x] A dashboard endpoint (added to the contract in T-0.7 — §15 has none)
      returns open assignments, attempts awaiting grading, active students,
      flagged attempts, and recent attempts (§8)
- [x] Awaiting-grading and flagged counts use `attempts_grading_queue_idx` and
      `attempts_flagged_idx`, both partial and near-empty
- [x] One round trip, not five
- [x] Test: `attempts/dashboard_test.go` — each count against a seeded fixture
- [x] `EXPLAIN (ANALYZE, BUFFERS)` recorded (§13.8)

---

### T-4.7 — Build the assignment monitor screen
**Depends on:** T-4.6, T-4.2
**Touches:** `web/src/features/attempts/pages/`
**Size:** M
**Done when:**
- [x] `/admin/assignments/:id` shows the per-student table from T-4.1, with live
      remaining time and live focus-loss (§8)
- [x] Polls every 15s **only while the assignment is `open`**, and stops when the
      tab is hidden (§8)
- [x] Extend / reset / void each have a confirm dialog with a required reason
      field (§8)
- [x] Dense table, ~40px rows, per §12
- [x] Test: `monitor.test.tsx` — polling stops on close and on tab hide
- [x] Test: `monitor.test.tsx` — an action is blocked until a reason is entered
- [x] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-4.8 — Build the attempt review and grading screen
**Depends on:** T-4.3
**Touches:** `web/src/features/attempts/pages/`
**Size:** L
**Done when:**
- [x] `/admin/attempts/:id` renders per question with auto-graded results shown
      (§8)
- [x] `short_answer` gets a points input, a comment field, and the **sample
      answer panel** (§8, §7 — admin only)
- [x] Audio questions show plays used vs allowed, including over-limit, presented
      neutrally (§8, §11.4)
- [x] "Finish grading" is disabled while any manual item is ungraded, with the
      count visible
- [x] Grading state is saved per question rather than in one giant submit, so a
      half-graded attempt survives a refresh
- [x] Test: `grading.test.tsx` — the sample answer renders for `short_answer` and
      for no other type
- [x] Test: `grading.test.tsx` — finish is blocked with a remaining item
- [x] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-4.9 — Build the integrity timeline tab
**Depends on:** T-4.4, T-4.8
**Touches:** `web/src/features/integrity/components/`
**Size:** M
**Done when:**
- [x] Chronological timeline with kind, wall-clock time, offset from attempt
      start, duration for paired events, and the question on screen (§10.4)
- [x] Summary strip: total away-time, episodes ≥ `minAwayMs`, paste count,
      resume count, audio replays (§10.4)
- [x] **Neutral presentation.** No red banners, no "CHEATING DETECTED", no alarm
      iconography. The teacher judges; the app reports (§10.4, §12)
- [x] Help text states §10.5's limits verbatim in spirit: this cannot see a
      second device, a phone beside the laptop, a person in the room, or a
      printed sheet
- [x] A `network_offline` episode is visually distinguishable from a focus loss,
      because §10.1 says distinguishing bad wifi from cheating matters for
      fairness
- [x] Test: `timeline.test.tsx` — a paired away episode renders its duration; an
      unpaired trailing event renders as open-ended
- [x] Test: `timeline.test.tsx` — no element carries a destructive/red semantic
      class (§12)
- [x] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-4.10 — Build the student result page
**Depends on:** T-4.5
**Touches:** `web/src/features/results/`
**Size:** M
**Done when:**
- [x] `/app/attempts/:id/result` shows the score if allowed and per-question
      review honouring `review.*` (§9)
- [x] Transcript rendered when `showTranscriptAfterSubmit` — this is also the
      accessibility fallback for hard-of-hearing students (§11.3)
- [x] "Chờ chấm" banner when `pendingManual > 0` (§9)
- [x] Nothing renders a field the API did not send; the UI has no fallback that
      could surface a leaked key
- [x] Test: `result.test.tsx` — each review-flag combination renders the right
      sections and no more
- [x] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-4.11 — Build the admin dashboard and close the phase
**Depends on:** T-4.10, T-4.7, T-4.9
**Touches:** `web/src/features/`, `web/tests/e2e/`
**Size:** M
**Done when:**
- [x] `/admin` renders open assignments, attempts awaiting grading, active
      students, flagged attempts and recent attempts (§8)
- [x] `/admin/students` list with create/edit, linked providers, `joined_via`,
      and reset password (§8). CSV import is **not** built — P1 (§16)
- [ ] **E2E 1** in full: create → publish → **assign** (the half Phase 2 deferred)
- [ ] **E2E 2** in full: password login → start → answer → reload mid-test →
      answers persist → submit → see result (the half Phase 1 deferred)
- [ ] **E2E 9**: `GET /app/attempts/:id` contains no `isCorrect`, `sampleAnswer`,
      `transcript` or `acceptedAnswers`, asserted recursively at every depth
      (§13.5, §14)
- [ ] E2E 3–8 still pass — **full suite green, §16 exit criterion met**
- [ ] `release/phase-4` merges to `main` and back to `develop` (§16)
