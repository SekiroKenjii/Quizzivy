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


### O-14 — DNS records for quizzivy.com · before first deploy
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


---|---|
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
