# 40 — Open items

Questions only Thuong can answer. Each is tagged **BLOCKING** or
**NON-BLOCKING** with the phase it blocks.

Every item has a stated default, so work never stalls waiting for an answer. If
no answer arrives, the default is what gets built — silence is consent. The two
exceptions are marked *no default*; those genuinely cannot proceed.

Four decisions were already taken during planning and are recorded in
`00-overview.md` rather than here: deployment topology, version snapshot shape,
audio probe implementation, and build order.

---

## BLOCKING

### O-01 — What is the real domain? · Phase 0
**Default:** the plan writes `quizzivy.x` as a placeholder throughout.

The topology decision (`00-overview.md` §4.1) depends on the SPA and API being
subdomains of **one registrable domain** — that is what makes `SameSite=Lax`
work on cross-origin requests. Any two hosts under the same domain are fine;
`app.` and `api.` are only conventions. What is *not* fine is a split across two
registrable domains, which silently breaks refresh entirely (R-07).

Needed before T-0.2 registers Google's authorized origins.

---

### O-02 — Google OAuth client · Phase 1 · *no default*
**Default:** none. This is a hard blocker.

T-0.2 owns the work. GIS cannot be meaningfully tested end to end without a real
client ID — the authorization-code + PKCE exchange (§5.3) needs a real client
secret on the backend, and E2E 3 mocks the GIS *widget*, not the exchange.

Phase 1 can be built to the point of password login without it. Everything from
T-1.5 onward stops.

---

### O-03 — R2 credentials · Phase 2
**Default:** develop and test against the MinIO service in T-0.6; provision R2
before Phase 2 deploys.

T-0.3 owns the work. Because `aws-sdk-go-v2/s3` targets both, only endpoint
configuration differs, so this blocks *deployment*, not development. The one
thing worth confirming early rather than late: R2's signed-URL behaviour and
`Cache-Control` handling under §11.2's 10-minute expiry.

---

### O-04 — `fill_blank` placeholder syntax · Phase 2
**Default:** `{{1}}`, `{{2}}` … in the Markdown prompt, **1-indexed**, matched to
`question_blanks.ordinal`. A `{{n}}` with no corresponding blank, or a blank with
no placeholder, is a publish-time validation error (T-2.10).

§7 defines `blanks[].ordinal` and keys answers by blank ID, but nothing says how
a blank is *marked in the prompt*. This is the single largest surface the spec
leaves to invention: it determines the editor UX, the renderer, and the shape of
the validation.

`{{n}}` was chosen because it does not collide with Markdown syntax, survives
`rehype-sanitize`, and is typeable without a toolbar. Alternatives worth a
sentence if you disagree: `___` runs (ambiguous with Markdown horizontal rules
and unordered by nature), or `[[1]]` (collides with wiki-link plugins).

Changing this after Phase 2 means migrating stored prompts, so it is worth thirty
seconds now.

---

### O-05 — Assignment closes while an attempt is in flight · Phase 3
**Default:** `deadline_at` wins — the student finishes. `deadline_at` is computed
at attempt creation as `min(now() + duration, closes_at)`, so an attempt started
ten minutes before the window closes gets ten minutes, not the full duration.

§7 has both `window.closesAt` and `durationMinutes` and never says which
dominates. The default is the conservative reading: a student who started legally
is never cut off mid-sentence by a window boundary they cannot see, but neither
can starting one minute before close buy a full hour.

The alternative — full duration regardless of `closes_at` — is defensible and is
one line in T-3.4. Say so if you prefer it; the tests encode whichever is chosen.

---

## NON-BLOCKING

### O-06 — `max_uses` default for join codes · Phase 1
**Default:** **40**, changed from §6.1's `null = unlimited`.

§6.1 defaults `max_uses` to unlimited, which means a forwarded code works
indefinitely until it expires or is rotated. Forwarding — not brute force — is
the realistic threat (R-02), and a use cap is the mitigation that costs nothing
and needs no new state machine, which is precisely what §17.2 declines to build.

40 is chosen as roughly double a class, so a legitimate cohort never hits it.
Adjust to taste; the admin can override per code.

---

### O-07 — `minAwayMs` default · Phase 3
**Default:** 3000ms, as §10.1 states.

Flagged only because it is the number most likely to be wrong in practice, and it
is trivially tunable once you have seen a real integrity timeline. A 3-second
threshold treats a notification banner as innocent and a quick glance elsewhere
as a strike. If Phase 4's timelines are noisy, raise it.

---

### O-08 — Attempt reset semantics · Phase 4
**Default:** reset **voids** the existing attempt (`status = 'voided'` with the
reason) and permits a new one at `attempt_no + 1`. Nothing is deleted.

§8 lists "reset" alongside extend and void without defining it. The default
follows §6.4's principle that attempts are retained, and keeps the audit trail
whole. The alternative — delete and let the student start over cleanly — loses
the record of what happened, which is the opposite of what §13.4 asks for.

---

### O-09 — `multiple_choice` scoring · Phase 3
**Default:** all-or-nothing. Every correct option selected and no incorrect one,
or zero points.

§7 gives `multiple_choice` a `points` value and an option list with `isCorrect`
flags, but never states the rule. Partial credit is a real pedagogical choice —
some teachers want *n* correct out of *m* to earn a proportion — and it changes
the grading code, the result display, and what "correct" means in the review UI.

All-or-nothing is the simpler default and the more common one for language
testing. Worth a sentence if you disagree, because retrofitting partial credit
after Phase 4 touches three layers.

---

### O-10 — Diacritic-insensitive search scope · Phase 5
**Default:** the trigram index from D-11 handles *matching* — `nghe` finds
`nghé`. Ranking Vietnamese results well is deferred to P1.

Good ranking would want `unaccent` wrapped in an `IMMUTABLE` function plus a
stored `tsvector` generated column. That is real work for a question bank of a
few hundred rows where every result fits on one screen. Revisit when the bank is
large enough that ordering matters.

---

### O-11 — Gitflow overhead · any phase
**Default:** follow gitflow as instructed. `develop` + `main`, `feature/*` per
task, `release/phase-<n>` per phase.

Raised once and not again. For one developer deploying to one environment, the
`develop`/`main` split doubles merge overhead without buying isolation, and
`release/*` branches are ceremony around a decision that is already recorded in
`docs/plan/`. What it *does* buy is a clean, taggable phase boundary, which pairs
well with §16's "each phase ends deployable".

If it becomes friction, the smaller change is to keep `main` + `feature/*` and
tag phase completions, dropping `develop` and `release/*`. Say the word.

---

### O-12 — Dark mode · post-v1
**Default:** not in v1, per §12. Theming goes through CSS variables and Tailwind
tokens from T-0.9, so it can be added later without touching components.

Listed only so the token discipline in T-0.9 is understood as load-bearing rather
than stylistic. Hard-coding a zinc value anywhere is what would make this
expensive later.

---

## Answered during planning

For the record, so a later session does not reopen them:

| Question | Answer | Where |
|---|---|---|
| Deployment topology | `api.quizzivy.x` + `app.quizzivy.x` | `00-overview.md` §4.1 |
| Version snapshot shape | Fully normalized, per §13.3 | `00-overview.md` §5 |
| Audio duration probe | Pure-Go, no ffprobe | `00-overview.md` §5, T-2.2 |
| Frontend/backend order | Contract-first OpenAPI, both generated | `00-overview.md` §2 |
| §17.1 Google-only self-join | Keep. Agreed; §6.3's reasoning holds | critique |
| §17.2 No approval queue | Keep, but change the `max_uses` default | O-06, R-02 |
| §17.3 mp3/m4a only | Keep. The probe was the real decision | T-2.2 |
