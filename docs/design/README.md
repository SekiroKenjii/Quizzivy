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
| `Quizzivy Student.dc.html` | Student | `/app/*` | Home, classes, join dialog, test intro, take test, result, learn, course, lesson, flashcards, grades, messages, this week, settings |
| `Quizzivy Teacher.dc.html` | Teacher (and Admin) | `/teacher/*` | Dashboard, calendar, messages, assignments, grading, classes, students, attendance, tests, shared with me, question bank, media, courses, vocabulary, gradebook, reports, imports, settings |
| `Quizzivy Admin.dc.html` | Admin | `/admin/*` | Overview, users, roles and permissions, all classes, audit log, API reference, system settings |

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
  shareVals`); the last layer wins, so a dialog is built from the layer that overrides the
  `toast*` stub beneath it.
- Breakpoints are measured from the page's own width with a ResizeObserver: `< 768` is mobile,
  `>= 1024` is wide, and most tables drop columns by the content width
  (`width − sidebar − padding`).

**Prototype chrome never ships:** the floating screen-switcher pills, the demo accounts box and
the "Try T6NB-4WLQ" hint on Sign in, the canvas theme buttons, the `frame: mobile` prop, the
Teacher page's "Coming next" screen, `RULES_UNUSED`, and the Word-import "Use a sample" slot.

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

## Updating the deck

A new export from Claude Design (project `49cb45cb-7a21-441e-bb39-4261e0f38372`) replaces the
files in `deck/` byte for byte:

1. Copy the pages, `support.js`, `qz-controls.js`, `github.md`, the SVGs and `brand/` over the
   old ones. Do not import `screenshots/`, `uploads/` or `.thumbnail` (see below).
2. Regenerate the manifest:
   `cd docs/design/deck && find . -type f ! -name MANIFEST.sha256 | sed 's|^\./||' | LC_ALL=C sort | while IFS= read -r f; do sha256sum "$f"; done > MANIFEST.sha256`
3. `node scripts/check-design-deck.mjs` — it checks the manifest, that every page is one
   template and one script, and that every file a page references is in the deck.
4. Add a line to the log below, and say in the PR which screens changed.

## Not imported

- `screenshots/` holds Claude Design's own before-and-after captures at about 924px, several of
  them broken intermediate renders. They are not a reference.
- `uploads/` holds eight red-pen markups on an earlier revision. All eight are resolved in the
  deck as imported:

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

## Log

- 2026-09-26 — First import (T-R0.1), from Thuong's 18:06 export. Teacher page sha256
  `93ded9aa…`. The inventories behind `docs/plan/70-*` were read from the 10:02 export the same
  day; the 18:06 export differs by a few hundred bytes in the Admin and Student pages and about
  3 KB in the Teacher page, so each release re-reads its screens from this deck, not from the
  inventories.

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
