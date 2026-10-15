# 40 — Open items

Questions only Thuong can answer. Each is tagged **BLOCKING** or
**NON-BLOCKING** with the phase it blocks.

Every item has a stated default, so work never stalls waiting for an answer. If
no answer arrives, the default is what gets built — silence is consent. The two
exceptions are marked *no default*; those genuinely cannot proceed.

Four decisions were already taken during planning and are recorded in
`00-overview.md` rather than here: deployment topology, version snapshot shape,
audio probe implementation, and build order.

Phase R (the redesign) records its eighteen decisions in
`70-redesign-overview.md` §1 and its design questions in `docs/design/gaps.md`;
questions that come up during a release go into that release's plan file under
"Open items" and move here at the release's docs task (R4's are O-R4.1 to O-R4.23, below).

---

## BLOCKING


### O-14 — DNS records for quizzivy.com · before first deploy — **RESOLVED**
**Resolved:** production serves the SPA on `app.quizzivy.com` and the API on
`api.quizzivy.com` (`docs/setup/dns.md`, `docs/setup/deploy.md`). The apex is kept for R11's
Landing (D10). The text below is the question as it was asked.

**Default:** `app.quizzivy.com` for the SPA, `api.quizzivy.com` for the API, as
already written throughout the plan. See `docs/setup/dns.md`.

The domain is bought but no records exist yet. **This does not block Phase 0 or
Phase 1** — local development runs entirely on `localhost`, and the Google client
already has the localhost redirect registered and verified. It blocks the first
real deploy.

Two things to get right when you do set it up, both of which break quietly
rather than loudly:

- Both hosts must stay under `quizzivy.com`. That is what makes them same-*site*
  and therefore what makes §5.2's `SameSite=Lax` refresh cookie work at all
  (R-07). A split across two registrable domains kills sessions with no error.
- ~~The Google OAuth client needs the production origin.~~ **Done and verified
  2026-08-28**: `https://app.quizzivy.com/auth/google/callback` is registered,
  confirmed by probing Google's token endpoint (returns `invalid_grant`, not
  `redirect_uri_mismatch`).
- The domain is **not on Cloudflare yet** — nameservers are still
  `ns1–ns4.zonedns.vn` and no records exist. Moving them is step 0 in
  `docs/setup/dns.md` and takes up to 24h, so it is worth starting before the
  hosting question is settled.
- The records themselves are blocked on O-16.

Using the apex `quizzivy.com` for the SPA instead of `app.` is equally valid and
equally same-site. The plan says `app.` so the apex stays free for a landing
page later; say so if you would rather have it the other way, since changing it
after students have bookmarks is unpleasant.

---

### O-13 — GIS `initCodeClient` or our own authorization request · Phase 1 — **RESOLVED**
**Resolved: B, own the authorization request** (see "Answered during planning" below,
`00-overview.md` §5, and AGENTS.md "Verified platform facts"). The options as they were put:

| Option | What | Trade-off |
|---|---|---|
| **A** | GIS `initCodeClient`, no PKCE | Keeps §2. Drops §5.3's PKCE. The code passes through browser JS, so an XSS could steal it — though redeeming it still needs the client secret, which never leaves the backend. |
| **B** | Own the authorization request | Keeps §5.3. Deviates from §2. ~40 lines: verifier, S256 challenge, `state`, popup, callback. |

**Recommended: B.** PKCE is cheap to do properly, it is what §5.3 asked for, and
it drops a dependency on undocumented GIS behaviour. We render our own button
either way — §12 dictates charcoal, not Google's default styling.

Not urgent: `docs/setup/google-oauth.md` registers both the JavaScript origins
(A needs them) and the redirect URIs (B needs them), so T-0.2 is not blocked by
this. **Decide before T-1.5.**

If you pick A, `api/openapi.yaml` needs `codeVerifier` made optional on
`POST /auth/google`, and §5.3 should be corrected to stop claiming PKCE.

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

### O-17 — `fill_blank` scoring with more than one blank · Phase 3 — **RESOLVED**
**Resolved 2026-09-01: per blank.** Each blank earns its share of the question's
points. Thuong's call, following the design deck.

I shipped all-or-nothing first, reasoning from O-09's rule for the other
multi-part type and recording the disagreement here. That was the wrong place to
look: S-05 already answers it, on the question itself, in the line the student
reads before answering —

> 2 điểm · mỗi chỗ trống 1 điểm

An all-or-nothing rule would have made that sentence a lie, and the sentence is
the part they see. Worth remembering as a research failure rather than a
judgement one: the answer existed in the deck and I searched the spec and this
file for it.

The share is computed off the total (`points × matched ÷ blanks`) rather than
accumulated per blank, so a question that divides unevenly still adds up. Three
blanks worth two points would otherwise round to 0.67 each and pay 2.01 for a
perfect answer — more than the question is worth, on the commonest answer there
is.

Diacritics are still not folded, and that remains closed. "ha noi" is not
"Hà Nội" the way "hanoi " is "Hanoi".

---

### O-20 — Offset pagination for admin lists · Phase 3 — **RESOLVED**

**Decided 2026-09-03 by Thuong.** §13.8 says keyset everywhere, and every admin
list shipped that way with a "Xem thêm" button. The teacher wants numbered
pages -- shadcn's Pagination, "trang 3 / 26", a URL that can be shared -- and
numbered pages need `total` and a jump to page N, which keyset cannot give.

**Resolution:** every admin list (`tests`, `questions`, `media`, `assignments`,
`attempts`, `students`, `classes`, class members) takes `page` + `limit` and
answers `{ items, page, pageSize, total }`. Server-side that is `OFFSET` plus
a `count(*)` with the same WHERE. §13.8's concern -- an insert landing
mid-pagination shifting a page by one row -- is real and accepted at this
scale (§1.3: one teacher, ~50 students); a duplicated row on page turn costs
less than a grid that cannot jump. `20-data-model.md` §12 keeps the keyset
indexes; they still serve the `ORDER BY id DESC`.

**Consequence:** `docs/quizzivy-spec-v0.3.md` §13.8 is overridden for lists.
The student's own lists (`/app/*`) are unpaged and unaffected.

### O-10 — Diacritic-insensitive search scope · Phase 5
**Default:** accent-insensitive **matching** ships in Phase 2. Accent-aware
**ranking** is deferred to P1.

This item originally said `pg_trgm` handled matching on its own. It does not —
verified on 18.6, `'nghé' ILIKE '%nghe%'` is false. D-11 now folds accents
explicitly via `app.immutable_unaccent`, which costs one extension, one wrapper
function and one index, so matching is in v1 rather than deferred.

What stays deferred is ranking: ordering results by relevance would want a
stored `tsvector` generated column over the unaccented text. That is real work
for a bank of a few hundred questions where every result fits on one screen.
Revisit when the bank is large enough that ordering matters.

One operational note that comes with the wrapper: it is marked `IMMUTABLE` as an
assertion, not a fact. If the `unaccent` dictionary is ever changed, every index
built on it must be `REINDEX`ed.

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

### O-15 — React Router v8 is out; §2 says v7 · any phase
**Default:** pinned to **v7.18.2**, as §2 names. Not upgraded silently.

`pnpm add react-router` now resolves to 8.3.0. §2 fixes the stack "unless Thuong
approves a change", so the install was pinned back to v7. Nothing in the plan
needs v8, and the router is small enough that upgrading later is cheap — the
sooner it happens the cheaper, though, since Phase 3's take-test engine is the
part that would make a migration awkward.

Worth deciding before Phase 3 rather than after.

---

### O-24 — Word import in production · before its pilot (W-22)
**Decided 2026-09-25: on,** for `.docx` and text-layer PDF. `fly.toml` runs the
worker as its own process group (`17-word-import.md` §1.40). Until then the
default was off: `fly.toml` set no `IMPORT_*`, and the web showed no way in
(#138).

What turning it on needed, and where each stands:

- **Storage.** A private R2 bucket for imports, and an R2 token that reaches it.
  This is an account action.
- **Cost.** A second Fly Machine for the worker. Since #143 the worker sleeps
  until woken, so Neon's compute can still suspend while it runs.
- **W-21 gates.** W-21 is in `17-word-import.md` §6, and decisions D-07 and
  D-08 are in §10:
  - D-07, capacity: still open. The quotas and concurrency ship at their
    development defaults until real use is measured.
  - D-08, retention: decided and built (30/7/60 days, `17-word-import.md`
    §1.38).
- **Storage support.** R2 rejects full-object SHA-256 checksums, so the import
  store checks the digest itself and sends it as metadata with Content-MD5.
  `make verify-r2-imports` proves the bucket accepts that. It passed on
  2026-09-26, once the token listed `quizzivy-imports`.

PDF needs nothing more: the worker reads it in its own WebAssembly sandbox
(D-10, `17-word-import.md` §1.39), sized into the worker's 1 GB Machine.

`.doc` stays off either way. Its converter needs a Docker daemon, the production
image has none, and hosting one means a separate privileged Machine. The steps
are in `docs/setup/word-import-worker.md` § Production. `deployment_test.go` keeps
the processing switch and the worker process together, and the deploy preflight
refuses the switch as a Fly secret.

---

### O-25 — A malformed option id in an autosave · any phase
**Default:** keep it. A student mid-test never loses a batch because one entry
is malformed, and the web app cannot send one.

What happens: `saveAnswers` accepts a choice answer whose option id is not a
uuid, answers 200, saves the rest of the batch and drops that answer. The server
logs a warning with the dropped question ids
(`server/internal/modules/attempts/http/attempts.go`, "autosave dropped answers
not on the paper"); the client is not told.
`TestTheValidatorRefusesNoIdTheDecoderAccepts`
(`server/internal/core/router/tests/validate_test.go`) pins it.

Accepted with it: a malformed uuid inside an answer names no field in a refusal,
because it is not refused (the rest of issue 280's gap).

The alternatives, for when he answers:

- Refuse the request, with a `uuid` format validator in
  `platform/httpx/validate.go`. One bad id then refuses the whole batch.
- Keep 200 and report the dropped ids, which is a contract change to the
  response.

Issue #307.

---

### O-26 — A tab another device took over, after a reload · round 5
**Default:** stay read-only until the student continues there. This default is
built by the merged session-claim fixes; it is not an owner decision.

What happens: each tab names its held session on every attempt read. A tab
naming an old session gets 200 with `superseded: true`, its own session id and
an empty beacon token, without changing the attempt. Reload or refetch keeps
it read-only; it ignores local drafts and removes one only if its session
wrote it. A tab holding no session gets the current one.

Accepted with that last default: two tabs of one browser can share a session.
A read naming no session still replaces the attempt's beacon token, so the
holding tab's closing events can be refused. The superseded read fixes this
only for a tab that names an old session.

The alternatives, for when the owner answers:

- Return 409 to a superseded reader, or always resume on a read. The latter
  makes GET write, rotates sessions on reload, and needs draft handling first.
- Require a tab holding no session to obtain one by a recorded resume before
  writing. This ends both remaining costs but changes engine load, its mocked
  tests, and records a resume whenever an attempt is reopened by its address.

Issue #337; round 5 questions 1 and 1b.

---

### O-27 — When a session counts as still open · round 5
**Default:** nothing is built until the owner answers. The candidate is that a
session which said it was leaving, and has not returned, is no longer live.

What happens today: any ordinary event received within two minutes makes the
session live, including `page_hide`. Continuing elsewhere can therefore report
"resumed from another device" after an intentional Leave.

The candidate reads the latest event by received time, then sequence, and
recognises `page_hide` or a new Leave event as farewell. Its cost: farewell
alone misreads reload or back-forward restoration as a tab that left. The
engine must also record an arrival on mount and restored `pageshow`, deliver
it immediately with the bearer token, and add event labels in both languages.
There remains a short farewell-to-arrival interval in which another Continue
is recorded as resume.

Three integration cases must prove the change: farewell then Continue within
two minutes is resume without takeover; ordinary events without farewell still
produce takeover; farewell then reload arrival then Continue elsewhere still
produces takeover. The lead also observes one live case.

The sub-question is whether Leave needs a new kind. Default: `engine_leave`,
"Rời khỏi bài làm" / "Left the test", because the page was not closed. This
is a candidate, not a merged signal or an approved owner decision.

The alternatives are a shorter live window, a heartbeat, or treating a
session-naming read as liveness. The last requires GET to stamp a new column
and a migration; none is built here.

Issue #341; round 5 question 2.

---

### O-28 — A play that never started is counted · round 5
**Default:** keep counting; no compensation is built. This awaits the owner.

What happens: a play refused by the browser or paused before it starts can
already have spent a server play. The player stays enabled at the limit; the
teacher can see an extra count beside the reported block.

The alternative is an idempotent compensation for a refused start: a play id,
a receipts table and migration, a server call, and a client able to claim a
block it did not have. D4's separate T-D4.8 proposal stops a resume spending a
play; it runs after v0.10.0 and does not resolve this default now.

Issue #339, count only; round 5 question 3.

---

### O-29 — A paper the server has called out of time · round 5
**Default:** submit at the device's recorded deadline, not at once. This
default is built; it remains open to an owner decision.

What happens: a deadline refusal from a save or play rearms the deadline
submission even if another refusal follows. It does not replace a closed or
superseded lock. Submission failures can still retry.

The alternative is immediate submission after the server refusal. It covers a
device clock set back, but rewrites the six existing time-up cases and makes
the time-up bar transient online.

Issue #353; round 5 question 5.

---

### O-30 — May a superseded tab or an ended attempt play question audio? · round 5 — **RESOLVED**
**Resolved (D-005, 2026-10-14):** no new grant. A question's play goes through the
same write lock as an answer and a shared recording's play: a session that is not
current answers `409 SESSION_SUPERSEDED`, an attempt not in progress
`409 ATTEMPT_CLOSED`, and a play past the deadline `409 DEADLINE_PASSED`. None is
counted and the client does not retry. Review playback after submit stays
unlimited and uncounted. Issue #370, step 2.

The request's `sessionId` is optional for one release, so an older client keeps
counting; a play without it is checked for status and deadline only. A later
release can make it required.

---

### O-12 — Dark mode · post-v1 — **RESOLVED**
**Resolved:** dark mode shipped in R1 (v0.7.0): the deck's tokens in both themes, the
`quizzivy.theme` preference, and `web/public/boot.js` applying the theme before paint. The
token discipline below is what made it cheap.

**Default:** not in v1, per §12. Theming goes through CSS variables and Tailwind
tokens from T-0.9, so it can be added later without touching components.

Listed only so the token discipline in T-0.9 is understood as load-bearing rather
than stylistic. Hard-coding a zinc value anywhere is what would make this
expensive later.

---

## Phase R — R4's open items (v0.10.0)

R4's questions, moved here from `74-r4.md` "Open items" at the release
(T-R4.50). Each default is what v0.10.0 ships; an answer that differs becomes a
task in the release it names. The design departures behind them are in
`docs/design/gaps.md`.

### O-R4.1 — Audio length and WAV · R6
**Default:** every screen states DG-63's limits: MP3 or M4A, 50 MiB, five
minutes; GIF is not taken for an image. The deck's "MP3 or WAV" is written
"MP3 or M4A". A listening section longer than five minutes, and WAV, wait for
this answer before the duration cap moves (spec §11.1).

---

### O-R4.2 — Images and audio inside a prompt or an explanation · R6
**Default:** not built in R4 (DG-109). The editor has no Image or Audio button
and refuses a pasted or dropped file. Building it needs a contract change,
asset references in the publish snapshot and in media replace and delete,
signed URLs in the student payload, and a rule for limiting inline audio. It is
built in R6, whose track runs through those areas, once Thuong confirms.

---

### O-R4.3 — A pasted image is left out, not refused · R4 (built)
**Default:** built (DG-115, T-R4.63). An image inside copied content is left
out and counted in the paste preview; every other refusal of spec §7.1 stays.
The alternative keeps the refusal and drops the "left out" line.

---

### O-R4.4 — "Choose from Media" lists the caller's own files · R7
**Default:** the caller's own files, under "Files you uploaded." (DG-112, DG-90).
Files other teachers share wait for R7's sharing to cover media, and for a
drawing.

---

### O-R4.5 — Rich text for options · any release
**Default:** prompts and explanations are rich for every teacher since R4, and
`VITE_RICH_QUESTION_EDITOR` is gone (T-R4.64d). Options stay plain inputs, as
the deck draws them, and `VITE_RICH_OPTION_EDITOR` stays off. Thuong decides
whether rich options are ever turned on.

---

### O-R4.6 — What the Markdown mode is · R4 (built)
**Default:** built (DG-110). The mode is the stored form, nothing converts
without the teacher's confirmation, and Markdown reads tables and
strikethrough and no other GFM syntax, for the teacher and the student.

---

### O-R4.7 — The bank's question editor was not redrawn · D4 or later
**Default:** the bank's editor page uses the builder's 2026-10-04 drawing of
the form inside the page's own frame (DG-108, T-R4.33). A later export that
redraws the screen is followed.

---

### O-R4.8 — "Students can pause", "Matching" and "Not given" · R6
**Default:** none has a model (DG-111, DG-113). "Students can pause" is not
built in R4 (D4 builds it as `allowPause`: T-D4.13, DG-183) and the two
existing playback switches stay; "Matching" and the "Include “Not given”"
switch arrive in R6 with their question types.

---

### O-R4.9 — Alt text belongs to the question · R4 (built)
**Default:** built (DG-111, T-R4.62): one nullable column on the question and its
snapshot, so two questions that share an image describe it in their own words.
The alternative is alt text on the media file, set once in the library.

---

### O-R4.10 — The import review's editor · R4 (built)
**Default:** built (DG-114, T-R4.38): the review keeps gaps, the stored content
and the read-only view, and has no Markdown mode. The deck draws the opposite
on each point, in a prototype that saves plain text.

---

### O-R4.11 — Join codes without a use limit · R4 (built)
**Default:** O-06 holds: Maximum uses is 1 to 1,000, 40 by default, and the
request field is not nullable. The deck's "Leave empty for no limit" is not
built.

---

### O-R4.12 — Username sign-in · R5 or later
**Default:** sign-in is by email (spec §5), so Add student asks for Email where
the deck asks for "Username or email".

---

### O-R4.13 — Question import in the bank · R5 or R6
**Default:** the bank's Import button stays absent. The deck's dialog (CSV,
XLSX, `.quizzivy`, a tag for every question, "Skip questions that already
exist") and `POST /teacher/questions/import` go to R5 after T-R5.13 brings
XLSX, or R6 after T-R6.3 so the importer knows every type. Otherwise Thuong
records a D1 exception in 70 and a DG entry.

---

### O-R4.14 — Import controls without a backend · R6 or R11
**Default:** absent in R4 (DG-67): saved profiles, source conventions, the Word
template and the audio slot, "Create another test from this file", "Delete
import", "Process again" after a cancel, Move / Split / Merge and the
processing history. Their backend and UI go to R6 beside T-R6.13 or to R11's
reconciliation, or Thuong lists them as an approved post-1.0 exception in 70 §2.
`18-word-import-ux.md` §12 has the kept and gated table.

---

### O-R4.15 — The note to students · R4 (built)
**Default:** built (T-R4.11, T-R4.26, T-R4.27b): `assignments.student_note` and a
plain "Note to students (optional)" field, which the Teacher deck does not
draw and the Student deck's Test intro shows.

---

### O-R4.16 — Wave 2's defaults (2026-10-04) · R4 (built)
**Default:** built, each where the plan and the deck left a choice.
- A paste that carries a file beside HTML text (an Excel or Word selection) is
  previewed, and Ctrl+Shift+V inserts its plain text (T-R4.63).
- A rotated legacy code answers through today's paths: "revoked" where self-join
  is open, an unknown code where it is closed or the class archived (T-R4.22).
- The stepper's value can be typed, so a 47-minute limit stays possible
  (T-R4.2c).
- "nghe" and "nghé", and "London" and "london" under Match case, can both be
  accepted answers, because the server grades them as different (T-R4.2c,
  T-R4.66).
- An import in review that names a replaced file keeps the old file and binds
  it at commit (T-R4.17b).
- A class still unrotated after its one retry waits for the next API start-up,
  so the release check re-runs the count query after the deploy (T-R4.22).
- A draft whose group graph cannot be read answers `unpublishedChanges` null,
  and the builder shows no banner for it (T-R4.16).

---

### O-R4.17 — The Admin's own-row lists · R5
**Default:** in the teacher workspace an Admin's six content lists hold only
their own rows (DG-53, T-R4.54); everything stays reachable by id and, from R5,
in the Admin console. An Admin who opens another teacher's draft by id cannot
add a question to it, because a draft accepts only its owner's questions.
Thuong may prefer an "All teachers" filter, which is a deck change.

---

### O-R4.18 — Student activity · R5
**Default:** "Not signed in", a true "Last active" and the password's "Last
changed" line wait for R5's activity tracking (DG-54).

---

### O-R4.19 — What a paste keeps of automatic numbering · R4 (T-R4.51)
**Default:** nothing changes: the paste page's tips ask for typed numbers and
letters. Whether the plain-text clipboard of Word and Google Docs keeps
auto-numbered questions and option letters decides how well a pasted test is
recognised; T-R4.51 records it with real documents.

---

### O-R4.20 — Remove and Move on Students, with one class filtered · any release
**Default:** built (T-R4.40): Remove from class and Move act on every class the
selected students are in, and the confirmation names those classes. The
alternative acts only on the class the Class facet filters to when exactly one
is chosen.

---

### O-R4.21 — A refresh whose answer never arrives · any release
**Default:** no change to §5.2 (F-66, 2026-10-12). A token rotation that commits while the browser
is closing or the network drops leaves the browser with the old token; its next refresh is reuse,
and the session ends: a forced sign-in, rare and self-healing. Options, in order of cost:
- `keepalive: true` on the refresh request, once a check in Chrome, Safari and Firefox shows the new
  cookie is stored after the page has gone;
- count the families revoked for reuse whose successor was never presented, to see how often it
  happens;
- a reuse interval of a few seconds, which changes §5.2 and would also close F-49's cross-tab race.

`docs/plan/30-risks.md` R-06 records the residual.

---

### O-R4.22 — The `/admin` alias · R5
**Default:** kept in v0.10.0. Removing it (T-R3.3) needs seven days of production
`legacy_admin_path` logs read first. R5 removes it when it gives `/admin` to the Admin console,
as T-R5.36 (`75-r5.md`), before T-R5.11's `/admin/classes`.

---

### O-R4.23 — A student can read their attempt's flag through the API · R5
**Default:** unchanged. `attempt.integrity.flagged` is in the student's attempt and result payloads
since v0.9.0 and carries a teacher's manual flag too. The student screens never show it. R5
decides whether the student payload drops it.

---

## Answered during planning

For the record, so a later session does not reopen them:

| Question | Answer | Where |
|---|---|---|
| Deployment topology | `api.quizzivy.com` + `app.quizzivy.com` | `00-overview.md` §4.1 |
| Version snapshot shape | Fully normalized, per §13.3 | `00-overview.md` §5 |
| Audio duration probe | Pure-Go, no ffprobe | `00-overview.md` §5, T-2.2 |
| Frontend/backend order | Contract-first OpenAPI, both generated | `00-overview.md` §2 |
| §17.1 Google-only self-join | Keep. Agreed; §6.3's reasoning holds | critique |
| §17.2 No approval queue | Keep, but change the `max_uses` default | O-06, R-02 |
| §17.3 mp3/m4a only | Keep. The probe was the real decision | T-2.2 |
| Google supports PKCE? | Yes — `S256`, verified from discovery. The gap is the GIS wrapper | O-13 |
| **O-13** GIS vs PKCE | **Own the authorization request.** Keeps §5.3's PKCE; approved deviation from §2 | `00-overview.md` §5 |
| **O-01** real domain | **`quizzivy.com`**, bought. `app.` + `api.` subdomains | `00-overview.md` §4.1 |
| **O-02** Google OAuth client | **Done and verified** — `make verify-google` passes | T-0.2 |
| **O-03** R2 credentials | **Done and verified** — `make verify-r2` passes | T-0.3 |
| **O-16** API hosting | **Fly.io, region `sin`**, always-warm. Database: Neon Singapore PG 18.6. SPA: Cloudflare Pages | `docs/setup/dns.md` |

### O-23 — Retention and requested anonymization · resolved 2026-09-22 · **superseded 2026-09-26**

Superseded by Phase R decision D16 (`70-redesign-overview.md` §1): the Admin
console's retention settings drive a nightly `cmd/maintenance` sweep under its
own database role, audited as System, and "delete disabled accounts after"
anonymises by the method below. Until R5 ships those settings, the rules below
still apply.


Thuong approved thirteen-month integrity retention, retained audit logs, manual
student anonymization and no automatic erasure of disabled accounts. Retention
requires both assignment closure and server receipt to be older than the UTC
cutoff; client event timestamps never authorize deletion. Structured identity is
removed while stable IDs preserve historical records. Free text, prior audit
entries and backups remain subject to separate review. Implementation:
`cmd/maintenance`, migration `00029_index_integrity_retention.sql`, and
`docs/setup/operations.md`. No public erase endpoint or API-role deletion grant
is introduced; production execution remains explicit.
