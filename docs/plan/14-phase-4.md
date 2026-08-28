# Phase 4 — Grade & results

**Deliverable (§16):** monitor, attempt review + integrity timeline, grading with
sample answers, result page honoring review + transcript flags.
**Exit criteria (§16):** full E2E suite green, including 9 — plus E2E 1 and E2E 2
in full, which Phases 2 and 1 could not run (see those files).

Backend first so each screen has real data the day it is built.

---

### T-4.1 — Implement the assignment monitor endpoint
**Depends on:** T-3.9
**Touches:** `server/internal/attempts/`
**Size:** M
**Done when:**
- [ ] `GET /admin/assignments/:id/attempts` returns one row per targeted student:
      not started / in progress / submitted / graded, with live remaining time,
      focus-loss count, flagged, and an audio summary (§8, §15)
- [ ] **Two queries total** — one for the roster, one for the attempts — not one
      per student. N+1 is the named default failure mode of this screen (§13.8)
- [ ] Students with no attempt appear as "not started"; the roster is the left
      side of the join, not the attempts table
- [ ] Remaining time is derived from `deadline_at` and `serverTime`, never from
      the caller's clock
- [ ] Test: `attempts/monitor_test.go` — a roster of 50 with 30 attempts issues
      exactly two queries
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` recorded in the PR (§13.8)

---

### T-4.2 — Implement extend, reset and void
**Depends on:** T-4.1
**Touches:** `server/internal/attempts/`, `server/internal/audit/`
**Size:** M
**Done when:**
- [ ] `POST /admin/attempts/:id/extend` `{ minutes, reason }`,
      `/reset` `{ reason }`, `/void` `{ reason }` per §15
- [ ] **Each writes its audit row in the same statement as the mutation**, using
      the `OLD`/`NEW` data-modifying CTE from `00-overview.md` §4.4 — not a
      read-then-write (§13.4)
- [ ] `reason` is required and non-empty; the API rejects a blank one rather than
      storing it
- [ ] `void` sets `status = 'voided'` and `void_reason`, satisfying the
      `attempts_void_has_reason` constraint
- [ ] `reset` voids the existing attempt and permits a new one; it never deletes,
      so §6.4's retention holds
- [ ] `extend` may push `deadline_at` past `closes_at` — that is the point of an
      accommodation, and the constraint only requires it exceed `started_at`
- [ ] Test: `attempts/admin_actions_test.go` — one audit row per action with a
      `diff` containing both old and new values
- [ ] Test: `attempts/admin_actions_test.go` — an empty reason is rejected
- [ ] Test: `attempts/reset_test.go` — after reset the old attempt is still
      readable and the new one starts at `attempt_no + 1`

---

### T-4.3 — Implement grading
**Depends on:** T-3.9
**Touches:** `server/internal/grading/`
**Size:** M
**Done when:**
- [ ] `POST /admin/attempts/:id/grade` `{ items: [{questionId, points, comment}] }`
      writes `manual_score`, `grader_comment`, `graded_by`, `graded_at` (§15,
      D-19)
- [ ] `points` is validated against the question's `points` ceiling and rejected
      above it
- [ ] `POST /admin/attempts/:id/finish-grading` recomputes `score_earned` from
      `SUM(final_score)` and sets `status = 'graded'`, `graded_at`
- [ ] Finish-grading is rejected while any `requires_manual AND manual_score IS
      NULL` row remains, using `attempt_answers_pending_idx`
- [ ] Grading is re-enterable: a graded attempt can be re-graded and the score
      recomputed
- [ ] Test: `grading/grade_test.go` — `SUM(final_score)` uses the manual score
      where present and the auto score otherwise, exercising the VIRTUAL column
- [ ] Test: `grading/finish_test.go` — finishing with an ungraded short answer is
      rejected
- [ ] Test: `grading/grade_test.go` — points above the question ceiling rejected

---

### T-4.4 — Implement the integrity events endpoint
**Depends on:** T-3.8
**Touches:** `server/internal/integrity/`
**Size:** S
**Done when:**
- [ ] `GET /admin/attempts/:id/events` returns the ordered timeline with
      `occurred_at`, offset from `started_at`, kind, question, and paired
      durations (§10.4, §15)
- [ ] Pairing (`tab_hidden`/`tab_visible`, `window_blur`/`window_focus`,
      `fullscreen_enter`/`fullscreen_exit`) is computed server-side so the client
      does not reimplement it
- [ ] An unpaired opening event at the end of the log (the student never came
      back) renders as open-ended rather than being dropped
- [ ] Summary strip data: total away-time, episodes ≥ `minAwayMs`, paste count,
      resume count, audio replays (§10.4)
- [ ] Ordering uses `client_seq` within a session and `occurred_at` across
      sessions, so clock skew cannot scramble a session's internal order (§10.6)
- [ ] Test: `integrity/timeline_test.go` — pairing across a resume boundary, and
      an unclosed final episode
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` recorded (§13.8)

---

### T-4.5 — Implement the result endpoint
**Depends on:** T-4.3
**Touches:** `server/internal/attempts/`
**Size:** M
**Done when:**
- [ ] `GET /app/attempts/:id/result` honours the assignment's review policy:
      `showScore`, `showCorrectAnswers`, `showExplanations` (§7, §9)
- [ ] `transcript` is returned **only** when `audio_show_transcript_after` is
      true, and only from this endpoint — never from `GET /app/attempts/:id`
      (§13.5)
- [ ] `sampleAnswer` and accepted blank answers are **never** returned here,
      regardless of review policy — §13.5 lists them without exception
- [ ] `pendingManual > 0` is reported so the client can show the "chờ chấm" banner
      (§9)
- [ ] Test: `attempts/result_test.go` — the eight combinations of the three
      review flags, each asserting exactly which keys are present
- [ ] Test: `attempts/result_test.go` — `sampleAnswer` is absent even with all
      review flags on
- [ ] Test: `attempts/result_test.go` — transcript present iff the policy allows

---

### T-4.6 — Implement the admin dashboard aggregates
**Depends on:** T-4.1
**Touches:** `server/internal/attempts/`
**Size:** S
**Done when:**
- [ ] A dashboard endpoint (added to the contract in T-0.7 — §15 has none)
      returns open assignments, attempts awaiting grading, active students,
      flagged attempts, and recent attempts (§8)
- [ ] Awaiting-grading and flagged counts use `attempts_grading_queue_idx` and
      `attempts_flagged_idx`, both partial and near-empty
- [ ] One round trip, not five
- [ ] Test: `attempts/dashboard_test.go` — each count against a seeded fixture
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` recorded (§13.8)

---

### T-4.7 — Build the assignment monitor screen
**Depends on:** T-4.6, T-4.2
**Touches:** `web/src/features/attempts/pages/`
**Size:** M
**Done when:**
- [ ] `/admin/assignments/:id` shows the per-student table from T-4.1, with live
      remaining time and live focus-loss (§8)
- [ ] Polls every 15s **only while the assignment is `open`**, and stops when the
      tab is hidden (§8)
- [ ] Extend / reset / void each have a confirm dialog with a required reason
      field (§8)
- [ ] Dense table, ~40px rows, per §12
- [ ] Test: `monitor.test.tsx` — polling stops on close and on tab hide
- [ ] Test: `monitor.test.tsx` — an action is blocked until a reason is entered
- [ ] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-4.8 — Build the attempt review and grading screen
**Depends on:** T-4.3
**Touches:** `web/src/features/attempts/pages/`
**Size:** L
**Done when:**
- [ ] `/admin/attempts/:id` renders per question with auto-graded results shown
      (§8)
- [ ] `short_answer` gets a points input, a comment field, and the **sample
      answer panel** (§8, §7 — admin only)
- [ ] Audio questions show plays used vs allowed, including over-limit, presented
      neutrally (§8, §11.4)
- [ ] "Finish grading" is disabled while any manual item is ungraded, with the
      count visible
- [ ] Grading state is saved per question rather than in one giant submit, so a
      half-graded attempt survives a refresh
- [ ] Test: `grading.test.tsx` — the sample answer renders for `short_answer` and
      for no other type
- [ ] Test: `grading.test.tsx` — finish is blocked with a remaining item
- [ ] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-4.9 — Build the integrity timeline tab
**Depends on:** T-4.4, T-4.8
**Touches:** `web/src/features/integrity/components/`
**Size:** M
**Done when:**
- [ ] Chronological timeline with kind, wall-clock time, offset from attempt
      start, duration for paired events, and the question on screen (§10.4)
- [ ] Summary strip: total away-time, episodes ≥ `minAwayMs`, paste count,
      resume count, audio replays (§10.4)
- [ ] **Neutral presentation.** No red banners, no "CHEATING DETECTED", no alarm
      iconography. The teacher judges; the app reports (§10.4, §12)
- [ ] Help text states §10.5's limits verbatim in spirit: this cannot see a
      second device, a phone beside the laptop, a person in the room, or a
      printed sheet
- [ ] A `network_offline` episode is visually distinguishable from a focus loss,
      because §10.1 says distinguishing bad wifi from cheating matters for
      fairness
- [ ] Test: `timeline.test.tsx` — a paired away episode renders its duration; an
      unpaired trailing event renders as open-ended
- [ ] Test: `timeline.test.tsx` — no element carries a destructive/red semantic
      class (§12)
- [ ] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-4.10 — Build the student result page
**Depends on:** T-4.5
**Touches:** `web/src/features/results/`
**Size:** M
**Done when:**
- [ ] `/app/attempts/:id/result` shows the score if allowed and per-question
      review honouring `review.*` (§9)
- [ ] Transcript rendered when `showTranscriptAfterSubmit` — this is also the
      accessibility fallback for hard-of-hearing students (§11.3)
- [ ] "Chờ chấm" banner when `pendingManual > 0` (§9)
- [ ] Nothing renders a field the API did not send; the UI has no fallback that
      could surface a leaked key
- [ ] Test: `result.test.tsx` — each review-flag combination renders the right
      sections and no more
- [ ] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-4.11 — Build the admin dashboard and close the phase
**Depends on:** T-4.10, T-4.7, T-4.9
**Touches:** `web/src/features/`, `web/tests/e2e/`
**Size:** M
**Done when:**
- [ ] `/admin` renders open assignments, attempts awaiting grading, active
      students, flagged attempts and recent attempts (§8)
- [ ] `/admin/students` list with create/edit, linked providers, `joined_via`,
      and reset password (§8). CSV import is **not** built — P1 (§16)
- [ ] **E2E 1** in full: create → publish → **assign** (the half Phase 2 deferred)
- [ ] **E2E 2** in full: password login → start → answer → reload mid-test →
      answers persist → submit → see result (the half Phase 1 deferred)
- [ ] **E2E 9**: `GET /app/attempts/:id` contains no `isCorrect`, `sampleAnswer`,
      `transcript` or `acceptedAnswers`, asserted recursively at every depth
      (§13.5, §14)
- [ ] E2E 3–8 still pass — **full suite green, §16 exit criterion met**
- [ ] `release/phase-4` merges to `main` and back to `develop` (§16)
