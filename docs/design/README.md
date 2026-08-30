# Design deck

High-fidelity mockups for every screen in the v1 spec, plus the shape the product takes as it
grows into an LMS. Static HTML, no build step, no dependency on the app.

The deck is a **design document that happens to render**. It is not a prototype, it is not
importable, and no file under `web/` reads anything from here. Its job is to settle layout,
density, copy and interaction questions before they are settled by accident during
implementation.

## Viewing it

```bash
python3 -m http.server 5175 --directory docs/design/mockups
```

Then open <http://localhost:5175>. The `design-deck` entry in `.claude/launch.json` runs the
same thing.

Opening `mockups/index.html` from the filesystem also works; only the Inter webfont needs the
network, and there is a system-stack fallback.

## Layout

| | |
|---|---|
| `mockups/index.html` | Entry point — the five sheets, the rules, and the proposals |
| `mockups/sheets/00-foundations.html` | Tokens, type, density, every component and its states |
| `mockups/sheets/10-student.html` | Join, login, home, intro, the test engine, integrity, results, failures |
| `mockups/sheets/20-teacher-authoring.html` | Navigation, dashboard, palette, tests, builder, publish gate, audio, bank, media |
| `mockups/sheets/30-teacher-assign-grade.html` | Assignment creation, monitor, grading, integrity timeline, classes, students |
| `mockups/sheets/40-lms-future.html` | Test → activity, navigation over three releases, course/lesson/vocabulary/gradebook, sequencing |
| `mockups/sheets/50-brand-and-errors.html` | The brand kit placed on this product's surfaces; 404 / unexpected error / no-access on the two-panel login shape |
| `brand/` | **Thuong's brand kit, copied verbatim.** Source of truth for every logo, icon and colour |
| `mockups/assets/brand.js` | The kit's SVGs as an injected sprite, generated from `brand/svg/` — do not hand-edit |
| `mockups/sheets/_head.html`, `_foot.html`, `_sidebar.html` | Templates for a new sheet — `__TITLE__`, `__N0__`…`__N4__` (nav active state), `__A_*__` (sidebar active state) |
| `mockups/assets/kit.css` | The design kit — see below |
| `mockups/assets/icons.js` | Lucide sprite; `assets/README.md` has the regeneration recipe |
| `mockups/check.mjs` | `node docs/design/mockups/check.mjs` — fails on an undefined class or a missing icon |
| `mockups/build-artifact.mjs` | `node docs/design/mockups/build-artifact.mjs out.html` — bundles all five sheets into one self-contained page for sharing |

## The kit

`assets/kit.css` copies the tokens from `web/src/index.css` **verbatim** and names its utilities
after their Tailwind equivalents. Two consequences worth knowing:

- A mockup cannot drift from the app's palette without the copy being noticed. If a token
  changes in `index.css`, change it here in the same commit.
- Markup transfers. `<div class="flex items-center gap-2 text-sm text-muted-foreground">` means
  the same thing in both places, so translating a board into a component is mostly deleting the
  static content, not re-deriving the layout.

The kit is a **subset**, not a Tailwind clone. It has the utilities these screens use and
nothing else; `check.mjs` fails the moment a sheet reaches for one that does not exist, which
is the signal to add it deliberately rather than silently rendering wrong.

Dark mode is present in the kit as a `.dark` block and a toggle in the deck header. It is
**not** v1 scope (§12, §16 P1). It is there because it is the only way to prove the claim that
dark mode "can be added without touching components" — every screen in the deck reads its
colours through tokens, and the toggle demonstrates that no rule needs changing.

## What the deck implements as specified

Every board carries the route it implements. §8 and §9 are covered screen for screen, including
the parts easy to skip: the restricted-review result page, the `timed_out` state, the offline
banner, the session-takeover dialog, `mustChangePassword`, 403, and the error boundary with a
copyable error id.

The §12 guidelines are treated as constraints, not suggestions. No gradients, no backdrop blur,
no glow, no radius above `rounded-lg`, no emoji in chrome, no colour that is not carrying
meaning, no entrance animation. The audio player is monochrome. Admin tables are 40px rows;
the student's question column is 720px with relaxed leading and 44px targets.

## What the deck proposes that the spec does not have

Six additions. Each is small, each has a stated reason on its board, and none of them changes a
data model.

| Proposal | Board | Suggested phase | Note |
|---|---|---|---|
| Command palette (⌘K) | A-02 | 2 | The only navigation model that survives an LMS-sized sidebar. Needs the accent-insensitive search that §13.8 and `pg_trgm` already imply. |
| "Chờ chấm" as a nav item with a count | A-00 | 4 | §8 reaches grading only through a monitor screen. One route; the count is the teacher's daily queue. |
| Grade by question | G-04 | 4 | Same endpoints plus one query — "all pending answers for question X in assignment Y". Decides a rubric once instead of per student. |
| Live "học viên sẽ đọc" preview on the assignment form | G-01 | 3 | Renders the exact sentences the student will see next to the switches that produce them. Makes §10.2 true rather than aspirational. |
| Manual-grading cost estimate | G-01 | 3 | `students × manual questions`, shown at the moment of commitment. |
| Saveable comment snippets | G-03 | 4 | One small table, one chip row. The same four sentences get typed all term. |

Two of these — the palette and grade-by-question — are the ones worth arguing about. The other
four are a few hours each.

## Open questions for Thuong

These came out of drawing the screens and cannot be answered from the spec.

1. **Does the teacher ever grade on a tablet?** §1.1 says desktop/tablet, and the grading
   screen is the only admin screen where 768px is genuinely tight. If tablet grading is real,
   the sample-answer panel needs to collapse rather than sit beside the answer.
2. **Should a student see the class average after grading?** The result page currently shows
   only their own score. A cohort comparison is one number and a real motivational lever, but it
   is also the kind of thing that lands badly in a small class where everyone knows everyone.
3. **What happens to a flagged attempt the teacher decides is fine?** The deck adds "Bỏ đánh
   dấu" and a private note field (G-05). If that clearing should be recorded in the audit log —
   and it probably should — that is a schema question, not a design one.
4. **How does the teacher send the join code in practice?** The deck assumes a projector and a
   Zalo message, which is why the QR is downloadable. If it is usually a printed sheet, the QR
   needs a print layout instead.

## The brand

`docs/design/brand/` is Thuong's kit, copied byte-for-byte from
`~/Developer/designs/quizzivy-brand`. **Nothing in the deck redraws it.** Every logo on every
sheet is a `<use>` of a symbol in `mockups/assets/brand.js`, which is generated from
`brand/svg/` — so a change to the kit reaches the whole deck by regenerating one file, and the
deck can never quietly diverge from the kit.

| | |
|---|---|
| `brand/svg/quizzivy-mark-*.svg` | Symbol alone — `color`, `on-dark`, `black`, `white` |
| `brand/svg/quizzivy-logo-horizontal-*.svg` | Lockup with the wordmark — same four variants |
| `brand/svg/quizzivy-logo-vertical-*.svg` | Stacked lockup — `color`, `on-dark`, `white` |
| `brand/svg/quizzivy-appicon-{light,dark}.svg` | 1024 app icons, corner radius already applied |
| `brand/svg/quizzivy-favicon.svg` | Square favicon; copy to `web/public/favicon.svg` |
| `brand/png/` | Pre-exported PNGs, and `brand/README.md` is the kit's own documentation |

Rules that come from the kit itself: light ground → `color`, dark ground → `on-dark` or
`white`, one-colour printing → `black` / `white`; symbol never below 24px tall, horizontal
lockup never below 120px wide. The wordmark is Quicksand Bold converted to paths — **never
re-set it in Inter**; Inter is the interface's typeface, not the logo's.

One decision the deck surfaces but does not make (board B-04): in the teacher's top bar the
`color` lockup sits about 40px from a green "Đã phát hành" badge, and green means "đúng" inside
this product (§12). The board draws that top bar both ways — `color` per the kit's rule, and
`black` — so the trade-off is visible. Both are the kit's own variants; neither recolours
anything.

## Maintenance

- After changing anything in `brand/svg/`, regenerate the sprite so the deck follows:
  `node docs/design/mockups/build-brand-sprite.mjs docs/design/brand/svg docs/design/mockups/assets/brand.js`
- `node docs/design/mockups/check.mjs` after any edit. It fails on an undefined class or a
  missing icon, which are the two ways a static mockup silently renders wrong.
- New screens go on the sheet for their audience, with an id, a name, and the route. A board
  without a route is a proposal, and should say so.
- If a component is needed that is not on the foundations sheet, add it there first. That is
  what keeps five sheets looking like one product.
