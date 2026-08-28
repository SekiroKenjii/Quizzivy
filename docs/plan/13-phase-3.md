# Phase 3 — Assign & take

**Deliverable (§16):** assignment create/list, student home, intro, take-test
engine, audio player, integrity capture.
**Exit criteria (§16):** E2E 5, 6, 7, 8 pass.

The riskiest phase. `features/take-test/`, `features/integrity/` and
`features/media/` are the high-risk areas AGENTS.md names — run their unit tests
before and after every change, and do not refactor them opportunistically.

**One scope change:** `onLimitExceeded: 'auto_submit'` (§10.2) is deferred to
Phase 5 (T-5.1). `warn` and `flag` ship here. The 10-second countdown with a
cancel that grants "one final strike" is the most intricate state machine in the
product, and §10.3 defaults the policy to `flag`, so nothing in the default
experience depends on it. Shipping the engine without it lowers the risk on the
phase that carries the most.

---

### T-3.1 — Add the assignment migrations
**Depends on:** T-2.9
**Touches:** `migrations/`
**Size:** M
**Migrations:** `00016_create_assignments.sql`,
`00017_create_assignment_targets.sql`
**Done when:**
- [ ] Matches `20-data-model.md` §9: flattened review and integrity policy
      columns with §10.3's defaults, `closed_at` and no `status` column (D-18),
      and the composite FK to `test_versions (id, test_id)` (D-17)
- [ ] Test: `constraints_test.go` — an assignment pointing at a version of a
      different test is rejected (D-17)
- [ ] Test: `constraints_test.go` — `closes_at <= opens_at` is rejected
- [ ] Test: `constraints_test.go` — a new row's integrity defaults equal §10.3
- [ ] DDL reviewed against §13 and the Neon skill (§14)

---

### T-3.2 — Implement assignment CRUD and target resolution
**Depends on:** T-3.1
**Touches:** `server/internal/assignments/`
**Size:** L
**Done when:**
- [ ] `GET /admin/assignments`, `POST`, `GET /:id`, `PATCH /:id` per §15
- [ ] Only a **published** test version may be assigned (§8)
- [ ] `status` is derived in the projection from `opens_at`, `closes_at` and
      `closed_at` — never stored (D-18)
- [ ] `test_version_id` may be changed **only while no attempt exists** for the
      assignment; otherwise 409. Existing attempts keep their own version
      regardless (§7's invariant)
- [ ] Target resolution unions `assignment_classes → class_members` with
      `assignment_students`, de-duplicated, in one query — not one query per
      class (§13.8)
- [ ] `GET /app/assignments` returns `{ dueNow, upcoming, completed }` (§15)
- [ ] Test: `assignments/targets_test.go` — a student in two targeted classes
      appears once
- [ ] Test: `assignments/repoint_test.go` — changing the version after an attempt
      exists is 409
- [ ] Test: `assignments/status_test.go` — the three derived states at
      boundaries, including an early `closed_at`
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` on the target-resolution query recorded in the
      PR (§13.8)

---

### T-3.3 — Add the attempts migration
**Depends on:** T-3.1
**Touches:** `migrations/`
**Size:** S
**Migrations:** `00018_create_attempts.sql`
**Done when:**
- [ ] Matches `20-data-model.md` §10, including `shuffle_seed` (D-02),
      `beacon_token_hash` (D-03), `attempts_one_live`, and the two partial
      dashboard indexes
- [ ] Test: `constraints_test.go` — a second `in_progress` attempt for the same
      (assignment, student) is rejected by `attempts_one_live`
- [ ] Test: `constraints_test.go` — `deadline_at <= started_at` is rejected;
      status `voided` without a reason is rejected
- [ ] DDL reviewed against §13 and the Neon skill (§14)

---

### T-3.4 — Implement attempt creation and resume
**Depends on:** T-3.3, T-3.2
**Touches:** `server/internal/attempts/`
**Size:** L
**Done when:**
- [ ] `POST /app/assignments/:id/attempts` creates **or resumes**, returning the
      attempt, the ordered questions, `sessionId` and the beacon token (§15)
- [ ] Eligibility checked at creation: assignment open, student targeted,
      `attempt_no <= max_attempts`
- [ ] `deadline_at` is computed **server-side** as
      `min(now() + duration, closes_at)` — see `40-open-items.md` for the
      in-flight-close default, which this encodes
- [ ] `shuffle_seed` is drawn once from a CSPRNG; question and option order are a
      pure function of `(seed, id)` so client, server and grading agree (D-02)
- [ ] Resuming issues a **new** `session_id`, writes a `resume` event, and — if
      the previous session was live — a `session_takeover` event (§10.1)
- [ ] Idempotent under a double-tap: the `attempts_one_live` unique loses the
      race and the handler resumes
- [ ] A fresh beacon token is minted per session and stored hashed (D-03)
- [ ] Test: `attempts/create_test.go` — two concurrent creates yield one attempt
- [ ] Test: `attempts/shuffle_test.go` — the same seed yields the same order
      across 1000 runs, and different seeds differ
- [ ] Test: `attempts/deadline_test.go` — a 60-minute duration on an assignment
      closing in 10 minutes yields a 10-minute deadline
- [ ] Test: `attempts/resume_test.go` — resume supersedes the prior session

---

### T-3.5 — Implement the student attempt payload
**Depends on:** T-3.4
**Touches:** `server/internal/attempts/`
**Size:** M
**Migrations:** `00019_create_attempt_answers.sql`
**Done when:**
- [ ] `00019` matches `20-data-model.md` §10, including the VIRTUAL
      `final_score`, `requires_manual`, `graded_by`, `graded_at` (D-19)
- [ ] `GET /app/attempts/:id` returns the attempt, questions in shuffled order,
      `serverTime`, and `audioPlays` (§15)
- [ ] **Explicit column lists only.** `is_correct`, `sample_answer`,
      `transcript` and accepted blank answers are not selected, and the Go
      response type has no field for them (§13.5)
- [ ] `serverTime` is returned so the client can compute clock offset rather
      than trusting the device clock
- [ ] Test: `attempts/payload_test.go` — a recursive walk of the JSON response
      finds no `isCorrect`, `sampleAnswer`, `acceptedAnswers` or `transcript` at
      **any depth**. This is E2E 9's unit-level twin; §13.5 requires it
- [ ] Test: `attempts/payload_test.go` — another student's attempt is 403
- [ ] DDL reviewed against §13 and the Neon skill (§14)

---

### T-3.6 — Implement answer autosave
**Depends on:** T-3.5
**Touches:** `server/internal/attempts/`
**Size:** M
**Done when:**
- [ ] `PATCH /app/attempts/:id/answers` takes `{ sessionId, answers[], events[] }`
      (§15) and upserts answers by `(attempt_id, question_id)`
- [ ] A `sessionId` that is not the attempt's current session is rejected with
      `SESSION_SUPERSEDED`, which is how the first tab learns it lost (§10.1's
      `session_takeover`, E2E 7)
- [ ] Writes after `deadline_at` are rejected; the client is told to submit
- [ ] Answers and events land in one transaction so a partial flush cannot record
      an event for an answer that was not saved
- [ ] Events are inserted with `ON CONFLICT (attempt_id, session_id, client_seq)
      DO NOTHING` (D-01)
- [ ] Test: `attempts/autosave_test.go` — a superseded session gets 409 and
      writes nothing
- [ ] Test: `attempts/autosave_test.go` — replaying the identical batch twice is
      a no-op
- [ ] `EXPLAIN (ANALYZE, BUFFERS)` on the upsert recorded in the PR (§13.8)

---

### T-3.7 — Implement server-authoritative audio play counting
**Depends on:** T-3.5
**Touches:** `server/internal/attempts/`
**Size:** S
**Migrations:** `00020_create_attempt_audio_plays.sql`
**Done when:**
- [ ] `POST /app/attempts/:id/audio-play` takes `{ questionId }` and returns
      `{ plays, maxPlays }` (§15)
- [ ] The increment is one `INSERT … ON CONFLICT DO UPDATE … RETURNING plays`
      (`20-data-model.md` §10)
- [ ] **The server never rejects a play and never rejects a submit based on play
      count** (§11.4). Over-limit plays are reported to the teacher, not enforced
- [ ] Test: `attempts/audio_plays_test.go` — ten concurrent increments yield
      exactly ten plays
- [ ] Test: `attempts/audio_plays_test.go` — a play beyond `maxPlays` returns
      200 with the higher count, not an error

---

### T-3.8 — Implement the event flush endpoints
**Depends on:** T-3.6
**Touches:** `server/internal/integrity/`
**Size:** M
**Migrations:** `00021_create_attempt_events.sql`, `00022_grant_app_role.sql`
**Done when:**
- [ ] `00021` matches `20-data-model.md` §10, with the `(attempt_id, session_id,
      client_seq)` unique (D-01)
- [ ] `00022` grants `quizzivy_app` DML on `app` and then **revokes `UPDATE` and
      `DELETE` on `attempt_events` and `audit_log`** — this is what turns §13.3's
      "no `UPDATE` or `DELETE` is ever issued" from a promise into a privilege.
      It runs last because it grants `ON ALL TABLES`, and adds
      `ALTER DEFAULT PRIVILEGES` so later tables are covered
      (`20-data-model.md` §11)
- [ ] `POST /app/attempts/:id/events` accepts both the bearer-authenticated JSON
      path and the **beacon path**: `text/plain` body carrying `beaconToken`
      (`00-overview.md` §4.3)
- [ ] The beacon token is verified against `beacon_token_hash` in constant time
      and grants **append-only event access** — no reads, no answer writes
- [ ] `occurred_at` is offset-corrected against `received_at` using the client's
      reported skew; both are stored (§13.3)
- [ ] Unknown `kind` values are stored, not rejected — this is telemetry, and a
      newer client must not fail against an older server
- [ ] Test: `integrity/beacon_test.go` — a valid beacon token appends events; an
      expired one is rejected; a beacon token cannot read the attempt
- [ ] Test: `integrity/events_test.go` — the same `client_seq` from two different
      `session_id`s both persist (the D-01 regression test)
- [ ] Test: `integrity/events_test.go` — a duplicate batch inserts nothing and
      returns 200
- [ ] Test: `db/grants_test.go` — connected as `quizzivy_app`, an `UPDATE` and a
      `DELETE` against `attempt_events` and `audit_log` are both refused
- [ ] DDL reviewed against §13 and the Neon skill (§14)

---

### T-3.9 — Implement submit and auto-grading
**Depends on:** T-3.6, T-3.7
**Touches:** `server/internal/attempts/`, `server/internal/grading/`
**Size:** L
**Done when:**
- [ ] `POST /app/attempts/:id/submit` is **idempotent**; a second call on a
      closed attempt returns 409 with `ATTEMPT_CLOSED` (§15)
- [ ] Auto-grading covers `single_choice`, `multiple_choice`, `true_false` and
      `fill_blank`; `short_answer` sets `requires_manual = true` (D-19)
- [ ] `multiple_choice` scoring rule is all-or-nothing — stated explicitly here
      because §7 does not, and partial credit is a different product decision
      (recorded in `40-open-items.md`)
- [ ] `fill_blank` matching trims surrounding whitespace, collapses internal
      runs, and compares case-insensitively unless `case_sensitive`
- [ ] A deadline passing without a submit results in `timed_out`, graded the same
      way, evaluated lazily on next read rather than by a scheduler
- [ ] `attempts.score_earned` / `score_total` are written at submit;
      `score_total` comes from the frozen `test_versions.total_points`
- [ ] Test: `grading/autograde_test.go` — one case per type, including a
      `fill_blank` with a diacritic and a trailing space
- [ ] Test: `attempts/submit_test.go` — double submit returns one graded attempt
- [ ] Test: `attempts/timeout_test.go` — an attempt past its deadline reads back
      as `timed_out` without a background job

---

### T-3.10 — Build the take-test store
**Depends on:** T-3.5, T-0.12
**Touches:** `web/src/features/take-test/store.ts`
**Size:** L
**Done when:**
- [ ] Zustand store owning: question order, answers, dirty set, timer, session
      state, submit state
- [ ] **Timer math uses the server offset**, not the device clock: remaining is
      `deadlineAt - (Date.now() + offset)`, with the offset from `serverTime`
- [ ] Answers buffer locally and flush on a debounce; a failed flush retries with
      backoff and never loses the local value
- [ ] **Resume merge:** on reload, server answers are the base and any
      un-flushed local answers newer than the server's `updated_at` win. Never
      the reverse — the spec's whole point is that a refresh does not lose work
      (§1.2)
- [ ] Submit is idempotent client-side: a second tap is a no-op while one is in
      flight
- [ ] `SESSION_SUPERSEDED` puts the store into a read-only state with a plain
      explanation, rather than throwing (E2E 7)
- [ ] Test: `take-test/timer.test.ts` — remaining is correct with a device clock
      5 minutes fast
- [ ] Test: `take-test/resume.test.ts` — local-newer wins, server-newer wins,
      and no answer is dropped in either direction
- [ ] Test: `take-test/submit.test.ts` — double submit issues one request
- [ ] TypeScript strict, no `any` (§14)

---

### T-3.11 — Build the question renderers and focus layout
**Depends on:** T-3.10
**Touches:** `web/src/features/take-test/components/`
**Size:** L
**Done when:**
- [ ] A renderer per `QuestionType`, reading from the store (§7)
- [ ] `fill_blank` interleaves inputs into the Markdown at the `{{n}}`
      placeholders, preserving surrounding formatting
- [ ] `FocusLayout`: one question centered at max-width ~720px, spacious,
      `leading-relaxed` (§12)
- [ ] Safe-area padding and a 360px-wide layout that does not overflow (§9, §16)
- [ ] Prompts render with react-markdown + rehype-sanitize (§2)
- [ ] Test: a renderer test per `QuestionType` (§14's component level)
- [ ] Test: `fill-blank-renderer.test.tsx` — three blanks produce three labelled
      inputs in prompt order
- [ ] Keyboard-operable; both locales; loading / error / empty states (§14)

---

### T-3.12 — Build the AudioPlayer
**Depends on:** T-3.7, T-2.4
**Touches:** `web/src/features/media/AudioPlayer.tsx`
**Size:** L
**Done when:**
- [ ] Custom controls over a native `<audio>`; no wavesurfer, no howler (§2)
- [ ] **`.play()` is called synchronously inside the click handler** — nothing is
      awaited before it, or iOS Safari blocks playback (§11.3)
- [ ] `preload="metadata"` so duration renders without downloading (§11.3)
- [ ] `allowSeek: false`: no range input, and an `onSeeking` handler that resets
      `currentTime` to the last known position. An `audio_seek` event is recorded
      rather than pretending OS media controls cannot seek (§11.3)
- [ ] Plays-remaining rendered from the **server** value, optimistically
      decremented on play and reconciled on the next fetch (§11.4)
- [ ] A failed play-event POST **does not block playback** (§11.4)
- [ ] The signed URL is treated as expiring: a 403 triggers a refetch (§11.2)
- [ ] Autoplay is never attempted and a blocked play is not an error state
      (§11.3)
- [ ] Real `<button>` elements with `aria-label`, keyboard-operable, and an
      `aria-live` announcement of plays remaining (§11.3)
- [ ] One instance per question; navigating away pauses and releases it (§11.3)
- [ ] Monochrome per §12: filled `zinc-900` play button, thin `zinc-200` track
      with `zinc-900` fill. No waveform, no equaliser, no colour accents
- [ ] Test: `AudioPlayer.test.tsx` — states idle / playing / plays exhausted /
      seek blocked / load error (§14's component level)
- [ ] Test: `AudioPlayer.test.tsx` — `play()` is invoked in the same synchronous
      tick as the click, with no intervening microtask
- [ ] Test: `audio-plays.test.ts` — optimistic decrement reconciles to the
      server value on refetch (§14's unit level)

---

### T-3.13 — Build the integrity monitor hook
**Depends on:** T-3.10
**Touches:** `web/src/features/integrity/`
**Size:** L
**Done when:**
- [ ] **One** `useIntegrityMonitor(attemptId, policy)` hook owns every listener,
      registered and torn down in a single `useEffect`. No scattered listeners
      (§10.6)
- [ ] Captures every §10.1 signal: `tab_hidden`/`tab_visible`,
      `window_blur`/`window_focus`, `fullscreen_enter`/`fullscreen_exit`,
      `copy`/`cut`/`paste`, `context_menu`, `network_offline`/`network_online`,
      `audio_*`, `page_hide`
- [ ] **Devtools-detection heuristics are not implemented** (§10.1) — they
      false-positive on zoom and split-screen and are bypassed in seconds
- [ ] Away episodes store both endpoints so duration is visible, and a strike is
      counted only when an episode exceeds `minAwayMs` (default 3000) (§10.1)
- [ ] `clientSeq` is monotonic within a session and persisted alongside the
      `sessionId` in `sessionStorage`, so a same-tab reload continues the
      sequence and a new session starts fresh — which is safe because the server
      key includes `session_id` (D-01)
- [ ] Events buffer in memory + `sessionStorage`, flush with the autosave batch,
      and flush immediately on `pagehide` via `sendBeacon` with the beacon token
      (§10.6, `00-overview.md` §4.3)
- [ ] **Fire-and-forget: a failed flush never blocks answering or submitting**,
      and integrity failure never blocks input (§10.6)
- [ ] `copy`/`cut`/`paste` are always recorded; blocked only when
      `blockCopyPaste` (§10.1). `context_menu` is recorded, never blocked
- [ ] Test: `integrity/buffer.test.ts` — buffering and flush batching (§14)
- [ ] Test: `integrity/strikes.test.ts` — a 2-second blur is no strike, a
      90-second blur is one (§10.1)
- [ ] Test: `integrity/beacon.test.ts` — `pagehide` calls `sendBeacon` with a
      `text/plain` Blob containing the beacon token
- [ ] Test: `integrity/failure.test.ts` — a rejected flush leaves answering and
      submitting fully functional

---

### T-3.14 — Build the student-facing integrity UI
**Depends on:** T-3.13
**Touches:** `web/src/features/integrity/components/`
**Size:** M
**Done when:**
- [ ] First violation shows a non-dismissible dialog stating what happened,
      strikes remaining and what happens at zero. **The timer keeps running**
      (§10.2)
- [ ] A small persistent indicator shows remaining strikes when
      `maxFocusLoss > 0` (§10.2)
- [ ] `warn` = dialog only; `flag` = attempt marked and the student told.
      `auto_submit` is deferred to T-5.1 and its UI is not built here
- [ ] Fullscreen: entering happens on the "Bắt đầu" click, because browsers
      require a gesture (§10.2). Exit shows a "Quay lại toàn màn hình" button
- [ ] **Never trap the student**: `Esc` always works, and there is always a
      visible way to leave and submit (§10.2)
- [ ] Copy tone per §12: plain dialogs, plain text, no alarm iconography, no
      shame, no red banners
- [ ] Help text states §10.5's honest limits — browser monitoring sees this tab
      losing focus and nothing else
- [ ] Test: `integrity/dialog.test.tsx` — dialog states (§14's component level)
- [ ] Test: `integrity/dialog.test.tsx` — `Esc` is never swallowed and a submit
      path is always reachable
- [ ] Both locales, Vietnamese written first; keyboard-operable (§14)

---

### T-3.15 — Build the student home and assignment intro
**Depends on:** T-3.2, T-3.14
**Touches:** `web/src/features/assignments/`, `web/src/features/take-test/pages/`
**Size:** M
**Done when:**
- [ ] `/app` — **Đến hạn** / **Sắp tới** / **Đã hoàn thành**, each with its own
      empty state (§9)
- [ ] `/app/assignments/:id` — title, instructions, duration, attempts
      used/allowed, review policy, and the **integrity rules stated plainly in
      Vietnamese before starting** (§10.2)
- [ ] Audio rules stated when the test has listening questions, e.g. "Mỗi câu
      nghe được phát tối đa 2 lần" (§9)
- [ ] Start / Resume reflects whether a live attempt exists
- [ ] `/app/classes` lists joined classes with "Tham gia lớp mới" → `/join` (§9)
- [ ] Test: `intro.test.tsx` — the stated rules match the assignment's policy for
      each of the four policy combinations
- [ ] Loading / error / empty states; both locales; keyboard-operable (§14)

---

### T-3.16 — Close the phase with E2E 5–8
**Depends on:** T-3.15, T-3.12, T-3.9
**Touches:** `web/e2e/`
**Size:** M
**Done when:**
- [ ] **E2E 5** — timer expiry auto-submits
- [ ] **E2E 6** — tab switch fires `tab_hidden`, the warning dialog appears, and
      the event is retrievable for the admin timeline
- [ ] **E2E 7** — a second tab on the same attempt supersedes the first; the
      first goes read-only
- [ ] **E2E 8** — audio `maxPlays`: play twice → button disabled → **reload** →
      still disabled. This is the one that proves the counter is
      server-authoritative rather than client state (§11.4)
- [ ] A manual pass of E2E 2's middle: answer, reload mid-test, confirm answers
      persist. The full E2E 2 lands in Phase 4 once the result page exists
- [ ] Phase 1 and 2 E2Es still pass
- [ ] `release/phase-3` merges to `main` and back to `develop`; deployable (§16)
