# 30 — Risk register

Twelve risks. Each: what goes wrong, how likely, blast radius, the cheapest
mitigation, and the phase it first bites.

Likelihood and radius are judged for **this** project — one teacher, ~50
students, one developer — not for a product with a support rota.

| # | Risk | Likelihood | Radius | First bites |
|---|---|---|---|---|
| R-01 | Take-test engine loses student work | Medium | Severe | Phase 3 |
| R-02 | Join code leaks and strangers enrol | Medium | Moderate | Phase 1 |
| R-03 | Audio silently broken on iOS Safari | High | Severe | Phase 3 |
| R-04 | Test versioning fails to isolate in-flight attempts | Low | Severe | Phase 2 |
| R-05 | Integrity events dropped on resume | High if unfixed | Moderate | Phase 3 |
| R-06 | Refresh stampede logs everyone out on load | High if unfixed | Severe | Phase 1 |
| R-07 | Cross-origin cookie/CORS misconfiguration | Medium | Severe | Phase 1 |
| R-08 | Pure-Go duration probe wrong on VBR MP3 | Medium | Minor | Phase 2 |
| R-09 | Credentials missing at a phase boundary | High | Moderate | Phase 0 |
| R-10 | Single-developer scope creep | High | Moderate | all |
| R-11 | Client clock skew corrupts the timer | Medium | Moderate | Phase 3 |
| R-12 | Beacon flush unauthenticated or dropped | Medium | Minor | Phase 3 |

---

## R-01 — The take-test engine loses student work

**What goes wrong.** A student answers ten questions, the tab reloads or the
network blips, and the resume merge picks the server's older copy. The work is
gone and the student has no recourse mid-exam. §1.2 makes not losing work goal
number one, so this is the failure that matters most.

**Why it is plausible.** Three writers touch the same state: the local store, the
debounced flush, and the server's copy on resume. Merge order is easy to get
backwards, and the bug only appears under a timing window that manual testing
rarely hits.

**Blast radius.** Severe and unrecoverable. There is no undo for an exam.

**Cheapest mitigation.** T-3.10's resume rule stated as an invariant rather than
an implementation detail — server answers are the base, un-flushed local answers
newer than the server's `updated_at` win, never the reverse — plus
`take-test/resume.test.ts` covering both directions and the drop case. Answers
buffer locally and a failed flush retries with backoff rather than discarding.
E2E 2 exercises the reload path end to end.

**Residual.** A device dying between the last flush and the next loses at most
one debounce window. Accepted; the alternative is a synchronous write per
keystroke.

---

## R-02 — A join code leaks and strangers enrol

**What goes wrong.** A student forwards the code, or a screenshot reaches a group
chat, and people who are not in the class enrol themselves.

**Why it is plausible.** §6.1 makes the code a bearer secret handed to teenagers
over messaging apps. This is the normal case, not the adversarial one.

Worth naming clearly: **brute force is not the threat.** 32⁸ ≈ 1.1 × 10¹² with a
handful of live codes makes random probing hopeless even unthrottled, so §6.5's
"still worth probing at scale" overstates the guessing risk. The rate limit is
still correct — it bounds a cheap nuisance and protects the endpoint — but it is
not what defends against the realistic failure.

**Blast radius.** Moderate. A stranger sees class name, teacher name, and any
assignment targeted at the class. No other student's data (§6.5 keeps the preview
to two fields, and there is no student directory).

**Cheapest mitigation.** The things that actually address forwarding: a
`max_uses` default equal to the expected class size rather than §6.1's
`null = unlimited` (see `40-open-items.md`), the 30-day expiry, and §6.4's member
list showing `joined_via` and `joined_at` so an unexpected enrolment is visible.
D-10 adds `join_code_id` so that after a rotation the teacher can tell which code
someone came through. Rotate-and-remove, per §6.5's deliberate no-approval-queue
decision.

**Residual.** A stranger can enrol before the teacher notices. Accepted — §17.2
judges an approval queue not worth the state machine, and I agree.

---

## R-03 — Audio silently broken on iOS Safari

**What goes wrong.** The play button does nothing on iPhone. Listening questions
are unanswerable, and the student is mid-exam with no recourse.

**Why it is plausible.** iOS requires `.play()` to originate from a gesture
handler, not from an async continuation. Any `await` before the call — a signed
URL fetch, a play-event POST, a store update that yields — breaks it. The
refactor that introduces the `await` will look harmless and will pass every test
run in a headless Chromium.

**Blast radius.** Severe. §1.1 says students are often on a phone, and iOS
Safari is the majority of those.

**Cheapest mitigation.** T-3.12 calls `.play()` synchronously in the click
handler with nothing awaited before it, and the signed URL is fetched **before**
the button becomes enabled, not on click. `AudioPlayer.test.tsx` asserts the call
happens in the same synchronous tick as the click, so the regression fails in
CI rather than on a student's phone. T-5.6 verifies on real iOS Safari, not a
simulator. AGENTS.md lists `features/media/` as high-risk: run its tests before
and after every change.

**Residual.** iOS also allows only one audio element to play at a time; §11.3's
one-player-per-question rule handles it, and navigating away releases the player.

---

## R-04 — Versioning fails to isolate in-flight attempts

**What goes wrong.** The teacher fixes a typo in a published test while a student
is taking it, and the student's paper changes underneath them — or worse, the
grading key changes and previously-correct answers become wrong.

**Why it is plausible.** The snapshot is only correct if *every* read path in the
student and grading flows goes through `test_version_*` and never through the
bank. One query joining back to `questions` for convenience reintroduces the
coupling, and it will look like a harmless optimization.

**Blast radius.** Severe and silent. Scores are wrong and nothing errors.

**Cheapest mitigation.** Structural: `attempt_answers.question_id` references
`test_version_questions` with `RESTRICT` (`20-data-model.md` §10), so an answer
cannot point at a bank row even by accident. T-2.10's
`publish/snapshot_test.go` — "editing the bank question's prompt after publish
leaves the version's prompt unchanged" — is the canary; AGENTS.md marks it as
never-delete. §7's rule that student test content is fetched **only** via
`GET /app/attempts/:id` keeps the surface to one endpoint.

**Residual.** `media_asset_id` intentionally points at the same immutable asset
rather than a copy (§11.1), so deleting the underlying R2 object would break an
old version. `ON DELETE RESTRICT` plus the 409 in T-2.4 prevents the row deletion
that would precede it.

---

## R-05 — Integrity events dropped on resume

**What goes wrong.** A student's tab crashes, they reopen the attempt, and every
integrity event from that point on fails to insert. The timeline shows the
teacher a clean record for exactly the period that warranted scrutiny.

**Why it is plausible.** §13.3's `UNIQUE (attempt_id, client_seq)` combined with
§10.6's `sessionStorage`-buffered counter guarantees it: `sessionStorage` does not
survive a tab close or a device change, the counter restarts at 1, and every
event collides. §10.6 also mandates fire-and-forget, so nothing surfaces the
failure.

**Blast radius.** Moderate — no student is harmed, but the feature §10 calls
"first-class" quietly stops working in its most important case.

**Cheapest mitigation.** D-01: add `session_id` to the table and make the key
`(attempt_id, session_id, client_seq)`, plus `ON CONFLICT DO NOTHING` so a
retried `sendBeacon` batch cannot fail on one duplicate row. Regression test in
T-3.8: the same `client_seq` from two sessions both persist. Cost is one column.

**Residual.** None material once fixed. This is on the register because it would
have shipped.

---

## R-06 — Refresh stampede logs everyone out on load

**What goes wrong.** Every page refresh logs the user out. The app looks broken
in the most visible possible way.

**Why it is plausible.** §5.2 holds the access token in memory only, so a cold
load has none. TanStack Query mounts several queries at once, all 401, all call
`/auth/refresh`. Rotation means the second call presents an already-rotated
token, and §5.2's reuse detection then revokes the entire family — correctly, by
its own rules. Neither §5.2 nor §5.4 mentions serializing the refresh.

**Blast radius.** Severe in perception, trivial in data. Every user, every load.

**Cheapest mitigation.** T-0.12's single-flight refresh: one shared in-flight
promise in `client.ts`, with `client.refresh.test.ts` asserting five concurrent
401s issue exactly one `POST /auth/refresh`. Roughly fifteen lines.

**Residual.** Two browser tabs racing on separate JS contexts can still collide.
Rare, self-corrects on the next login, and the alternative (a cross-tab lock via
`BroadcastChannel`) is not worth it at this scale.

---

## R-07 — Cross-origin cookie or CORS misconfiguration

**What goes wrong.** The refresh cookie is never sent, so sessions die after 15
minutes with no error anyone can see. Or CORS is loosened to `*` during
debugging and credentials stop working — or worse, keep working.

**Why it is plausible.** `SameSite=Lax` is subtle: it permits same-*site* cross-
*origin* requests, which is exactly the `app.quizzivy.com` → `api.quizzivy.com`
case, but a reader who conflates site with origin will "fix" it to
`SameSite=None`. A `127.0.0.1` vs `localhost` split in local dev makes requests
genuinely cross-site and produces a bug that vanishes in staging.

**Blast radius.** Severe — nobody can stay logged in.

**Cheapest mitigation.** `00-overview.md` §4.1 states the topology and why it
works, so the reasoning is not rediscovered. T-1.3's `auth/cookie_test.go`
asserts the exact `Set-Cookie` attributes including the **absence** of `Domain`.
T-0.14's CORS middleware rejects `*` with credentials by construction, and
`httpx/cors_test.go` asserts an unlisted origin gets no allow header. T-0.6 pins
local dev to `localhost` on both ports.

**Residual.** A future move to a genuinely cross-site host would break this.
That is why the topology is recorded as an architecture decision rather than a
deployment note.

---

## R-08 — The pure-Go duration probe is wrong on VBR MP3

**What goes wrong.** A VBR MP3 without a Xing header reports the wrong duration.
A 4-minute file is rejected as over 5 minutes, or the "3:20" shown to the student
does not match what plays.

**Why it is plausible.** VBR without a Xing/Info header requires counting every
frame; a naive bitrate-times-size estimate is wrong by a wide margin. M4A adds
32-bit versus 64-bit `mvhd` and inconsistent atom ordering between encoders.
This is the known cost of choosing pure-Go over `ffprobe` (`00-overview.md` §5).

**Blast radius.** Minor and visible. The teacher is the only uploader, sees the
result immediately, and can re-encode. Nothing silently corrupts.

**Cheapest mitigation.** T-2.2's fixture corpus, which exists precisely to cover
the failure modes: VBR with a Xing header, VBR **without** one, a large ID3v2
tag, iTunes M4A, ffmpeg M4A, plus a renamed WAV and a truncated file as rejects.
Duration asserted within 1% of known values. A file that sniffs correctly but
cannot be probed is **rejected** rather than stored with a null duration, so the
failure is a clear message instead of a bad record.

**Residual.** An encoder nobody tested. Escalation path is one commit: shell out
to `ffprobe` and add the binary to the image.

---

## R-09 — Credentials missing at a phase boundary

**What goes wrong.** Phase 1 finishes and cannot be verified because the Google
OAuth client does not exist. Phase 2 cannot deploy because R2 has no bucket.

**Why it is plausible.** Neither exists today, both need Thuong, and both are the
kind of task that slips because no code depends on them until the moment
everything does.

**Blast radius.** Moderate — schedule, not correctness.

**Cheapest mitigation.** T-0.2 and T-0.3 are the second and third tasks in the
plan, owned by name, with their blocked phases stated. R2 is de-risked further by
T-0.6's MinIO service: local development and every test run against the same S3
API, so only deployment waits on the real bucket. Google has **no** workaround —
GIS cannot be meaningfully end-to-end tested without a real client ID — which is
why it is flagged BLOCKING with no default in `40-open-items.md`.

---

## R-10 — Single-developer scope creep

**What goes wrong.** v1 grows a vocabulary module, an analytics dashboard, or a
notification system, and test-taking never ships end to end.

**Why it is plausible.** §1.3's non-goals are long precisely because the
temptations are obvious, and there is no second person to push back.

**Blast radius.** Moderate — the one capability §1 says v1 must deliver slips.

**Cheapest mitigation.** Phase exit criteria are E2E tests, not judgement calls,
so "done" is a green suite. This plan already cuts two things toward that end:
`auto_submit` moves to Phase 5, and §16's unachievable seq-scan criterion is
replaced with a measurable budget. `40-open-items.md` holds a P1 list so a good
idea can be recorded and dropped in the same minute.

---

## R-11 — Client clock skew corrupts the timer

**What goes wrong.** A student's device clock is fifteen minutes fast. The
countdown shows zero while the server still considers the attempt live, or the
reverse — the student believes they have time they do not.

**Why it is plausible.** Phone clocks drift, and a manually-set clock is not
rare. §7 marks `durationMinutes` server-enforced and §15 returns `serverTime`,
but nothing says the client must use it as an offset rather than as a display
value.

**Blast radius.** Moderate — confusing and unfair, but the server's
`deadline_at` remains authoritative so no grade is actually wrong.

**Cheapest mitigation.** T-3.10 computes remaining as
`deadlineAt - (Date.now() + offset)` where the offset comes from `serverTime`,
with `take-test/timer.test.ts` running the case of a device five minutes fast.
The offset is refreshed on every autosave response, so drift during a long test
self-corrects.

**Residual.** A clock that changes mid-attempt (a timezone move, an NTP
correction) causes one visible jump. Acceptable, and the next autosave fixes it.

---

## R-12 — The beacon flush is unauthenticated or dropped

**What goes wrong.** The `pagehide` flush never lands, so the final events before
a student closes the tab — the most interesting ones — are missing.

**Why it is plausible.** §10.6 specifies `sendBeacon`, which cannot set an
`Authorization` header, and §5.2's access token is ~15 minutes while a test runs
45–90. At `pagehide` the token has almost always expired. A CORS preflight on
unload is also not reliably delivered.

**Blast radius.** Minor. Integrity data is evidence for a conversation (§10.5),
never proof, and never blocks a student.

**Cheapest mitigation.** The attempt-scoped beacon token from
`00-overview.md` §4.3: minted at attempt creation, valid until `deadline_at`,
stored hashed as `attempts.beacon_token_hash` (D-03), sent in a `text/plain` Blob
body so the request is CORS-safelisted and skips preflight. The endpoint grants
append-only event access, so the weaker credential buys no extra authority.
`integrity/beacon_test.go` covers valid, expired, and the attempted-read case.

**Residual.** `sendBeacon` is best-effort by design and a killed process may
still lose the last batch. §10.1 calls it best-effort; not worth more.
