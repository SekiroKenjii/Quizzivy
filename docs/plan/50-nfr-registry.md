# 50 — Non-functional requirements registry

One place for every quality the system must have that is not a feature: how
fast, how safe, how reachable, how it degrades, what it must never leak. The
spec scatters these across §5, §6.5, §10, §11, §12, §13.5, §14 and §16; the
plan adds more in `30-risks.md` and `15-phase-5.md`; and several only became
visible while building. This file is where they are tracked from now on.

**How to use it.** Each requirement has an id, a measurable statement, where it
came from, its status, and the evidence or the gap. A PR that satisfies or
changes one edits the row in the same PR. New requirements are added as rows,
not paragraphs. The tables are reviewed at every `release/*` cut; the summary
at the end says how much is covered and what the next round is.

| Status | Meaning |
|---|---|
| ✅ | Met, with evidence in the repo (a test, a config, a CI job) |
| 🟡 | Partly met — the mechanism exists, the verification or a corner does not |
| ⬜ | Planned and specified (a T-5.x task or an issue), not started |
| ❌ | Not started and not yet planned |
| ❓ | Needs a decision from Thuong before it can be planned |

---

## A. Security

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-S01 | Passwords are hashed with Argon2id; hashing concurrency is bounded so a login burst cannot exhaust the 512 MB machine | §5.1, R-13 | ✅ | `server/internal/auth`, `core/modules.go` `boundPasswordHashing`, `fly.toml` note |
| NFR-S02 | Access token lives in memory only (~15 min); refresh token is an `httpOnly; Secure; SameSite=Lax; Path=/auth` cookie, stored hashed, rotated on use, with reuse detection revoking the family | §5.2, R-06 | ✅ | `server/internal/auth`, `web/src/stores/auth.ts`, single-flight refresh in `lib/api/client.ts` |
| NFR-S03 | Google sign-in verifies the ID token (`iss`, `aud`, `exp`, JWKS) and rejects unverified emails; PKCE on the authorization request | §5.1, §5.3, O-13 | ✅ | `server/internal/auth/google*`, `web/src/features/auth` |
| NFR-S04 | A student on `/admin/*` gets a 403 page, never a redirect; `mustChangePassword` fences every route | §5.4 | ✅ | `web/src/app/guards` |
| NFR-S05 | Every unauthenticated operation is rate-limited (per IP 10/min · 60/h; per code 30/h); the API refuses to start if a `public` route has no limiter | §6.5, §18 | 🟡 | `httpx/publicroutes.go` `AssertPublicRoutesLimited`, `ratelimit/`; **T-5.2** still has to drive each limit past its threshold and prove the LRU evicts |
| NFR-S06 | Public responses leak nothing: `/join/preview` returns only class and teacher name; join failures do not reveal which classes exist; login does not distinguish unknown user from wrong password | §6.5, §9 | 🟡 | Structural (see #27); the second leak review in **T-5.2** is not done |
| NFR-S07 | Join codes are stored as SHA-256 hashes, shown once, expire, carry `max_uses`, and are rotatable in two clicks | §6.1, §13.3, R-02 | ✅ | `server/internal/join`, G-06 panel |
| NFR-S08 | The student attempt payload never carries `isCorrect`, `sampleAnswer`, `acceptedAnswers`, `transcript` or `teacherNote` | §13.5, §14 E2E 9 | ✅ | Explicit column projections; `web/tests/e2e/payload-leak.live.spec.ts` |
| NFR-S09 | No server secret reaches the SPA bundle; only `VITE_*` values that are public by design | AGENTS.md, #8 | ✅ | `web/tests/integration/bundle-secrets.test.ts`, deploy preflight |
| NFR-S10 | CORS is an exact-origin allowlist with credentials, never `*`; `Vary: Origin` | §4.1 overview, R-07 | ✅ | `httpx/cors.go` + `cors_test.go` |
| NFR-S11 | Every teacher intervention that changes a student's record (extend, reset, void, reopen, flag, note, close early) is audited with actor, reason and the previous value, in the same statement | §8, D-04 | ✅ | `OLD`/`NEW` in `RETURNING` inside data-modifying CTEs; `app.audit_log`; `classes.Update` is the one write still unaudited (decision pending) |
| NFR-S12 | The database roles are split: `quizzivy_migrate` owns, `quizzivy_app` has DML only; the audit log is append-only for the app role | §13.5, #11 | ✅ | `migrations/00001`, `internal/db` grant tests |
| NFR-S13 | Browser hardening headers on both hosts: HSTS, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `frame-ancestors 'none'`, and a CSP on the SPA that admits only the Google script and the API origin | good practice; not in spec | ❌ | The API sets none (`grep -r Strict-Transport server/` is empty); Cloudflare Pages has no `_headers` file. Tracked as an issue |
| NFR-S14 | Request bodies are size-limited and the server has read/header/idle timeouts | good practice | 🟡 | `core/server.go` sets `ReadHeaderTimeout`, `ReadTimeout`, `IdleTimeout`; upload limits per §11.1 (#33); a generic JSON body cap (`http.MaxBytesHandler`) is not set — folded into the headers issue |
| NFR-S15 | Dependencies: none added without a stated reason; Sonar runs on every push | §14 DoD, §18 | ✅ | CI `Sonar` job; PR template |

## B. Privacy and data honesty

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-P01 | Integrity monitoring records only this tab's focus, copy/paste and network events; no devtools heuristics, no camera, no keystrokes | §10.1 | ✅ | `web/src/features/integrity/useIntegrityMonitor` |
| NFR-P02 | Integrity is reported as evidence for a conversation, never as a verdict: no auto-zero, no "cheating detected", calm copy; the help text states the limits | §10.4, §10.5, §12 | ✅ | G-05 timeline copy; `timeline.*` strings |
| NFR-P03 | Media objects are private; every read is a short-lived signed URL, never a public bucket | §11, `docs/setup/r2.md` | ✅ | R2 private bucket, `media.SignedURL`; #20 fixed cache headers |
| NFR-P04 | Personal data is minimal: email, name, optional Google picture; no phone, no address, no birthdate | §7 | ✅ | `app.users` columns |
| NFR-P05 | Retention: how long `attempt_events` (≈50 rows per attempt), audit rows and disabled accounts are kept, and whether a student can be erased | not in spec | ❓ | Nothing is ever deleted today. Vietnam's personal-data decree (13/2023/NĐ-CP) applies to a practice holding students' data; needs Thuong's call on retention and erasure before it needs a migration |
| NFR-P06 | Logs carry request id, route, status and latency — never a body, a token or an email | good practice | ✅ | `httpx/logging.go` |

## C. Reliability and data integrity

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-R01 | The server holds the clock: `deadline_at` is set at start, the client only renders it; closing early never truncates a running attempt | §9, O-05, R-11 | ✅ | `attempts.Start`, monitor skew correction, G-09 close-early copy |
| NFR-R02 | Autosave is idempotent and ordered; a reload mid-test loses nothing; a second tab supersedes the first | §9, §14 E2E 2/7, R-01 | ✅ | `answers-persist.live.spec.ts`, `session-takeover.live.spec.ts` |
| NFR-R03 | Submit is idempotent; a race between auto-submit, timer expiry and a manual tap yields one submission | §14 E2E 5, T-5.1 | 🟡 | `timer-expiry.live.spec.ts`; `auto_submit` mode itself is **T-5.1** (refused by the server until built) |
| NFR-R04 | Integrity events survive tab close: `(attempt_id, session_id, client_seq)` uniqueness, `ON CONFLICT DO NOTHING`, beacon flush with an attempt-scoped token | §10.6, R-05, R-12 | ✅ | `20-data-model.md` D-01, `attempts.beacon_token_hash` |
| NFR-R05 | An assignment pins a published version; editing the test never changes a paper a student has started | §13.3, R-04 | ✅ | `tests.TheVersionIsLockedOnceAnybodyHasStarted` |
| NFR-R06 | Every migration is reversible and CI proves it (up → down → up) on a clean PG18 | §13.7, T-0.17 | ✅ | CI `Server` job; `make migrate-redo` |
| NFR-R07 | Invariants live in the database, not only in code: status ↔ version, audio policy ↔ audio asset, one roster, disabled users leave every denominator | §13, D-04/D-05, #55 | ✅ | CHECK constraints, composite FKs, `internal/db` construct tests |
| NFR-R08 | Auto-grading is Unicode-normalised so NFD and NFC answers score the same | #49 | ✅ | `grading` package tests |
| NFR-R09 | A failed integrity flush never blocks answering or submitting | §10.6 | ✅ | fire-and-forget buffer |
| NFR-R10 | DB-backed Go tests cannot be answered by the build cache | this project's own lesson | ✅ | `make test-api` runs `-count=1`, as CI does |

## D. Availability and operability

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-A01 | The API is always warm: one machine minimum, never auto-stopped, `/healthz` checked by the platform | O-16, `fly.toml` | ✅ | `min_machines_running = 1`, `auto_stop_machines = false`, `[[http_service.checks]]` |
| NFR-A02 | Nothing deploys from `develop`; a merge to `main` deploys only after CI is green, API before SPA, migrations in the release command with rollback on failure | `docs/setup/deploy.md` | ✅ | `deploy.yml` `workflow_run` gate, preflight, post-deploy verification |
| NFR-A03 | The deploy verifies itself: the machine is running and the API answers, or the job fails loudly with the crash log | `deploy.md` | ✅ | "Verify the API answers", "Why it crashed" steps |
| NFR-A04 | Database backups exist and a restore has been rehearsed: Neon history retention is set to a known window and a point-in-time restore has been done once into a branch | good practice | ❓ | Neon `ap-southeast-1` PG 18.6 is the database; its retention window is not recorded anywhere in the repo and no restore drill is documented. Tracked as an issue |
| NFR-A05 | An outage is noticed by a person, not by a student: uptime check on `/healthz` and the SPA, with a notification | good practice | ❌ | Fly's check restarts the machine but tells nobody. Decision issue (what to alert with) |
| NFR-A06 | Unexpected errors are collected with their request id so a student's "it broke" can be found | §9 error boundary | 🟡 | The boundary shows a copyable id and the server logs it; no error tracker aggregates them. Same decision issue as A05 |
| NFR-A07 | Secrets are provisioned once and checked before use; a missing one fails the deploy with a name | `deploy.md` | ✅ | "Check the token is present" steps |
| NFR-A08 | Fitting the machine: 512 MB with Argon2 bounded; memory is raised only alongside the hash bound | R-13 | ✅ | `fly.toml` comment |
| NFR-A09 | Local development reproduces production: compose PG18 + MinIO, seed, `make test-api`; one documented loop | AGENTS.md, `00-overview.md` §9 | ✅ | `docker-compose.yml`, `Makefile`, `seed/` |

## E. Performance and capacity

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-E01 | Sized for one practice: ~50 students, a class sitting a test at once, a few thousand attempts a year | §1.1 | ✅ | Design assumption throughout; in-process rate limiter, one API instance |
| NFR-E02 | No N+1 on the monitor, grading, timeline or bank screens: a fixed number of statements per screen | §13.8, #28 | ✅ | Monitor = 2 queries; bank list fixed (#28); `ReferencesFor` one query per page |
| NFR-E03 | Polling is bounded: monitor every 15 s only while open and only in a visible tab | §8 | ✅ | `refetchIntervalInBackground: false`, `tests/units/attempts/monitor.test.tsx` |
| NFR-E04 | p95 latency at three years of data: monitor < 200 ms, attempt payload < 150 ms, timeline < 300 ms, autosave < 100 ms, dashboard < 200 ms | T-5.5 (replaces §16's "no seq scans") | ⬜ | Volume seed, `EXPLAIN` capture and a nightly job do not exist yet |
| NFR-E05 | Route-level splitting: a student's bundle carries no admin code, an anonymous visitor's neither; `@dnd-kit` only in the builder | §2, T-5.4 | 🟡 | `tests/integration/router-chunks.test.ts` proves the entry chunk is clean; the student and public graphs, and a size budget, are **T-5.4** |
| NFR-E06 | Initial JS budgets on `/login`, `/app`, `/app/attempts/:id`, enforced in CI; take-test measured on a throttled mobile profile | T-5.4 | ⬜ | No budget, no measurement |
| NFR-E07 | Uploads are bounded: mp3/m4a only, size and duration limits asserted in one place | §11.1, #33 | ✅ | `media` limits test |
| NFR-E08 | List endpoints paginate uniformly and honour the contract's page size | O-20, #42 | ✅ | `TestEveryPageSizeMatchesItsContract` |

## F. Accessibility

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-F01 | Lighthouse accessibility ≥ 95 on the nine key routes; `axe` in CI failing on serious/critical | §16, T-5.3 | ⬜ | Not measured; no axe run |
| NFR-F02 | Every interactive element is a real element with a visible focus ring; icon-only buttons carry `aria-label`; decorative icons are `aria-hidden` | §12 | 🟡 | The convention holds in shared components (RowMenu, PageHeader, SideColumn's splitter); a route-by-route audit is **T-5.3** |
| NFR-F03 | The whole attempt is completable by keyboard, including the audio player and fill-blank inputs; `Esc` always works, fullscreen never traps | §10.2, §14 DoD | 🟡 | #66 fixed the token field and palette; a full keyboard E2E is **T-5.3** |
| NFR-F04 | Motion is 150 ms ease-out only, no entrance animation; `prefers-reduced-motion` honoured | §12 | ✅ | `web/src/index.css` reduced-motion block |
| NFR-F05 | Colour never carries meaning alone: correct/incorrect, flagged and status badges also carry text | §12, F-07 | ✅ | `StatusBadge` words + colour; #67 |
| NFR-F06 | Side columns resize by keyboard as well as pointer: a focusable separator with a value | F-13 | ✅ | `components/shared/SideColumn.tsx`, `tests/units/shared/side-column.test.tsx` |

## G. Usability and information architecture

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-U01 | Every list and detail route has loading, empty and error states; empty is one sentence and one action | §12, §14 DoD, F-08 | 🟡 | The six shared blocks exist and every list uses them (Đợt 2 re-evaluation); the route-by-route audit and the network-failure-during-attempt copy are **T-5.7** |
| NFR-U02 | Every mutation answers: a toast on success, the server's reason on failure, a confirm (with a reason where the audit needs one) before anything destructive | #69, #70, F-09/F-10 | ✅ | `ConfirmDialog`, sonner toasts, `InterventionDialog` |
| NFR-U03 | One route per thing; states change the bar, not the URL | G-09, A-03 | ✅ | Assignment detail, test detail |
| NFR-U04 | Every relationship is reachable from both ends: an assignment names its classes with a link, a class lists its assignments, and the list can arrive narrowed to a class | this round (Thuong's two requirements), G-02/G-06/G-12 | ✅ | `TargetsLine`, `ClassAssignmentsCard`, `?classId=` on the list and its facets |
| NFR-U05 | The teacher never has to guess which link to bookmark: the dashboard, the sidebar counts and the grading queue are the daily entry points | A-00, G-10 | ✅ | Dashboard queue, `nav.grading` count |
| NFR-U06 | The deck is authoritative: no screen is built without a board, and a board's route is what ships | `docs/design/README.md` | ✅ | Every route has a board id in its component comment; deck `check.mjs` runs by hand — **not in CI yet** (issue) |
| NFR-U07 | Side columns keep one width per role (F-11), adjustable and remembered per browser (F-13) | F-11, F-13 | ✅ | `useColumnWidth`, `SideColumn` |
| NFR-U08 | Icon sizes follow one scale by position: badge or chip 12; table cell, 28px button, tab or text-xs line 14; default 16; student alert 18; top-bar toggle and card list item 20 | §12, F-14 | ✅ | Measured on the running app 2026-09-05; `Badge`, `TableCell` and `Button` xs/icon-xs size their own icons; `tests/units/ui/icon-scale.test.tsx` |
| NFR-U09 | Destructive actions are reversible where the domain allows: archive not delete, void not delete, close then reopen, undo on the toast | §8, G-08, G-09 | ✅ | No hard delete of a class, test or attempt |
| NFR-U10 | The admin is usable at 768 px with nothing clipped or unreachable; the panel becomes a sheet below 1024 | §8, F-12 | ✅ | F-12 walk (#65); `PageAside` sheet |

## H. Internationalisation and time

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-I01 | Vietnamese first: every string through `t()`, both locales, no key missing on either side, no empty string | §14 DoD, §18 | ✅ | `tests/units/i18n/parity.test.ts`, `no-hardcoded-strings.test.ts` |
| NFR-I02 | Server error messages are localised by `Accept-Language`, `vi` default | overview §7 | ✅ | `httpx/errors.go` |
| NFR-I03 | Store UTC, render `Asia/Ho_Chi_Minh` everywhere, through one module | #68 | ✅ | `lib/i18n/datetime.ts`, `datetime.test.ts` |
| NFR-I04 | Search is accent-insensitive on both sides (`pg_trgm`, client `fold`) | §13.8, D-11 | ✅ | Trigram index; `lib/fold.ts` |
| NFR-I05 | A native review of every `vi` string, prioritising join, integrity and intro copy; layouts hold with the longest strings | T-5.8 | ⬜ | Not done |

## I. Compatibility

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-C01 | Audio plays on iOS Safari: one `<audio>` element, `.play()` synchronous in the click handler, plays counted server-side | §11.3, §11.4, R-03 | ✅ | `AudioPlayer`, `audio-plays.live.spec.ts` |
| NFR-C02 | Student and public routes usable at 360 px with no horizontal scroll; safe-area insets respected | §9, §16, T-5.6 | ⬜ | Not measured on a device |
| NFR-C03 | Admin: desktop and tablet, 768 px floor; not made phone-friendly | §1.1, F-12 | ✅ | `min-w-[768px]` shell |
| NFR-C04 | Fullscreen is a gesture, never trapped; the exit bar always offers a way to submit | §10.2 | ✅ | Intro's Start, engine's exit bar |
| NFR-C05 | Supported browsers are named (current Chrome, Safari, Firefox, Edge; iOS Safari 16+) and E2E runs on at least Chromium | §14 | 🟡 | Chromium in CI only; the list is implicit, not written down |

## J. Maintainability and delivery

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-M01 | The contract is the source of truth: Go and TS are generated and CI fails on drift; test fixtures are validated against it | overview §3 | ✅ | `make gen-check`, `contractJson` in every MSW handler |
| NFR-M02 | CI runs every check a merge needs: Contract, Web (lint, typecheck, format, unit, integration, build), E2E stubbed, E2E live against a real PG18, Server (vet, staticcheck, golangci-lint, gofmt, DB tests), Sonar | §14 | ✅ | `.github/workflows/ci.yml`, six jobs |
| NFR-M03 | Gitflow: features off `develop`, `--no-ff` merges, releases through `release/*` to `main`, back-merged | overview §10 | ✅ | History |
| NFR-M04 | Comments describe abstractions and definitions, one line inside a function, none in tests; descriptions in `openapi.yaml` are the generated docs | AGENTS.md | ✅ | Convention held in review |
| NFR-M05 | Migrations are sequential, one concern each, never edited once merged | §13.7 | ✅ | `migrations/00001`–`00028` |
| NFR-M06 | The deck's self-check (undefined class, missing icon, unbalanced tag) runs in CI, not only by hand | `docs/design/README.md` | ❌ | Not in `ci.yml`. Tracked as an issue |
| NFR-M07 | Every finding becomes a self-contained issue; the queue is worked in rounds and closed with a comment saying what changed | working agreement | ✅ | #1–#77 closed |
| NFR-M08 | Sonar's rules fail locally and in CI before SonarCloud sees them: `eslint-plugin-sonarjs` in `pnpm lint`, `golangci-lint` with `server/.golangci.yml` in `make lint`; both at zero findings | §14 DoD | ✅ | `web/eslint.config.js`, `server/.golangci.yml`, CI `Web`/`Server` jobs |

## K. Observability

| ID | Requirement | Source | Status | Evidence / gap |
|---|---|---|---|---|
| NFR-O01 | Structured JSON logs with a request id on every line; the id is returned in the error envelope and shown by the error boundary | overview §7, §9 | ✅ | `httpx/requestid.go`, `logging.go`, `ErrorBoundary.tsx` |
| NFR-O02 | Health is observable: `/healthz` answers only when the database does | `router.go` | ✅ | `healthz(deps.DB)` |
| NFR-O03 | A person is told when it breaks (see A05/A06) | — | ❌ | Decision issue |
| NFR-O04 | The five dashboard counts and the sidebar counts are one round trip each, so the admin shell never fans out | §8, D-19 | ✅ | `GET /admin/dashboard` |

---

## Coverage

| Category | ✅ | 🟡 | ⬜ | ❌ | ❓ | Total |
|---|---|---|---|---|---|---|
| A. Security | 11 | 3 | 0 | 1 | 0 | 15 |
| B. Privacy | 5 | 0 | 0 | 0 | 1 | 6 |
| C. Reliability | 9 | 1 | 0 | 0 | 0 | 10 |
| D. Availability | 6 | 1 | 0 | 1 | 1 | 9 |
| E. Performance | 5 | 1 | 2 | 0 | 0 | 8 |
| F. Accessibility | 3 | 2 | 1 | 0 | 0 | 6 |
| G. Usability | 9 | 1 | 0 | 0 | 0 | 10 |
| H. i18n | 4 | 0 | 1 | 0 | 0 | 5 |
| I. Compatibility | 3 | 1 | 1 | 0 | 0 | 5 |
| J. Maintainability | 6 | 0 | 0 | 1 | 0 | 7 |
| K. Observability | 3 | 0 | 0 | 1 | 0 | 4 |
| **Total** | **64** | **10** | **5** | **4** | **2** | **85** |

Seventy-four of eighty-five are met or mechanically in place. What remains is
almost entirely Phase 5's hardening list plus four things the plan never wrote
down: browser headers, backups, someone being told when it breaks, and the deck
check in CI.

## The next rounds

**Đợt 4 — before `release/*` to `main`.** Everything that changes what a
stranger on the internet can do, or what is lost if the database is:

1. NFR-S13 browser headers on the API and a `_headers` file for Pages (issue).
2. NFR-A04 record Neon's retention and rehearse one restore into a branch (issue).
3. NFR-S05/S06 **T-5.2** — drive every public limit past its threshold, re-run the leak review.
4. NFR-M06 deck check in CI (issue; ten lines of YAML).
6. NFR-A05/A06/O03 — Thuong decides what tells him: the cheapest honest answer is an external uptime check on `/healthz` and `app.` plus Fly's log alerts; error aggregation can wait (issue with options).

**Đợt 5 — Phase 5 proper, after the first release.** T-5.3 accessibility
(NFR-F01–F03), T-5.4 bundle budgets (E05/E06), T-5.7 state audit (U01), T-5.8
language review (I05), T-5.6 360 px on a real phone (C02), T-5.1 `auto_submit`
(R03), and NFR-C05 writing the browser list down.

**Đợt 6 — once there is a year of data.** T-5.5 the volume seed and p95 budgets
(E04), NFR-P05 retention and erasure (needs the decision first), the audit-log
screen the plan already lists under "not in v1".

**Standing.** The two ❓ rows (P05, A04) cannot move without Thuong; everything
else has an owner in a task or an issue.
