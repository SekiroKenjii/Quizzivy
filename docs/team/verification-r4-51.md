# T-R4.51 — browser verification of the teacher console against the deck

The release's whole-matrix check of every `/teacher/*` route (`74-r4.md`, "T-R4.51";
`verification.md` §4). Written 2026-10-12 by `senior_tester`. It records what was measured,
what was not, and what is wrong. Nothing in the product was changed.

**Second pass, 2026-10-12 (retest at `5ac243ee`): section 14.** Sections 1 to 13 are the first
pass at `86f21905` and stay as written, apart from each finding's status line and one correction
to QA-51-2 (14.4). The matrix of closures is 14.2.

## 1. Verdict

**Fail for "every `/teacher/*` route matches the deck" (the R4 exit criterion).** Three
blocking findings, each owned by the screen's task:

| ID | Severity | One line |
|---|---|---|
| QA-51-1 | major | On ten routes the **document** scrolls as well as `<main>`: with real scrollbars a second, window-level scrollbar appears beside the page's own (up to 4,442 px of overflow). One missing `position: relative`. |
| QA-51-2 | major | The **Imports history** differs from the deck's: page title and breadcrumb, filter and status copy, status tones, row anatomy (101 px rows against 62 px), the row's primary action. |
| QA-51-3 | major | The **new import page** (file and paste modes) differs from the deck's in structure: a back link the deck does not draw, no sub under the heading, the Source switch at full width, the Files card, the title field, the footer. |

What passes, measured: no route scrolls sideways at any of 720 cells; every page's content
width is its handle's, at every width and sidebar state; the shell (sidebar 248 / 60 px,
drawer 272 px, top bar 56 px, `main` padding) equals the deck's at every cell; the teacher
skeleton equals the Splash deck's pixel for pixel; the Imports history switches between
cards and grid exactly where DG-18 says; the content editor's frame, toolbar, modes, switch
band, table row, gap chips and paste dialog equal the deck's in size at five widths. The
cold-worker paste import takes **2.3 to 2.4 s** from Start processing to the Ready card (DG-16:
10 s or less, so the deck's footer sentence ships, QA-51-8).

Not run, needs Thuong: Word and Google Docs clipboards, Firefox and WebKit, the
automatic-numbering paste item (section 9).

## 2. Revision and environment

| | |
|---|---|
| Candidate | `86f21905` (`origin/work/redesign-r4` after #515, T-R4.48). `git status` clean, branch `verify/t-r4-51`. |
| Since the candidate | `origin/work/redesign-r4` moved to `604d172e` while this ran: #513 (live import spec), #517 and the ledger (docs), #518 (seed, F-65), #519 (web minors F-25/26/38/44, none in a file a finding below points at). Findings are against `86f21905`; none was retested at the tip. |
| Product | Vite dev server on `localhost:5273`, cwd `/home/user/wt-r451/web`, started 04:11:23 after the checkout; `curl localhost:5273/src/layouts/TeacherLayout.tsx | grep -c registerContentElement` = 3 (served, not read from disk). Never `localhost:5173`: other lanes may use it. |
| API | Go 1.27.0 build of the candidate (`go build ./cmd/api`, exit 0), port 8280, started 04:11:12, `CORS_ALLOWED_ORIGINS=http://localhost:5273`, import processing on. Restarted three times to clear the sign-in limiter (20 per hour per email); each restart followed the same build. |
| Worker | `cmd/import-worker` built from the candidate (exit 0), idle poll 1 h, woken by the API at `localhost:8391`. No converter (pasted text needs none). |
| Deck | `python3 -m http.server 5175 --directory docs/design/deck`. `unpkg.com` answers 403 through the proxy, so React 18.3.1, react-dom 18.3.1, `@babel/standalone` 7.29.0 and `lucide-static` 0.469.0 (the versions `support.js` pins) were fetched as npm tarballs and served to the page by Playwright route interception; the deck file is untouched. Google Fonts load. `document.fonts.check('600 16px "Be Vietnam Pro"')` was true on every cell, product and deck. |
| Database | scratch `qa_r451` as `quizzivy_migrate`: `goose up` to 106 (exit 0), every `seed/*.sql` in order including `08` and `99-assert` (exit 0), dropped at the end. The dev database was not touched. |
| Browser | Playwright 1.62.1 driving `/opt/pw-browsers/chromium` (Chromium 141), `Asia/Ho_Chi_Minh`, fresh context per sidebar state and theme. Headless hides scrollbars; the scrollbar checks relaunch without `--hide-scrollbars`. |
| Toolchain | Node 22.22.0, pnpm 10.28.0, Go 1.27.0 (`GOTOOLCHAIN`), psql 16.15 against Postgres 18.6. CI uses Node 24 and pnpm 11. |
| Account | `giaovien@quizzivy.com` (Trần Thị Lan). Her stored theme and language override the browser's (`setAccountTheme`), so the sweeps set both through `PATCH /me/preferences` and `PATCH /auth/me`: light and Vietnamese, then dark and English. |

## 3. Method and coverage

Scripts are in the tester's scratchpad
(`/tmp/claude-0/-home-user-Quizzivy/ba2b3d7f-be0f-5c90-95ea-21b8356bbaad/scratchpad/sweep/`, results in
`../out/`); they are not committed. They can be promoted to `web/tests/support/` if T-R4.52 wants
the matrix again.

| Check | Cells | Measured |
|---|---|---|
| Product sweep (`sweep.mjs`) | 30 routes x 6 widths (360, 767, 768, 1024, 1280, 1440) x sidebar key (none, `collapsed`) x theme = **720** | `scrollWidth <= innerWidth`; elements past the content box that no scroller owns; the h1's box and type; `[data-slot=page]` width against its handle (1320, 1080, 860, 720, full) with the sidebar and the padding; sidebar, header and `main` rects; console errors, page errors and 4xx/5xx responses; the drawer opened below 768 (240 cells). Light was swept in Vietnamese, dark in English. |
| Deck shell (`decksweep.mjs`) | 19 deck screens x (360, 767 expanded; 768 to 1440 expanded and collapsed) light; 360 and 1280 dark | the same rects, per deck screen |
| Skeleton (`skeleton.mjs`) | Splash `#skeleton` with `destination` set to `teacher`, 360 and 1280, light and dark, reduced and full motion | every block's rect, gradient, radius, and pixels |
| Imports (`imports.mjs`) | 360, 768, 1024, 1280, 1440 x sidebar, 1280x600 with scrollbars, the container threshold pairs (1263/1264 expanded, 1075/1076 collapsed), light and dark | layout mode, page width, scrollbar, sideways scroll |
| New import (`paste.mjs`, `sidebyside.mjs`) | file mode, paste mode empty and with the deck's sample, 360 and 1280 | every text node's box, type and colour, side by side |
| Editor (`editor.mjs`, `editor2.mjs`) | builder Prompt in 7 states at 1280 and 5 at 360, 768, 1024 and 1440, light and dark; Explanation, gap chip and the review's Question at 1280 | frame, toolbar, mode buttons, band, popover, dialog rects |
| Text anchors (`textcmp.mjs`) | every mapped route at 1280, light and dark, English | position, size, weight and colour of every text node the deck shares |
| Scrollbars (`scrollbars.mjs`, `docscroll.mjs`) | every route at 1280x700, and 1280x900, 768x700, 360x700, 360x900 | `main`'s scrollbar and the document's |
| Sidebar and drawer (`interactions.mjs`) | stored expanded and collapsed; drawer at 360 | see section 8 |

A "match" is the tolerance in `verification.md` §4.5: 0.5 px, exact computed colour in both themes, the DG
register's exceptions only. The `sr-only`-style noise a text anchor can pair wrongly (the same label twice)
was read by eye before anything below was written down.

## 4. Route matrix

Cells are 6 widths x 2 sidebar states x 2 themes. "Deck deltas" counts cells (of 13 compared) where the
shell differs from the deck's by more than 0.6 px. `h1.h` is DG-122 (section 6). `main.w`, `page.w`,
`page.x` and `sidebar.w` on the review are the transient collapse (`handle.sidebar: "collapsed"`, DG-67): the
deck was loaded expanded. The media route's blocked images and the review's 404 on `/source` are the seed's:
its media rows and its pasted import have no object in the store (`seed/README.md`, "Media objects"); the same
review route on an import made through the page has no 404 (rows `importReviewLive`).

| Route | Product URL | Deck page | Handle | Cells | hScroll | Clipped | Width = handle | Settled | Console and network beyond DG-123 | Deck cells | Deck deltas |
|---|---|---|---|---|---|---|---|---|---|---|---|
| dashboard | `/teacher` | dashboard | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| tests | `/teacher/tests` | tests | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| testDetail | `/teacher/tests/{id}` | testDetail | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | none |
| builder | `/teacher/tests/{id}/edit` | builder | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | none |
| builder, five kinds | `/teacher/tests/{id}/edit` (Unit 4) | builder | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | none |
| imports | `/teacher/imports` | imports | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| importNew | `/teacher/imports/new` | importNew | 860 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13, **h1.y 13** |
| importRun (processing) | `/teacher/imports/{id}` | importRun | 720 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 (20 px h1) |
| importFailed | `/teacher/imports/{id}` | importRun | 720 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| importReady (made here) | `/teacher/imports/{id}` | importRun | 720 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| importReview (seed) | `/teacher/imports/{id}/review` | importReview | full | 24 | 0 | 0 | 24/24 | 24/24 | 404 `/source` x8 (seed, above) | 13 | transient collapse x5 |
| importReview (made here) | `/teacher/imports/{id}/review` | importReview | full | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | transient collapse x5 |
| importConfirm | `/teacher/imports/{id}/confirm` | importConfirm | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| importConfirm (made here) | same | importConfirm | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| bank | `/teacher/question-bank` | bank | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| groups | `/teacher/question-bank/groups` | none | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 0 | no deck page |
| groupEditor | `/teacher/question-bank/groups/{id}` | none | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 0 | no deck page |
| questionNew | `/teacher/question-bank/new` | question | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | none |
| question | `/teacher/question-bank/{id}` | question | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | none |
| media | `/teacher/media` | none | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | seed objects absent | 0 | no deck page |
| assignments | `/teacher/assignments` | assignments | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| assignmentNew | `/teacher/assignments/new` | newAssignment | 1080 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| assignment | `/teacher/assignments/{id}` | assignment | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | none |
| assignmentEdit | `/teacher/assignments/{id}/edit` | none | 1080 | 24 | 0 | 0 | 24/24 | 24/24 | none | 0 | no deck page |
| attempt | `/teacher/attempts/{id}` | none | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 0 | no deck page |
| grading | `/teacher/grading` | grading | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | none (QA-51-5) |
| students | `/teacher/students` | students | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| classes | `/teacher/classes` | classes | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | h1.h 13 |
| classDetail | `/teacher/classes/{id}` | classDetail | 1320 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | none |
| settings | `/teacher/settings` | settings | 1080 | 24 | 0 | 0 | 24/24 | 24/24 | none | 13 | none |

`/teacher/assignments/{id}/attempts` is a redirect, not a page. Route ids are the seed's fixed ids
(`…0000008005nn` tests, `…00000080030n` classes, `…00000080070n` assignments, `…00000080080n` imports).

Console: every cell logs the dev-only `Brand: mark at height 18 measures 18px, under the kit's 24px floor
(B-05)` warning up to four times (DG-123 built as drawn; QA-51-9). The cold-load `401 GET /auth/me` is the
single-flight refresh. Nothing else.

### 4.1 Where the shell was compared with the deck, and what it measured

At every cell compared, sidebar 248 px expanded and 60 px collapsed (`quizzivy.sidebar`), header 56 px, `main`
x and width, padding 28 px from 768 and 14 px below, page x and width, and the h1's x and font size equal the
deck's. The exceptions, all listed:

| Delta | Cells | Reason |
|---|---|---|
| h1 box 30 px, deck 36 px (20 px h1: 27 and 30); the content under it 6 px higher | 10 routes (rows `h1.h`) | DG-122, cited and not reopened |
| h1 at y 100 (92 below 768), deck 80 (72) | `importNew`, 13 of 13 | **not covered by any DG:** QA-51-3 |
| `main` 60 px from the left, deck 248 | the review, from 768 | the review starts collapsed for the visit (`handle.sidebar`, DG-67); the stored choice survives (section 8) |
| `main`'s scrollbar 15 px, deck 10 px | every scrolling page, scrollbars shown | the deck styles its own; `verification.md` §4.5 lists it |
| Image tiles blocked, review source 404 | media, seeded review | the seed holds no objects; not a product defect |

### 4.2 Evidence the tasks already hold, cited and not re-derived

Each screen task passed its own deck check; the plan's "As built" lines hold it (`74-r4.md`). What this pass
added is what a single task cannot see: all five widths with the sidebar collapsed on screens whose task
measured two or three (T-R4.40 to T-R4.43, 768 and the collapsed sidebar were left to the matrix), real
scrollbars, and the question bank, which T-R4.32 compared with the deck's markup because the deck would not
render then.

| Route | Task and prior evidence |
|---|---|
| dashboard | T-R4.23, "As built, staged 2026-10-06": ten settled width/theme boards |
| assignments, assignment | T-R4.24 (box ticked), T-R4.25 and T-R4.26 (VER-26) |
| assignmentNew, assignmentEdit | T-R4.27a, 27b (VER-27b) |
| grading | T-R4.28 (VER-416b, `matrix416.mjs`) |
| attempt | T-R4.47 (VER-47); no deck page |
| tests, testDetail | T-R4.29, T-R4.30 (VER-30) |
| builder | T-R4.31a, T-R4.31b (VER-31b), T-R4.63 to T-R4.66 for the editor |
| bank | T-R4.32: compared with markup at 360, 1024, 1280, "the full matrix is the tester's". First rendered comparison is here: row pitch 66 px, aside 220 px and the table's columns equal; the tab pair "Standalone questions / Question groups" (T-R4.34, DG-68) shifts the filters 46 px down; no Import or Shared button (DG-67, release hand-off). |
| groups, groupEditor | T-R4.34 (DG-68); no deck page |
| question, questionNew | T-R4.33 |
| media | T-R4.35 (VER-35); no deck page; image objects absent |
| imports, importNew, importRun, review, confirm | T-R4.36, 37, 38, 39, 57 (VER-39, VER-57). Findings below. |
| students | T-R4.40 (VER-40, three rounds) |
| classes | T-R4.41 (VER-41); "768, 1024 and 1440 are VER-41's" |
| classDetail | T-R4.42 (VER-42): 768 and the collapsed sidebar were left; **looked at here by eye, both match the deck** (members card, columns "Joined via" and "Average", header actions at the same x; the product lists 24 members, the deck's fixture 10) |
| settings | T-R4.43, T-R4.44 (VER-43) |

## 5. Findings

### QA-51-1 — the document scrolls inside the teacher shell, on ten routes

- **Task and criterion:** T-R4.4/T-R4.5 (`TeacherLayout`); T-R4.51 "every route compared"; `verification.md` §4.5 "never scrolls sideways", and a page has one scroller.
- **Severity: major.** With classic scrollbars (Windows, Linux, any browser set to "always show") a second window scrollbar appears next to the page's own and the whole shell can be scrolled out from under the top bar. At 1280x700 it is 11 px on the question editor, 34 px on the builder, 2,095 px on the test detail, 2,192 px on the import confirm page. macOS overlay scrollbars hide it; a mouse wheel over a page that ends scrolls the shell.
- **Steps:** launch Chromium without `--hide-scrollbars`, sign in as `giaovien@quizzivy.com`, open `/teacher/tests/01935000-0000-7000-8000-000000800501` at 1280x700. `document.scrollingElement.scrollHeight - innerHeight` is 2095 and `innerWidth - documentElement.clientWidth` is 15.
- **Expected:** 0, as in the deck (one 10 px scrollbar, on `main`).
- **Actual:** 37 of 150 cells over five viewports. Routes and the worst overflow (px, at the viewport named): testDetail 4,442 (360x700), importConfirm 2,763 (360x700), importConfirm made here 2,233, builder 792 (360x700), builder with five kinds 892 (360x700), question 1,220 (360x700), question new 982 (360x700), importFailed 339 (360x700), settings 263 (768x700), new import 85 (360x700). The routes with none: dashboard, tests, imports, the import's processing and review pages, bank, groups, media, assignments (list, detail, new, edit), attempt, grading, students, classes, class detail.
- **Cause, proven:** the offending nodes are `position: absolute` elements marked `sr-only` (visually hidden inputs and paragraphs) inside `<main>`. `main` is `overflow-y: auto` and `position: static`, so it is not their containing block and their positions below the fold extend the document's scrollable overflow. Setting `main.style.position = 'relative'` in the page took the overflow to 0 on all four cases tried (test detail 1280x700: 2095 to 0; builder 768x700: 728 to 0; builder 1280x700: 34 to 0; question 360x900: 1020 to 0). `TeacherLayout.tsx` line 192 is where `main` is declared.
- **Why the sweep missed it:** headless Chromium hides scrollbars; the 720-cell sweep measured sideways scroll only. The task's own deck checks ran headless too. This is the weakness to fix (section 10).
- **Revision and environment:** `86f21905`, Chromium 141, scrollbars shown, light; theme does not matter.
- **Evidence:** `out/docscroll-light.json`, `out/log-docscroll.txt`, `out/docscroll-testDetail-1280x700.png`, `sweep/rootcause.mjs`.
- **Owner:** frontend (`TeacherLayout`). **Status: closed (#524, retested at `5ac243ee`).** 342 cells (every route, scrollbars shown, 1280x700, 768x700, 360x700, light and dark): document overflow 0 in all, window scrollbar 0 in all, no inner scroller past its content by more than 0.5 px (limit 1.5). Section 14.3. **Retest (first pass):** `docscroll.mjs` over the 150 cells; zero cells with overflow.

### QA-51-2 — the Imports history is not the deck's screen

- **Task and criterion:** T-R4.36 "History header ... Filters ... Cells", T-R4.51 "Imports: the history"; plan boxes list the deck's labels.
- **Severity: major** (a whole screen, the one the exit criterion names first).
- **Steps:** `/teacher/imports` at 1280 expanded, English, against deck `imports` with the same viewport.
- **Expected and actual (1280 expanded, `out/sbs-ihist.json`, `out/sbs-ihist-app.png`, `out/sbs-ihist-deck.png`):**

| Part | Deck | Product |
|---|---|---|
| Page title | "Imports" | "Word and PDF import history" |
| Breadcrumb | Tests › Imports | "Word and PDF import history" alone |
| First filter, failed filter, failed pill | "All", "Couldn't process" | "All statuses", "Processing failed" (the plan's own list reads "All ... Couldn't process") |
| File cell | file name, then creator · when · key (2 lines, row pitch 62) | file name, then the import's title, then creator · when · key (3 lines, row pitch 101) |
| Test column when empty | "—" | "none yet" |
| "Ready for review" pill | info tone (`oklch(0.8 0.08 230)` dark, `0.45 0.09 235` light) | warning tone (`oklch(0.85 0.12 80)`, `0.48 0.11 60`) |
| Row action "Continue review" | outline button, 13 px | filled primary button, 12 px |
| Column heads | 12.5 px, weight 500 | 12.5 px, weight 400 |
| Header buttons | 13.5 px | 13 px |
| Pager | none drawn **(wrong: the deck draws one inside the card, "9 of 9 imports" and "Page 1 of 1"; see 14.4, QA-51-11)** | shown below the card ("Rows per page", DG-67 says the API pages; accepted) |

  Columns' x (293, 575, 747, 916.5) and the layout switch equal the deck's; the divergence is in what the cells hold. At 360 the card stacks status, chips and action differently from the deck's, with the same pill and button deltas.
- **Revision and environment:** `86f21905`, dark and light English; light Vietnamese shows the same rows.
- **Owner:** frontend, T-R4.36's lane. **Status: closed (#523, retested at `5ac243ee`)**, with one residual filed as QA-51-11 (the pager, which the first pass wrongly called "none drawn") and one cosmetic family filed as QA-51-12. Every row of the table above now reads "equal" apart from those; section 14.4. **Retest:** side by side again; the table above all "equal".

### QA-51-3 — the new import page differs from the deck in both Source modes

- **Task and criterion:** T-R4.36 "Upload", T-R4.57; T-R4.51 "`/teacher/imports/new` compared in both Source modes, the paste mode empty and with text".
- **Severity: major** (structure, not tokens).
- **Evidence:** `out/sbs-inew.json` and `out/sbs-inew-*.png` (file mode, 1280), `out/paste-dark.json` and `out/paste-dark-*-*.png` (paste mode, empty and with the sample, 360 and 1280).
- **Actual against the deck, 1280 expanded:**
  - a "Back to imports" link (13 px) above the heading pushes the h1 to y 100 (deck 80; 92 against 72 at 360);
  - no sub under the heading ("Upload the Word or PDF exam and ..." / "Copy a test from Word ..."): the file mode keeps its sentence inside the card as a 13 px line, the paste mode keeps the deck's sentence under the h1 but 16 px lower;
  - the Source switch is the page's full width (860 px, 428 px an option) where the deck's is 344 px (2 x 168 px; DG-21 asks for a minimum, not a stretch), at 12.5 px against 13 px;
  - the Files card: heading "Choose the files to import" (14 px) where the deck has "Files" (15 px) and "DOCX, DOC or PDF · up to 25 MB each"; each slot is one line, "Choose file" and "or drop the file here", where the deck draws a tile, "Drag a file here, or choose one" and a two-line hint; the "Required"/"Optional" tags are 12 px weight 400 and at the right, the deck's 11.5 px weight 500 beside the label;
  - "Test title" is in its own card in the deck; here it sits bare on the page, its hint 12 px against 12.5 px;
  - "How to read the content" is 14 px (deck 15 px) and shows one choice, "Recognise automatically" (the rest are gated, DG-67, accepted);
  - the footer's primary button is weight 500 (deck 600); the hint is 12 px (deck 12.5 px);
  - paste mode: the Test content card, counts, tips and the example are the deck's to the pixel (x, width, text size; the textarea is 294 x 340 at 360 and 822 x 340 at 1280, both) but sit 13 to 14 px lower from the back link; the refusal message reads "Cmd+V" where the deck writes the glyph "⌘V".
- **Owner:** frontend. **Status: closed (#523, retested at `5ac243ee`)**, with QA-51-12 and QA-51-13 filed as residuals (line-heights inside the file slot; the order of the formats list when `.doc` is enabled). Every other difference is a cited DG; section 14.5. **Retest:** as above, both modes.

### QA-51-4 — breadcrumb tails differ from the deck's

- **Severity: minor.**
- **Actual:** header text at 1280, English, from `out/crumbs.json`:

| Route | Deck | Product |
|---|---|---|
| builder (both) | Tests › {test title} | Tests › "Test title" (the label of the title input) |
| imports | Tests › Imports | "Word and PDF import history" |
| processing, ready, failed | Tests › Imports › {file name} | "Word and PDF import history" › "Import" |
| new import | Tests › Imports › New import | Tests › Imports › "Import from Word/PDF" |
| confirm | Tests › Imports › {file name} | Tests › Imports › {title} › "Preview and create" |

  Every other route's crumb and `document.title` equal the deck's (dashboard, tests, test detail, bank, assignments, assignment, grading, students, classes, class detail, settings).
- **Owner:** frontend (probably the builder never sets the tail through `CrumbTailContext` and the imports handles carry static crumbs; not read further). **Status: closed (#524, retested at `5ac243ee`).** Every crumb in the table reads as the deck's, in English and in Vietnamese; section 14.6.

### QA-51-5 — the Grading heading keeps 1.5, where DG-122 decides 1.25

- **Severity: minor.** The Grading h1 is one of the deck's sixteen headings that inherit 1.5; DG-122 says the product keeps 1.25 on all of them. `GradingPage.tsx` sets `leading-[1.5]`, so it is the one route of the sixteen whose box is 36 px (not 30) and whose content is not 6 px higher. It matches the deck, not the decision. Either the page takes `text-h1` or DG-122 names the exception.
- **Owner:** frontend. **Status: closed (#524, retested at `5ac243ee`).** The Grading heading is 24 px with a 30 px line-height and 30 px box, as the other fifteen; section 14.6.

### QA-51-6 — the editor's link popover overhangs its field

- **Severity: minor.** `out/ed-d1280.json`. The popover is 272 x 122.8 as the deck's and opens at the link button's left edge. The deck's opens 31 px further left at 1280 (30 at 360, 44 at 768 and 1024, equal at 1440), inside the field; the product's right edge is 24 px past the field's at 1280 (the field ends at 1235, the popover at 1259). Size, text and colour equal.
- **Owner:** frontend (`LinkPanel` anchor alignment). **Status: closed (#524, retested at `5ac243ee`).** The popover's x, width and height equal the deck's at 360, 768, 1024, 1280 and 1440 in both themes, and it ends inside its field; section 14.6.

### QA-51-7 — the seed's processing import held the worker's only slot (found independently; F-65)

- **Severity: major for local verification, not shipped.** `seed/08` wrote the processing import's run `running` with `lease_until = now() + 10 years`. The claim query counts `status = 'running' AND lease_until > now()` against `IMPORT_WORKER_GLOBAL_LIMIT` (1) and the actor limit (1), so a worker at the default limit never claimed any of the teacher's new imports: they stayed "Queued — Waiting for a turn" for 118 s until the lease was cleared in the scratch database. The first two runs of the first timing set were lost to it.
- **Status:** fixed on #518 (`34c475f4`, F-65), merged after the candidate. **Status: closed (#518, retested at `5ac243ee`)**; section 14.7. Cleared in the scratch database only, as `status = 'failed'`, before the timing in section 7.

### QA-51-8 — the paste footer still says "about a minute"

- **Severity: minor, follows DG-16.** Section 7 measures 2.3 to 2.4 s. DG-16 and T-R4.57's box say the footer then takes the deck's "Processing takes a few seconds." The product's footer reads "10 questions found. Processing takes about a minute. You can leave this page. Press Ctrl+Enter to start." (`imports.paste.processingHint`); `en.json` and `vi.json` have no "few seconds" key for it. `gaps.md` DG-16 is updated in this PR.
- **Owner:** frontend (the footer sentence, both languages). **Status: closed (#524, retested at `5ac243ee`).** The footer reads the deck's sentence in both languages; section 14.6.

### QA-51-9 — a dev-only console warning on every teacher page

- **Severity: minor.** `Brand.tsx` warns when a mark measures under 24 px; DG-123 builds the sidebar tile at 18 px as drawn, so every teacher page logs it up to four times in the dev server. It trains whoever reads the console to skip warnings. Suppress it for that one call site (or give the call an explicit exemption). **Owner:** frontend. **Status: open at `5ac243ee`**: the console warning is still logged, 1,128 times over 240 cells (564 for `mark`, 564 for `markOnDark`). Not on the retest list; unchanged.

### QA-51-10 — the API logs 500 for requests the client cancelled

- **Severity: minor, outside this task, seen in passing.** 474 `status: 500` lines in the API log of this run; 466 of the 473 `ERROR` lines are `err: context canceled` (navigation aborting a request); the other seven are `write failed: ... i/o timeout` to Postgres under a load average of 12. A client abort logged as a 5xx and an `ERROR` will inflate any 5xx alert. **Owner:** backend. **Status: open, partly fixed (#521, retested at `5ac243ee`)**: every `context canceled` is now a 499 at INFO (1,263 of them, no `context canceled` ERROR line), but 18 aborted requests still log `ERROR handler` and a 500: 17 `write: broken pipe` or `connection reset by peer` on the response, one `students: count: timeout: read tcp ... i/o timeout`. Section 14.8.

### Observations, not findings

- The builder and the import review have no `h1` (the title is an input); the deck draws none either.
- DG-122 counts sixteen 24 px headings. The processing page's 20 px heading (`{{ wi.cur.file }}`) is a seventeenth that inherits 1.5; the product's 27 px against the deck's 30 px is the same decision. DG-122's text can name it.
- Reduced motion: the deck's skeleton keeps a static gradient, the product's is the flat `--muted`; the product's choice ("static under `prefers-reduced-motion`", `AGENTS.md`).
- At 1264 px expanded with a 15 px scrollbar the Imports history shows cards where the deck's viewport arithmetic says grid; DG-18 as decided (T-R4.36 records it).

## 6. DG-122, cited

The deck's sixteen 24 px headings that set no line-height are Dashboard, Assignments, Grading, Tests, Question bank, Students, Classes, New assignment, Gradebook, Reports, Attendance, Imports, "Import from Word/PDF", "Paste a test", "Preview and create a draft" and "Shared with me". In the matrix the product's heading is 30 px high where the deck's is 36 px, with equal x, y and font size, on dashboard, tests, imports, import new, confirm, bank, assignments, new assignment, students and classes, at every width and sidebar state; the text anchors below them sit 6 px higher (a histogram of -6 and -6.8 px in `out/tc-*-1280.json`). No gap was opened. The exception is QA-51-5.

## 7. DG-16, the cold-worker timing

- **What:** `/teacher/imports/new?source=paste`, the deck's `WI_PZ_SAMPLE` (912 characters, ten questions, "Unit 5 Quick Check") filled into the box, Start processing, timed from the click to the visible "Ready for review" card on the detail page. The worker is idle (poll interval 1 h, no run for at least 60 s) and woken by the API's `POST /wake`. Machine idle (load average 0.1 to 0.6); the three API calls (create, text source, process) take about 70 ms of the total.
- **Ten clean runs in three sets** (`out/coldtimer.json`, `coldtimer-set1.json`, `coldtimer-set2-partial.json`; the first set's first two runs are discarded, QA-51-7; the second set stopped at the import quota of ten, `IMPORT_ACTOR_COUNT`, and was rerun with it raised in the scratch API): the last set **2386, 2347, 2369, 2332, 2334 ms**; the first set's three valid runs 2383, 2342, 2419 ms; the interrupted set's two 2409, 2371 ms. Range 2.33 to 2.42 s, maximum **2.42 s**.
- **Where the time goes:** the run's `completed_at - created_at` is 76 to 96 ms and the worker logs 67 to 85 ms for the pipeline; the page notices on its next 2 s poll (`IMPORT_POLL_MS`), so about 2.3 s is the poll and the render.
- **Result:** 10 s or less, so the footer takes the deck's sentence (QA-51-8) and DG-16 records the number.
- **What it does not cover:** a stopped Fly machine that must boot before it hears the wake (not reproducible here), the converter and Docker (a pasted text needs neither), and any machine but this 4-core container. `verification.md` §9.3 proposed five runs and a maximum; this is that.

## 8. Shell behaviour checked by hand-driven script

`out/interactions.json`.

| Check | Result |
|---|---|
| Stored expanded: open the review (SPA and cold load) | sidebar 60 px, stored key stays unset; toggled once it is 248 px; leaving returns to 248 px; the key is never written |
| Stored collapsed: the same | 60 px, 248 px for the visit when toggled, back to 60 px on leaving; key stays `collapsed` |
| 360: menu button | `aria-label` "Mở menu" (Vietnamese), `aria-expanded` false then true; the drawer is a `dialog`, 272 px wide (deck 272), 8 destinations, focus inside, Tab stays inside |
| Esc | closes, focus returns to the menu button |
| Choosing a destination | navigates, closes the drawer, `main` takes focus |
| 360 to 1280 to 360 on the new import page | the typed title survives both resizes |

## 9. Done-when checklist

| Done-when | Result |
|---|---|
| The dev seed holds the deck's fixtures (T-R4.51a, #511) | Observed, not audited row by row: seven tests plus 06's, four classes, assignments open, scheduled, closed and draft, a flagged attempt, answers waiting to be graded, four imports one of them pasted and in review, five question kinds in "Unit 4". Loads without error (exit 0) and `99-assert` passes. Defect: the lease (QA-51-7). Left unticked: the box belongs to #511. |
| Skeleton against `Quizzivy Splash.dc.html#skeleton`, `destination` teacher, 360 and 1280, light and dark | **Pass.** 32 and 56 blocks at 360 and 1280, zero geometry deltas in all four combinations and both motion settings; the gradient strings are equal under full motion; the page and sidebar pixels at the probe points equal. The hidden sidebar at 360 is the only frame the product has and the deck does not (a 0x0 box). The product's "collapsed sidebar" skeleton has no deck page. `out/skel-*`. |
| Every route compared at 360, 768, 1024, 1280, 1440, light and dark, sidebar expanded, collapsed and drawer | **Compared, with findings.** Section 4. Not compared: routes with no deck page (groups, group editor, media, assignment edit, attempt): shell only, since there is nothing to compare against. |
| Where a heading sits 6 px lower, DG-122 is cited | **Pass**, section 6. |
| Imports: cards at 360, 768, 1024, grid at 1280, 1440, both sidebar states; 1280 expanded with scrollbars at a short viewport; never sideways | **Pass for layout.** `out/imports-dark.json`, `imports-light.json`: cards at 360, 768, 1024 expanded and collapsed (page 317, 449, 705, 637, 893 px), grid at 1280 and 1440 both ways (961 and 1149 with the 15 px scrollbar at 1280x600, the deck's 966 and 1154); with the 15 px scrollbar shown the page is cards at 1263 and 1264 expanded and at 1075 and 1076 collapsed where the deck's viewport arithmetic gives the grid from 1264 and 1076 (DG-18 as decided; T-R4.36 records it); no sideways scroll in any cell. **Not a pass for the screen:** QA-51-2. |
| `/teacher/imports/new` in both Source modes, paste empty and with text | **Compared**; QA-51-3. |
| Content editor against the deck: Prompt, Explanation, review Question; Rich text and Markdown, Write and Preview, table with its row, a gap, the link popover, the notice band, both dialogs | **Pass in Chromium for the frame and its states; one minor (QA-51-6).** Prompt at 360, 768, 1024, 1280, 1440, light and dark; Explanation and the review's Question at 1280 (frame 628 x 159.3 and 604 x 139.3, toolbar 43 px, footer 30.3 px, equal to the deck's). Frame, toolbar and mode buttons equal to 0.1 px in all 38 state cells (the toolbar is a different element in Markdown write mode, whose header is 43 px in both); the "Switch to Markdown" band is 107.3 px, equal at 768, 1280 and 1440, 20 px shorter at 1024 and 360 (a shorter sentence wraps less); the table row is 626x67, 370x67, 428x67, 296x97 and 786x37 at 1280, 1024, 768, 360 and 1440, the deck's size each time; the gap chip is dashed, 6 px radius, numbered, as drawn. Departures that DG already covers: no Image and Audio buttons (DG-109), no mode switch on the review's Question (DG-114), a longer band sentence without "images and audio" (DG-109, DG-110), the "Uploading to Media…" file path (the product refuses a file with the band "Files can't be added to the text", DG-109), the paste preview dialog is 640 x 579 against the deck's 640 x 373 for the same HTML (not examined further). **Not run:** the Word and Google Docs pastes in Chromium, Firefox and WebKit, and the left-out line from a real picture. |
| Pasting for real; "Paste from clipboard" allowed, refused, empty in Chromium, Firefox, WebKit | **Chromium only:** allowed fills the box (912 characters, as the deck), empty toasts "The clipboard is empty" (as the deck), refused (an injected `NotAllowedError`, because headless Chromium leaves the permission "prompt" and never answers) shows "Your browser blocked the clipboard. Click in the box and press Ctrl+V (Cmd+V on Mac)." (deck: "⌘V"). **Not run:** Firefox, WebKit, Word, Google Docs, and the open item on what the plain-text clipboard keeps of automatic numbering and option letters (O-R4.19). |
| The deck's sample, cold worker, Start to Ready card | **Measured, section 7: 2.3 to 2.4 s.** The footer change is QA-51-8. |
| Live E2E job builds and starts `cmd/import-worker` beside the API and `import.live.spec.ts` takes a synthetic `.docx` to a draft (#139) | **Done by #513**, merged into `work/redesign-r4` at `a598a44d` (2026-10-12 04:18 UTC). Read at the merge: `ci.yml` builds `cmd/import-worker`, waits for "import worker started", sets `IMPORT_CONVERTER_IMAGE`; the spec writes its documents in the test (`support/docx.ts`) and goes Imports, review, confirm, "Tạo bản nháp đề", the builder; CI result and E2E (live API) were `success` on its head `8511a820`. Not run here: the converter image cannot be built in this container. |
| Deviations fixed in the owning branch or filed; Vite and the API stopped afterwards | First pass: filed here as QA-51-1 to QA-51-10, none fixed. Vite (5273), the API (8280), the worker (8391) and the deck server (5175) were stopped and `qa_r451` dropped. **Second pass (section 14): QA-51-1 to QA-51-8 closed; QA-51-9, QA-51-10 (partly), QA-51-11, QA-51-12 and QA-51-13 open, all minor. Not every finding is closed, so the plan's box stays unticked.** |

## 10. Not run, and why

| Not run | Why |
|---|---|
| Firefox, WebKit | only Chromium is installed; `playwright install` is forbidden here |
| Word, Google Docs real clipboards, O-R4.19 | no Office, no Google; needs Thuong or a machine that has them |
| The converter image, `.docx` and `.pdf` through the worker | the image build needs the Alpine mirror, which the proxy refuses (`environment.md`); #513's job covers it |
| States other than the ones above (loading, empty, error, success, disabled), per screen | the tasks' own passes hold them; this pass added the skeleton, the drawer and the editor's |
| Keyboard paths and focus order, other than the shell's | same |
| Media objects: image tiles, audio, `seed/README.md`'s copy step | would overwrite keys other agents' databases share in the one bucket |
| Real scrollbars at every width (`docscroll.mjs` ran at five viewports, light only) | the cause is theme-independent |
| The retest of QA-51-7 at #518, and any finding at `604d172e` | the candidate is `86f21905`; no file a finding points at changed |
| Neon, production, real devices | out of scope; Thuong |

## 11. The weakness that let QA-51-1 through, and one change

Every deck check in the plan, and the 720-cell sweep above until its second pass, ran in headless
Chromium, which hides scrollbars; "never scrolls sideways" is measured as `scrollWidth` and a second
scroller on the page is invisible to it. The change: `docscroll.mjs` (`scrollingElement.scrollHeight
- innerHeight`, scrollbars shown) joins `verification.md` §4.3 as a check on every screen, at 1280x700
and 360x700, and the sweep launches Chromium with `ignoreDefaultArgs: ['--hide-scrollbars']`.

## 12. Evidence

Under `/tmp/claude-0/-home-user-Quizzivy/ba2b3d7f-be0f-5c90-95ea-21b8356bbaad/scratchpad/out/`:
`sweep-{light,dark}-{expanded,collapsed}.jsonl` (720 cells), `deck-{light,dark}.jsonl`,
`skeleton.json`, `skel-*` (PNG and JSON), `imports-{light,dark}.json`, `sbs-*.json`, `sbs-*.png`,
`paste-dark.json`, `ed-*.json`, `ed2-*.json`, `ed*-*-{app,deck}.png`, `tc-{light,dark}-1280.json`,
`crumbs.json`, `docscroll-light.json`, `scrollbars-light.json`, `interactions.json`, `clipboard.json`,
`coldtimer.json` (second set), `coldtimer-set1.json`, `worker.log`, `api.log`.

MinIO left behind (nothing deleted): twelve objects of 912 bytes in the `quizzivy-imports` bucket,
`originals/<import id>/<source id>`, from the pasted imports the timing runs made, 04:56 to 05:14 UTC;
the keys are in `out/minio-keys-imports.txt`. Retention sweeps them with their imports (30 days after a
commit, 60 idle). The `quizzivy-media` bucket was not written.

## 13. Deck comparison (handoff table)

| Screen | Deck page | Widths | Light | Dark | Differences |
|---|---|---|---|---|---|
| Shell, every route | all | 360 and 767 drawer; 768 to 1440 expanded and collapsed | yes (vi) | yes (en) | none beyond section 4.1 |
| Skeleton | Splash `#skeleton`, teacher | 360, 1280 | yes | yes | none |
| Dashboard, tests, test detail, assignments, assignment, new assignment, students, classes, class detail, settings, bank, question | the same names | shell at all; 1280 by text anchors; class detail and bank by eye | yes | yes | DG-122 only (QA-51-5 for grading); the bank's tab pair |
| Builder | builder | shell at all; editor at all five | yes | yes | editor deltas of section 9; QA-51-4 |
| Imports history | imports | all five, both sidebar states, 1280x600 | yes | yes | QA-51-2 |
| New import | importNew | 360, 1280 (and the shell at all) | yes | yes | QA-51-3 |
| Processing, ready, failed | importRun | shell at all; 1280 by eye | yes | yes | DG-67 gated items; "In progress" badge absent; QA-51-4 |
| Review, confirm | importReview, importConfirm | shell at all; the review's Question | yes | yes | DG-147, DG-148, DG-67 |
| Grading | grading | shell at all | yes | yes | QA-51-5 |

## 14. Retest at `5ac243ee` (second pass, 2026-10-12)

Written by `senior_tester`. It retests the findings of section 5 against what merged since
(#518, #521, #522, #523, #524, #525) and closes T-R4.51's measurement. Nothing in the product was
changed. Sections 1 to 13 above are the first pass at `86f21905`.

### 14.1 Revision and environment

| | |
|---|---|
| Candidate | `5ac243ee` (`origin/work/redesign-r4` after #525, which merged develop). Branch `verify/qa-51-retest`, worktree `/home/user/wt-qa51r`, created at `bea02592` and fast-forwarded to the tip before anything ran. `git diff --stat bea02592..5ac243ee -- web api server seed` lists one file, `server/internal/platform/config/tests/deployment_test.go`: no web, contract or seed file moved with #525, so every measurement below is of the tip's product. `git status` clean apart from the untracked `web/pw.qa.config.ts`, which is never committed. |
| Left over from the stopped run | Found and stopped: its Vite (5373, cwd `/home/user/wt-qa51r/web`), API (8380, database `qa_r451r`), import worker (8491) and deck server (5375); `qa_r451r` dropped. Not touched: the other agents' servers (ports 20326, 20826, 21777) and databases (`qa_fixaccess`, `qa_fixaccess_queue`, `qa_rvweb`). |
| Product | Vite dev server `localhost:5373`, cwd `/home/user/wt-qa51r/web`, started 08:45:57, after the merge. Served, not read from disk: `curl localhost:5373/src/layouts/TeacherLayout.tsx \| grep -c "containing block of everything positioned"` = 1. |
| API | Go 1.27.0 build of the candidate (`go build ./cmd/api`, exit 0), port 8380, import processing on, `CORS_ALLOWED_ORIGINS=http://localhost:5373`. First instance without `IMPORT_LEGACY_DOC`; restarted with `IMPORT_LEGACY_DOC=true` for the file-mode comparison (the deck's formats line names `.doc`), and three more times to clear the sign-in limiter (20 per hour per email). Last start 09:47:37. |
| Worker | `cmd/import-worker` built from the candidate (exit 0), idle poll 1 h, woken by the API at `localhost:8491`. No converter. |
| Deck | `python3 -m http.server 5375 --directory docs/design/deck`; React, Babel and lucide fonts vendored to the page by Playwright route interception, as in section 2; `document.fonts.check('600 16px "Be Vietnam Pro"')` true on every cell. |
| Database | scratch `qa_r451t` as `quizzivy_migrate`: `goose up` to 106 (exit 0), every `seed/*.sql` in order including `08` and `99-assert` (exit 0); dropped at the end. Fixtures made on top through the API, in that database only: five student attempts (one per engine fixture), a published test with one eight-option question, an assignment and an attempt on it (the engine's tallest pane), two pasted imports. |
| Browser | Playwright 1.62.1 driving `/opt/pw-browsers/chromium` (Chromium 141.0.7390.37), `Asia/Ho_Chi_Minh`. The document-scroll cells launch it with `ignoreDefaultArgs: ['--hide-scrollbars']`, so a 15 px scrollbar is real. Chromium only. |
| Toolchain | Node 22.22.0, pnpm 10.28.0, Go 1.27.0, psql 16.15 against Postgres 18.6. |
| Accounts | `giaovien@quizzivy.com` (teacher; theme and language set through `PATCH /me/preferences` and `PATCH /auth/me`, light and English, light and Vietnamese, dark and English), `hocvien@quizzivy.com` (student), `thuong@quizzivy.com` (made the engine fixture). |

### 14.2 Closure matrix

| ID | Result | PR | Retested at | Measurement |
|---|---|---|---|---|
| QA-51-1 | **closed** | #524 | `5ac243ee` | 342 cells (30 teacher routes and 11 student routes, 1280x700, 768x700, 360x700, scrollbars shown; light and dark, plus Vietnamese on the teacher routes, plus the eight-option engine pane): document overflow 0, sideways overflow 0, window scrollbar 0 in every cell; 294 inner scrollers measured, the largest `past` 0.5 px (limit 1.5). The control fails as it should (2,300 px, 15 px scrollbar). `document-scroll.spec.ts` passes (10 tests). 14.3. |
| QA-51-2 | **closed**, residuals filed | #523 | `5ac243ee` | Title, crumb, sub (DG-73), filters, pill tones and sizes, row pitch (62 px grid, equal to the deck's), row action, column heads, header buttons equal at 360, 768, 1280, 1440, both sidebar states, light and dark. Residual: the pager (QA-51-11), and the 1 px line-height family (QA-51-12). 14.4. |
| QA-51-3 | **closed**, residuals filed | #523 | `5ac243ee` | Back link gone, sub under the heading, Source switch 344x38 (332x38 at 360) with 168x32 options, Files card, drop tiles, Test title card, option card and footer equal to the deck's in x, width, size, weight and colour in both Source modes, light and dark. Every other difference is a cited DG (14.5). Residuals: QA-51-12 (tile height 1.9 px), QA-51-13 (formats order). DG-181 checked on the page. |
| QA-51-4 | **closed** | #524 | `5ac243ee` | Every crumb of the table equals the deck's, English and Vietnamese. 14.6. |
| QA-51-5 | **closed** | #524 | `5ac243ee` | Grading h1 24 px, line-height 30 px, box 30 px, as the other fifteen (DG-122). 14.6. |
| QA-51-6 | **closed** | #524 | `5ac243ee` | Link popover x, width and height equal the deck's at 360, 768, 1024, 1280, 1440, light and dark; right edge inside the field. `link-popover.spec.ts` passes (3 tests). 14.6. |
| QA-51-7 | **closed** | #518 | `5ac243ee` | The seed's processing run is `queued` with no lease; a new paste import reaches the Ready card in 2.36 to 2.42 s. 14.7. |
| QA-51-8 | **closed** | #524 | `5ac243ee` | Footer reads "10 questions found. Processing takes a few seconds. Press Ctrl+Enter to start." (vi: "Quá trình xử lý chỉ mất vài giây."); x, width, 12.5 px, weight and colour equal the deck's, y within 0.6 px (1.8 px at 768 expanded). |
| QA-51-9 | **open** | | `5ac243ee` | Not on the retest list, unchanged: 1,128 console warnings over 240 cells. |
| QA-51-10 | **open, partly fixed** | #521 | `5ac243ee` | `context canceled` is now a 499 at INFO (1,263 of them, 0 ERROR lines of that kind, against 466 before). 18 aborted requests still log `ERROR handler` and a 500. 14.8. |
| QA-51-11 | **open**, new | | `5ac243ee` | The Imports history pager sits below the card with "Rows per page" and first and last buttons; the deck's is the card's own footer. Minor. 14.9. |
| QA-51-12 | **open**, new | | `5ac243ee` | Text carries the product's 1.45 and 1.4 line-heights where the deck inherits 1.5: each file drop tile is 1.9 px shorter, a row's text sits 1 px higher. Minor. 14.9. |
| QA-51-13 | **open**, new | | `5ac243ee` | With `.doc` enabled the formats read "DOCX, PDF, or DOC" and ".docx, .pdf, .doc", the deck's "DOCX, DOC or PDF" and ".docx, .doc and .pdf". Minor. 14.9. |

No blocker and no major remains. T-R4.51's "Deviations fixed" box stays unticked because QA-51-9, 10, 11, 12 and 13 are open.

### 14.3 QA-51-1: the document scroll

- **What ran.** The probe (`docscroll2.mjs`, promoted to `verification.md` §4.3 as a standing step) over every route of `routes.mjs`
  (30 teacher routes; the seed's ids, plus the import this run made for the ready, review and confirm pages) at 1280x700,
  768x700 and 360x700, light and dark in English and light in Vietnamese on the teacher routes: 123 + 123 + 90 cells; the student routes
  (home, classes, intro for two tests, settings, result, and the engine on five attempts) are in the first two sets; the engine with
  an eight-option single-choice pane, light and dark, 3 + 3 cells. 342 cells in all.
- **Per cell.** `scrollingElement.scrollHeight - innerHeight`, `scrollWidth - innerWidth`, `innerWidth - documentElement.clientWidth`, and for every
  element with `overflow-y: auto|scroll` and `scrollHeight > clientHeight`, the gap between its `scrollHeight` and the bottom of its
  in-flow, visible, unclipped content plus padding (`past`).
- **Result.** All three document figures 0 in 342 of 342 cells. 294 scrollers (`main` on the teacher and student pages, the test detail's and the
  confirm preview's inner panes, the engine's panes); the largest `past` is 0.5 px (the tests list at 360, `main`), under the 1.5 px limit. Paths
  equal the requested ones, so no cell measured a redirect or a sign-in page.
- **The control.** On the test detail at 1280x700, `main.style.position = 'static'` and an `sr-only` box at `top: 3000px`: document overflow 2,300 px and a 15 px window scrollbar, plus an inner scroller with 251 px past its content (a scroller with an absolute `sr-only` child at 300 px). The probe can fail.
  `docs/team/verification.md` §4.3's snippet was extracted from the file and run as written on the test detail at the three viewports (0, 0, 0; control 2,300 and 15).
- **E2E.** `pnpm exec playwright test -c pw.qa.config.ts --project=chromium tests/e2e/document-scroll.spec.ts tests/e2e/link-popover.spec.ts`: 13 passed, exit 0 (the config wraps `playwright.config.ts` with port 4673 and the installed Chromium, and builds the production bundle first).

### 14.4 QA-51-2: the Imports history

Compared with deck `imports`, widths 360, 768, 1280 and 1440, sidebar expanded (all) and collapsed (from 768), light and dark: 28 cells per theme
(`imports2.mjs`), plus `rows.mjs` (anatomy of each status's row) and `hist-styles.mjs` (computed styles of pills, actions, column heads).
The seed holds six imports and the deck nine, so rows are compared by status, not by position.

| Part | Deck | Product at `5ac243ee` | Result |
|---|---|---|---|
| Page title | "Imports" | "Imports", x equal | equal (box 30 against 36 px, DG-122) |
| Breadcrumb | Tests › Imports | Tests › Imports (Vietnamese: Đề thi › Lịch sử nhập) | equal |
| Sub | "Word and PDF files you are turning into tests. ..." | "Word and PDF files and pasted text you are turning into tests. ..." | DG-73 |
| Filters | All, Processing, Ready for review, Couldn't process, Draft created, Cancelled | the same, counts on every option | equal; counts follow the search, DG-17 |
| Status pills | oklch tones per status | background, colour, radius, size and weight equal for Processing, Ready for review, Couldn't process and Draft created (the seed holds no cancelled import, so that pill was not measured) | equal, both themes |
| File cell | name, then creator, when, key (two lines) | the same; row pitch 62 px at 1280 and 1440 against the deck's 62, every status, both sidebar states, both themes | equal |
| Test column empty | "—" | "—" | equal |
| Row action | outline button, 13 px | outline button, same size, border and colours; "Continue review", "View progress", "See what failed", "Open test" | equal |
| Column heads | 12.5 px, weight 500 | 12.5 px, weight 500 | equal |
| Header buttons | 13.5 px | 13.5 px, same x | equal |
| Cards (360, 768, collapsed 768) | stacked card per import | stacked card; "Couldn't process" 143.4 px against 143.4, "Draft created" 142.4 against 143.4 | equal where the content is equal; the 1 px is QA-51-12. Where the deck's fixture row has a longer meta line (Processing and Ready at 768: 160.8 and 159.8 against 143.4) the card is as tall as its text needs. |
| Layout switch | cards below the container width, grid from it | cards at 360 and 768, grid at 1280 and 1440, no sideways scroll in any cell | equal (DG-18) |
| Pager | inside the card: "9 of 9 imports", "Page 1 of 1", previous and next | below the card: "1–6 of 6 imports", "Rows per page", "Page 1 of 1", first, previous, next, last | **differs, no DG: QA-51-11** |

**Correction to the first pass.** QA-51-2's table said the deck draws no pager. It does: `hRange` ("{n} of {n} imports") and a static "Page 1 of 1" with two 32 px buttons, as the card's footer
(`Quizzivy Teacher.dc.html` line 3013 and the `border-top` row under the list). The plan's T-R4.36 text repeats the first pass's mistake ("the deck draws no pager here").

### 14.5 QA-51-3: the new import page

Compared with deck `importNew`, file mode and paste mode (empty, and with the deck's 912-character sample), widths 360, 768, 1280, 1440, expanded and collapsed, light and dark: 21 cells per theme, 42 in all
(`fileiso.mjs`). The deck's prototype chrome and gated items were removed from the deck's DOM before the diff ("Use a sample", "Use a saved profile", "Source conventions", the Word template link), so what is left is
what the product draws that the deck does not, and the other way round. After the 6 px of DG-122 (3 px for the h1's own box), **every matched text node's x, size, weight and colour equals the deck's in all 42 cells**, with four exceptions
(the 360 switch track, the recognition note's line breaks at 360, the tips at 768 collapsed, the option hint at 768 collapsed) listed below, and the vertical
differences are the ones listed here. Text the two pages do not share (the product's privacy and retention lines, the deck's prototype copy) is in the table.

| Difference | Cells | Cited by |
|---|---|---|
| h1 box 30 px, deck 36 px; everything under it 6 px higher | all | DG-122 |
| No "Use a sample" in the file slots or the paste box | file, paste | DG-15 |
| No saved profile, source conventions or template link; the card is shorter | file, paste | DG-67 |
| The privacy note reads "Files and pasted text belong only to your import, not to learner media." and what the server reports; the retention line | all | DG-181 |
| The kind pill "Plain text · mark answers with * or “Answer:”" under typed text | paste with text | DG-12 |
| The example is the fixture's, not the deck's two-question one; its card is taller (+70 px at 1280, +114 at 360) | paste empty | DG-15 |
| Tips in two columns by the card's own width: 768 collapsed (and 624 to 767 as a drawer) | paste, 768 collapsed | DG-142 |
| "Found so far" 34 px lower at 360: the pill line and no "Use a sample" row | paste with text, 360 | DG-12, DG-15 |
| At 360 the two 168 px options (336 px) are wider than the 332 px switch in both pages, so the track scrolls sideways as drawn; with Paste text active the product's track is scrolled 3 px further than the deck's ("Upload a file", "Paste text" at dx −3) | 360, paste | DG-17, DG-21 |
| At 768 collapsed the option hint "Works with most free-form exams..." wraps where the deck's, with its second option removed for this diff, does not: the deck draws "Use a saved profile" beside it in half the row; the product keeps the same half-width for its one option | 768 collapsed, both modes | DG-67 (an effect of the diff, not a defect) |
| Refusal message names "Ctrl+V (Cmd+V on Mac)", the deck "⌘V" | not in a cell (a state) | DG-141 |
| Formats line and list read what the server accepts: "DOCX or PDF" by default (`.doc` needs the converter and `IMPORT_LEGACY_DOC`) | file | the server's `limits`, not a DG: with `.doc` on, QA-51-13 |

Measured equal: no back link (the h1 is at y 80, the deck's 80 plus the 3 px of DG-122), the sub under the heading in file mode (14 px, x 334, 788 px wide), the Source switch 344x38 with two 168x32 options (332x38 at 360), "Files" 15 px 600, "Exam file"/"Answer key file" 13 px 500,
"Required"/"Optional" 11.5 px 500 beside the label, the drop text 13.5 px, hints 12 px, "Choose file" 13 px 500 at the same x, "Test title" in its own card, "How to read the content" 15 px, the footer's buttons 13.5 px (Start 600, 112.3 px wide), the 12.5 px hint, `formatBytes` ("up to 25 MB each").

**DG-181 against the page** (English and Vietnamese, `dg181d.mjs`): the note reads "Where your content goes. Files and pasted text belong only to your import, not to learner media. This server enables intake and document processing." followed by "Original files and the review are kept for 30 days after the test is created and 7 days after cancelling. An import with no changes for 60 days is closed automatically." (Vietnamese: "Nội dung của bạn được xử lý ở đâu. Tệp và văn bản đã dán chỉ thuộc lượt nhập của bạn, không phải tài liệu học viên. Máy chủ đang bật tiếp nhận và xử lý tài liệu. Tệp gốc và bản rà soát được giữ 30 ngày sau khi tạo đề và 7 ngày sau khi huỷ. Lượt nhập không có thay đổi trong 60 ngày sẽ tự đóng."). The retention numbers equal `GET /teacher/imports/capabilities` (30, 7, 60). With the capabilities request held, the page is the skeleton (no note); with it answering 500 the page shows "Could not check whether this server supports Word and PDF import." and Retry, in both languages. The "Checking this server's processing availability." wording of DG-181 is therefore not reachable on a cold load; it is unreachable text rather than a defect.

### 14.6 QA-51-4, 5, 6 and 8

- **QA-51-4.** Header text at 1280 (`crumbs.mjs`, 30 routes, English; Vietnamese for the builder and the five import pages): builder "Tests › {title}"; imports "Tests › Imports"; processing and failed "Tests › Imports › {file name}" ("Grade 9 Mock Test 4.docx", "IELTS Reading Pack B.doc"); ready, review and confirm of a pasted import "Tests › Imports › Pasted text"; new import "Tests › Imports › New import". Vietnamese: "Đề thi › Lịch sử nhập › ...". `document.title` follows ("Imports · Quizzivy"). The deck's tails are file names of its own fixtures; the product's are the import's, which is what the deck's `{file name}` means.
- **QA-51-5.** `GradingPage` h1 24 px, 600, line-height 30 px, box 30 px at y 80, the next element at y 112, equal to the dashboard, assignments, students, classes and tests (the deck's 36 px box and y 118 are DG-122). Light and dark.
- **QA-51-6.** `linkpop.mjs`: popover x/width/height 50/272/122.8 (360), 444/272/122.8 (768), 700/272/122.8 (1024), 956/272/122.8 (1280), 987/272/122.8 (1440), the deck's, in light and dark. Right edges 322, 716, 972, 1228, 1259, all inside the field (329, 723, 979, 1235, 1395).
- **QA-51-8.** In `imports2.mjs` the footer text node is equal in text, x, width, 12.5 px, weight 400 and colour in 14 cells per theme; y is 0.3 to 0.6 px from the deck's, 1.8 px at 768 expanded where it wraps. The strings are `imports.paste.processingHint`: "Processing takes a few seconds." and "Quá trình xử lý chỉ mất vài giây."

### 14.7 QA-51-7 and DG-16

`select ... from app.word_import_runs`: the seeded processing import's run is `queued`, `lease_until` null, `available_at` ten years ahead; the worker is not blocked by it. Two paste imports of the deck's sample on a cold worker (idle, woken by the API): **2,423 and 2,363 ms** from Start processing to the Ready card (DG-16 records 2.33 to 2.42 s; consistent, and not a re-measurement of its ten runs).

### 14.8 QA-51-10: what is fixed and what is not

Log of the API over the retest (four instances, 8,996 `http` lines): 6,349 x 200, 1,263 x 499 at INFO, 18 x 500, the rest 4xx. **No `ERROR` line is a `context canceled`** (466 of 473 were, in the first pass); the 499s come from navigation in the sweeps. Still 500, each with an `ERROR handler` line:

| Error | Count | Where |
|---|---|---|
| `write tcp ...: write: broken pipe` | 9 | `GET /teacher/imports/{id}/review`, `GET /teacher/questions` |
| `write tcp ...: write: connection reset by peer` | 8 | the same |
| `students: count: timeout: read tcp ...: i/o timeout` | 1 | `GET /teacher/students`, `duration_ms: 0` |

- **Steps (reproducible in a minute).** Sign in, then 300 times for each of `GET /teacher/imports/{id}/review` and `GET /teacher/questions?limit=100`: open a socket, set `SO_LINGER` (1, 0), send the request, sleep 0 to 12 ms, close (a reset). 10 of the 600 logged `ERROR handler` and a 500; the other 590 were 499 at INFO or completed. The other eight of the 18 came from the browser sweeps (navigation during a response): 8 of about 1,270 aborted requests, 0.6 %.
- **Expected.** A request whose client has gone is a 499 at INFO, whichever call notices it first.
- **Actual.** `ClientGone` (`httpx/clientgone.go`) answers only when the request's context is already cancelled and the error is a `context.Canceled` or a network timeout. A write to a socket the peer has already reset returns `EPIPE` or `ECONNRESET` before the server has seen the close, so the context is still live and the error is neither; `ResponseErrorHandlerFunc` logs `ERROR handler` and writes a 500. The database read timeout is the same race from the other side (the cancel arrived as a deadline on a connection read, and the repository's wrap, "students: count: timeout: ...", is not an `errors.As`-visible `net.Error`, or the context was not yet cancelled).
- **Impact.** 0.6 % of the browser's aborted requests and 1.7 % of the probe's still count as 5xx and as `ERROR`; any alert on either keeps firing at the rate of navigations. Minor.
- **Owner:** backend. **Suggested direction (not a fix):** treat a failed write of the response (`EPIPE`, `ECONNRESET`, `net.ErrClosed`) as the client's whatever the context says, since the server cannot answer a peer that is gone.

### 14.9 New findings

#### QA-51-11 — the Imports history's pager is not the deck's

- **Task and criterion:** T-R4.36 "History"; T-R4.51 "Imports: the history". **Severity: minor.**
- **Steps:** `/teacher/imports` at 1280 expanded, English, against deck `imports`, scrolled to the end of the list (`pager.mjs`, `out/pager-app.png`, `out/pager-deck.png`).
- **Expected:** the pager is the card's footer, inside its border: a `border-top` row, 10 px 16 px padding, "9 of 9 imports" on the left, "Page 1 of 1" and two 32 px buttons (previous, next) on the right.
- **Actual:** the pager is below the card, outside the border: "1–6 of 6 imports" at x 278, "Rows per page" with a select, "Page 1 of 1", and four buttons (first, previous, next, last). The count reads "1–6 of 6" where the deck's reads "{n} of {n}".
- **Revision and environment:** `5ac243ee`, Chromium, light (dark the same).
- **Evidence:** text anchors in `pager.mjs`; the deck's markup (`hRange`, line 3013). The plan's T-R4.36 says "the deck draws no pager here", which is wrong, so no DG covers the difference.
- **Owner:** frontend. **Status:** open. **Retest:** side by side; the pager inside the card, or a DG that says why not (DG-67 says only that the API pages).

#### QA-51-12 — body text takes the product's line-heights and wrapping, not the deck's

- **Task and criterion:** T-R4.36, T-R4.51 (spacing against the deck, tolerance 0.5 px, `verification.md` §4.5). **Severity: minor.**
- **Steps:** `/teacher/imports/new` at 1280 expanded, file mode, English; the deck's "Use a sample" buttons removed so the tiles wrap equally (`fileiso.mjs`, `tile.mjs`).
- **Expected:** each drop tile 68.3 px high: the drop text 13.5 px at the page's inherited 1.5 (20.25 px), the hint 12 px at 1.5 (18 px).
- **Actual:** 66.4 px: `text-ui` sets 13.5 px at 1.45 (19.575 px) and `text-xs` 12 px at 1.4 (16.8 px) in `FileSlot.tsx`. So the "Answer key file" label is 1.9 px higher, "Test title" 3.8 px higher, and at 360 the stack is 7.4 px shorter by "Test title". The same family: the history's row text sits 1 px higher inside a 62 px row (file name, status pill, "View progress"; the row itself is 62 px as the deck's), "Draft created" cards are 142.4 px against 143.4, the paste footer is 1.8 px lower at 768, and the text in the paste buttons 0.7 px higher (the rects are equal).
- **Second cause, same finding:** the recognition note under "Test title" computes `text-wrap: wrap`, the deck's `text-wrap: pretty`: at 360 the same four lines break differently (the first line 270.2 px against 247.5 px) and the note's height is equal (88.5 px). #524 added `text-pretty` to `PageHead` only.
- **Cause:** `--text-ui--line-height: 1.45` and `--text-meta--line-height: 1.45` (`index.css`) where the deck's text sets none and inherits the root's 1.5, as DG-117, DG-119 and DG-122 record for headings only. No DG covers body text.
- **Evidence:** `out/fileiso-light.json`, `fileiso-dark.json` (`light-expanded-1280-file`), `tile.mjs` output above.
- **Owner:** frontend. **Status:** open. **Retest:** tile 68.3 px and "Test title" at the deck's y minus 6 px, the note's first line 247.5 px at 360, or a DG for the token line-heights and the wrap.

#### QA-51-13 — the formats line reads in another order when `.doc` is enabled

- **Task and criterion:** T-R4.36, QA-51-3's "Files" card. **Severity: minor.**
- **Steps:** start the API with `IMPORT_LEGACY_DOC=true`; `/teacher/imports/new`, file mode, English, 1280.
- **Expected (deck):** "DOCX, DOC or PDF · up to 25 MB each" and "Accepts .docx, .doc and .pdf files up to 25 MB each. Files with a password or macros are not accepted."
- **Actual:** "DOCX, PDF, or DOC · up to 25 MB each" and "Accepts .docx, .pdf, .doc files up to 25 MB each. ...": the order is the server's `limits.formats` (docx, pdf, doc), joined by `Intl.ListFormat` (an Oxford comma in English). Without `.doc` (the default, `.env.example`) the line reads "DOCX or PDF", which is true to the server and not the deck's; no DG says so.
- **Revision and environment:** `5ac243ee`, light and dark, both sidebar states, 360 to 1440.
- **Owner:** frontend. **Status:** open. **Retest:** the deck's order and wording when `.doc` is on; a DG that says the line follows the server (as DG-181 does for the privacy note).

### 14.10 Regression sweep

`sweep.mjs`: 30 routes x 360 and 1280 x sidebar (stored expanded, `collapsed`) x theme = **240 cells**; at 360 each cell also opens the drawer (120 drawers).

- Sideways scroll: 0 of 240. `main` overflow: 0. Elements past the content box that no scroller owns: 0.
- `[data-slot=page]` width equals its handle (1320, 1080, 860, 720 or the full width) in every cell; the h1 count is one except where the first pass recorded none (builder, import review: the title is an input).
- Drawer: 272 px wide in 120 of 120, no sideways scroll with it open.
- Settled in 240 of 240; no redirect; theme and `data-scale="deck"` as expected.
- Console and network beyond DG-123: the seed's 404 on `/source` (the pasted import has no stored object) and the blocked media images (no objects in the bucket), as in the first pass, plus the 401 on `/auth/me` of the cold start and the dev-only Brand warning (QA-51-9).

### 14.11 Not run, and why

| Not run | Why |
|---|---|
| Firefox, WebKit, Word and Google Docs clipboards, O-R4.19 | unchanged from section 10: needs Thuong |
| The 1024 width of the Imports history and the 1263/1264 boundary pairs | the first pass measured them; #523 and #524 did not touch the layout switch, and this pass was asked for 360, 768, 1280 and 1440 |
| A cancelled import's pill (QA-51-2) | the seed has no cancelled import |
| The converter, `.docx` and `.pdf` through the worker | as in section 10 |
| Neon, production, real devices | out of scope |

### 14.12 Process

The first pass called the Imports pager "none drawn" from a 900 px screenshot that cut the deck's list at its ninth row. Rule for the next whole-screen comparison: a screen is compared to its last pixel, so scroll the deck's page to the end (or take the full document height) before writing "not drawn". The comparison script keeps the text-anchor diff, which found the pager only as a text it could not pair.
A second, smaller point: the deck's prototype chrome and the product's gated controls both distort a text-anchor diff, so `fileiso.mjs` removes the DG-listed deck elements first and prints only what is left. That turned "29 deviations" per cell into the short tables above. It belongs in `web/tests/support/` if T-R4.52 wants the matrix again.

### 14.13 Evidence and MinIO

Under `/tmp/claude-0/-home-user-Quizzivy/ba2b3d7f-be0f-5c90-95ea-21b8356bbaad/scratchpad/r3/out/` (scripts in `../sweep/`): `docscroll2-*.json`, `sweep-*-*.jsonl`, `i2-light.json`, `i2-dark.json`, `i2-*-{app,deck}.png`, `fileiso-{light,dark}.json`, `rows-{light,dark}.json`, `hist-styles-*.json`, `linkpop-*.json`, `lp-*.png`, `pager-*.png`, `coldtimer.json`, `e2e-qa.log`, `api*.log`, `worker.log`, `minio-keys-r3.txt`.

MinIO left behind (nothing deleted): 25 objects in the `quizzivy-imports` bucket, for five pasted imports of the deck's sample (each an `originals/<import>/<source>` of 912 bytes and four `artifacts/...` of 56 to 13,100 bytes): three from the stopped run (07:59 to 08:00 UTC, imports `01a12f20-1936-...`, `01a12f20-59fd-...`, `01a12f20-99db-...`, whose database is gone) and two from this run (08:46 and 08:47 UTC, `01a12f4b-2628-...`, `01a12f4b-8d8f-...`). The keys are in `minio-keys-r3.txt`. Retention does not reach the first three, whose rows no longer exist. The `quizzivy-media` bucket was not written.
