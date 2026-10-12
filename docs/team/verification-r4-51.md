# T-R4.51 — browser verification of the teacher console against the deck

The release's whole-matrix check of every `/teacher/*` route (`74-r4.md`, "T-R4.51";
`verification.md` §4). Written 2026-10-12 by `senior_tester`. It records what was measured,
what was not, and what is wrong. Nothing in the product was changed.

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
- **Owner:** frontend (`TeacherLayout`). **Status:** open. **Retest:** `docscroll.mjs` over the 150 cells; zero cells with overflow.

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
| Pager | none drawn | shown ("Rows per page", DG-67 says the API pages; accepted) |

  Columns' x (293, 575, 747, 916.5) and the layout switch equal the deck's; the divergence is in what the cells hold. At 360 the card stacks status, chips and action differently from the deck's, with the same pill and button deltas.
- **Revision and environment:** `86f21905`, dark and light English; light Vietnamese shows the same rows.
- **Owner:** frontend, T-R4.36's lane. **Status:** open. **Retest:** side by side again; the table above all "equal".

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
- **Owner:** frontend. **Status:** open. **Retest:** as above, both modes.

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
- **Owner:** frontend (probably the builder never sets the tail through `CrumbTailContext` and the imports handles carry static crumbs; not read further). **Status:** open.

### QA-51-5 — the Grading heading keeps 1.5, where DG-122 decides 1.25

- **Severity: minor.** The Grading h1 is one of the deck's sixteen headings that inherit 1.5; DG-122 says the product keeps 1.25 on all of them. `GradingPage.tsx` sets `leading-[1.5]`, so it is the one route of the sixteen whose box is 36 px (not 30) and whose content is not 6 px higher. It matches the deck, not the decision. Either the page takes `text-h1` or DG-122 names the exception.
- **Owner:** frontend. **Status:** open.

### QA-51-6 — the editor's link popover overhangs its field

- **Severity: minor.** `out/ed-d1280.json`. The popover is 272 x 122.8 as the deck's and opens at the link button's left edge. The deck's opens 31 px further left at 1280 (30 at 360, 44 at 768 and 1024, equal at 1440), inside the field; the product's right edge is 24 px past the field's at 1280 (the field ends at 1235, the popover at 1259). Size, text and colour equal.
- **Owner:** frontend (`LinkPanel` anchor alignment). **Status:** open.

### QA-51-7 — the seed's processing import held the worker's only slot (found independently; F-65)

- **Severity: major for local verification, not shipped.** `seed/08` wrote the processing import's run `running` with `lease_until = now() + 10 years`. The claim query counts `status = 'running' AND lease_until > now()` against `IMPORT_WORKER_GLOBAL_LIMIT` (1) and the actor limit (1), so a worker at the default limit never claimed any of the teacher's new imports: they stayed "Queued — Waiting for a turn" for 118 s until the lease was cleared in the scratch database. The first two runs of the first timing set were lost to it.
- **Status:** fixed on #518 (`34c475f4`, F-65), merged after the candidate; not retested here. Cleared in the scratch database only, as `status = 'failed'`, before the timing in section 7.

### QA-51-8 — the paste footer still says "about a minute"

- **Severity: minor, follows DG-16.** Section 7 measures 2.3 to 2.4 s. DG-16 and T-R4.57's box say the footer then takes the deck's "Processing takes a few seconds." The product's footer reads "10 questions found. Processing takes about a minute. You can leave this page. Press Ctrl+Enter to start." (`imports.paste.processingHint`); `en.json` and `vi.json` have no "few seconds" key for it. `gaps.md` DG-16 is updated in this PR.
- **Owner:** frontend (the footer sentence, both languages). **Status:** open.

### QA-51-9 — a dev-only console warning on every teacher page

- **Severity: minor.** `Brand.tsx` warns when a mark measures under 24 px; DG-123 builds the sidebar tile at 18 px as drawn, so every teacher page logs it up to four times in the dev server. It trains whoever reads the console to skip warnings. Suppress it for that one call site (or give the call an explicit exemption). **Owner:** frontend. **Status:** open.

### QA-51-10 — the API logs 500 for requests the client cancelled

- **Severity: minor, outside this task, seen in passing.** 474 `status: 500` lines in the API log of this run; 466 of the 473 `ERROR` lines are `err: context canceled` (navigation aborting a request); the other seven are `write failed: ... i/o timeout` to Postgres under a load average of 12. A client abort logged as a 5xx and an `ERROR` will inflate any 5xx alert. **Owner:** backend. **Status:** open, one question for the Tech Lead: is it known.

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
| Deviations fixed in the owning branch or filed; Vite and the API stopped afterwards | Findings are filed here and reported to the Tech Lead. Vite (5273), the API (8280), the worker (8391) and the deck server (5175) are stopped; `qa_r451` is dropped. |

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
