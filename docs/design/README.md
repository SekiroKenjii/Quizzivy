# Design

`deck/` is the product design: seven Claude Design pages that draw every screen of the three
consoles, the front door and the public site. **It is the single source of truth for the UI.**
It replaced the earlier hand-built mockups (`docs/design/mockups`, removed with T-R0.1) and every
rule that came with them. Where the product deliberately departs from the deck, the departure is
listed below, and the reason is in the plan (`docs/plan/70-redesign-overview.md`).

`brand/` is Thuong's brand kit and stays the kit of record for every logo, icon and brand colour.

## Viewing the deck

```bash
python3 -m http.server 5175 --directory docs/design/deck
```

Open <http://localhost:5175/Quizzivy%20Teacher.dc.html> (or any other page). The `design-deck`
entry in `.claude/launch.json` runs the same server.

The pages need the network: `support.js` loads React, ReactDOM and Babel from unpkg, and each
page loads Be Vietnam Pro from Google Fonts and the lucide icon font from unpkg. Opening a page
from the filesystem does not work, because the pages link to each other and to their assets by
relative name.

## The pages

| Page | Audience | Console / route tree | What it draws |
|---|---|---|---|
| `Quizzivy Landing.dc.html` | Anyone | `/` on quizzivy.com | Marketing page, join by code, role tabs, consultation form, FAQ; en + vi copy |
| `Quizzivy Sign in.dc.html` | Signed out | `/login`, `/forgot-password`, `/join`, `/change-password` | Sign in, forgot password, join a class, joined, first sign-in password change |
| `Quizzivy Splash.dc.html` | Everyone | App boot | Loading steps, skeleton hand-off per shell, slow, offline, new version, session expired; en + vi copy |
| `Quizzivy System pages.dc.html` | Everyone | `*`, `/403`, error boundary, maintenance | 404, no access, unexpected error, maintenance |
| `Quizzivy Student.dc.html` | Student | `/app/*` | Home, classes, join dialog, test intro, take test (one data-driven screen that the Teacher page also mounts), result, learn, course, lesson, flashcards, grades, messages, this week, settings |
| `Quizzivy Teacher.dc.html` | Teacher (and Admin) | `/teacher/*` | Dashboard, calendar, messages, assignments, grading, classes, students, attendance, tests, shared with me, question bank, media, courses, vocabulary, gradebook, reports, imports (a Word or PDF file, or pasted text), settings, and the AI assistant (its panel and entry points) |
| `Quizzivy Admin.dc.html` | Admin | `/admin/*` | Overview, users, roles and permissions, all classes, AI credits, audit log, API reference, system settings |

Supporting files: `support.js` is the Claude Design canvas runtime (nothing in it is product
behaviour); `qz-controls.js` draws the select popover and the date and time picker the pages use;
`github.md` is Claude Design's own sync log and its screen-to-repo map; the SVGs are the assets
the pages reference by name. `MANIFEST.sha256` pins every file.

## Reading a page

Each `.dc.html` is one `<x-dc>` template followed by one `<script type="text/x-dc">`:

- The template holds the markup. `data-screen-label` names each screen; `{{ bindings }}`,
  `<sc-if>` and `<sc-for>` carry the logic; `style-hover="…"` is the hover state.
- The script holds the fixtures (which show the data model), the state, every handler and
  `renderVals()`. Most option sets, labels, defaults and breakpoints live here, not in the
  template. The Teacher page builds its screens in layers (`renderValsInner → wire → … →
  shareVals → peVals`); the last layer wins, so a dialog is built from the layer that overrides
  the `toast*` stub beneath it. `peVals` (2026-10-04) is the content editor: it feeds the
  builder's "Prompt", "Explanation" and "Question media", the review's "Question" field and the
  two dialogs they open.
- The Teacher page mounts the Student page three times through `<dc-import name="Quizzivy Student">`
  with the props `exam` and `dark` (the builder's preview, Test detail's preview and the import's
  preview), and `scripts/check-design-deck.mjs` reads the import as a reference to
  `Quizzivy Student.dc.html`. On the Student page `renderVals()` is `renderValsBase()` overlaid by
  `examVals()`, which holds every take-test rule: the sections rail, the sheet, the stimulus pane,
  the gap card, the phone drawer and the recording card.
- Breakpoints are measured from the page's own width with a ResizeObserver: `< 768` is mobile,
  `>= 1024` is wide, and most tables drop columns by the content width
  (`width − sidebar − padding`).

**Prototype chrome never ships:** the floating screen-switcher pills, the demo accounts box and
the "Try T6NB-4WLQ" hint on Sign in, the canvas theme buttons, the `frame: mobile` prop, the
Teacher page's "Coming next" screen, `RULES_UNUSED`, and the import page's "Use a sample" buttons
(one in each file slot and one in the paste box, which loads `WI_PZ_SAMPLE`). Since the fourth
import also `previewExam` and `builderPreviewVals` (the previews' demo wiring), and `sharedVals`'
`pickFile`, which always attaches "part-1-asking-the-way.mp3", and its `+plays || 99`.

A pasted import's numbers are demo data, not behaviour: fixture row `i9` ("Pasted text", 1,186
characters), the counts the prototype copies from its own quick count (`WI_detect`) into the
history row, the stages and the Ready card after Start, beside a hard-coded "1 to confirm" and
"1 note", and the fixed date of the fallback title ("Pasted test · 2 Oct"). The product reads the
counts from the server's review summary and puts the day of the import in the fallback title.

The content editor's engine and data are a prototype too (2026-10-04 export). `PE_LIB` (the
eight Media files, their counts and their "Used in …" and "Shared by …" lines), `B_BLANK_DEF`,
`B_MATCH_DEF`, the sample answer of question 6, the option list every question shares and the
prompts seeded from question titles are demo data. The upload that always succeeds after about
a second, the playback that jumps to 0:34, the name "pasted-image-{n}.png" and the `media:{file
name}` addresses in Markdown are stand-ins. The rich mode runs on `document.execCommand`; the
product's editor is TipTap (`web/src/components/shared/content/editor/`), and the drawing
specifies the controls, not the engine. In the review the prototype saves the editor's plain
text only; the product stores the content.

The Student page's take-test fixtures are demo data too (fourth import): `EXAMS`, `EX_RAW`,
`EX_GROUP`, `EX_MATCH`, `EX_UL`, `EX_SIGN`, `AUD_LEN` and `MAP_CELLS`, including their answer keys
(the seventh tuple entry and `ans`), which a student payload never carries. The 45:00 timer of a
preview, the two toasts after Submit for papers s2 and s9 and the dead branches `hasPaneTabs`,
`dotsMode`, `hasRef` and `isSign` are prototype plumbing, and so are the fixture fields nothing
reads: `q.orig`, `q.stem`, `printed`, `multiSec`, `pauses` and `examBold` (S 1470, on q12). On the
Teacher page the script values no markup reads any more are `wi.preview`, `wi.pvOptCols` (T
7427-7428), `td.preview`, `td.pvFoot` ("Showing 5 of … questions") and `td.pvOptCols` (T
7508-7509). Three slips of the prototype are not to be copied: the arrow keys set the open question
without going through `go()` (S 1616 against 1639), so they neither close the drawer or the sheet
nor pause a recording when the section changes; Esc clears the old `sheet` state, not `exSheet` (S
1530); and in a preview the header still says "All answers saved" and "Saving…" (S 961-963, `saved()`
in `exPick`, S 1617). "S n" and "T n" are lines of the pretty form of the fourth export's pages (see
`docs/plan/74d-d4-deck-update.md`, "Reading the references").

The AI assistant of the v2 export is a prototype too. `aiAsk` calls a browser-side model with inline
prompts and shows the `AI_*_DEMO` arrays when it fails; `AI_USE0` and `aiSpend` fake usage,
`AI_LIMIT` is 50,000, the per-kind costs are placeholders and every run takes at least 3.2 seconds.
The Admin page's `AI_POOL`, `AI_DAY`, `AI_DAYS`, `AI_TEACHERS` and `AI_REQ0` and the attendance
fixtures `ATT_TODAY`, `ATT_ROOM` and `ATT_SES` are demo data. The fixture persona (Giàu Bùi, Thường
Võ, the demo accounts) is not content to copy.

## Where the product departs from the deck

Decided with Thuong on 2026-09-26. The design team has these as requests
(`docs/design/gaps.md`), so later exports should converge.

- **Join codes are always shown in full** to their teacher and to admins (card, class detail,
  admin table, copy link, QR). The deck's "shown only once" and "only the last 4 characters"
  copy is rewritten. Codes keep the §6 alphabet, so the `ABCD-1234` placeholder changes.
- **The public join preview never shows a student count.** It may show schedule and room.
- **Every control has a visible `:focus-visible` ring.** The deck draws none.
- **"Create student accounts" is a permission row** (Admin and Teacher), because the Teacher page
  has "Add student" while the matrix gives "Add and disable users" to Admin only.
- **Assistant** stays hidden from role pickers until the deck draws how an assistant joins a
  class.
- **Emails carry a one-time set-password link, never a temporary password.**
- **The consultation form gets a consent checkbox and a privacy notice.**
- **Media replace and delete follow the publish snapshot:** a replaced file reaches drafts and
  bank questions only, and a file in use cannot be deleted. The deck's copy says otherwise.

Decided on 2026-10-03, with the second import (DG-12 to DG-19 in `docs/design/gaps.md`):

- **A pasted test is plain text.** The paste box keeps no formatting, so its kind pill has one
  state and the "Formatting kept from …" lines do not ship.
- **The paste box's quick count follows the server's recognition rules,** not the prototype's
  `WI_detect`, so the page and the review cannot disagree.
- **Status filters are a group of toggle buttons with counts,** not a `tablist`, and the counts
  follow the search (in the audit log, the date range as well).
- **The Imports history switches between its grid and its cards by a container query** at 960px
  of the list's own width, not by the deck's viewport arithmetic.
- **The To review chips and the Ready card's pill read "{n} need action" for every import,**
  pasted or not (DG-14).

Defaults taken on 2026-10-04, with the third import (DG-108 to DG-116 in `docs/design/gaps.md`).
Thuong has not decided these; the plan builds them until he or the design team answers:

- **The toolbar's "Image" and "Audio" are not built in R4.** A prompt and an explanation hold no
  media in the contract, so a pasted or dropped file is refused with a notice, and audio or an
  image is attached as question media (DG-109).
- **The editing mode is the form the field is stored in.** Rich text is stored content, Markdown
  is the stored Markdown string, and nothing converts without the teacher's "Switch to Markdown"
  or "Apply conversion". The prototype opens every question in Rich text (DG-110).
- **One question editor serves the builder and the bank.** The builder's drawing of the prompt,
  the explanation, the answer areas and the question media is built on both pages; the bank's
  Question editor screen was not redrawn (DG-108).
- **The type menu offers the five types the product stores.** "Matching" and the "Include “Not
  given”" switch arrive with R6 (DG-60, DG-113).
- **"Students can pause" is not built in R4** (D4 reopens it, DG-183); "Allow skipping ahead" and
  "Show transcript after submitting" stay. Audio is MP3 or M4A up to 50 MB and an image JPG, PNG or WebP up to 10 MB;
  the builder's "25 MB", "WAV" and "GIF" do not ship (DG-63, DG-111).
- **"Choose from Media" lists the caller's own files.** Media is not a shared kind, so "files
  other teachers shared with you" and "Shared by …" do not ship (DG-112).
- **A pasted image is left out and counted.** Today a paste that holds an image is refused
  whole (spec §7.1). The converter keeps every other refusal, and the paste preview says how
  many images were left out (DG-115).
- **The review's "Question" editor keeps what the product has:** stored content, gaps for a
  fill-in-the-blank item, a read view where the review is read-only, and no Markdown mode
  (DG-114).

Defaults taken on 2026-10-12, with the fourth import (DG-182 to DG-195 in `docs/design/gaps.md`).
Thuong has not decided these either; D4 (`docs/plan/74d-d4-deck-update.md`) builds them until he or
the design team answers:

- **A recording never locks at its play limit.** The button stays live and the status reports the
  extra plays, as spec §11.4 says (DG-182).
- **"Students can pause" is one boolean, `allowPause`, on by default,** on a question's audio and on
  a group's recording (DG-183).
- **A cloze stays one question per gap.** The engine shows a run of them as one screen (DG-184).
- **Every question has one number,** and the cloze screen reads "Questions 23–27 of {total}" (DG-185).
- **The builder keeps two levels:** the deck's group is the product's section, and shared content
  belongs to a shared-context group inside it (DG-186).
- **The Shared content dialog uses the content editor and the product's rules** (targets, limits,
  both switches); its English says "beside the questions", not "on the left" (DG-187).
- **A preview shows no time and claims no save:** the timer pill reads "––:––" and the save line
  "Answers here are not saved." (DG-188).
- **The frames with no model are not built:** the sign card, the word tiles, the image caption, the
  per-question hint and the paragraph letters (DG-189).
- **The phone drawer gains Esc, a focus move, `inert` and a reduced-motion form** (DG-190).
- **Every move in a paper, by key or by click, goes through one function,** and the rail, the sheet
  and the squares follow the 44px floor below 1024 (DG-191).
- **Section instructions show above every question,** and the builder's sentence follows (DG-192).
- **A start spends a play and a resume none;** unlimited plays read "Play {n}" (DG-193).
- **Matching waits for R6** (DG-194).
- **The next sync reads the contract, the engine and the group editor** (DG-195).

Defaults taken on 2026-10-12, with the v2 export (DG-196 to DG-202):

- **The centre pool has one rule, "Pause AI for everyone".** "Add credits" is the Admin's approval and
  nothing is billed (DG-196).
- **A variant is a new draft test, and the bank clean-up merges nothing** (DG-197).
- **The Attendance sentences wait for R8** (DG-198).
- **The permissions matrix gains "Use AI features" and "Manage AI credits"** (DG-199).
- **Settings › AI assistant gains "Ask for more credits"** (DG-200).
- **The AI panel's off, paused, exhausted, failed and invalid states are built from deck parts,** with
  Esc, a focus trap and focus return (DG-201).
- **"Suggest a score" uses the grading card's own set;** the band form joins R6 (DG-202).

## Updating the deck

A new export from Claude Design (project `49cb45cb-7a21-441e-bb39-4261e0f38372`) replaces the
files in `deck/` byte for byte:

1. Copy the pages, `support.js`, `qz-controls.js`, `github.md`, the SVGs and `brand/` over the
   old ones. Do not import `screenshots/`, `uploads/` or `.thumbnail` (see below).
2. Regenerate the manifest:
   `cd docs/design/deck && find . -type f ! -name MANIFEST.sha256 | sed 's|^\./||' | LC_ALL=C sort | while IFS= read -r f; do sha256sum "$f"; done > MANIFEST.sha256`
3. `node scripts/check-design-deck.mjs` — it checks the manifest, that every page is one
   template and one script, and that every file a page references is in the deck, a
   `<dc-import name="X">` included (it must find `X.dc.html`). A `src` or `href` that a page's
   script builds (its target holds a quote or a `$`) is not read as a file reference.
4. Add a line to the log below, and say in the PR which screens changed.

## Not imported

- `screenshots/` holds Claude Design's own before-and-after captures at about 924px, several of
  them broken intermediate renders. They are not a reference.
- `uploads/` holds eleven red-pen markups on earlier revisions. All eleven are resolved in the deck as
  imported:

  | Markup | Resolved as |
  |---|---|
  | Assignment detail, Settings tab: a cramped key/value table | Grouped sections (test and time, window, integrity, results), locked while live |
  | Test builder: narrow, truncating outline | A draggable "Resize outline" splitter; double-click resets it |
  | Student join dialog: "6-character" copy and `ABC-123` | "8-character" copy, `XXXX-XXXX` |
  | Roles and permissions: the Teacher column | "Edit permissions" scrolls to the matrix and highlights that role's column |
  | Calendar: the sticky day header clipped 08:00 | The first hour is fully visible |
  | Test builder under a very long title | Marquee titles, points never wrap, "Drag questions here" in an empty group |
  | New assignment stepper: truncated summaries | Four steps (test, students, schedule, rules) with marquee summaries |
  | Test builder title row: badges drifted right | Status and save state sit beside the title |
  | Imports history: the action column and the row menu clipped at the card's right edge | No sideways scroll: a grid from 960px of content width, stacked cards below |
  | Imports history: a cramped row of search and six filter chips | Segmented status tabs with counts on the left, the search on the right |
| Student take test, a cloze: the gap's options cut in one row and the footer's count cut | Options wrap in a grid of 132px minimum; the count gives way to a key hint when the rail shows, and is a short form otherwise |

  The fourth export also fixed two defects the drawing shows and the designer did not mark: the
  rail's "Grammar and vocabulary" overlapping its own second line (S 1011), and the header's "All
  answers saved" on two lines (the label is now `white-space: nowrap` and the leave button `flex:
  none`, S 952, 963; the product already truncates the line and pins the button).

## Log

- 2026-09-26 — First import (T-R0.1), from Thuong's 18:06 export. Teacher page sha256
  `93ded9aa…`. The inventories behind `docs/plan/70-*` were read from the 10:02 export the same
  day; the 18:06 export differs by a few hundred bytes in the Admin and Student pages and about
  3 KB in the Teacher page, so each release re-reads its screens from this deck, not from the
  inventories.
- 2026-10-03 — Second import, from Thuong's 09:55 export. Two pages changed: Teacher (sha256
  `12767a0c…`) and Admin (`72d4837a…`); the other thirteen files are byte-identical. Teacher:
  "Paste a test" (a button on the Tests list and on the Imports history, the Source switch
  "Upload a file | Paste text" on the import page, and the pasted variants of processing, the
  Ready card and the review's source pane); the Imports history no longer scrolls sideways (a
  grid from 960px of content width, stacked cards below), and its status filters are segmented
  tabs with counts, with the search on the right. Admin: the Audit log filters are the same
  segmented tabs with counts. The decisions taken with this import are DG-12 to DG-19 in
  `gaps.md`.
- 2026-10-04 — Third import, from Thuong's 01:14 export. Two files changed: the Teacher page
  (sha256 `44342870…`, 658,831 to 781,532 bytes) and `github.md`; the other thirteen are
  byte-identical. Teacher, Test builder: "Prompt" and "Explanation" are a rich content editor
  with a "Rich text | Markdown" switch, a toolbar (marks, heading, lists, "Link", "Image",
  "Audio", "Insert table", "Insert gap"), table tools, a notice band and a "{n} words" footer;
  the answer area has one block for each of six types, with a grading note, and the type menu
  gains "True / False" and "Short answer"; a "Question media" block attaches audio or an image
  ("Plays", "Students can pause", "Transcript", alt text); two dialogs are new, "Preview
  content after pasting" and "Choose from Media". Teacher, Review workspace: the question
  card's "Question" field is the same editor, with the same two dialogs. Nothing else on the
  page moved: the bank's Question editor screen, the builder's outline, title bar and publish
  dialog, the Student preview and the rest of the review are as they were. `github.md` records
  a sync of 2026-10-03 against `develop` and maps the editor to
  `web/src/components/shared/content/editor/` and the question bank's editor components. The
  check script changed with this import: a `src` or `href` built in a page's script is not
  read as a file reference. What the export answers and leaves open is DG-108 to DG-116 in
  `gaps.md`, with notes on DG-35, DG-36, DG-60, DG-63, DG-69, DG-80, DG-83 and DG-90.
- 2026-10-12 — Fourth import (T-D4.1), from the design team's export of 2026-10-04, 17:31, as
  carried by the v2 export below. Three files changed against the third import: the Student page
  (sha256 `0dfd2d54…`, 123,460 to 173,426 bytes), the Teacher page (`a4c08424…`, 781,532 to 794,309
  bytes) and `github.md`; the other twelve were byte-identical. Student, take test: one
  data-driven screen whose layout follows the open question's shared content (a passage, a cloze
  passage or a recording), with the sections rail, the stimulus pane, the phone drawer, the grid
  button and its question sheet, the gap card, matching, the inline blank and a "Student view"
  chip. Teacher, builder: "Shared content" in the group's menu and under its header, with its
  560px dialog; the builder's preview, Test detail's preview and the import's preview now embed
  the Student page. The check script changed with this import: a `<dc-import name="X">` is a
  reference to `X.dc.html`. What the export answers and leaves open is DG-182 to DG-195 in
  `gaps.md`, with notes on DG-36, DG-60, DG-63, DG-68, DG-69, DG-80, DG-111, DG-113, DG-115 and
  DG-116.
- 2026-10-12 — Fifth import (T-D4.1), the v2 export of 2026-10-10, 10:44 (`uiux/update-uiux-v2` @
  `26b60795`), which holds the fourth export in every file it did not touch. Five pages changed:
  Teacher (sha256 `184536f0…`, 794,309 to 907,000 bytes), Admin (`8c043d26…`, 151,390 to 171,480),
  Student (`dc6d6077…`, 173,426 to 173,401, fixtures only), Landing (`52196e86…`, 49,717 to 49,703,
  fixtures only) and Sign in (`15ab7e2b…`, 22,899 to 22,896, fixtures only). The take-test
  engine, the builder's shared content and the three previews are byte-identical to the fourth
  export. New: the AI assistant (the panel and its five kinds, "Suggest a score", "Explain results
  with AI", "Write with AI" and "Suggest wrong options", "Read with AI" on the import page,
  Settings › AI assistant, Admin › AI credits); the assignment detail's progress card; the grading
  pane's heights; the join-code card; sticky side columns and the collapsed sidebar's separator;
  the Attendance page and the class-detail Attendance tab; the Audit log's search and date range;
  the word list's progress card; and the persona renames. The decisions it needs are DG-196 to
  DG-202 in `gaps.md`, with DG-01 answered and notes on DG-22, DG-36, DG-63, DG-71, DG-100 and
  DG-106.
- 2026-10-12 — The approved page-width design (DG-205), from `origin/uiux/update-uiux-v2` at
  `c1472f4c`, in its own folder `page-width-proposal/`: `Page Width Proposal.dc.html`, its
  `support.js` and `github.md`, and the two brand marks it loads from
  `page-width-proposal/web/public/brand/`. The other pages are unchanged. It draws one page frame
  for eight Teacher screens at seven widths, the tokens behind it, the workspace pager and the
  merged import workspace (not yet built). The check script changed with this import: a page's
  references resolve from the page's own folder.

## The brand

`brand/` is Thuong's kit, copied byte for byte from `~/Developer/designs/quizzivy-brand`. The
deck's own SVGs are re-serialised copies of it (the same geometry; only `<path/>` against
`<path></path>` and the trailing newline differ). If the two ever disagree, `brand/` wins.

| | |
|---|---|
| `brand/svg/quizzivy-mark-*.svg` | Symbol alone — `color`, `on-dark`, `black`, `white` |
| `brand/svg/quizzivy-logo-horizontal-*.svg` | Lockup with the wordmark — same four variants |
| `brand/svg/quizzivy-logo-vertical-*.svg` | Stacked lockup — `color`, `on-dark`, `white` |
| `brand/svg/quizzivy-appicon-{light,dark}.svg` | 1024 app icons, corner radius already applied |
| `brand/svg/quizzivy-favicon.svg` | Square favicon; copy to `web/public/favicon.svg` |
| `brand/png/` | Pre-exported PNGs, and `brand/README.md` is the kit's own documentation |

Rules that come from the kit itself: light ground → `color`, dark ground → `on-dark` or `white`,
one-colour printing → `black` / `white`; symbol never below 24px tall, horizontal lockup never
below 120px wide. The wordmark is Quicksand Bold converted to paths — never re-set it in the
interface typeface (Be Vietnam Pro).
