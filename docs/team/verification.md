# R4 verification strategy

How the Senior Tester decides that R4 work is done, and how everybody else can
predict that decision. `AGENTS.md` is the rule set and wins over this file;
`docs/plan/70-redesign-overview.md` §8 and `docs/plan/74-r4.md` (exit criteria,
each task's "Done when", the release checklist) are the requirements. This file
adds the commands, the levels, the order and the evidence. The environment
(services, ports, toolchains) is in [`environment.md`](environment.md).

Written 2026-10-07 against `work/redesign-r4` @ `bb4d4000`. Re-date it when a
procedure changes.

## 1. Rules of evidence

- A claim is not evidence. A verdict names the **revision** (`git rev-parse HEAD`
  of the checkout that served or ran the check), the **environment**, every
  **command with its exit code**, and what ran, was skipped, was mocked or could
  not run.
- A green CI run is evidence about the tree CI built. CI skips a job whose file
  set already passed (`docs/setup/ci.md`), so "CI green" can mean "nothing ran".
  Read which jobs ran before relying on it.
- Playwright retries twice in CI (`retries: 2`), so a flaky test passes. Grep the
  job log for `flaky` and treat each as a finding (section 7).
- Vitest in CI uses its defaults (5 s test timeout, all cores); locally
  `vite.config.ts` gives three workers and 15 s. A timing-sensitive test can pass
  locally and fail in CI. For pre-merge parity run unit tests as CI does:
  `CI=1 pnpm --dir web test:unit` (add `--shard=1/2` and `2/2` to match).
- A tester never edits production code, and never removes, skips or weakens a
  failing test. A failure becomes a QA finding (`QA-<task>-<n>`, format in the
  definition) for the owner. Throwaway mutations (section 7) run in a scratch
  copy outside the shared checkout.
- Levels follow spec §14: **unit**, **integration**, **end-to-end** (Playwright
  stubbed, or Go in-process), **live** (Playwright against the real API).

Who and when, used in every table below:

| Tag | Meaning |
|---|---|
| PR | CI runs it on every pull request (the implementer reads the result) |
| Pre | the implementer runs the focused subset before opening the PR |
| V | the Senior Tester re-runs it on the candidate revision at `in_verification`, before merge |
| Rel | at T-R4.52, once, locally and in order (the tester runs it; the Tech Lead checks the evidence; production steps are Thuong's) |

## 2. Gate map

### 2.1 70 §8.1, every CI step

Run from the repository root; `web` commands as `pnpm --dir web ...`.

| # | Step | Command | Level | Needs | When | Who |
|---|---|---|---|---|---|---|
| 1 | Server lint | `make lint` (web lint, `go vet`, staticcheck, golangci-lint, gofmt) | static | toolchains | PR, V, Rel | CI; tester |
| 2 | Contract drift | `make gen-check` | static | Go, Node | PR, Rel; V when `api/openapi.yaml` changed | CI; tester |
| 3 | Go unit | `make test-api-unit` | unit | none | PR, V (touched modules), Rel | CI; tester |
| 4 | Go integration | `make test-api-integration` (`-tags integration`, `-count=1`) | integration | Postgres 18, MinIO | PR, V, Rel | CI; tester |
| 5 | Go end-to-end | `make test-api-e2e` (`-tags e2e ./tests/...`, isolation suite included) | end-to-end | Postgres 18 with `CREATEDB`, MinIO | PR, V, Rel | CI; tester |
| 6 | Import converter tests | the step in `ci.yml` "Import converter tests" (needs `docker/word-converter` image id) | integration | Docker daemon | PR; Rel if Docker exists | CI |
| 7 | Migrations up/down/up | `make migrate-redo`; CI runs `goose up`, `reset`, `up` on a throwaway DB | integration | disposable DB | PR (when a migration changed), V, Rel | CI; tester |
| 8 | Web lint | `pnpm lint:ci` (no cache) | static | | PR, V, Rel | CI; tester |
| 9 | Typecheck | `pnpm typecheck`, never `tsc --noEmit` (it checks zero files) | static | | PR, V, Rel | CI; tester |
| 10 | Format | `pnpm format:ci` (no cache) | static | | PR, V, Rel | CI; tester |
| 11 | Web unit | `pnpm test:unit` (CI shards 2) | unit | | PR, V, Rel | CI; tester |
| 12 | Web integration | `pnpm test:integration` (includes `router-chunks.test.ts`, a real build) | integration | | PR, V, Rel | CI; tester |
| 13 | Build | `pnpm build` | build | | PR, V, Rel | CI; tester |
| 14 | E2E, API stubbed | `pnpm e2e` (`chromium` and `mobile-chromium` projects, production build on 4173) | end-to-end | Chromium at `/opt/pw-browsers` | PR, V, Rel | CI; tester |
| 15 | Content editor prototype | `pnpm e2e:content` | end-to-end | Chromium | PR, V when the editor changed, Rel | CI; tester |
| 16 | E2E, live API | `pnpm e2e:live` | live | section 7.3 | PR, V when take-test, imports, auth, join or media changed, Rel (mandatory in R4: join codes, imports and the start path changed) | CI; tester |
| 17 | Deck pinned | `node scripts/check-design-deck.mjs` | static | | PR (Deck job), Rel | CI |

"Tests run" is judged by exit code **and** by the count: a Go package whose tests
all skipped reports `ok`. For the integration and e2e tiers confirm
`TEST_DATABASE_URL` was set (`go test -v ... | grep -c SKIP` must be explained),
and that the storage tests ran against MinIO and did not skip.

### 2.2 R4 exit criteria (`74-r4.md`)

| Criterion | Procedure | Level | When | Who |
|---|---|---|---|---|
| Every `/teacher/*` route matches the deck at 360, 768, 1024, 1280, 1440, light and dark, sidebar expanded, collapsed and drawer | Section 4, once per route, against the T-R4.51 seed; evidence in the PR template table and a route-by-route matrix kept by the tester | browser | each screen task at V; whole matrix at T-R4.51 and Rel | tester |
| Old teacher tree deleted; `router-chunks.test.ts` passes with regexes updated, not weakened | `grep -rn "admin" web/src/app/router.tsx` for a surviving old tree; diff of `router-chunks.test.ts` read by the tester: only path regexes may change, every assertion count stays (an assertion deleted is a finding); `pnpm test:integration -- router-chunks` | integration | T-R4.48 at V; Rel | tester |
| No active join code in production uses the legacy lookup | The count query T-R4.22 records, run on a Neon branch before and on production after the deploy; API log shows `legacy_join_codes_rotated` with the rehearsed count | production | Rel | Thuong runs, tester supplies the query and expected numbers |
| Pasted-text count fixture passes in Go and web | `go test ./internal/modules/imports/domain/recognition/...` and `pnpm test:unit -- paste-scan`; both read `api/testdata/pasted-text-counts.json`; also check CI's `web` and `server` sets still list the file (`scripts/ci/plan.mjs`) | unit | T-R4.57 at V; Rel | tester |
| Five canaries green and untouched | Section 3 | unit/integration | every PR touching a high-risk area; Rel | tester |
| Every CI step green on every PR, and once locally in order at T-R4.52 | 2.1 in order, stop at the first failure, record each exit code | all | Rel | tester |
| Released as `v0.10.0`; Neon hours measured for the week after (PR-7) | Release checklist, below. Pass threshold is not defined (see section 9) | production | after Rel | Thuong |

### 2.3 Release checklist (`74-r4.md`)

| Item | Procedure | When | Who |
|---|---|---|---|
| 70 §8.1 locally in order | 2.1; one log per step kept in the scratchpad, summarised in the handoff | Rel | tester |
| Canaries untouched | Section 3; `git diff <v0.9.0 or branch point>..HEAD -- <five files>` shows only the disclosed T-R4.51b fixture change in `publish_snapshot_test.go` and the router-chunks regexes | Rel | tester |
| Security block | Section 5: isolation suite entries (kinds `notification` and `override` exist), payload-leak walk, credential-limit test lists `resetStudentsPasswords` and `listClasses`, `STUDENT_SHARED` case, `/me/*` responses `no-store` | Rel | tester |
| Migrations rehearsed on a Neon branch | Timings, count of questions with more than 8 options, count of legacy codes to rotate. Not reproducible in this container | Rel | Thuong or platform engineer; tester checks the numbers are recorded |
| Exam windows | Query assignments closing inside the deploy window | Rel | Thuong |
| No import queued or running | Query imports by status; `word-pipeline-v3` retires v2 runs | Rel | Thuong |
| Cloudflare headers, `.env.example`, Fly config | `grep MEDIA_OWNER_QUOTA_MIB .env.example fly.toml`; `grep VITE_RICH_QUESTION_EDITOR -r .env.example web/ .github/` must find nothing except history | Rel | tester (repo part), Thuong (Cloudflare) |
| Deploy and smoke | `node scripts/verify-deployment.mjs` (`/healthz`, `/livez`, headers) then the checklist's manual smoke per role | Rel | Thuong; tester supplies the click path |
| Smoke of #330, #320, #314, #331 | Phone-class checks (airplane mode, reload mid-question, roster events). Only the engine's unit and stubbed phone specs run here; the real-device part is Thuong's | Rel | Thuong |
| Release notes | Read against the shipped behaviour: new join code sentence, notifications, extensions, versions, media replace, paste a test, editor | Rel | tester reads, Tech Lead edits |
| Hand-offs to R5 and R6 | Check each is written down; none is testable now | Rel | Tech Lead |

## 3. Canaries

Five tests plus one guard. A failing canary means something load-bearing broke:
fix the cause, never the test (`AGENTS.md`).

| Canary | Command | What it guards | Touching it means |
|---|---|---|---|
| `server/internal/modules/tests/application/tests/publish_snapshot_test.go` | `go test -tags integration ./internal/modules/tests/application/tests/ -count=1` (the whole package: the canary file holds five cases) | Editing a bank question after publish never changes the version | Untouched except T-R4.51b's disclosed fixture lifecycle change (say "fixture-only", never "byte-identical file"). Runs on T-R4.11, 12, 13, 16, 17b, 62, 65, 66 and on any editor change, because the editor writes what the snapshot freezes |
| `web/tests/units/media/audio-player.test.tsx` | `pnpm test:unit -- audio-player` | `.play()` in the click's tick; an `await` before it breaks iOS Safari silently | T-R4.35 adds a second play button (the media card). The canary covers `AudioPlayer` only: require a test of the card's path too (finding if absent) |
| `server/internal/modules/attempts/application/tests/events_test.go` (`TestTheSameClientSeqFromTwoSessionsBothPersist`) | `go test -tags integration ./internal/modules/attempts/application/tests/ -count=1` | Resumed attempt's timeline survives | T-R4.10b, 11, 12, 13, 46: run on the branch point and the last commit, both logs in the PR (plan requires it) |
| `web/tests/units/api/client.refresh.test.ts` | `pnpm test:unit -- client.refresh` | Five concurrent 401s issue one refresh | T-R4.9 (revoke others makes the current tab hit a 401), T-R4.8, anything under `web/src/lib/api/` |
| `web/tests/integration/router-chunks.test.ts` | `pnpm test:integration -- router-chunks` | The old tree and the editor stay out of the entry chunk; real build | T-R4.48 updates regexes. Any new eagerly imported teacher module (palette, bell, sidebar) is a candidate regression |
| Guard: `web/tests/units/builder/autosave-unmount.test.tsx` | `pnpm test:unit -- autosave-unmount` | Pending autosave flushed on unmount and on publish | T-R4.31a/b, T-R4.33, T-R4.63 to 66 (editor remounts) |

Canary protocol for a high-risk PR: record the exit code and test counts at the
branch point and at the head; compare `git diff <base>..<head> --stat` for the
canary paths and for `server/internal/modules/access/`, `features/take-test/`,
`features/integrity/`, `features/media/`.

## 4. Deck comparison, step by step

Applies to every changed or new screen, at V, and again for the whole route
matrix at T-R4.51. The deck is the single source of truth; `docs/design/gaps.md`
lists the decided departures (DG-nn). A difference is a finding unless a DG entry
covers it exactly; cite the entry in the PR table and never open a new gap
silently.

### 4.1 Servers and ports

| Port | What | Start |
|---|---|---|
| 5175 | `design-deck` | `python3 -m http.server 5175 --directory docs/design/deck` (the `design-deck` entry of `.claude/launch.json`) |
| 5173 | app, dev | `pnpm --dir web dev` |
| 8080 | API | `make dev-api` (needs `.env`, `JOIN_CODE_KEY`, Postgres, MinIO) |
| 4173 | production-build preview, E2E only | started by Playwright |
| 4175 | deck harness (composites with no route yet) | `web/tests/support/deck-harness/README.md` |

Use `localhost` everywhere, never `127.0.0.1`. Services are started by the Tech
Lead's schedule, not by the tester (README, "Shared files and resources"). The
deck loads React, Babel, Be Vietnam Pro and the lucide font from unpkg and
Google Fonts: if the container cannot reach them the deck renders unstyled or
blank. Check this first (`curl -sI https://unpkg.com | head -1` through the
proxy) and report it as "could not run" rather than comparing against a broken
page. Without the network the deck cannot be a reference at all.

### 4.2 Confirm nothing is stale

Run all of these and record the output in the evidence:

1. `git -C <checkout> rev-parse HEAD` equals the candidate SHA, and
   `git status --short` is clean (an unstaged edit is not the candidate).
2. `ss -ltnp | grep -E ':(5173|5175|4173|8080)'`; for each PID,
   `readlink /proc/<pid>/cwd` is the candidate checkout and `ps -o lstart= -p <pid>`
   is later than the last checkout or merge. Kill leftover servers: Playwright's
   `reuseExistingServer` is on outside CI and silently serves an old build.
3. Fetch a string unique to the change from the server, not from disk:
   `curl -s http://localhost:5173/src/<changed file>.tsx | grep -c '<unique token>'`
   (dev), or `curl -s http://localhost:4173/assets/*.js` (preview) after
   `pnpm --dir web build`.
4. The API serves the candidate contract: `curl -s localhost:8080/healthz`, and
   the process start time is later than the last Go change.
5. Browser: a fresh context (no storage from a previous run), cache disabled,
   `document.fonts.check('600 16px "Be Vietnam Pro"')` is true on both pages.
   A fallback font moves every measurement.
6. No console errors or failed requests on load (`browser_console_messages`,
   `browser_network_requests`), except those the DG register explains.

### 4.3 Viewports, themes, sidebar, language

- Widths: **360, 768, 1024, 1280, 1440**; height 900. The deck reads its own
  width (ResizeObserver), so the window width is the page width. Add the
  boundary pair around any threshold the screen uses (767/768 shell, 959/960
  imports container, 1023/1024, a table's column thresholds, which are on
  **content** width: `width - sidebar(248 or 60) - 56`, or `width - 28` under 768).
  Add one short viewport (1280 x 700) to see the real scrollbar.
- Theme: app `localStorage['quizzivy.theme']` = `light` or `dark`; deck
  `localStorage['qz-teacher']` = `{"route":"<id>","dark":true|false,"curId":"a1"}`
  then reload. Both before first paint (`boot.js` reads the app's keys).
- Language: `localStorage['quizzivy.locale']` = `vi` (default, written first) and
  `en` (taken from the deck). The deck is English only: compare structure and
  English copy against it, and read the vi copy for length and wrapping.
- Teacher sidebar, three states. Expanded: no `quizzivy.sidebar` key, width
  >= 768, sidebar 248 px. Collapsed: `localStorage['quizzivy.sidebar']='collapsed'`
  (60 px), or the sidebar's own toggle; the deck's collapse button, or the
  `sidebar` prop. Drawer: width < 768, opened from the top bar's menu button
  (both pages). A route whose handle says `sidebar: "collapsed"` (the builder,
  the import review) starts collapsed for the visit without changing the stored
  choice: test that the stored choice survives leaving it.
- Deck route ids for `qz-teacher.route`: `dashboard assignments assignment
  newAssignment grading tests testDetail builder bank question classes classDetail
  students imports importNew importRun importReview importConfirm settings`
  (others in the deck belong to later releases and must not appear in the
  product: no nav item, palette entry or button may point at them).
  Dialogs, menus and sheets are opened by clicking the same control on both pages.

### 4.4 Data shaped like the deck's fixtures

The deck's fixtures are constants at the top of its script (`TESTS`, `BANK`,
`WI_IMPORTS`, the assignment rows and so on). Both pages must show the same
rows, in the same counts and lengths (a title that wraps in one and not the
other proves nothing).

- When T-R4.51 lands: `make seed` loads `seed/08-dev-teacher-workspace.sql`
  (seven tests, eight media, four classes, assignments in every status, a flagged
  attempt, ungraded answers, four imports one of them pasted, one test with a
  question of each of the five types). Sign in as `giaovien@quizzivy.com` /
  `quizzivy-dev` (the teacher the seed relies on), and as `thuong@quizzivy.com`
  for the Admin's own-rows lists (DG-53).
- Until then the seed has only `01`..`07`. For a screen verified earlier, build
  the fixtures as Playwright route stubs from the deck's constants (the e2e
  support in `web/tests/e2e/support/api.ts` shows the pattern; responses must
  pass `contractResponse` validation), or write them through the API into a
  scratch database. Say which in the evidence: a stubbed run proves the
  rendering, not the server.
- Cover the states the deck draws, not only the happy list: empty, one row, a
  full page, a long Vietnamese title, a title with no spaces, 99+ counts, a
  student with no avatar, an error from every query (stub 500 and 403), a slow
  response (skeleton), and a disabled control.

### 4.5 Measuring

Never by eye or by screenshot alone. Run the same script on both pages
(`browser_evaluate` or `page.evaluate`), with a selector map written once per
screen (deck selector and app selector side by side), and diff the JSON.

```js
const props = ["font-family","font-size","font-weight","line-height","letter-spacing",
  "color","background-color","border-top-color","border-top-width","border-radius",
  "padding","margin","gap","box-shadow","opacity"];
const read = (el) => {
  const r = el.getBoundingClientRect(), s = getComputedStyle(el);
  return { x: +r.x.toFixed(1), y: +r.y.toFixed(1), w: +r.width.toFixed(1), h: +r.height.toFixed(1),
    text: el.textContent.trim().slice(0, 60), ...Object.fromEntries(props.map(p => [p, s.getPropertyValue(p)])) };
};
return Object.fromEntries(Object.entries(MAP).map(([k, sel]) => [k, [...document.querySelectorAll(sel)].map(read)]));
```

Tolerances: lengths within 0.5 px after rounding of the container (a sub-pixel
sidebar or scrollbar shifts every x); colours equal as computed `rgb()`/`oklch()`
in both themes; font family, size, weight, line-height exactly; text exactly,
except the vi string the deck does not have. Known, expected differences: DG-122
(a Teacher h1 sits 6 px higher because the product keeps `line-height: 1.25`),
DG-117, the main scrollbar (15 px when scrollbars show; compare at 1280 with
and without), prototype chrome that never ships (screen-switcher pills, demo
accounts, "Try" hints, canvas theme buttons, "Coming next").
Also assert on the product page alone:
`document.documentElement.scrollWidth <= innerWidth` (never scrolls sideways),
no raw colour (the unit test covers source; check computed colours resolve from
tokens), `data-scale="deck"` present on the root.

### 4.6 Dimensions to cover, every screen

| Dimension | Check |
|---|---|
| Layout and spacing | Container max-width, grid tracks, gaps, paddings, sticky regions, the content width at each sidebar state, no overlap, no horizontal scroll; thresholds at their boundary pair |
| Type | Be Vietnam Pro loaded; size, weight, line-height, letter-spacing per role (h1, sub, label, meta, badge, table cell); digits where the deck uses tabular figures (the face has none: cell-per-digit timers) |
| Colour tokens | Computed colours equal the token in light and dark; semantic tones (success, warning, danger, info, lime accent) used where the deck uses them; contrast of text on every tone; `--focus` ring colour |
| States | Loading skeleton, empty, error with retry, success toast, disabled, hover, active/pressed, selected, focus-visible, read-only, over-limit, partial failure (bulk) |
| Copy | Every string through `t()`; vi and en both present and reading naturally; en equals the deck; vi does not truncate or break the layout (+30 % length); plurals and singular forms; numbers and dates in the user's zone (`Asia/Ho_Chi_Minh` default) |
| Keyboard and focus | Tab order follows the visual order; every control reachable and operable (Enter, Space, Esc, arrows in toolbars/menus/tabs/segmented); a visible `:focus-visible` ring on each; dialogs trap focus and return it to the opener; no keyboard trap; shortcuts do not fire inside inputs or during IME composition |
| Accessibility tree | `browser_snapshot` names/roles: landmarks, headings in order, `aria-pressed`/`aria-current`/`aria-live`, alerts as `role="alert"`, icon buttons with names |
| Motion | `prefers-reduced-motion` emulated: marquee, live dot, skeleton shimmer static; hover/focus pauses continuous motion |
| Responsive behaviour | Resize across thresholds with state kept (forms, dialogs, selection survive); the drawer closes on navigation |

### 4.7 Recording

One row per screen in the PR description, in the template's table:
`| Screen | Deck page | Widths checked | Light | Dark | Differences |`
with sidebar states named in "Widths checked" for teacher screens
("360 drawer, 768/1024 expanded+collapsed, 1280/1440 expanded+collapsed"), and a
Differences cell that lists each measured delta with its DG id or "finding
QA-...". Behind the table the tester keeps, in the scratchpad, the measurement
JSON per width/theme and screenshots named
`<task>-<screen>-<width>-<theme>-<sidebar>-<app|deck>.png`, and quotes their
paths in the handoff's "Deck comparison" field. "Not compared at ..., because
..." is a valid entry; an empty cell is not.

## 5. Security checks

| Check | Command or procedure | Applies when | Level |
|---|---|---|---|
| Two-teacher isolation | `cd server && go test -tags e2e ./tests/... -run TestAnotherTeachersIdsAnswerAsMissingOnes -count=1` (needs MinIO); the table is `server/tests/isolation_cases_test.go` | Every new or changed `/teacher/*`, `/app/*`, `/me/*` operation: the PR adds its entry and an `x-resource` kind for every uuid in path, query and body (`resource_contract_test.go` fails on a missing kind); a `/teacher/*` list declares `x-resource-list`. R4 adds: duplicate (`classIds`), results export (`ids`), item analysis, version diff, extend, overrides (new kind `override`), reset-passwords (`studentIds`), notifications, avatar, media rename/replace | end-to-end |
| `/auth/*` is outside the suite | T-R4.9 needs its own test that another user's `familyId` answers 404 and the current one 409 | T-R4.9 | integration |
| Permission map | `go test ./internal/core/router/tests -run 'TestThePermissionMapIsPinned\|TestEveryNonOpenOperationDeclaresAPermission' -count=1`; read the diff of `testdata/permissions.golden` line by line against `70` §4.1: every new operation has the least permission that fits, and none moved silently | any contract change | unit |
| Payload-leak walk | `pnpm e2e:live -- payload-leak` (E2E 9, banned keys `isCorrect`, `sampleAnswer`, `acceptedAnswers`, `transcript` at any depth) plus the Go boundary tests (`server/gen/openapi/boundary_test.go`, `web/tests/units/contract/`) and the student-payload invariants of the contract tests. Never weaken the key list. Beyond the keys, read the new fields by meaning: no other student's name, email or score (class average is an aggregate with a floor of three), no override `reason`, no teacher-only note, no `geoLabel` of another user | Any added or changed `/app/*` or `/me/*` response: R4 has `/me/notifications`, the result's `classAverage`, `studentNote`, the effective close for an override, avatar URLs | live + unit |
| Rate limits | `ratelimit_contract_test.go` (every `x-rate-limit` pinned to the registry), `principal_limits_test.go` (per-actor position after the gate), `credential_limits_test.go` (credential minters listed: `resetStudentsPasswords`, `listClasses`); a new test drives the real limiter to 429 with `Retry-After` and proves two users behind one address do not share a bucket | New public operation (rate limit **and** leak review in the same PR); every authenticated operation that mints a credential or does heavy work (avatar PUT 10/h per user, reset 2/min and 10/h per actor) | unit/integration |
| Escalation | `go test -tags integration ./internal/modules/identity/application/tests/ -run 'Escalat\|StudentTarget\|OnlyAStudentNoOne\|ReachRule\|CreatedStudent' -count=1`; add: bulk reset of an Admin who has `learning.take_tests` fails per item, teacher B resetting a student shared with teacher A gets `STUDENT_SHARED` in `failed`, a reset student's old access token fails on its next request, and a revoked device does the same | T-R4.9, T-R4.20, anything touching `access/` or `users.role_id` | integration |
| Session revocation | A revoke bumps `session_epoch`, revokes the family and calls `Principals.Forget` in one command; test across two app instances sharing a database for the 10 s cache claim | T-R4.9, T-R4.20 | integration |
| `/me/*` caching and secrets | Response headers `Cache-Control: no-store` (also the temporary password response); `bundle-secrets.test.ts`; the API log of a reset run contains no temporary password | T-R4.8, 9, 20 | integration |
| Upload hardening | PNG declaring 30000x30000 refused before decode; GIF 415; 2 MiB + 1 byte 413; 199x199 and 2049x2049 refused, 200x200 and 2048x2048 accepted; EXIF/GPS stripped; polyglot file; two uploads at once respect the semaphore of two | T-R4.8, T-R4.17b | integration |
| Injection | CSV: cells starting with `=`, `+`, `-`, `@`, tab, CR, and the same after leading spaces or as full-width characters; HTML paste: `onerror`, `javascript:`, `data:`, `style`, `<img src=http://x>` must make no network request (assert in Playwright) | T-R4.13, T-R4.63 | unit + e2e |
| Dependencies and secrets | `git diff <base>..HEAD -- web/package.json server/go.mod`: any new dependency has its reason in the PR; no secret in the diff or `.env.example` | every PR | PR |

## 6. Per-bundle verification

Bundles are the Tech Lead's candidate objectives. The "Ready" backend and
frontend tasks are listed in `ledger.md`. Each task also gets the section 2.1
subset and, where it has a screen, section 4.

### A. Backend wave: T-R4.16, 11, 12, 13, then 8, 9, 20

The tasks touch `api/openapi.yaml` and migrations at once. Risks: contract and
migration numbering collisions between parallel branches (a number is taken
before the PR runs CI and changes when another PR merges first); an
additive-only rule broken (the R3 binary must keep inserting during
the rolling deploy); attempts/publish regressions.

| Task | Risk | Checks |
|---|---|---|
| Common | Migration breaks the old binary or a populated table | `make migrate-redo` on a database that already holds the seed; an integration test inserts rows "the old binary's way" (without the new columns); constant defaults only; `-- +goose Down` drops cleanly; the PR names its file; no `SELECT *` (`grep -rn 'SELECT \*' server --include=*.go`) |
| Common | Generated drift | `make gen-check`; generated files in their own commit; `x-permission` on every authenticated operation; isolation entries |
| 16 versions/diff | Diff wrong on reorder, restore-as-draft copies, group members; `getTest` write amplification | Unit table per kind (`added removed changed answer points`) and a reorder-only case that reports nothing; unmatched questions match by identical content; integration: bank edit makes the draft differ; `GET` is read-only (compare `tests.updated_at` and `pg_stat_user_tables.n_tup_upd` before and after); `unpublishedChanges` null in all four stated cases and a number otherwise; `publishTest` returns `testUpdatedAt` and a following autosave does not hit `STALE_WRITE`; `publish_snapshot_test.go` green and untouched; a 100-question paper diff stays within a stated query count (N+1 check) |
| 11 review options, note, lock | Result withholding and aggregate leak; lock boundary | Lock: PATCH of `testVersionId`, `durationMinutes`, `maxAttempts` while derived status is `open` returns 409 `ASSIGNMENT_LOCKED`, other fields allowed, `scheduled` and `closed` unlocked, and the status flip exactly at `opens_at`/`closes_at`; `after_close` withholds score and answers until the **effective** close (see section 9 for overrides); `classAverage`: 2 vs 3 target students with a graded attempt, voided and ungraded excluded, best attempt per student, absent before close, absent when the switch is off; note: 500 after trim passes, 501 and whitespace-only behave as specified, plain-text rendering; leak walk on the student result and intro; the MSW sweep (about 14 fixtures) is its own mechanical commit; both canary runs (branch point and head) in the PR |
| 12 extensions/overrides | Concurrency with start, resume and submit; deadline recompute; privilege on the maintenance role | Boundaries: minutes 1/10080/10081, `studentIds` 100/101, `extraAttempts` 10/11, duration 1/600/601, reason 1/500/501/empty, CHECK (at least one of the three); non-target student returns 422 naming them; extend vs reopen on a closed assignment (`ASSIGNMENT_CLOSED`); concurrent `extend` x `startOrResume` x `submit` on one attempt row (two connections, assert `deadline_at = least(started_at + duration, close)` and only-later); a closed assignment with an override reads open **for that student only** (another student still closed); monitor row shows "Extended to ..."; audit rows via the CTE pattern; maintenance extension also moves override times (can only be proven with the restricted role; if the test database lacks it, record "not run"); `join_and_take_flow_test.go` case; both canaries |
| 13 duplicate/analysis/CSV | Formula injection; sizes; ordering | Escape cases in section 5; BOM and `Content-Disposition`; 50 and 51 ids, 20000 and 20001 rows (behaviour at the limit is unspecified, section 9); item analysis ordering with ties and nulls last; manual full-points rule; list filters `q`, `classId[]`, `questionCount`; duplicate copies version, window length and rules, opens now for the original's length; isolation for all three; both canaries |
| 8 avatar | Image bombs, EXIF, storage orphans | Section 5 upload row; replacing removes the old object; a failing delete of the old object still succeeds (fake store); `DELETE` clears; the presigned URL expires in 24 h (clock or parse `X-Amz-Expires`); rate limit 10/h per user, not per address; runs against real MinIO in the integration tier |
| 9 devices | Refresh stampede, wrong "current", geo spoofing | Another user's family 404; current family 409; revoke-others; a revoked device's token fails on its next request on the same instance and within 10 s elsewhere; label built only when `CLIENT_IP_HEADER=CF-Connecting-IP` and ignores `X-Forwarded-For`; rotation copies the label; revoked/expired absent; `client.refresh.test.ts` green and untouched; live test: revoke-others in one browser context, the other tab recovers through one refresh; a refresh from a just-revoked family does not trip reuse detection against the user's other families (to verify, not assumed); user agent fuzz (empty, 10 kB, control characters) never panics and labels stay at most 80 characters |
| 20 bulk reset | Hash cost vs request time; secret handling | 40 and 41 ids; two hashes at most at once (instrumented counter, inside `MAX_CONCURRENT_PASSWORD_HASHES`); the server's write timeout exceeds the measured time of 40 resets on the CI-class machine; partial failure returns `items` and `failed`; `no-store`; password absent from logs and audit; credential-minter test; rate limit; `STUDENT_SHARED` single and bulk agree (shared helper); each reset revokes families, bumps epoch, calls `Forget`; `listStudents` `classId` repeated and single value, invalid uuid 400 |

Bundle gate: the combined branch (all merged) runs 2.1 end to end once, plus the
isolation suite with every new entry, before the screens that consume them start.

### B. Content editor chain: T-R4.63, then 64, then 66 (65 later)

One shared editor in three hosts, and it writes what the publish snapshot
freezes and the student engine renders. Risks: silent data change (rich/Markdown
conversion), unsafe pasted HTML, regressions in the option and group editors,
autosave losing edits, IME and Vietnamese input, bundle size.

- Behaviour: toolbar order per profile; "Insert gap" absent in the explanation;
  no Image or Audio button; link popover (applied, refused with the alert text,
  removed, Esc returns focus); table row and its disabled titles; words count
  on a deferred value; `readOnly` renders `ContentView`; placeholder only while
  empty; a file pasted or dropped is refused with the host's notice.
- Paste: converter drops `img picture svg video canvas`, counts them, **loads
  none** (Playwright asserts zero requests to the image host); every other
  refusal stays (`del`, `h4`, unknown tag, `http:` link); over 262,144 characters
  opens no dialog; stale preview; Ctrl+Shift+V plain text; HTML beside a file.
  Word and Google Docs real clipboards need Firefox and WebKit and the real
  applications: this container has Chromium only (section 9), so those items of
  T-R4.51 are "not run" unless Thuong runs them.
- Data safety: open an existing question stored as Markdown and one stored as
  rich content, edit one character, confirm the stored form does not change
  without "Switch to Markdown"/"Apply conversion"; round trip through the API
  and `publish`, then through the student engine's `ContentView` (live); NFC and
  NFD Vietnamese, Telex composition (`compositionstart/end`), "nghe" vs "nghé"
  as separate accepted answers, "London" vs "london" under Match case.
- Regression suites: `rich-options.spec.ts`, `group-authoring.spec.ts`,
  `rich-question-content.spec.ts`, `pnpm e2e:content`, `content/clipboard.test.ts`
  moved (not deleted) image case, `builder/autosave-unmount.test.tsx`, the five
  canaries, `router-chunks` (the editor stays a lazy chunk). Removing
  `VITE_RICH_QUESTION_EDITOR` (T-R4.64) is tested in the world where the flag is
  absent everywhere: `.env.example`, `playwright.config.ts`, CI, Pages variables.
- Deck check: builder "Prompt", "Explanation", import review "Question";
  Rich and Markdown, Write and Preview, table row, gap chip, link popover, notice
  band, both dialogs; five widths, both themes.

### C. Ready screens: T-R4.57, 35, 32, 27a

Frontend-only against merged backends. Each runs section 4 completely, plus:

- **57 paste import:** run every case of `api/testdata/pasted-text-counts.json`
  in `paste-scan.test.ts`; boundaries at 99,999 / 100,000 / 100,001 code points
  with astral characters (counted as one) and decomposed Vietnamese (NFC first);
  Ctrl+Enter not firing during composition; clipboard allowed, refused, missing
  and empty; `useBlocker` and `beforeunload` only when unstarted text exists;
  retry reuses `uploadId`, an edit takes a new one; 409 re-reads, 429 shows the
  server's message; `TEXT_MARKS_UNAVAILABLE` has copy; pasted text with markup is
  shown as text; "Use a sample" absent; the DG-16 cold-worker timing is T-R4.51's.
- **35 media:** `audio-player.test.tsx` untouched; a same-tick `.play()` test for
  the card; `upload-panel.test.tsx` moved to the hook with **no case dropped**
  (diff the `it(` names before and after); `QuestionMediaField` and
  `MaterialAssetDialog` unchanged in behaviour; facets follow the search; Replace
  and Delete copy matches DG-09 (published versions keep the file; delete refused
  while used); quota numbers; upload progress, failure, cancel; object URLs
  revoked; real iOS Safari is residual risk R-03 and cannot run here.
- **32 question bank:** filters, tags, page and sort in the URL and restored on
  reload; server counts; Match Any/All from two tags; popover keyboard (Enter
  toggles first match, Esc closes); column thresholds on content width with the
  sidebar expanded and collapsed; the Filters aside (220 px sticky from 1024) vs
  sheet; bulk delete copy ("Tests that already use them keep their copy.") is
  true (snapshot canary); no Import button.
- **27a wizard:** `?step=`, `?test=`, `?class=` (renamed from `testId` and
  `classId`: grep all four link sites and `web/tests/e2e/**` for the old names);
  dedupe count with a student in a class and added singly; "Save draft"
  persists every field that exists at this step and the draft reopens in the
  wizard at `/teacher/assignments/:id/edit`; unsaved-change prompt; stepper two
  columns on phones; marquee only on overflow. It edits `support/live.ts` and
  `admin-change-requests.live.spec.ts`: run the **whole** live suite, not only
  that file.

### D. The two red drafts: #416 (T-R4.28), #414 (T-R4.31a)

Both are drafts; their PR bodies list the outstanding gates. Causes of the red
runs are in section 10. After they are green:

- **#414 builder frame and outline:** `autosave-unmount.test.tsx` and the other
  autosave suites; `router-chunks`; keyboard drag and drop (dnd-kit keyboard
  sensor) with arrow keys, plus pointer and touch; `quizzivy.builder.outline`
  persistence and the clamp (content minus 374); title input commit/escape/IME;
  marquee geometry; deck check at five widths. The builder does not force the
  sidebar collapsed: `handle.sidebar: "collapsed"` is T-R4.38's (import review)
  only. The red was a fixture, not behaviour:
  confirm the fix made the fixture valid and did not loosen
  `contractResponse` validation.
- **#416 grading:** the PR states browser acceptance and deck comparison are
  still owed, and "authored browser cases are not execution evidence": run them.
  Network log assertions: a score click sends Grade only; Save & next flushes
  the comment then Grade; Finish rereads the paper first; a failed Finish keeps
  the mark and Retry sends Finish only; reload recovery pages through submitted
  and timed-out attempts and offers review/Finish. Half points (0.5 steps, no
  float drift), two tabs on one answer (stale write), actor/permission change
  mid-session, renewed media URLs (fake clock), keyboard shortcuts inert in
  inputs and IME, `AnswerReview` prop-type-only change leaves re-grading intact,
  `attempts` canary run before and after. Isolation entries for grading routes.

## 7. Test-infrastructure health

### 7.1 Determinism

- **Clock-dependent fixtures.** #419 fixed a Go fixture that set a fixed
  `deadline_at` and a default `started_at = now()`, so it failed when the clock
  crossed a threshold. 87 Go test files call `time.Now()` and 35 web test files
  call `new Date()`/`Date.now()` (64 use fake timers). Rules: a fixture derives
  every time from one injected `now`; a stored absolute date is compared with
  an injected clock, not the wall clock; no sleep to wait for time. Audit
  `grep -rn 'time.Now()' server --include=*_test.go` and
  `grep -rnE 'new Date\(\)|Date\.now\(\)' web/tests` per PR that adds one.
- **Zone and boundary sweeps.** Run the web unit suite under `TZ=Asia/Ho_Chi_Minh`
  (the Playwright zone), `TZ=UTC` and `TZ=America/Los_Angeles`; date features
  (calendar, schedule, expiry at 23:59 local, due tomorrow) get cases at local
  midnight, week and month ends and 31 Dec. The container clock cannot be moved,
  so this is by injected clock only.
- **Ordering.** Rows created in one transaction share `now()`: an `ORDER BY
  created_at` without a tie-breaker is unstable; tests that assert order must
  say which key breaks the tie (`uuidv7` ids order by creation). Go map iteration
  and un-sorted `SELECT`s must not be asserted positionally.
- **Shared rows.** The live suite and the dev seed share one database and
  `fullyParallel` is on (CI uses one worker; locally Playwright defaults to
  several): run `pnpm e2e:live --workers=1`, and never two live runs at once.
  The Go integration tier commits to one `TEST_DATABASE_URL` across parallel
  packages; a test that commits owns its removal (registered before the first
  insert, errors checked), and a "recent" list read must not see another
  test's rows. A failure that passes alone (`-run <name> -count=1`) and fails in
  the package or under `-p 1` vs default is an isolation defect: raise it, do not
  retry it green.
- **Flake handling.** No rerun-until-green. A flaky pass in CI (`flaky` in the
  Playwright output) and a vitest timeout are findings with the test name.
  Quarantine is not allowed.
- **Mutation spot checks.** When a PR claims "each rule mutated once, and a case
  fails each time" (T-R4.57) or "meaningful mutations passed" (#416), re-do three
  of them in a scratch copy of the touched files in the scratchpad, not in the
  shared checkout.

### 7.2 Isolation of E2E runs (T-R4.51a, PR #400)

`go test -tags e2e ./tests/...` uses `TEST_DATABASE_URL` only as a control
connection, creates a fresh database per invocation, applies migrations, creates
uniquely named buckets and drops both afterwards (`server/tests/README.md`). Its
role needs `CREATEDB` and the object store credentials to create and delete
buckets. Consequences: two agents may run the Go e2e tier concurrently; a
SIGKILL leaves a database and buckets (the README says to clean by the logged
names); with `TEST_DATABASE_URL` unset the tests **skip**, so check for SKIP. The
publish fixture lifecycle (T-R4.51b) keeps the canary's setup in one outer
transaction. Browser-side E2E has no such isolation: ports 5173, 4173, 8080 are
shared, so the Tech Lead schedules one browser/dev-server user at a time.

### 7.3 Live suite prerequisites (`pnpm e2e:live`)

Everything in `ci.yml` job `e2e-live` must hold locally: Postgres 18 with the
roles from `scripts/provision-db.sh`; `goose up` then every `seed/*.sql` in
order (`04-dev-e2e.sql` holds the take-test fixtures); MinIO with its buckets
(S3 path-style on); the API on 8080 with `CORS_ALLOWED_ORIGINS` including
`http://localhost:4173`, `JWT_SIGNING_KEY`, `JOIN_CODE_KEY` (base64 of 32 bytes),
`S3_*`; port 4173 free or serving the **current** build; Chromium from
`/opt/pw-browsers` (never `playwright install`); the mp3 fixture under
`web/tests/e2e/fixtures/`. From T-R4.51 it also needs the import worker and the
converter image (Docker daemon) for `import.live.spec.ts`. Stop Vite, the API
and preview afterwards. A live case needs a reason of the "two halves meet" kind
(`AGENTS.md`); stubbed is the default.

### 7.4 CI-specific traps to check on every red or suspicious-green run

- A `Plan` skip is proof-based: a job's "success" may be a skipped job. Read the
  run's job list, not only the final check.
- A run can end `failure` with all jobs green and no **CI result** job (see #416,
  section 10); "Re-run failed jobs" re-runs only failures, "Re-run all jobs" forces
  everything.
- Node in this container is 22 and the CI uses 24 and pnpm 11.25, Go 1.27: record
  `node -v`, `pnpm -v`, `go version` in the evidence; a difference is an
  environment limit to state, not a result.
- A test that reads a file outside its job's set fails with "no such file" in CI
  only: new cross-tree reads need `scripts/ci/plan.mjs` updated in the same PR.

## 8. Order of a verification pass (one task)

1. Read the task's Done-when and this file's bundle block; list checks and name
   the level of each *before* the work starts; flag ambiguous criteria.
2. Confirm candidate SHA, clean tree, servers current (4.2).
3. Static and focused unit tiers; canaries if a high-risk area is touched.
4. Integration/e2e tiers the change calls for; migration up/down/up.
5. Security rows that apply (section 5).
6. Browser: deck comparison (section 4); live suite when required.
7. Negative, boundary, concurrency and recovery cases of the bundle table.
8. Handoff in the definition's format; findings as `QA-<task>-<n>`; retest each
   fix myself before closing.

## 9. Ambiguous or untestable criteria found

To be settled by the Tech Lead or Thuong before anyone builds against them.

1. **"Matches the deck" (exit criterion, T-R4.51)** has no tolerance. Proposed:
   section 4.5 (0.5 px, exact tokens and text, DG-listed exceptions only).
2. **Firefox and WebKit pastes from real Word and Google Docs (T-R4.51)** cannot
   run in this container (Chromium only, no Office, no Google). They need Thuong
   or a machine with those browsers. Mark "not run" in evidence otherwise.
3. **DG-16's 10 s cold-worker measurement** depends on hardware, converter image
   and Docker. No run count or percentile is stated. Proposed: five cold starts,
   report max, on the machine named in the evidence.
4. **T-R4.11 and T-R4.12:** with `after_close`, is "the effective close" per
   student (override-aware) or the assignment's? The student result is withheld
   until which time for an extended student? Untestable until stated.
5. **T-R4.13:** behaviour at the 20,000-row and 50-id limits is not specified
   (truncate, refuse with which code?). "Submitted (actor's zone)" when the
   actor's zone is invalid.
6. **T-R4.11:** "at least three target students have a graded attempt": does a
   student with only an ungraded or voided attempt count as a target? Does
   unsubmitted count? Needs the rule in the task.
7. **T-R4.9:** "within R2's 10 s cache on any other [machine]" implies several
   API instances; the deploy has one machine. Testable only with two app
   instances over one database in the harness; state that this is the proof.
8. **T-R4.12:** the maintenance extension moving override times needs a grant
   that arrives in R5 (release hand-off). In R4 it is unverifiable with the
   restricted role; say which role the test uses.
9. **T-R4.27a:** "Save draft persists every field (DG-65)" refers to fields
   that 27b adds; at 27a it can only cover test and students.
10. **T-R4.51:** "Vite and the API stopped afterwards" has no check. "The plan's
    Done-when boxes lag" (ledger): ticks are not evidence.
11. **Neon compute hours after release (PR-7, NFR-A10):** no pass threshold in the
    exit criterion; the NFR names "well under 6 CU-hours" for a zero-traffic day.
    Confirm that is the criterion. Not testable before release; polling with
    idle-aware stop (T-R4.3) is the only pre-release proxy (count requests in an
    idle tab for 5 minutes).
12. **Two sidebar keys, no disagreement:** `70` §6 documents both.
    `quizzivy.column.sidebar` is the old `AdminLayout`'s width and goes at
    T-R4.48; `quizzivy.sidebar` is `TeacherLayout`'s collapsed state
    (`web/src/layouts/shell/sidebarState.ts`). The deck procedure above uses
    `quizzivy.sidebar`.
13. **"Every CI step green in GitHub CI on every pull request"** can be met by
    skipped jobs (proof-based skipping). The release run (2.1 locally, in order)
    is what actually closes it.
14. **Real-device behaviour** (iOS Safari audio, airplane-mode drafts, #330, #320,
    #314, #331) cannot run here; only the stubbed phone specs can.

## 10. State of the two red PRs (read-only, 2026-10-07)

- **#414 (T-R4.31a), run 37641434454 @ `c4579d26`:** job **Web unit (2)**, step
  **Unit tests**, exit code 1: 10 failures in two files,
  `tests/units/builder/deck-frame.test.tsx` (6) and
  `tests/units/builder/settings-dialog-width.test.tsx` (4), 1962 others pass.
  First real error, in the stderr of the test: `Mock response for GET
  /teacher/tests/{id} (200) does not match api/openapi.yaml: (root) must have
  required property 'skills'`. `contractResponse.ts` rejects the fixture, the
  builder shows its load-error card ("Không tải được đề thi.") and every lookup
  (`role=button name "Tên đề thi"`, `/tả thói quen/`, "Cài đặt câu hỏi")
  fails. Cause: the test fixtures for the two new files build a `Test` without
  `skills`, which the merged metadata work (PR #415, commit `20eccba8`) made required in the
  contract (`api/openapi.yaml`, schema `Test`, `required` includes `skills`). Behaviour is not implicated; fix is the
  fixtures (use the shared builder, or add `skills: []`). Failed runs 706 and 714
  on the same branch were not read.
- **#416 (T-R4.28), run 37642168689 @ `5ef93476`:** the run concludes `failure`
  but all 11 jobs it lists succeeded (Plan, Contract, Web checks, Web unit 1 and
  2, E2E, E2E live; Server tests, Server lint, Deck, Sonar skipped by the plan)
  and **no "CI result" job appears** in the run or on the PR's check list; the
  run was last updated at 15:16:44, about five minutes (the job's
  `timeout-minutes`) after the last job finished. No failing step or error text
  exists to quote. Probable cause: the gate job (`result`) did not start or was
  stopped by the platform; not a code failure. Earlier run 711 on the same
  branch passed, and run 697 failed on an older head. Action for the owner:
  "Re-run failed jobs" on run 718 and confirm **CI result** reports; if it does
  not, escalate to the platform engineer (the gate is `scripts/ci/gate.mjs`).
  This conclusion is from the metadata only; I could not read the missing job's
  log.
