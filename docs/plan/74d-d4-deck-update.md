# D4 — Deck update: the fourth export and v2 (v0.11.0)

The phase that brings the product level with the design team's fourth export (2026-10-04,
17:31) and with the **v2 export** built on it (`uiux/update-uiux-v2` @ `26b60795`, 2026-10-10,
10:44). The fourth export rebuilds the Student take-test screen as one data-driven screen, whose
layout follows the shared content of the open question's group: a passage, a cloze passage or a
recording. It gives the Teacher builder's groups "Shared content", a passage with cloze gaps
bound to questions or a recording with plays, pause and a transcript. It mounts the student's
take-test screen inside the builder's preview, Test detail's preview and the import's preview.

**v2** (Thuong, 2026-10-10: "analyse it, and implement it in the UIUX update phase planned right
after this one") adds the **AI assistant**: seven teacher features (Generate questions, Check this
test, Make a variant, Write an explanation, Suggest wrong options, Suggest a score, Explain results),
"Read with AI" on the import page, a Settings section "AI assistant" with usage and preferences, and an
Admin page "AI credits" with a centre pool, per-teacher allowances, requests and rules. Beside the AI it
redraws four screens R4 builds (the assignment detail's stats as a progress card that filters the
roster, the grading pane's heights, the join-code card, a separator in the collapsed sidebar), makes
every side column sticky, and redraws screens of R5 (the Audit log's search and date range), R8 (the
Attendance page and a class-detail Attendance tab) and R10 (the word list's progress card). Its
inventory is under "What the v2 export changed". Thuong's four AI decisions of 2026-10-10 (D19 to D22
in `70-redesign-overview.md` §1: Anthropic's API with cloud processing on, data minimisation per
feature, a $200 monthly pool that is never overspent without an Admin's approval, built in D4) are
folded into sections F and G below.

D4 is a phase of its own and runs after R4 (Thuong, 2026-10-04). R4 (`74-r4.md`) builds from the
deck of record in `docs/design/deck/`, the third import, and none of its tasks is amended to the
export. Neither export is in the repository until T-D4.1 imports **v2**, which contains the fourth
export byte for byte in every file v2 did not touch.

**What that order costs.** Six R4 tasks build a drawing the export has replaced. D4 reopens each
of them:

| R4 task | What R4 builds, from the deck of record | What the export draws | Reopened by |
|---|---|---|---|
| T-R4.31a | The outline. A shared-context group keeps its row, restyled, with no summary line and no menu | A "Shared content" menu item and a shared-content line under the group's header | T-D4.2a |
| T-R4.34 | Question groups restyled; the gap row is built from primitives, because nothing draws it | The gap row: a dashed chip, a select, "Remove gap" | T-D4.2b |
| T-R4.31b | The Student preview as a 720px dialog with Previous / Next | A full-window overlay: a bar, "Computer \| Phone" and a frame | T-D4.2c |
| T-R4.30 | Test detail's preview as a list, changed questions outlined with "New" / "Changed" chips | A frame; nothing inside it is outlined or chipped | T-D4.2c |
| T-R4.39 | The import's preview as a list with "Showing {k} of {n} questions…" | The same frame, with no count line | T-D4.2c |
| T-R4.65 | "Plays" in the question media block; "Students can pause" is not built | "Plays" on a group's recording too, and "Students can pause" on both | T-D4.15 |

v2 reopens thirteen more (verdicts in "What the v2 export changed"):

| R4 task | What R4 builds, from the fourth export | What v2 draws | Reopened by |
|---|---|---|---|
| T-R4.4 | A sidebar with group labels; nothing between groups when collapsed | A `role="separator"` between groups while labels are hidden (the Admin page draws the same) | T-D4.20 |
| T-R4.25 | Four stat tiles: Submitted, In progress, Average, Flagged | One progress card: "{n} of {total} submitted", a two-segment bar, legend buttons with counts that filter the roster, the average | T-D4.22 |
| T-R4.28 | The grading page grows with the card; the queue is capped on phones only | From 768 the page fills the viewport and the queue and the card scroll inside themselves | T-D4.21 |
| T-R4.30 | The version history `sticky; top: 0` | `top: 16px`, capped at `calc(100vh − 88px)`, scrolling; "Make a variant" in the header | T-D4.20, T-D4.34 |
| T-R4.31a | The outline `align-self: flex-start`; a title bar of Question settings, Versions, Preview, Publish | The outline sticky and scrolling inside itself; "AI check" and "Generate" before Preview | T-D4.20, T-D4.34 |
| T-R4.32 | New question and Share; the filter aside sticky at the top | "Clean up" and "Generate" before Share; the aside at `top: 16px` from 1024 | T-D4.34, T-D4.20 |
| T-R4.33 | The question editor's aside static | The aside sticky from 900; "Write with AI" on Explanation and "Suggest wrong options" beside Add option | T-D4.20, T-D4.35 |
| T-R4.27a | The wizard's summary aside `sticky; top: 0` | `top: 16px`, capped and scrolling from 800 | T-D4.20 |
| T-R4.38 | The review's aside static | Sticky from 1000 | T-D4.20 |
| T-R4.42 | The join-code card: the full code (DG-01), link rows, the paused paragraph, QR and Download | The QR as a 112px button opening a projector dialog, the code at 22px with the link under it, a "Joining paused" chip, Copy code / Copy link / Download QR in a row, two new sentences | T-D4.23 |
| T-R4.43 | Five settings sections; the nav static | The nav sticky from 768; a sixth section, "AI assistant" | T-D4.20, T-D4.38 |
| T-R4.36 | The upload card's one accepts line | A "Read with AI" toggle card and two accepts lines | T-D4.39 |
| T-R4.26 | The Questions tab's item-analysis list | "Explain results with AI" and the insights panel above it | T-D4.37 |

The rework is T-D4.2a (S), T-D4.2b (XS), T-D4.2c (M) and the first bullet of T-D4.15 for the
fourth export, and T-D4.20 (S), T-D4.21 (S), T-D4.22 (M) and T-D4.23 (S) for v2; the AI entry
points on R4's screens arrive with the AI tasks (section G). The builder, a high-risk area, is
opened a second time and compared with the deck again at five widths in two themes. The three
previews are filled twice: with R4's read-only `StudentPreview`, then with the engine (T-D4.16).
From v0.10.0 to D4's release the teacher workspace in production shows the third import's builder
and previews, and the student engine is R3's.

**No fix-only release, so the id T-D4.17 is not used.** The fixes merged into `develop` since
v0.9.0 (#330, #331, #314, #321, #329, #328, #320, the units of #284, #315) reach production with
v0.10.0, because `work/redesign-r4` takes `develop` by sync. R2's contract steps (T-R3.1 to
T-R3.3) run as T-R4.49 if v0.9.1 has not shipped before R4 (`74-r4.md`). Three things the
fixes owe a release go on v0.10.0's checklist (T-R4.52), and the lead carries them to the R4
branch: release notes for the student-facing fixes (answers typed offline, the timer at 00:00,
sign-out, a recording's length, the language of errors); a smoke step for #330, one answer
typed in airplane mode on a phone; and the v0.9.0 items still open with the owner, closed or
carried (the two sign-ins, the short test, the `TEMPORARY` check of
`docs/setup/operations.md`, #194).

**Deliverable:** the take-test engine as the fourth export draws it: a sections rail from 900
(1180 beside shared content), a grid button and a question sheet everywhere else, a drawer for
shared content below 768, a section header above every question, the recording card in the
stimulus pane, a cloze answered on one screen, answer areas in the deck's rows, columns and
fields. A play is spent by a start and never by a resume. A teacher can stop students pausing a
recording (`allowPause`). In the builder, a shared-context group has its menu, its
shared-content line and the Shared content dialog. The three previews are the engine in a
Computer or Phone frame, with no saving, no integrity record and no play counting.

**Deliverable, v2:** the AI assistant behind a module port, metered in credits against a $200
monthly pool that a database constraint keeps from being overspent (section G): a teacher
generates questions from a passage, checks a test, makes a variant, writes an explanation, gets
wrong options, gets a score suggestion and an explanation of a class's results, and imports a
scanned test; every output is a suggestion the teacher accepts or edits, and nothing the AI
returns is saved as a grade, a question or a comment by itself. The teacher sees the month's usage
and sets AI preferences in Settings; an Admin sets allowances, adds credits and decides requests
(through the API and `cmd/maintenance` in D4; the Admin page is R5's T-R5.35: Q28, decided
2026-10-10).
Beside it, the four R4 screens v2 redraws (section F) and every side column sticky at 16px.

**Exit criteria:** every engine frame matches the v2 import at 360, 768, 1024, 1280 and
1440 and at 419 / 420, 899 / 900 and 1179 / 1180, in light and dark (T-D4.12); the builder's
group row, the Shared content dialog and the three previews match it at the five widths, and
every difference left is a row in `docs/design/gaps.md`; the four redrawn R4 screens and the AI
surfaces match it at the five widths; the five canaries are green and
their files are as they were at `v0.10.0`; a draft, a flag set and an open-question id written
by v0.10.0 are read unchanged; the AI operations (T-D4.31, T-D4.32) are the only operations
added, `permissions.golden` changes in T-D4.32's pull request and nowhere else, every
`/teacher/ai/*` operation has its isolation-suite entry, and the open list is unchanged; with
`ANTHROPIC_API_KEY` unset every AI operation answers 501, no AI entry point is drawn, and every
other criterion still passes (the key is not needed in CI); with the key set, a live smoke meters
one generation, one score suggestion and one scanned page and reads them back from the ledger, and
an integration test shows the pool's constraint refusing a reservation that would overspend it;
every CI step of 70 §8 is green, `pnpm e2e:live` included; released as `v0.11.0` (Q29, decided
2026-10-10).

**Depends on:** v0.10.0 released: tagged, and `main` merged back into `develop`. D4 uses what R4
leaves on `develop`: the numeric form of the `viewport()` test helper (T-R4.3a), `useMediaUpload`
(T-R4.35), the content editor's `document` profile with "Insert gap" in its toolbar (T-R4.63),
`AudioPolicyPanel` with "Once | Twice | Unlimited" (T-R4.65), the rebuilt builder (T-R4.31a,
T-R4.31b), Test detail (T-R4.30) and `ImportConfirmPage` (T-R4.39).

The export is the one the three hashes under "What the export changed" name. If the design team
sends a newer export before T-D4.1 runs, it is read against that inventory first and this file
is corrected before a task starts.

**Schema changes** (T-D4.13; numbered at merge, above whatever `develop` holds then; none
rewrites or indexes a populated table, so none is `-- +goose NO TRANSACTION`; two columns are
nullable and two have a constant default, so the v0.10.0 binary keeps inserting during the
rolling deploy without a fill trigger):

| File | Task |
|---|---|
| `NNNNN_add_questions_audio_allow_pause.sql` | T-D4.13 |
| `NNNNN_add_test_version_questions_audio_allow_pause.sql` | T-D4.13 |
| `NNNNN_add_group_recordings_allow_pause.sql` | T-D4.13 |
| `NNNNN_add_test_version_group_recordings_allow_pause.sql` | T-D4.13 |
| `NNNNN_add_ai_permissions.sql` (two rows in `app.permissions`, `ai.use` granted to the Teacher role) | T-D4.32 |
| `NNNNN_create_ai_settings.sql` | T-D4.31 |
| `NNNNN_create_ai_credit_periods.sql` | T-D4.31 |
| `NNNNN_create_ai_teacher_credits.sql` | T-D4.31 |
| `NNNNN_create_ai_reservations.sql` | T-D4.31 |
| `NNNNN_create_ai_usage_ledger.sql` (append-only: no `UPDATE` or `DELETE` for the app role) | T-D4.31 |
| `NNNNN_create_ai_credit_requests.sql` | T-D4.31 |
| `NNNNN_add_word_imports_read_with_ai.sql` (`boolean NOT NULL DEFAULT false`) | T-D4.33 |

The AI tables are new and empty, so none is `-- +goose NO TRANSACTION` and the v0.10.0 binary is
unaffected by them; `read_with_ai` has a constant default. The permission rows must be in the
database before the new binary starts (`core/wiring/access.go` refuses a database whose
`app.permissions` lacks a key the binary knows), which the deploy order already guarantees.

D4 contracts nothing of its own and owes no contract step to a later release. R4's contract
steps stay T-R5.1 (`75-r5.md`), which now follows D4's release.

**Branches.** `work/deck-d4` is cut from `develop` after `v0.10.0` is back-merged. Every task,
the import included, is `feature/t-d4-<nn>-<slug>` off `work/deck-d4` and merges into it;
`develop` is merged into it at least weekly and before every verification run (70 §3). So
`develop` keeps the third import as its deck of record until D4 merges, and neither `develop`
nor production ever holds a deck that is ahead of the engine. An engine fix made on `develop`
during D4 is compared with the deck on `develop`; when a sync brings it into `work/deck-d4`,
the engine task that is open re-checks the frame it touched.

**Order of work.** Two lanes, each serial, each one agent (the team's two-agent setting,
2026-10-10); the server lane stays ahead of the web lane throughout. Where the TL has a third
agent, the split noted at step 6 applies, inside 70 §3's ceiling of four.

| Step | Server lane | Web lane |
|---|---|---|
| 1 | **The import** (T-D4.1) with the R5 to R10 plan corrections (T-D4.19): documents, either agent | T-D4.3, which changes no drawing and does not wait for the import |
| 2 | T-D4.13 `allowPause`, D4's first contract pull request; it unblocks T-D4.9 and T-D4.15 | **The engine chain**: T-D4.4, T-D4.5, T-D4.6, T-D4.7. Every chain task edits `Paper` in `TakeTestPage.tsx`, so no two are open at once; T-D4.6 and T-D4.11 edit `QuestionBody.tsx` and run in the chain's order after T-D4.5 |
| 3 | T-D4.30 the provider adapter and its configuration; T-D4.31 the credits | T-D4.8, T-D4.9 (after T-D4.13), T-D4.10, T-D4.11 |
| 4 | T-D4.32 the AI operations, D4's second contract pull request | **The builder chain**: T-D4.20 (sticky columns, first, so T-D4.2a's comparison sees the scrolling outline), T-D4.2a, T-D4.2b, T-D4.14, T-D4.15 (after T-D4.13) |
| 5 | T-D4.33 "Read with AI" in the import worker | **The previews**: T-D4.2c, then T-D4.16 once T-D4.8 (and T-D4.10, if built) are merged; then T-D4.21, T-D4.22, T-D4.23 |
| 6 | T-D4.40 the documents; the migration rehearsal | **The AI surfaces**: T-D4.34, T-D4.35, T-D4.36, T-D4.37, T-D4.38, T-D4.39. With a third agent, T-D4.34 (`features/ai/` alone) runs beside the other five, which touch one feature folder each |
| 7 | T-D4.12 suites, browser verification and documents (both lanes) | T-D4.18 the release: `work/deck-d4` → `develop` (`--no-ff`), `release/<version>` → `main` with Thuong's go |

The web lane is the long one, about ten L or M tasks in series; the server lane idles after step
6 and can take T-D4.23 and T-D4.21 from the web lane if it is ahead.

**Lanes.**

- Two lanes, each serial. A lane never has two pull requests open on the same file. The engine
  chain, the builder chain, the preview chain and the AI web tasks run one after another on the
  web lane; the server tasks on the server lane.
- One contract pull request at a time. D4 has two: T-D4.13 (with the `StudentSection`
  description that T-D4.5 corrects) and T-D4.32 (the AI operations and the two permission keys;
  `permissions.golden` changes there and nowhere else). T-D4.31's `/admin/ai/*` operations and
  T-D4.33's `readWithAi` field ride in T-D4.32's pull request, so the contract changes twice in
  D4. If another release's contract lane is open on `develop` at the same time, the two take
  turns, and whoever merges second reruns `make gen`.
- Migrations are numbered at the merge into `work/deck-d4` and renumbered at a `develop` sync
  that brings a higher number (70 §3).
- Removed locale keys and `make gen` output are their own commits. A behavioural change never
  rides in a refactor: T-D4.3 and T-D4.20 hold nothing else.
- The AI tasks never need `ANTHROPIC_API_KEY` to pass CI: the adapter is nil without it and every
  suite asserts the 501 path too. A task that calls the provider for real does so in a live smoke
  on the builder's machine, with the key in `.env` and never in a test fixture or a log.

**High risk.** `features/take-test/`, `features/integrity/`, `features/media/`, anything that
touches `attempts` or `test_versions`, and the builder's autosave (AGENTS.md, "High-risk
areas"). Every task that touches one runs these on its branch point and on its last commit,
and puts both runs in its pull request:

- the five canaries: `publish_snapshot_test.go`, `audio-player.test.tsx`,
  `attempts/application/tests/events_test.go`, `client.refresh.test.ts`,
  `router-chunks.test.ts`. None is edited by any task in this file;
- the engine tasks: `pnpm test:unit tests/units/take-test tests/units/integrity
  tests/units/media`. The draft store's suites (`draft.test.ts`, `draft-compat.test.ts`,
  `stranded-draft.test.ts`) and integrity capture's (`tests/units/integrity/**`,
  `leave-events.test.tsx`) pass with no edit;
- `AudioPlayer`'s same-tick play: `toggle()` calls `element.play()` first, with nothing awaited
  or set before it. T-D4.8 and T-D4.9 keep the order, and `audio-player.test.tsx` passes
  untouched;
- autosave's two flushes (on unmount and before publish): `builder/autosave-unmount.test.tsx`,
  `builder/autosave-flush.test.tsx`, `tests/units/question-groups/*` and
  `builder-group-recovery.spec.ts` for T-D4.2a, T-D4.2b, T-D4.2c, T-D4.14 and T-D4.15;
- the server task: `make test-api-integration`, which holds the two server canaries and the
  two lock tests of `questions` and `media` (all four carry the `integration` tag, so
  `make test-api-unit` runs none of them), and the Go unit tests of `questions`, `tests`,
  `attempts` and `media`.

**Reading the references.** "S 1631" and "T 6986" are line numbers in the *pretty* form of the
fourth export's Student and Teacher pages, the page with a newline between every two tags; the
v2 inventory's "T 9696" and "A 1821" are the same form of the v2 Teacher and Admin pages. After
T-D4.1 a builder makes it with
`perl -0pe 's/>\s*</>\n</g' "docs/design/deck/Quizzivy Student.dc.html" > student.pretty.html`
(and the same for the Teacher and Admin pages); the result has 1,848, 9,781 and 1,960 lines. The
fourth export's Teacher references ("T n" in sections 1 to 6) were taken on its 8,675-line form:
v2 inserts above most of them, so a builder finds each by the identifier beside it, or adds the
offsets the v2 hunk map gives (none before T 83; +4 from T 87; +20 from T 607; +60 from T 742;
+101 from T 848; +113 from T 1512; +424 from T 4609; +428 from T 5781; +633 from T 6112; +806 from
T 6777; +1,106 from T 8412). Each reference also
names its place by something a reformat keeps: the script function, the element's id, class or
ARIA name, or the text it shows. Code paths are under `web/src/` unless they start with
`server/`, `api/`, `migrations/` or `docs/`. Code and contract line numbers are those of
`develop` at `fd9c763a` (v0.9.0 and the fixes after it). Two later fixes already move some:
since #315's, `api/openapi.yaml` is one line shorter past line 492, and since #320's
(`49c9f0e1`), `TakeTestPage.tsx` is up to twenty lines longer past line 45. R4 moves more, so
each number is quoted beside an identifier. R4's task ids and their text are those of
`work/redesign-r4`, where T-R4.0 corrected `74-r4.md` (T-R4.1a, T-R4.2a and T-R4.3a are
sub-PRs there); they reach `develop` with v0.10.0.

**Two fields a task here may carry** beyond the form of `74-r4.md`. "Reopens" names the R4
task whose result the task changes. "Frozen" lists what the task leaves exactly as it is.

**The gap rows.** T-D4.1 opens fourteen rows in `docs/design/gaps.md`. Their ids are the next
free ones when it runs: `develop` ends at DG-119 today, the R4 branch holds DG-122 and reserves
DG-120 and DG-121, and R4 may open more. This file calls them **DG+1** to **DG+14** (DG+1 is
the first free id), and T-D4.1 replaces each label here with its id.

**If Thuong allows the documents to go ahead of R4.** The lead has asked whether the import and
a documents-only amendment of the six R4 tasks may still happen before those tasks are built.
This file is written for "no". On "yes" every build task still runs after v0.10.0, and these
change:

- **The import goes to `develop` at once.** T-D4.1 and T-D4.19 depend on nothing and their pull
  requests target `develop`. With them change T-D4.1's "Depends on" and its last bullet, the
  "Branches" paragraph, the decision "The import lands on `work/deck-d4`", section A's
  opening paragraph and step 1 of "Order of work". The gap rows take their ids on that day, so
  the `DG+n` labels, "The gap rows" and fact 7 go.
- **A fifteenth gap row returns**, for the weeks in which the deck is ahead of the engine: an
  engine change is compared with the Student page at tag `v0.9.0` until the engine chain
  ships. It returns with its bullet in AGENTS.md.
- **The six tasks in `74-r4.md` are amended**, with what T-D4.2a, T-D4.2b, T-D4.2c and the
  first bullet of T-D4.15 build, written as a first build: no bullet then names what R4 left,
  what a frame stands "in place of", or a key that is removed. Those three tasks and that
  bullet leave this file. With them go the table and the paragraph under "What that order
  costs"; the group's row and the previews' frame in "Deliverable" and "Exit criteria"; the
  verdict "reopens R4" and the "Task" entries of S-32, B-02, B-03, B-07, B-09, B-13, T-01
  and T-02, and B-15's "Task" entry; the "Built by" entries of DG+5, DG+6 and DG+7; the three
  tasks' names in Q8, Q11 and Q26; and their entries in step 4 of "Order of work", in
  "Lanes", in the high-risk rules and in the release checklist.
- **T-D4.15** loses its "Reopens" line and keeps the pause switch. **T-D4.14** depends on
  T-R4.31a and T-R4.34, and **T-D4.16** on T-R4.31b, T-R4.30 and T-R4.39.
- **T-D4.12** adds no closing lines to `74-r4.md`. T-R4.51 compares the group's row, the gap
  row and the previews' bar and frame; T-D4.12 keeps the Shared content dialog, the pause
  switch and the engine in the frame.
- **Outside this file:** AGENTS.md's bullet on the fourth export and ordering rule 10 of
  `70-redesign-overview.md` are reworded. Both say today that the export is imported after
  v0.10.0 and that no R4 task is amended.

**Decisions taken in this file, worth a second look.** Each is a default, with its question
under "Open items".

- **A recording never locks at its play limit** (Q3, DG+1). The export locks it. Spec §11.4,
  the contract and a canary case say the opposite, and the teacher judges; the app reports.
  The card is built; at the limit the button stays live.
- **"Students can pause" is one boolean, `allowPause`, on by default** (Q4, DG+2), on a
  question's audio and on a group's recording. An absent field on update keeps the stored
  value, so an old tab cannot switch pausing back on.
- **A start spends a play; a resume spends none** (Q5, DG+12). Today every press of play
  counts. A play in progress is remembered while the tab lives, and a place that cannot be put
  back costs a play.
- **A cloze stays one question per gap** (Q6, DG+3). The engine shows a run of gap-bound
  members as one screen. Numbers, squares, flags, points, grading and the result do not move.
- **The builder keeps two levels** (Q8, DG+5). The deck's group is the product's section;
  shared content belongs to a shared-context group inside it.
- **The Shared content dialog has no writer of its own** (T-D4.14). It saves through the
  group's one editor, so the revision, the recovery draft and the conflict message keep one
  owner.
- **A preview shows no time and claims no save** (Q11, DG+7). A test has no time limit until
  it is assigned, so the deck's 45:00 has nothing to read.
- **The rail returns and R3's footer strip goes** (Q12), as drawn. It reverses T-R3.9c.
- **The import lands on `work/deck-d4`, not on `develop`.** The deck of record and the engine
  reach `develop` together.

Taken with v2 (2026-10-10), each under "Open items" too. Thuong confirmed Q28 to Q34 on
2026-10-10 ("I agree with the proposals"), so the eight below are decisions, not defaults; Q35
stays open for two items only (the students' sentence and the provider account's retention):

- **T-D4.1 imports v2, not the fourth export** (Thuong's request). v2 holds the fourth export byte
  for byte in the files it did not touch, so the fourth export's inventory stands and the v2
  inventory adds to it.
- **One credit is $0.001 of provider list price**, so the $200 pool (D21) is 200,000 credits, the
  deck's `AI_POOL`. Credits are computed from the response's reported usage with the tier's
  prices from configuration, rounded up. The deck's example costs are placeholders.
- **The default allowance is 20,000 credits a month** (Q30), the deck's lowest rule value: ten
  teachers fill the pool exactly, and the centre has fewer.
- **The pool's rule has one value: pause everyone** (Q31, DG+15). The deck's "Allow 10% extra"
  and "billed on the next invoice" contradict D21 and the spec's non-goal on payments; "Add
  credits" is the approval and the product bills nobody.
- **A variant is a new draft test, never the test's draft** (Q32, DG+16): a test has one draft
  and a variant must not overwrite unpublished edits.
- **The bank clean-up merges nothing** (Q32, DG+16): "Merge into one" has no operation and
  crosses the publish snapshot; near-duplicates are listed, tags are applied.
- **"Let AI read student answers" stays a per-teacher switch, on by default** (Q33), gating the
  two features that send answer text; no feature sends a name, email or id (D20).
- **Buttons stay visible and disabled when the teacher is paused or out of credits**, with the
  reason, and are hidden only when the key is unset (Q34).
- **The Admin page waits for R5; D4 ships its operations and a maintenance command** (Q28).
- **D4 is v0.11.0** (Q29): a release with a module, nine operations and twelve migrations is a
  minor by 70 §3's own rule.

---

## What the export changed

Read against the deck of record (the third import) and `develop` at `fd9c763a`. Three files
changed; twelve are byte-identical to `MANIFEST.sha256`.

| File | sha256 (new) | Size |
|---|---|---|
| `Quizzivy Student.dc.html` | `0dfd2d54b9bc874d8acd7376bb3cbd0661a8f6e701b87db992fe235dbae13575` | 123,460 to 173,426 bytes |
| `Quizzivy Teacher.dc.html` | `a4c08424f0e0b95d7989ff850f2dc03d5edb1ac2d830e2b2209064800e247cee` | 781,532 to 794,309 bytes |
| `github.md` | `bf0598f3ed26bdb73c29c1feb86dbdb0832c9bce6c2ffd3d8c3171a3d3973a62` | 3,547 to 3,433 bytes |

The Student page has seven changed hunks, all in the take-test screen and its fixtures. The
Teacher page has fifteen, all in the builder's outline group, its group dialog, the three
previews and their script. The inventory is 65 rows in four tables and fifteen facts the tasks
rest on.

Verdicts: **rework** (code that shipped in R3 changes), **reopens R4** (R4 builds the deck of
record's drawing and a D4 task redoes it), **new** (work no release has), **R6** (belongs to a
later release that already plans it), **contradicts** (the drawing goes against a decision),
**matches**, **nothing** (prototype plumbing, or nothing to build). "Product today" is the code
at `fd9c763a`; for a teacher screen it is what R4 starts from. The last column names the D4
task that builds the row, or the gap row that records why nothing is built.

### Facts the tasks rest on

1. **The play-limit lock contradicts a canary, not only the spec.**
   `web/tests/units/media/audio-player.test.tsx:105-112` is the case "still plays when the hint
   says none are left", under the comment "Exhausted is a number, not a disabled button." That
   file is one of the five canaries. The lock as drawn (S 1677-1681) cannot be built without
   editing a canary, which AGENTS.md forbids. It needs the owner, not a default.
2. **"Shared content saved on a section turns its questions into group members" is not
   buildable.** A member is owned by its group (`questions.context_group_id`,
   `migrations/00035`; "Members belong to one group in one section", `api/openapi.yaml`
   2276-2283). A section's standalone question is a bank reference. No operation turns one
   into the other. Two levels stay.
3. **"Partly" is a display state only.** `take-test/answered.ts` must keep returning a boolean
   with today's rule, because the server counts with the same rule in SQL
   (`StudentAssignmentCard.liveAnsweredCount`, T-R3.4, `student_test.go` runs
   `answered.test.ts`'s cases). The deck's "one pick of two is partly" needs R6's
   `selectCount`; until then only a fill-in with some blanks typed is partly.
4. **Counting every resume as a play is a defect today, with or without the deck.**
   `features/media/components/AudioPlayer.tsx:108-110` calls `onPlay` on every start, and both
   hosts count it. With `maxPlays` 2, one pause and resume reads "Played 2/2 times".
5. **The rail comes and goes between questions.** `rail = !isMobile && w >= (stim ? 1180 : 900)`
   (S 1631) is evaluated for the open question, so between 900 and 1179 the rail shows on a
   question without shared content and gives way to the grid button on one with it. It is
   drawn, and it is a layout change inside one paper.
6. **Three thresholds are new to the engine: 420, 900 and 1180** (and 1000 for R6's matching).
   At `fd9c763a`, `web/tests/support/viewport.ts` answers every `min-width` query alike. Engine
   unit tests need the numeric form T-R4.3a adds, which is on `develop` from v0.10.0.
7. **Gap ids are taken when T-D4.1 runs.** `develop` ends at DG-119 today; `work/redesign-r4`
   holds DG-122 and reserves DG-120 and DG-121 for T-R4.2a and T-R4.1a. DG+1 is the first id
   free after R4.
8. **Migration numbers are taken at merge.** `develop` ends at 00079 today. T-R3.1 and T-R3.2
   ("numbers from 00080", `73-r3.md`), R4's migrations and this export's `allowPause` all
   number above it, in the order they merge.
9. **The drawing shows a third defect the designer did not mark:** in the rail, "Grammar and
   vocabulary" wraps into its own second line and overlaps it. The export fixes it too (the
   title is `flex:1; min-width:0; line-height:1.35` and prints the part after " · ").
10. **One open item of `76-r6.md` is stale.** "True / false answers already marked 0":
    `server/internal/modules/attempts/domain/grading.go:100-110` already reads `optionIds`.
11. **A recording never comes without a material.** The server accepts a recording only when
    its asset is an audio node in one of the group's materials, and refuses an audio node that
    has no recording (`server/internal/modules/tests/domain/group_validation.go:170-177`, 235,
    243-246; `api/openapi.yaml` 2376-2381: "One explicit playback binding per audio asset used
    in the group's materials"). A listening group therefore reaches the student with
    `stimuli.length >= 1` (the shape is `web/tests/support/groupPreview.ts:41-64`), and
    `hasPassage` is already true for it. The "recording-only group with no left pane" exists
    only in a unit fixture the server would refuse (`tests/units/take-test/panes.test.tsx:93-100`,
    `stimuli: []`). What the deck calls an audio stimulus is, in the product, a material whose
    content is one audio node.
12. **There is one `AudioPolicy`, and a group has one shape for reading and writing.** It is
    referenced by `AdminQuestion.audio` (2031), `StudentQuestion.audio` (2141),
    `StudentGroupRecording.policy` (2175), `GroupRecording.policy` (2385) and
    `QuestionInput.audio` (3124); `QuestionGroupBundle` is both the response (`StoredQuestionGroup`)
    and the request (`GroupCreateInput`, `GroupUpdateInput`). A field cannot be "required in
    responses, optional in requests" without a second schema for the student shapes or a split
    of the bundle. And both update operations replace the whole object, so what an absent
    field means on update decides whether an old tab can undo a teacher's setting.
13. **A test has no time limit and no integrity policy.** Both are the assignment's
    (`AssignmentInput.durationMinutes`, 3175, 3195; `IntegrityPolicy`, 2615-2637);
    `previewTest` returns `version`, `questions`, `sections` and `groups` only (4391-4398).
    The deck's preview timer (45:00) has nothing to read in any of the three hosts.
14. **Every member of a group is a full question** with its own required prompt, and may carry
    rich prompt content, media and points (`GroupQuestionInput`, 2399-2413; `StudentQuestion`,
    2112-2144). The deck's cloze has one prompt for five gaps.
15. **The deck's own English for the drawer handle is "Open the audio"** (S 992, 1703), beside
    a tip that says "The recording is here."

### 1. Student, take test

| ID | Where | Before (deck of record) | After (fourth export) | Product today | Verdict | Size | Task |
|---|---|---|---|---|---|---|---|
| S-01 data model | S 933-1316 markup; fixtures S 1457-1515; `exam()`, `startExam`, `examVals` S 1615-1719 | Route `take`: one hard-coded reading paper (PASSAGE + 8 `QS`), always two panes | Route `exam`. A paper is `{title, secs:[[id,title,instructions]], groups:[{title, members, stim}], items:[[id,no,secId,type,prompt,opts,_,extra]]}`; `stim` is `{kind:'passage',title,paras}`, `{kind:'passage',cloze:<itemId>,title}` or `{kind:'audio',key,title,plays,canPause}`. Layout is picked per question from the stimulus of its group (S 1630). Three fixture papers: s1 reading (8), s2 listening (two audio groups; 10), s9 "Grade 9 Mock Test 3" (31 items, four sections) | `take-test/pages/TakeTestPage.tsx:311-487` (`Paper`) reads `AttemptSession.sections/groups/questions`; the picker is `hasPassage` (`panes.ts:15-17`, `stimuli.length > 0`). A group with a recording always has a material holding its audio node (note 11), so a listening group already gets the left pane: it shows the group's title, the material's heading and the node's label (`GroupContext.tsx:57-62`), while the player sits in the question pane | rework | S | T-D4.3 |
| S-02 rail | S 1006-1039; rule S 1631 | not drawn (R3 removed the 256px rail in T-R3.9c) | `<aside aria-label="Sections">`, 236px, `--sidebar`, right border, padding 16px 14px, gap 16px. Shown when `w >= 900` on a question with no stimulus and `w >= 1180` on one with a stimulus, never below 768. Per section: title (12.5px, line-height 1.35; 600 `--fg` for the current section, else 500 muted) and "n / m" (11.5px tabular); a 5-column grid, gap 5px, of 34px squares (radius 8, 1.5px border, 12.5px 600). Legend at the bottom: Answered, Partly, Flagged (11.5px) | none: `components/Navigator.tsx:29-35` deletes the old rail's stored width; `tests/units/student/side-column.test.ts` proves no `/app` route loads `SideColumn` | rework (reverses T-R3.9c) | M | T-D4.4 |
| S-03 footer | S 1239-1275; flags S 1714 | From 768: Previous, a wrapping strip of 34px squares, Next. Below 768: Previous (icon), "4 / 8 · 3 answered", Next | The strip never renders (`dotsMode: false`; S 1245-1255 is dead markup). With the rail: Previous, a centred key hint (two 22px `<kbd>` "←" "→", then "to move between questions · A–D to choose an answer", or "to move · A–D to choose" beside a stimulus; 12.5px muted), Next. Without the rail, at every width: Previous, the grid button (flex 1, 44px, radius 10, `layout-grid` 16px), Next | `Navigator.tsx:98-102` (`wide ? <Strip> : <SheetButton>`), `Strip` 135-163 | rework (reverses T-R3.9c) | M | T-D4.4 |
| S-04 grid label | S 1265-1270; `gridLabel` S 1714 | "{cur} / {total} · {n} answered" | `(stim && !isMobile) || w < 420`: "{no} · {answered}/{total}" ("23 · 0/31"); else "Question {no} · {answered} of {total} done". In a span with ellipsis. Answers red-pen mark 2 | `Navigator.tsx:174-192`, strings `takeTest.navPosition`, `takeTest.navAnswered`; breaks onto two lines at 320 | rework | XS | T-D4.4 |
| S-05 sheet | S 1280-1316; geometry S 1716 | Phone only: bottom sheet with "Questions", "{n} of {total} answered", one 6-column grid of 44px squares, legend Answered, Flagged | Opens from the grid button at any width without the rail. Below 768: bottom sheet, radius 16px 16px 0 0. From 768: a floating card 560px wide (`left/right: calc(50% - 280px)`), `bottom: 76px`, radius 14px. Both: 1px border, `max-height: 80%`, padding 12px 16px 20px, gap 16px, grabber 40 by 4. No title and no total: one block per section (title, "n / m"), 6 columns, gap 5px, squares 42px high, 12.5px. Legend 11.5px with "Partly" | `Navigator.tsx:193-267`; mounted only when `!wide`; squares `h-11` (44px), `gap-2` | rework | S | T-D4.4 |
| S-06 squares | `stIt` S 1621; `dot` S 1640; S 1016-1019, 1294-1297 | Two states; current = `--accent-c` border; flag dot 10px; 13px | Three states: answered (primary fill), partly (`--accent-soft` fill, primary border), empty. Current adds `box-shadow: 0 0 0 2px var(--accent-soft)`. Flag dot 9px with a 2px `--card` border. 12.5px. Names "Question 23, answered" / ", partly answered" | `Navigator.tsx:18-22` (`DotState {answered, flagged}`), `toneOf` 291-295, flag dot `size-2.5`; `answered.ts:10-30` is a boolean | rework | S | T-D4.4 |
| S-07 phone drawer | keyframes S 26-27; S 986-1005, 1041-1053; script S 1686-1710 | "Passage \| Question n" switcher (two 34px tabs); the passage replaces the question | `hasPaneTabs: false` (S 1710). The stimulus is a drawer from the left: `W = min(round(w * 0.88), 420)`, z 60, `transform .32s cubic-bezier(.22,.8,.24,1)`, `--shadow-lg`; backdrop `rgba(15,20,22, 0 to 0.45)`. Closed: a handle at `left: 0; top: 42%`, 30 by 84px, radius 0 12px 12px 0, icon `file-text` or `audio-lines`, a 7px `--accent-c` dot while the recording plays, `chevrons-right`; "Open the passage" / "Open the audio"; title "Swipe right or tap to open". Until first opened per group: `qz-nudge 1.4s ease-in-out .4s 3` (7px) and a tip (primary, 12.5px, max 220px): "The passage is here. Swipe right or tap to read it." / "The recording is here. Swipe right or tap to open it.". Open: sticky bar "Swipe left to go back to the question" and a 34px close button. Swipe starts after 10px when `|dx| > 1.2 |dy|`; opens past 25% of W; closes past 30%; `touch-action: pan-y`; a click within 60ms of a drag is swallowed. A gap tapped in the drawer closes it and opens that gap. The pane has an explicit `background: var(--bg)` and is `aria-hidden` while the drawer is closed (S 1041, 1700), which leaves its gap buttons in the Tab order | `TakeTestPage.tsx:400-406`, 531-575 (`PaneSwitch`, `PaneTab`), `reading` 370-372; `panes.ts` `questionShowing`, `useKeptScroll`; no pointer code in the engine | rework (reverses T-R3.9b's switcher) | L | T-D4.7 |
| S-08 pane surface | S 1101-1103; `qBg`, `colMax` S 1709 | Always `--sidebar`; column 600px | `qBg = stim && !isMobile ? --sidebar : --bg`; `colMax = stim ? 600px : 720px` | `TakeTestPage.tsx:427` (`bg-sidebar`), 442 (`max-w-150`), unconditional | rework | XS | T-D4.5 |
| S-09 section header | S 1104-1107; S 1699 | not drawn | On every question: the section's title (12px, 600, muted, uppercase, letter-spacing .04em) over its instructions (13.5px muted), gap 2px, 12px padding and a 1px border below | `TakeTestPage.tsx:388`, 444-448 (part eyebrow only on a multi-part paper with no passage, .02em); `SectionInstructions.tsx` on a section's first question only (`opensSection`) | rework | S | T-D4.5 |
| S-10 question line | S 1109; `typeLabel` S 1655; `qWord` S 1699 | "Question {n} of {total} · {type}" with "Choose one", "True, false or not given", "Choose two", "Short answer" | "{Question\|Questions} {no} of {total} · {type}"; `no` may be a range ("1–5"); `total` counts a range as that many. Types are the builder's names: "Single choice", "Multiple choice", "True / False", "True / False / Not given", "Cloze · {n} gaps", "Fill in the blank", "Matching", "Short answer" | `questionType.ts:20-54`; en `takeTest.type.*` = "Choose one", "Choose one or more", "True or false", "Fill in the blanks", "Short answer"; the number is `index + 1` | rework | XS | T-D4.5; the range and "Cloze · {n} gaps", T-D4.10 |
| S-11 stimulus head | S 1054-1056, 1063 | Literal "READING PASSAGE 1" (letter-spacing .02em) over the passage title | Kicker = the group's title, uppercase, 12.5px 600, letter-spacing .04em; h2 (22px) = the stimulus's own title; an audio stimulus has the kicker and no h2 | `PassageBody.tsx:37-44` (.02em); `GroupContext.tsx:57` passes eyebrow = section title, title = group title; `GroupMaterials` omits a material named like its group | rework | XS | T-D4.5 |
| S-12 cloze passage | S 1062-1073; script S 1675 | not drawn (DG-80: gaps that lead to their question, from primitives) | h2, then a card (padding 18px 20px, 1px border, radius 12, `--card`, 16px or 18px larger, line-height 2.3). A gap is an inline button: min-width 76px, height 30px, radius 7, 1.5px border, a 20px primary circle with the gap number, then the chosen word. Empty: `--muted`, `--ring` border. Filled: `--accent-soft`, `--border`. Active: `--card`, primary border, `0 0 0 3px --accent-soft`. The card stays, filled in, beside the group's other questions | `GroupContext.tsx:63-77` draws a gap as `.content-gap` (`content.css:75-80`, a 1px underline) with the gap's label only; never the chosen answer, no active state | rework | M | T-D4.10 |
| S-13 cloze stop | S 1145-1170; `gapStrip` S 1685; `gapOpts` S 1664 | not drawn | One navigator stop, "Question 23 of 31 · Cloze · 5 gaps". Prompt 17px; "The gaps are in the passage. Pick a gap here or in the passage." (12.5px); a strip of gap chips (32px, radius 8, 20px circle, the word or "—"; active = primary fill). A card (`--sidebar`, 1px border, radius 12, padding 14): "Gap {n}" (13.5px 600) and "{k} of {m} gaps filled"; options in `repeat(auto-fill, minmax(132px, 1fr))`, gap 8, rows 46px with the round letter marker. Choosing moves to the next unfilled gap. Answers red-pen mark 1 | Each gap-bound member is its own `single_choice` question with its own square, drawn by `Choice` (`QuestionBody.tsx:83-163`); `TakeTestPage.tsx:377-386` `jumpToGap` | new (presentation over data that exists) | L | T-D4.10 |
| S-14 recording card | S 1074-1097; script S 1677-1682 | not drawn (DG-80: shared recordings in the question pane) | An audio stimulus fills the left pane (the drawer on a phone): the kicker, no h2 (S 1054-1056 puts the h2 inside `isPassage`), then the card. Card: padding 18, radius 14, 1px border, `--card`, `--shadow`. A 52px round primary button (22px icon: play, pause, rotate-ccw, lock), a 6px track on `--muted` filled `--accent-c`, under it "{m:ss} / {m:ss}" and "Play {n} of {N}" (12.5px tabular), then a line with a 15px headphones icon (13px muted): "Press play when you are ready.", "Playing", "Paused", "Finished · {k} play left", "You have used all {N} plays". Button names: Play, Pause, Resume, Play again, No plays left. No seek, no title, no transcript | The player is in the question pane: `GroupContext.tsx:90-199` (`GroupListening`, `SharedAudio`), mounted at `TakeTestPage.tsx:462-468`, one per recording, each under a printed "Recording {n}". The left pane meanwhile prints the recording's material: its heading (`GroupMaterials.tsx:67-69`) and, at the audio node, the node's label as a line of text (`GroupContext.tsx:62`). `AudioPlayer.tsx:150-250`: 40px button, 4px `bg-primary` track, hint "n plays left". Not drawn by the deck and present in the product: a material that mixes text and audio, and a group with several recordings | rework (reverses T-R3.9b: "Shared recordings sit in the question pane"); the card goes to its audio node, and an audio-only material loses its heading and label | M | T-D4.8 |
| S-15 pause policy | `canPause` S 1503; S 1677-1681 | not drawn | Per recording. False: while playing, the button is inert at opacity .45 and the status reads "Playing · this recording can’t be paused"; before the first play "… It can’t be paused once it starts." True: Pause and Resume. The intro note still says "you can pause once" (S 1378) and the script keeps an unread `pauses` counter | nothing: `AudioPolicy` is `maxPlays`, `allowSeek`, `showTranscriptAfterSubmit` (`api/openapi.yaml` 1695-1709); no "pause" in the contract; `AudioPlayer` always pauses | contradicts DG-111's default; needs the contract | M | T-D4.13, T-D4.9 |
| S-16 play-limit lock | `locked` S 1677; S 1679-1681 | not drawn; the product keeps the button live and says extra plays are recorded | After the last allowed play: lock icon, button at .45 named "No plays left", "You have used all {N} plays", and `toggle` returns without playing | `GroupContext.tsx:158-183`, `QuestionAudio.tsx:49-53`: live button, "Played 2/2 times", `takeTest.extraPlaysRecorded` | contradicts spec §11.4, the contract (`api/openapi.yaml` 1701-1707, 6858) and the canary case `audio-player.test.tsx:105-112` | S | not built (DG+1) |
| S-17 play counting | `toggle` S 1681; `go` S 1639 | not drawn | A play is spent only when playback starts from 0:00 or after it finished; Resume spends none. "Play 1 of 2" before the first play. Starting one recording pauses the others. Moving to another section pauses everything; "Resume" then continues without a play | `AudioPlayer.tsx:104-119` fires `onPlay` on every start; `GroupContext.tsx:175`, `QuestionAudio.tsx:34-38` count each; the shared player unmounts when the question leaves the group | rework; changes what spec §11.4 calls a play | S | T-D4.8 |
| S-18 unlimited plays | T script 6904 (`plays: +x.shared.plays \|\| 99`); S 1681 | not drawn | No student state: the builder preview reads "Play 1 of 99" | `takeTest.sharedAudioUnlimited` ("Unlimited plays") | new (undrawn) | XS | T-D4.8 |
| S-19 choice rows | S 1135-1142; `mk` S 1643; `optCols` S 1660 | One column; chosen marker shows a check; multi marker radius 6px | The chosen marker keeps its letter in `--primary-fg` on primary, no check. Multi radius 7px. Rows are `<button aria-pressed>` in `role=group` "Answer". Columns: every option at most 16 characters and no stimulus gives 2 columns below 768 and `min(4, options)` from 768; else one. Line-height 1.4 | `QuestionBody.tsx:114-160`: `flex-col gap-2`, `min-h-13`, `leading-[1.45]`, `<Check>` at 152, `rounded-[6px]`, real radio and checkbox inputs | rework | S | T-D4.6 |
| S-20 fill-in field | S 1171-1176 | not drawn (DG-80) | The sentence at 18px (20px larger), weight 500, line-height 2.1, with the blank as an inline input: width `min(100%, 340px)`, height 38px, no box, a 2px bottom border (`--ring`, primary once typed), `--muted` fill, radius 6px 6px 0 0, padding 0 8px, placeholder "type here", `autocomplete=off spellcheck=false`. An optional hint (13px). No case-rule line, no worth line | `QuestionBody.tsx:165-254`: prompt 17px; `BlankInput` is a bordered box `h-11 w-32` (44 by 128px, 40px from 1024), centred, no placeholder | rework | S | T-D4.6 |
| S-21 short answer | S 1227-1236; `countText` S 1666 | 52px input, "n of L words · too many" | Same, plus `autocomplete=off spellcheck=false` and "1 word" in the singular | `QuestionBody.tsx:258-302`: a growing `<textarea>`, "{n} words", the worth line; no limit in the contract | matches, with R3's recorded departures | XS | T-D4.6 |
| S-22 sentence gap | S 1127-1129; `hasSentence` S 1645 | not drawn | A choice whose prompt holds "___": the sentence at 18px (20px larger), weight 500, line-height 1.8, with the blank as an inline box (min-width 88px, radius 7, a 2px bottom border): empty `--muted` with a `--ring` line; chosen, the option's text on `--accent-soft` with an `--accent-c` line | `QuestionBody.tsx:106-110` prints the prompt as written | new (no field; derived from the prompt) | S | T-D4.11 |
| S-23 sign card | S 1115-1122; `EX_SIGN` S 1494 | not drawn | Above the prompt: a card with a 3px `--fg` border, radius 12, padding 18px 24px, min-width `min(100%, 320px)`; head 20px 700 letter-spacing .06em; optional sub 15px. Data from a table keyed by question id | nothing; a question image is the nearest thing | new, no model; not built (default) | M | not built (DG+8) |
| S-24 word tiles | `EX_UL` S 1493; `wordMode` S 1644-1646; S 1140 | not drawn | Pronunciation and stress items: row min-height 60px, text 20px, the tested letters underlined (2px, offset 4px, 700). Chosen by fixture ids | `OptionText` renders `StudentOption.content`; `ContentMark` has `underline`; nothing marks a "word" question | new, no model; the underline is authorable, the size is not built (default) | S | not built (DG+8) |
| S-25 image and caption | S 1178-1202; `MAP_CELLS` S 1497 | not drawn (DG-80: image) | Above the prompt: a 13px 600 caption ("Map of the town centre") and the picture in a bordered 12px-radius frame on `--sidebar` | `QuestionSheet.tsx:64-70` draws `question.media` above the prompt; alt fixed; no caption | answers DG-80's image position; the caption has no field | XS | none; the caption is not built (DG+8) |
| S-26 hint line | S 1133-1135, 1173-1175, 1229-1231 | "Choose two answers.", "Write NO MORE THAN THREE WORDS." as fixture hints | The same, per question (`extra.hint`), 13px muted | no per-question hint in the contract; multiple choice shows `takeTest.chooseMultiple` there | new, no field; `maxWords` is R6 | XS | not built (DG+8) |
| S-27 multiple choice cap | `exPick` S 1617; `stIt` S 1621 | "Choose two", any number of picks | "Multiple choice"; the third pick drops the oldest; one pick is partly. The Teacher preview feeds the same type with "Choose all the answers that are correct." (T 6907) | `TakeTestPage.tsx:592-603`, `QuestionBody.tsx:93-102`: no cap; the contract has no count | R6 (T-R6.5 `selectCount`, T-R6.10) | XS | R6 (T-D4.19) |
| S-28 TFNG | S 1482-1484; `typeLabel` S 1655 | "True, false or not given" | "True / False / Not given", three options | no such type; a three-option `single_choice` today | R6 (label changes in T-R6.10) | XS | R6 (T-D4.19) |
| S-29 matching | S 1177-1226; script S 1667-1670 | not drawn | Prompt; "{k} of {m} matched · pick the {pick} for each {what}" (12.5px). One card per item (1.5px border, primary once matched, radius 11, padding 10px 12px) with its key, text (14.5px 500) and a radiogroup of 38 by 38 chips (radius 9, 14px 600), one per option; a chip used elsewhere is at opacity .5, choosing it moves it, tapping the chosen one clears it. With option text, no stimulus and `w >= 1000`: a 220px reference card on the left. s2 numbers one matching "1–5" (five in the total), s9 another "31" (one) | no type; `UnknownType.tsx` | R6 (T-R6.10's matching bullets change) | L | R6 (T-D4.19, DG+13) |
| S-30 keys | `onExKey` S 1616; `go` S 1639; Esc S 1530 | arrows move, a–e choose | Same, off in a field or under a dialog; a–e act on single, tf, tfng, multi only (not on the cloze card or matching chips, though the hint says "A–D to choose"). No F. The arrows set `exCur` directly and do not go through `go()`, so unlike a click on a square or on Next they do not close the drawer or the sheet and do not pause a recording when the section changes; Esc clears the old `sheet` state, not `exSheet`, so it does not close the new sheet | `TakeTestPage.tsx:164-208`: arrows, A–E, F flags; every move goes through one function, and the keys are off under the sheet | matches for the keys; the bypass and the Esc slip are the prototype's and are not copied; F stays a departure, now beside a drawn hint | XS | T-D4.4 (the hint) |
| S-31 submit dialog | `exSubmit` S 1691-1697 (markup S 1317-1362 unchanged) | "Submit with {n} unanswered?" counting questions; "Unanswered questions score zero. …" | n counts empty gaps and match rows one each; body "Unanswered gaps, matches and questions score zero. You still have {m} minutes."; a partly answered item is listed under "Go to"; a chip may read "1–5" | `components/SubmitDialog.tsx:63`, 89-93, 118-129 | rework only if a cloze becomes one item; none under the default | XS | none; T-D4.10 adds one case |
| S-32 preview header | S 950-978; S 1522, 1541, 1620, 1713 | Leave, title and save line, timer, Submit | Embedded with an `exam` prop: no leave button, no Submit, a chip "Student view" (30px, radius 999, `--info-soft` / `--info-ink`, 12.5px 600, eye 14px); Finish toasts "This is a preview. Students submit here."; no focus counting; root height 100%; the timer starts at 45:00 and runs. The save line is still drawn and `saved()` still runs on every pick (S 961-963, 1617, 1633), so the preview says "Saving…" and "All answers saved" | `components/EngineHeader.tsx:19-76`; the teacher's preview is `features/tests/components/StudentPreview.tsx`, not the engine. No preview host has a time limit or an integrity policy to show (note 13) | reopens R4 (T-R4.31b, T-R4.30, T-R4.39) and new; the timer and the save line cannot be built as drawn | S | T-D4.2c, T-D4.16 |
| S-33 focus dialog | S 933-947 | after the sheet, inside the take screen | Same markup and strings, moved above the screen; not counted in a preview | `features/integrity/components/StrikeDialog.tsx` | matches | XS | none |
| S-34 submit toasts | `exSubmit.go` S 1694 | result page r1 | s1 opens the result; s2 and s9 go Home with a toast | `components/SubmittedScreen.tsx` | nothing (prototype shortcut) | XS | none |
| S-35 still undrawn | S 933-1316 and `examVals`, checked | not drawn | Still not drawn: a question's own audio; a group with a passage and a recording, or several of either; group instructions; tables, lists and images in a passage; blanks in a table and the case rule; the worth line; essay; the timer at zero; the save line's failed and offline states; the strike count; the focus dialog under `warn` and `auto_submit`; the fullscreen bar; takeover, time-up and closed locks; a sealed paper's footer; the unknown-type block; the Leave dialog's unsaved states; the Submit dialog under a minute, busy and failed; the submitted screen; loading and load-failed | all built from deck primitives (T-R3.9a to T-R3.9d) | nothing; DG-80 keeps them | XS | none |
| S-36 dead branches | S 979-985 (`hasPaneTabs`), 1123-1126 (`hasRef`), 1245-1255 (`dotsMode`), `isSign` on matching rows S 1210-1212, 1670; `examBold` S 1470 | live tabs and strip | Markup that never renders, and one fixture field nothing reads: q12 carries `{ examBold: 0 }` | — | nothing; reading the template without the script would rebuild them | XS | T-D4.1 (README) |
| S-37 header save line | S 952, 963 | the save label could wrap; the leave button could shrink | The label is in a `white-space: nowrap` span and the leave button is `flex: none`. The drawing shows what it fixes: "All answers / saved" on two lines at about 1247px, not marked by the designer | `components/SaveState.tsx:58` truncates the label on one line; `EngineHeader.tsx:88` gives the button `flex-none` | matches | XS | none |

### 2. Student, other

| ID | Where | Before | After | Product today | Verdict | Size | Task |
|---|---|---|---|---|---|---|---|
| O-01 keyframes | S 26-27 | none | `@keyframes qz-nudge` (7px) and `qz-tip`; the only stylesheet change in either page. No token, colour, radius, shadow or font moved | `index.css` has `qz-marquee` only | part of S-07 | XS | T-D4.7 |
| O-02 embed props | S 33 (`height: {{ rootH }}`), S 1522, 1527, 1541, 1615 | root `100vh`; no props | The page takes `exam` (a fixture id or an exam object) and `dark`; root height 100% when embedded; nothing written to localStorage | — | nothing (how the Teacher page mounts it) | XS | none |
| O-03 fixtures | `TESTS` S 1376-1382; Home S 1782-1783 | s2 has 20 questions; three upcoming rows | s2 has 10; new s9 (31 questions, 60 min, "0 of 2", a teacher note); four upcoming rows, count "4" (the nav badge stays "3"); "Grade 9 Exam Prep" is not in the Classes fixture | Home reads the API | nothing | XS | none |
| O-04 entry points | S 1611, 1618, 1623-1624 | Continue, the due row and Start open route `take` | All three call `startExam(id, resume)`; the old `take` script is dead state | `app/router.tsx` takeTestTree | nothing | XS | none |
| O-05 fixture keys | `EX_RAW` 7th entry, `ans: […]` S 1485-1488 | — | The Student fixture holds correct indexes and accepted answers as demo data | the leak test forbids four keys on `/app/*` | nothing; a test double copied from the deck must not carry them | XS | T-D4.3 (its fixtures carry no key) |

Unchanged on the Student page (no hunk; S 128-930 and the dialog block S 1317-1372 diffed line
for line): Home, Classes, Join dialog, Test intro, Result, Settings, Learn, Course, Lesson,
Flashcards, Grades, Messages, This week, the shell, tokens, the engine header's geometry (60px,
timer pill, Submit 38px; its only change is S-37), the flag toggle, prompt and option sizes,
Previous / Next / Finish, the copy-and-paste toast, the Leave, Start, Join and Submit dialog
markup, `FOCUS_ALLOWED = 2`, `isMobile = w < 768`, larger text at 18 / 19 / 17.

### 3. Teacher, test builder

| ID | Where | Before | After | Product today | Verdict | Size | Task |
|---|---|---|---|---|---|---|---|
| B-01 group model | T 1581-1587, 1655-1732; script T 6881-6895, 6968, 6986, 7029 | The outline "group" had a title, instructions and questions; R4 maps it to the product's Section (T-R4.31a) | Every outline group can carry one "Shared content": None, Passage or Audio. Body: "A passage or recording shown beside every question in this group. Students see it on the left while they answer." The deck's group is now section and shared-context group in one level | Two levels: Section (`api/openapi.yaml` 2492-2508) holding units, each a question or a group (`migrations/00037` `test_section_units`); `QuestionGroup` 2274-2313 (200 members, 16 stimuli, 16 recordings); members owned by the group. Web: `features/tests/components/OutlineTree.tsx:334`, `OutlineGroupRow.tsx`, `BuilderGroupPane.tsx` | contradicts R4's reading, in which the deck's group is the product's section (T-R4.31a); the product's model is a superset | L if built literally; S under the default | T-D4.2a (DG+5) |
| B-02 outline line | T 1581-1587; script T 6968 | not drawn | Under the instructions line, same geometry (margin 0 0 4px 26px, padding 4px 6px, radius 6, 12px muted, 13px icon, one line with ellipsis): `file-text` "Passage · {title or Untitled}" plus " · {n} gaps", or `audio-lines` "Audio · {file or no file yet} · {n} plays" / "· unlimited plays". Shown while the group is open. Opens the dialog. The prototype prints "1 plays" | `OutlineGroupRow.tsx:53-92`: a `Layers` icon and the title, no summary | reopens R4 (T-R4.31a) | S | T-D4.2a |
| B-03 menu item | script T 6986 | Rename, Instructions, Move up, Move down, Remove group | "Shared content" (icon `panel-left`) in third place | `OutlineTree.tsx:462-488`: the section menu; the group is added from the footer (334) and has no menu | reopens R4 (T-R4.31a) | XS | T-D4.2a |
| B-04 dialog frame | T 1650, 1655-1656; script T 7026-7030 | one dialog, 440px, two variants | Width per variant: 440px for Remove and Instructions, 560px for Shared content. Title "Shared content for “{group}”", a scroll area (gap 12px, max-height 56vh), Cancel and Save, toast "Shared content saved". No validation, no dirty guard, no Esc | no dialog: `BuilderGroupPane.tsx:150-204` renders `GroupComposer` in the editor pane, with autosave and device recovery | new | M | T-D4.14 |
| B-05 kind switch | T 1657-1663; script T 6887 | not drawn | Segmented, full width, 30px buttons, 13px 500, 14px icons: "None" (`circle-slash`), "Passage" (`file-text`), "Audio" (`audio-lines`). One kind at a time; the other kind's fields stay in the draft | a group holds up to 16 materials mixing text, images and audio (`question-groups/components/GroupComposer.tsx:136-150`) | new; must not drop what the dialog cannot show | S | T-D4.14 |
| B-06 passage fields | T 1664-1669 | not drawn | "Title" (36px, "e.g. Why cities need green space"); "Passage", a plain textarea, 7 rows, 13.5px, line-height 1.55, "Paste or type the passage students read" | "Material name" and `ContentEditor` (`MaterialContent.tsx:39-58`); title 1 to 200 characters, required | contradicts T-R4.63 (the material is a content editor host): keep the editor, take the labels | S | T-D4.14 |
| B-07 gaps | T 1670-1693; script T 6889-6890 | not drawn in the builder | Hint "Optional: add gaps for a cloze. Each gap is answered by a single-choice question in this group, shown at the gap." and "Insert gap" (30px, `text-cursor-input`), which appends "[gap n] " at the end of the text. Row: a chip "gap {n}" (min-width 52px, 24px, 1.5px dashed `--ring`, `--muted`, 12px 600), a select (32px) "Choose a question…" then "Question {id} · {title}" for every question, a danger border while unbound, and "Remove gap" (28px). One question may take two gaps; Save accepts unbound gaps | Exists in full: `GapBindings.tsx:14-133` (targets: choice members or one blank of a fill-in; a used target is disabled), gaps inserted at the caret; rules in `model.ts:113-155` and `server/internal/modules/tests/domain/group_validation.go:176-212` | reopens R4 (T-R4.34): the drawn row is taken; the product's rules stay | S | T-D4.2b, T-D4.14 |
| B-08 audio file | T 1696-1712; script T 6891 | not drawn | Empty: a dashed zone (1.5px `--border`, radius 10, padding 12), "MP3, M4A or WAV up to 25 MB", "Choose file", "From Media" (30px). Attached: a card with the file name and "Replace" (28px). No progress, failure, length or player | `MaterialAssetDialog.tsx`, `MaterialTools.tsx:107-118`; T-R4.35 moves the upload into `useMediaUpload`; T-R4.65 builds "Choose from Media" | new; DG-63's limits stand (MP3 or M4A, 50 MB, 5 minutes) | M | T-D4.14 |
| B-09 plays | T 1714-1719; script T 6892 | not drawn for a group | A select: "1 play", "2 plays", "3 plays", "Unlimited"; default 2. A third drawing of Plays (question media: "Once \| Twice \| Unlimited", T 2155-2159; the bank: a select) | one component for both: `question-bank/components/AudioPolicyPanel.tsx:13` (1, 2, 3, 5, Unlimited) | reopens R4 (T-R4.65): its control serves the group too | XS | T-D4.15 |
| B-10 students can pause | T 1720-1724; script T 6893, 6904; also T 2161-2165 | drawn once, on question media; DG-111: not built | Drawn a second time, on the group's recording: a switch, on by default (34 by 20 track, 16px knob `#fff`). The value reaches the Student page as `stim.canPause`, which acts on it | nothing in the contract, the schema or the player | contradicts DG-111's default; needs `allowPause` | M (server) + S (web) | T-D4.13, T-D4.15 |
| B-11 transcript | T 1726-1729 | not drawn for a group | "Transcript", "Optional. Students see it after they submit.", 3 rows. No "Show transcript after submitting", no "Allow skipping ahead". The preview does not pass it on | `AudioPolicyPanel.tsx:66-108`: both switches and the transcript; `GroupRecording.transcript` up to 100,000 | contradicts DG-111 (both switches stay) | XS | T-D4.14 |
| B-12 undrawn states | outline rows T 1591-1623; publish checks script T 7706 | not drawn | Still nothing for: a question bound to a gap (in its row or its card), deleting or retyping a bound question, publishing with an unbound gap or with "no file yet", a group that holds more than the dialog shows | `GroupComposer.tsx:372-381` (a bound member cannot be deleted), `model.ts:162`, `group_validation.go:52-67` | nothing; product behaviour kept, listed in a gap row | XS | T-D4.1 (DG+6) |
| B-13 preview overlay | T 5737-5761; script T 6896-6901 | A 720px form dialog "Student preview" / "What students see. Answers here are not saved.", one question at a time, Previous / Next, four demo options | A full-window overlay (`role=dialog`, "Student preview", z 95, `--overlay`, padding 14px, gap 12px). Bar (card, radius 12, `--shadow-lg`, padding 8px 10px 8px 14px): eye 16px, "Student preview · {title}" 14px 600, a segmented "Computer \| Phone" (28px buttons, 12.5px, icons `monitor`, `smartphone`), a 34px "Close preview". Frame: `min(1320px, 100%)` and radius 12px, or 390px and radius 28px, full height, holding the Student take-test screen. No Esc, no focus trap | `features/tests/components/DraftPreviewDialog.tsx:76-90` (max-w-3xl, 85svh), `StudentPreviewPane.tsx` (phone 320px), `StudentPreview.tsx:44-95` (panes stacked, disabled) | reopens R4 (T-R4.31b: "Previous / Next" is no longer drawn) and new | M frame + L engine | T-D4.2c, T-D4.16 |
| B-14 `previewExam` | script T 6902-6914 | not drawn | The prototype's mapping from builder state to the Student page. It passes no gaps, no question media, no rich prompt, no per-question options, no transcript; Unlimited becomes 99 | `question-groups/preview.ts`, `DraftPreviewDialog.tsx:39-75` map the real draft, gaps and assets included, transcripts stripped | nothing (demo plumbing; never the data contract) | XS | none |
| B-15 fixtures | `INIT_SECS` T 6632 | two demo groups | Both carry a shared passage ("Why cities need green space", "The history of maps") and `gaps: []`, so the outline shows a "Passage · …" line under each open group by default | — | nothing; it is the fixture a deck check of the outline sees | XS | T-D4.2a (its deck check) |
| B-16 semantics | T 1650-1741, 5737-5761 | — | `role=dialog` with no name on the group dialog, no Esc, no focus trap or return; `outline: 0`; the knob `#fff`; plain buttons for the kind switch | R1 primitives (`FormDialog`, `Segmented`, `Switch`, focus rings) | nothing; DG-116, DG-36, DG-34, DG-30 decide these | XS | none |

### 4. Teacher, other

| ID | Where | Before | After | Product today | Verdict | Size | Task |
|---|---|---|---|---|---|---|---|
| T-01 test detail preview | T 5846-5853; script T 7508 | A scrolling list (max-height 680px): five questions, changed ones outlined with "New" / "Changed" chips in compare mode, footer "Showing 5 of {n} questions. Answers are hidden from students." | A frame holding the Student take-test screen: padding 16px; width 100% and height 620px, radius 12px, or 390px and 720px, radius 24px. No outline, no chips, no footer. The "Changes from version {n}" list above and "Computer \| Phone" are unchanged. The prototype feeds it the builder's draft, whatever the version | `features/tests/pages/TestDetailPage.tsx:168` mounts `StudentPreviewPane` | reopens R4 (T-R4.30, its preview bullet) | S | T-D4.2c |
| T-02 import preview | T 4975-4980; script T 7427-7428 | A scrolling list (max-height 640px) built from the import, footer "Showing 7 of 34 questions. Answers and your notes are hidden from students." It drew the cloze as the product models it: the passage with gap chips, then a member question "23.1 Choose the word for gap 23.1." with its own options and underlined runs. It was the one place where the deck drew one question per gap on a student-eye surface | The same frame holding Student fixture `s9`, not the import's own data, so the per-gap cloze is gone from every student-eye drawing (evidence for DG+3; its data stays in the script, T 7428). No footer | no confirm page yet; T-R4.39 creates `ImportConfirmPage.tsx` | reopens R4 (T-R4.39, its preview bullet) | S | T-D4.2c |
| T-03 cross-page import | T 4978, 5756, 5850: `<dc-import name="Quizzivy Student" exam="…" dark="…">` | no page imported another | The Teacher page mounts the Student page three times; `support.js` already implements `dc-import` | `scripts/check-design-deck.mjs:20-31` reads `src`, `href` and quoted names ending `.svg`, `.dc.html`, `.js`: `name="Quizzivy Student"` is unchecked | the import teaches the check script | XS | T-D4.1 |
| T-04 question count | S 1380 (`q: 31`), 1634-1637; T script 7430 ("… of 35"); old footer "34" | 34 and 35 | The same paper is "31 questions" on the Student page (the cloze counts once) and 35 on the Teacher import. Inside the Student page a matching numbered "1–5" counts five and a cloze of five gaps counts one | every member is a question; `TestSummary.questionCount` counts questions | design question; no build under the default | S | none (DG+4) |
| T-05 instructions copy | script T 7030 (unchanged) against S 1104-1107 | "Students see this above the group’s first question." | The builder still says so; the Student page shows the instructions above every question | `SectionInstructions` on the first question | reopens R4 (T-R4.31a): the dialog's line is reworded when S-09 ships | XS | T-D4.5 |
| T-06 `github.md` | "Last sync", "Sync history" | 2026-10-03T17:53:01Z, four bullets on the content editor | 2026-10-04T09:04:30Z, "branch read: develop (question group graph: migrations/00037, 00002)", two bullets. The Screen map is unchanged: no row for `features/take-test/**` or `features/question-groups/**` | — | nothing; explains the drift: pause is in no migration, `allow_seek` and the `blank` gap kind were not drawn, the contract and the engine were not read | XS | T-D4.1 (DG+14) |
| T-07 dead script values | script T 7427-7428, 7508-7509 | read by the two list previews | `wi.preview`, `wi.pvOptCols`, `td.preview`, `td.pvFoot` ("Showing 5 of … questions. Answers are hidden from students.") and `td.pvOptCols` are still computed and no markup reads them | — | nothing; reading the script without the markup would rebuild the list previews | XS | T-D4.1 (README) |

Unchanged on the Teacher page (all fifteen hunks are covered above): the builder's title bar,
outline header, group header geometry, question rows, drag, footer, splitter, the whole editor
pane (content editor, type menu, answer areas, grading note, question media with its own
"Students can pause" and "Once | Twice | Unlimited", "Choose from Media", "Preview content after
pasting", More options), the Publish dialog and its three checks; the bank's Question editor,
Question bank, Media, Tests list; Imports history, upload and paste, processing, the Review
workspace; Test detail's header, banner, changes list and history; the shell and every other
screen. No token or CSS change. So T-R4.33, T-R4.35, T-R4.38, T-R4.62, T-R4.63, T-R4.64 and
T-R4.66 are untouched.

### 5. Drawings (`uploads/`, eleven PNG files)

| File | Screen | Marks | In the export | Verdict |
|---|---|---|---|---|
| `draw-99b474b9-…` (new) | Student, take test, the cloze of "Grade 9 Mock Test…" at about 1247px, on an intermediate revision (rail, passage, question) | 1: the "Gap 1" card's four options in one row, cut ("relax", "relaxin", "relaxe", "relaxes"), and its header wrapping. 2: the footer's centre button cut to "23 · 0/…" | 1: options are `repeat(auto-fill, minmax(132px, 1fr))` with `min-height: 46px`, both header spans `nowrap` (S 1157-1163). 2: with the rail (from 1180 beside a stimulus) the button is not drawn and the key hint takes its place; without it the label is the short form in a span with ellipsis (S 1265-1270, 1714). Unmarked, and both fixed: the rail's "Grammar and vocabulary" overlapped itself (S 1011), and the header's "All answers saved" stood on two lines (S 963, row S-37) | nothing to build beyond S-03, S-04, S-13; README's "ten red-pen markups" becomes eleven with one new row |
| `draw-8128a579-…` | Student, Join dialog ("6-character") | already "8-character" in the deck of record | unchanged | already resolved |
| the other nine (`47e9ff40`, `788a17fb`, `79c88341`, `8ccee204`, `9ac2b5fe`, `a3f40015`, `b7e91ca8`, `bf32bf87`, `ed818f3d`) | Teacher and Admin | the rows of README's "Resolved feedback" table, one to one | unchanged | already resolved |

`screenshots/` (39 files) is Claude Design's own captures; not imported, as before.

### 6. Unchanged, and what the contract already holds

Byte-identical to the manifest: Admin, Landing, Sign in, Splash, System pages, `support.js`,
`qz-controls.js`, `google-g.svg`, both mark SVGs, `brand/` (two SVGs). `docs/plan/75-r5.md` is
not touched.

The student payload already carries everything the new take-test screen draws for a passage, a
cloze and a recording, with one exception; the teacher's preview lacks one more thing, which is
not a missing field but the assignment's:

| The deck draws | The contract has | Where |
|---|---|---|
| sections with title and instructions | `StudentSection {id, title, instructions}` | `api/openapi.yaml` 2098-2110 |
| a group, its members, its passage | `StudentGroup {title, instructions, questionIds, stimuli[], recordings[], assets[]}` | 2146-2165 |
| a gap bound to a question | `GroupStimulus.gaps[]`: `GroupQuestionGap {gapId, questionId}` or `GroupBlankGap` | 2326-2371 |
| a recording, its length, its allowed plays | `StudentGroupRecording {id, assetId, policy}`, `MediaAsset.durationMs`, `AudioPolicy.maxPlays` | 2167-2175, 1695-1709 |
| plays used, across reloads and devices | `groupAudioPlays`, `POST /app/attempts/{id}/group-audio-play` (`x-permission: learning.take_tests`) | 6837-6870 |
| a question's image | `StudentQuestion.media` | 2112-2144 |
| underlined letters in an option | `StudentOption.content` marks (`underline`) | ContentMark |
| **"Students can pause"** | **nothing** | no "pause" in the file |
| **a time limit in a preview (45:00)** | **nothing a preview can read**: `durationMinutes` is the assignment's | 3175, 3195, 2640-2645; `previewTest` 4391-4398 |

What the deck draws and no release plans a field for: a per-question hint line, an image
caption, a sign or notice block, the "word" option size, matching's nouns (`what`, `pick`,
`colsTitle`) and free key styles, a cloze as one item with sub-ids ("23.1"), display numbers and
ranges. `tfng`, `matching`, `selectCount` and `maxWords` are R6's (T-R6.1a to T-R6.5). A
transcript is correctly absent from every student shape and already released after submission
through `SharedReviewContext.transcripts`.

---

## What the v2 export changed (2026-10-10)

Read against the fourth export (`uiux/update-teacher-student-view` @ `fd66dd6a`, byte-identical to
the export the six tables above inventory) and `uiux/update-uiux-v2` @ `26b60795`. Five pages
changed; `github.md`, `support.js`, `qz-controls.js`, the SVGs, `brand/`, Splash and System pages
are byte-identical to the fourth export, so with the three files above the manifest changes in
**eight** files and `node scripts/check-design-deck.mjs` still prints "Deck OK: 15 files, 7 pages".
No file is added; the three `<dc-import name="Quizzivy Student">` mounts are unchanged; no `<link>`,
font, token, colour, radius, shadow or keyframe moved (`qz-pulse` and `qzSpin`, which the AI panel
uses, exist in the fourth export). Every new icon name exists in the web's `lucide-react` 0.548.
The take-test engine, the builder's shared content and the three previews are byte-identical
between the two exports, so nothing in sections 1 to 6 is contradicted.

| File | sha256 (v2) | Size | Hunks (pretty) |
|---|---|---|---|
| `Quizzivy Teacher.dc.html` | `184536f0c6d9732ed4fae5eaf5d5e45dc188fda03d64f0ce682372364c096450` | 794,309 to 907,000 bytes | 54 |
| `Quizzivy Admin.dc.html` | `8c043d26e1c4981eaee58e2b65972666a6c17e1521b9235f382cff771b44ae6a` | 151,390 to 171,480 bytes | 14 |
| `Quizzivy Student.dc.html` | `dc6d6077fdfe6d79de7acba73db7dce5fab4cc670d223b7be34aee3b0da4d7bb` | 173,426 to 173,401 bytes | 10, all fixtures |
| `Quizzivy Landing.dc.html` | `52196e8610429e455d619c00d4d2c13934c855c4cb98bda7dae95ccdbc5cd0db` | 49,717 to 49,703 bytes | 3, all fixtures |
| `Quizzivy Sign in.dc.html` | `15ab7e2bbcb14e96979c0cc000fdb90e464796593485ab2ccc8e886649321f6f` | 22,899 to 22,896 bytes | 2, all fixtures |

Verdicts as above, plus **R5**, **R8** and **R10** for a screen a later release owns. The last column
names the D4 task, or the release and the correction T-D4.19 makes.

### 7. Teacher, v2

| ID | Where (v2 T) | Fourth export | v2 | Product / plan today | Verdict | Size | Task |
|---|---|---|---|---|---|---|---|
| V-01 sidebar separator | T 86-89; `showSep: !!g.label && !g.showLabel` (T 9696); the same on the Admin page (A 79-82) | Group labels only | A 1px `role="separator"` between nav groups while labels are hidden (collapsed) | `layouts/shell/Sidebar.tsx` (T-R4.4), shared by R5's `AdminLayout` | reopens R4 (T-R4.4) | XS | T-D4.20 |
| V-02 sticky side columns | `layoutVals` T 9624-9630; the outline T 1633 (`outPos`, `outMaxH`, `outOv`), the wizard aside T 2864 (`naSide`), the course aside T 3596 (`crsSide`), the settings nav T 3853 (`setNavPos`), the editor aside T 4649 (`qeSide`), the class detail column T 4277 (`cdSide`), the review aside T 5405 (`wiSide`), the version history T 6282, the bank filters T 1100 (`bfTop`) | `sticky; top: 0` on two asides; the rest static | Every side column `position: sticky; top: 16px; max-height: calc(100vh − 88px); overflow-y: auto` from a content width of 800 (wizard, course) or 1000 (class detail, review); the editor aside from 900; the settings nav from 768; the outline whenever the builder splits, scrolling inside itself; the bank's aside `top: 16px` from 1024 | T-R4.31a, T-R4.27a, T-R4.33, T-R4.42, T-R4.38, T-R4.30, T-R4.32, T-R4.43; the course aside is R10's (T-R10.13) | reopens R4 (geometry); R10 for the course | S | T-D4.20 |
| V-03 section menu placement | T 1675; `openMenu` T 7782 (`secMenuXY`) | `position: absolute` inside the row | `position: fixed` at the button's rect, clamped to the viewport, so the scrolling outline cannot clip it | T-R4.31a ("menus are never clipped") | matches; re-checked once the outline scrolls | — | T-D4.20 |
| V-04 grading pane | T 847, 859-875; `gPane`, `queueH` T 9629 | The page grows with the card; the queue is 180px high on phones | From 768 the page is `calc(100vh − 120px)` high (min 560px) and the queue and the card scroll inside themselves; on phones everything is static and the queue keeps 180px | T-R4.28 | reopens R4 (T-R4.28) | S | T-D4.21 |
| V-05 assignment progress card | T 607-631; `asgProgVals` T 9612-9623; `rosterGroup`, `rosterCounts` T 9610-9611; `asgRow` T 9705; `roster: rosterShown` T 9713, 9722 | Four stat tiles | One card: "{n} of {total} submitted" ("All {n} submitted", "Not assigned yet"; 13.5px 600), an 8px two-segment bar (submitted `--success`, in progress `--info`), legend buttons `aria-pressed` (28px, radius 7, 12.5px) with a dot or the flag icon in danger ink and a count (Submitted, In progress, Not started, Flagged) that **filter the roster** (`rosterF`; the page resets; the Students tab opens; an entry hides at 0 unless selected), "Average {avg}" (15px 600) after a 1px divider; opening the assignment clears the filter | T-R4.25 built the tiles ("four columns from 640 content width", ticked); `Monitor` holds the counts | reopens R4 (T-R4.25) | M | T-D4.22 |
| V-06 "Explain results with AI" | T 742-776; `ins` T 9529-9536; `aiInsights` T 9463-9469 | "Questions sorted by how often students got them wrong." | Beside that line an outline button (32px): "Explain results with AI", a spinner and "Reading results…" while running, "Refresh" after; under it a panel on `--accent-soft` (radius 10, padding 14): "What AI noticed" (sparkles, accent ink), "Based on {n} attempts", "Hide insights" (26px); three or four cards (`--card`, radius 9, padding 12) with a 28px icon tile (`key-round` warning, `triangle-alert` danger, `trending-up` success), a title under nine words (13.5px 600), a body with numbers (13px muted), and an action button "Review answers" (→ Grading) or "Make a practice set" (→ the Generate panel with types tf and short, primary "Save as practice test") | T-R4.26 (Questions tab), T-R4.13 (item analysis) | new (AI) | server M, web S | T-D4.32, T-D4.37 |
| V-07 "Suggest a score" | T 908-948; `gai` T 9517-9527; `aiGrade` T 9451-9462 | Student answer, then Score | Between them a box on `--accent-soft` "AI suggestion": idle, a sentence ("Compares the answer with the accepted answers. You decide the score." or, for a band, "Scores against the four IELTS criteria. You decide the final band.") and "Suggest a score" (30px); running, a spinner and "Reading the answer…" / "Reading the essay…"; done, a confidence pill (High / Medium / Low confidence in success / warning / danger tones), "{score} / {max}" (17px 600), bullet reasons (13px), "Comment for the student: …" in a card, "Use score and comment" (primary; fills the score and the comment; toast "Score and comment filled in. Check them, then Save & next."), "Try again", and in the demo "Sample output. AI didn’t respond." Drawn only while "Let AI read student answers" is on | T-R4.28 (score buttons 0..max in 0.5 steps, comment with snippets); bands and rubrics are R6 (T-R6.7, T-R6.11) | new (AI); the band form is R6 | server S, web S | T-D4.32, T-D4.36 |
| V-08 bank header | T 1077-1086 | New question, Share | "Clean up" (`wand-sparkles`, accent ink) and "Generate" (`sparkles`) before Share, outline 36px | T-R4.32 | reopens R4 + new (AI) | XS | T-D4.34 |
| V-09 builder title bar | T 1619-1631 | Question settings, Versions, Preview, Publish | "AI check" (`scan-search`) and "Generate" before Preview; the actions row is `flex-wrap: wrap` | T-R4.31a ("keeps one row wherever the deck does") | reopens R4 + new (AI); the one-row measurement is redone with two more buttons | XS | T-D4.34 |
| V-10 question editor | T 4560-4607, 4649-4681; `qai` T 9493-9515; `aiExplain` T 9435-9442; `aiDistract` T 9443-9450 | Add option; the Explanation textarea in a `<label>` | For single and multiple choice, beside Add option: "Suggest wrong options" (30px, muted; "Thinking…" with a spinner) and a panel "Suggested wrong options" (accent-soft) with up to four rows `{text, why}` and an "Add" button each (fills the first empty option or appends; "Added" after; the deck's toast "A question can have up to 6 options. Remove one first." is DG-63's 8 in the product), "Close suggestions", and an error line "AI didn’t respond. Try again in a minute.". Explanation: the label row gains "Write with AI" (28px; "Writing…"; "Write again" after) and the textarea takes `aria-label="Explanation"`; a panel "Suggested explanation" with the text (14px) and "Use this" (primary), "Shorter", "In Vietnamese" / "In English" (the other language than the preference), "Discard". Both guarded: "Write the question first", "Mark the correct answer first" | T-R4.33 and T-R4.66: one `QuestionEditor` for the bank and the builder (DG-108), so both pages gain it | new (AI) | server S, web M | T-D4.32, T-D4.35 |
| V-11 "Read with AI" on the import page | T 5015-5033; `aiImp` T 9491 | One accepts line, scanned PDFs refused | A toggle card (1px border, radius 10, padding 12px 14px; a 32px sparkles tile; `role="switch"` 38 × 22): "Read with AI", "Handles scans, photos of printed tests and unusual layouts. About 400 credits per page."; on: "Accepts .docx, .doc and .pdf files up to 25 MB each, including scanned or photographed PDFs. Files with a password or macros are refused."; off: the fourth export's line. It mirrors the Settings preference "Read imports with AI" | T-R4.36; the `PDF_NO_TEXT` refusal (`17-word-import.md` §1.39); `18-word-import-ux.md` §4 (say where processing occurs and what is sent) | new (AI) | server L, web S | T-D4.33, T-D4.39 |
| V-12 "Make a variant" | T 6206-6209; `AI_K.variant` T 7649-7661 | Version history, Share, Open builder, Assign | "Make a variant" (`sparkles`) before Share. Its panel: "A second version for another class or a retake, so students can’t share answers."; chips What to change (Reword questions, Reorder options, New wrong options; the first two on), Difficulty (Keep the same, A little easier, A little harder); an info callout "Answers, points and passages stay the same. The variant is saved as a draft version, so nothing changes for students until you publish it."; steps Reading the test / Rewriting questions / Checking that every answer still matches; items tagged "Q{n}" and Reworded / Options reordered / New wrong option with a struck-through before and an after; primary "Save {n} changes as a draft" → "Draft version saved. Review it in Version history before publishing." | T-R4.30; a test has one draft (T-R4.16) | new (AI); "saved as a draft version" is not taken (DG+16) | server M, web S | T-D4.32, T-D4.34 |
| V-13 Settings › AI assistant | T 3861-3921; `aiu`, `aiLang`, `aiPrefs`, `setAi`, `setTabs` T 9484-9490 | Five sections | A sixth section "AI assistant" (`sparkles`) after Appearance. Card "AI usage this month": "Your school gives each teacher a monthly allowance. It resets on 1 October."; "{used} of {limit} credits" (22px) and "{left} left"; an 8px accent bar; a bordered list of five rows with icons: Generating questions and variants, Grading suggestions, Explanations and wrong options, Reading imports, Checks, word lists and insights. Card "Preferences": "Applies to every AI feature in your workspace. AI output is always a draft until you approve it."; "Explanations and feedback in" English \| Tiếng Việt ("You can still switch language on any suggestion."); switches (38 × 22) "Tag AI drafts" ("Adds an “AI draft” tag until you edit or approve the item. Only teachers see it."), "Let AI read student answers" ("Needed for grading suggestions and class insights. Student names are never sent."), "Read imports with AI" ("… About 400 credits per page."). No request control, though the Admin page lists requests (DG+19) | T-R4.43 (sections, `/teacher/settings/:section`), T-R4.7 `UserPreferences` | new (AI) | web M | T-D4.38 |
| V-14 the AI panel | T 6540-6750; `aiVals` T 9470-9591; `AI_K` T 7577-7663; `aiOpen`, `aiStart` T 9405-9429 | not drawn | A right-side `role="dialog"` named by its title, `min(560px, 100%)` wide, full height, `--card`, a left border, `--shadow-lg`, over a `--overlay` backdrop (z 75 / 76). Header (padding 16px 16px 16px 18px): a 36px accent icon tile, the title (15px 600), the sub (12.5px muted), Close (32px). Body (padding 18, gap 18) in three stages. **Setup**: fields by type: source chips (28px pills with an icon: "Passage 1 · Urban green space", "Start empty"), a textarea (13.5px, `--bg`), a segmented control (30px), toggle chips (30px pills, `aria-pressed`, a check when on), a toggle card (border, radius 10), an info callout (`--muted`, `info` icon). **Running**: three step lines (a check in `--success`, a spinner, or a 7px dot) advancing every 1.3s, then a pulsing skeleton (`qz-pulse`); a run lasts at least 3.2s. **Done**: in the demo a warning callout "AI didn’t respond, so this is sample output. Try again in a minute."; a summary line (13px muted) and "Select all" / "Clear selection"; a dashed empty card; item cards (border, radius 12, padding 14; `--ring` border when selected): a `role="checkbox"` "Keep this" (18px) when selectable, tone pills, a right-aligned meta, a 14px 500 title, a 13px body, lettered options (the correct one in success tones with a check), an "Answer" pill, a before / after grid (the before struck through for a variant), dashed "+ {tag}" chips, a note with a bold label, action buttons (30px; the primary filled). Footer: a credit note with `coins` ("About {cost} credits · {left} left this month"; after a run "Used about {cost} credits"; or the validation sentence with `circle-alert` in warning ink), a secondary (Cancel / Stop / Start over), a primary (the run label with `sparkles`; after a run the apply label, at .5 opacity until something is selected; "Done" when the kind applies nothing). No Esc, no focus trap, no focus return (DG-36) | nothing in the product | new (AI) | web L | T-D4.34 |
| V-14a Generate questions (`gen`) | T 7577-7605 | — | "Generate questions" / "From a passage, a transcript or a topic. Questions arrive as drafts for you to check.". Fields: Source (8 rows; chips "Passage 1 · …" with the test's passage, "Start empty"; placeholder "Paste a passage or transcript, or describe a topic"), Question types (Single choice, True / False / Not given, Fill in the blank, Short answer; default single and tf), How many 3 / 5 / 8 (5), Level B1 / B2 / C1 (B2), "Write an explanation for each question" / "Students see it after results are released." (on). Validation: "Add a source of at least a few sentences" under 40 characters, "Pick at least one question type". Steps: Reading the source, Writing questions, Checking every answer against the source. Items: tags the type, the level, "AI draft" (when tagging is on); the prompt; options with the key marked; an "Answer" pill for blank and short; the explanation as a note. Primary: in the builder "Add {n} to {section}" (appends the picks to the open question's section and opens the first; toast "{n} questions added to {section}. Check each one before publishing."); in the bank "Save {n} to the bank" ("… with an AI draft tag"); from insights "Save as practice test" ("Practice test with {n} questions saved as a draft in Tests"). Cost 1,800 | the section and bank question writes exist; `tfng` is R6 | new (AI); TFNG is offered once R6 ships | — | T-D4.32, T-D4.34 |
| V-14b Check this test (`check`) | T 7606-7617 | — | "Check this test with AI" / "Looks for answer keys that don’t match the passage, unclear wording and questions that give answers away.". Look for: Wrong or doubtful answers, Unclear wording, Answers given away, Spelling and grammar (all on); info "Checks the questions in “{title}” against their passages. Nothing changes until you apply a fix.". Steps: Reading both passages, Checking answers against the text, Looking at wording and options. Items: tags Likely error (danger) / Worth a look (warning) / Minor (info) and "Q{n}"; meta the type; title the question's; body the issue; note "Suggested fix"; actions "{fix label}" (applies and toasts), "Open question", "Dismiss". Summary "{n} things worth a look"; empty "Nothing left to look at. The test is ready to publish."; no primary. Cost 2,400 | the draft read; `updateQuestion` for a fix; the demo's fixes name R6 types and `maxWords` | new (AI); fixes whose write exists are one click, the rest open the question | — | T-D4.32, T-D4.34 |
| V-14c Clean up the bank (`dups`) | T 7618-7633 | — | "Clean up the question bank" / "Finds near-duplicates and questions with missing tags, so search and filters work better.". Look for: Near-duplicates, Missing tags; info "Scans the questions you own. Questions shared with you are skipped, and nothing changes until you confirm.". Steps: Reading 1,284 questions, Comparing similar questions, Suggesting tags. A duplicate card (not selectable): "Possible duplicate", meta "Used in 4 tests and 1 test", the reason as title, One / Other, actions "Merge into one" (toast "Merged. Tests that used either question now share one copy.") and "Keep both". A tag card: "Missing tags", the level, the prompt, dashed "+ {tag}" chips; primary "Add tags to {n} questions". Empty "All done. Your bank is tidy.". Cost 3,200 | `listQuestions` with `Own()`; the bulk tag write (T-R4.32); no merge operation | new (AI); no merge (DG+16): a duplicate card has "Open" for each and "Keep both" | — | T-D4.32, T-D4.34 |
| V-14d Generate words (`vocab`) | T 7634-7648; the button T 3678-3682 | — | "Generate words from a text" / "Picks the words your students need to understand a passage, with meanings and new example sentences.": Text (7 rows), How many words 8 / 12 / 20, Students’ level A2 / B1 / B2, Meanings in Tiếng Việt \| English; items the term, part of speech and level as tags, the meaning, the example as a note; primary "Add {n} to {list}". Cost 1,200 | word lists are R10 (T-R10.4, T-R10.12) | R10: the entity does not exist before it | — | R10 (T-D4.19: T-R10.12 gains the button and the kind) |
| V-15 class detail: tabs | T 4145-4155; `cdTabs`, `cdOverview`, `cdAttTab` T 9655-9658 | One screen | Underline tabs Overview \| Attendance under the header; Attendance carries an accent badge with the number of sessions not yet taken | T-R4.42 (one screen); attendance is R8's | R8 | S (R8) | R8 (T-D4.19) |
| V-16 class detail: join-code card | T 4287-4345; `cdCodeFull`, `cdLink`, `cdQrOp`, `cdShowQr`, `cdShare` T 8328-8331; `VIA` T 8303; `fresh` T 8320 | "••••-{last 4}" in a box, "The full code is shown only once, when it is created."; the paused paragraph; link rows; the New-code dialog "Copy it now. After you close this, only the last 4 characters are shown."; "Joined via" "Code ••{hint}" | A 112px QR button (`qr-code` 88px, `cursor: zoom-in`, `--qr-bg`; "Show full size for the projector"; opacity .4 while joining is paused) opening a 420px dialog "Join {class}" ("Show this on the projector. Students scan it or type the code." or "Joining is paused, so this code won’t work until you turn it back on."); beside it the full code (22px monospace, 600, letter-spacing .08em) with the join link under it (12.5px muted, ellipsis) and a "Joining paused" chip (`pause`, warning tones) in place of the paragraph; a row of three outline buttons (32px, flex 1, ellipsis) Copy code (`copy`), Copy link (`link`), Download QR (`download`); Expires and Uses under a 1px divider; the footer "A new code makes the old code, link and QR stop working."; the New-code dialog's desc "Share the code, the link or the QR with your students."; members' "Joined via" shows the full code | T-R4.42 built DG-01 (the full code, no "shown only once") in the fourth export's geometry | reopens R4 (T-R4.42); **DG-01 is Answered** by v2 | S | T-D4.23 |
| V-17 class detail: Attendance tab | T 4359-4488; `attNewVals` T 9636-9695 | not drawn | A session picker (`calendar-days`, "{day} · {time}", chevron; a 260px menu titled "Sessions": each session with Today / Saved / Not taken), a status chip (Not taken yet / Not taken / Saved {time}); a 240px search "Search students"; "Mark all present" or "Mark the rest present". A card: a progress row "{k} of {n} marked" / "All {n} marked" with a three-segment bar (present / late / absent) and legend filters (Present, Late, Absent, Not marked); a selection bar on `--primary` ("{n} selected", Mark present, Mark late, Mark absent, Clear selection); a 40px header on `--muted` (Select all with an indeterminate `minus`; Student; "This term" from 640; Status; Note; the three from 760); rows (min 56px): a checkbox, avatar and name, the rate in danger ink under 80%, a `radiogroup` "Attendance" of Present / Late / Absent (68 × 30px, each in its tone when chosen), a Note button (`message-square-text` with the note, or "+ Note"; a dialog "Note for {name}" / "Only you and co-teachers see it.", placeholder "e.g. Left early for a doctor’s appointment"); empty "No student matches “{q}”" / "Nobody here yet."; footer "Students see their own attendance. Late counts as present in the attendance rate.". A sticky bar (12px from the bottom): "{n} changes not saved · {m} not marked yet", Discard, "Save attendance"; "Save with {m} not marked?" / "They count as absent until you mark them." / "Save anyway"; toast "Attendance saved · {a} absent, {l} late" | R8 T-R8.6 (present / late / absent / `unmarked`; "No student operation exposes attendance in R8"), T-R8.12 | R8; two sentences contradict R8's decisions (DG+17) | M (R8) | R8 (T-D4.19) |
| V-18 Attendance page | T 3189-3235; `attNewVals` T 9641-9653; `ATT_*` T 7665-7667 | One class's register at 960: "{class} · {date}", Change session, Save, a counts bar with Mark all present, a segment per row | A cross-class list at 1320: "Take the register for today’s classes, or catch up on sessions you missed."; three cards "Today · {date}" ("{n} classes"), "Needs a register" ("Past sessions with no attendance taken"), "Recent" ("Last 7 days"); rows: time (13.5px 600) over the day ("Today"), the class name, a meta ("Room 2 · 24 students", or "{p} present · {l} late · {a} absent" once saved), a status chip (Not taken yet, warning; Not taken, danger; Saved, success), "Take attendance" (primary, `calendar-check`) or "Open"; empties "No classes today.", "All caught up.", "Nothing yet."; a row opens the class's Attendance tab on that session | R8 T-R8.12 | R8; T-R8.12's screen no longer exists | M (R8) | R8 (T-D4.19) |
| V-19 word list progress | T 3689-3714; `wlProgVals`, `wordCounts`, `WBAND` T 7664, 9592-9609; `paginate` T 8379 | Four stat tiles (Words, Students practising, Mastered on average, Hardest word) | One card: "{p}% mastered on average", a three-segment bar (mastered `--success` at 70% or more of students, learning `--warning` 50–69%, hard `--danger` under 50%), legend filters with counts that filter the word table (`wordF`, the page resets), "Practising {a} / {b}" after a divider; "Generate from text" (`sparkles`) before Add word | R10 T-R10.12 | R10 | S (R10) | R10 (T-D4.19) |
| V-20 prototype plumbing | `AI_*` T 7413-7663; `aiAsk` T 9399-9404 (`window.claude.complete`); `AI_USE0`, `AI_LIMIT` 50,000; `aiSpend`; `AI_WAIT` 3.2s | — | The prototype calls a browser-side model with inline prompts and falls back to demo arrays when it fails; usage is faked; a run waits 3.2s. None of it ships; the prompts are evidence of what each feature sends (section G) | — | nothing | — | T-D4.1 (README) |
| V-21 persona renames | T 111-131, 215, 3931, 7065, 7782, 8143, 8202, 8278, 8592, 8828; the Student, Admin, Landing and Sign in pages | Hoàng Thương (HT, thuong@), Trần Quang (TQ), "Ms Thương" | Giàu Bùi (GB, giau.bui@), Thường Võ (TV, thuong.vo@); the honorific is gone; the Landing's testimonials name Thường Võ as "Centre manager" | fixtures; DG-22, DG-71 and DG-106 quote "Ms Thương"; DG-100 wants consent for testimonials | nothing; a one-word note on three rows | XS | T-D4.1 |

### 8. Admin, v2

| ID | Where (v2 A) | Fourth export | v2 | Plan today | Verdict | Size | Task |
|---|---|---|---|---|---|---|---|
| A-01 nav separator | A 79-82; A 1803 | — | As V-01 | T-R5.20 on R4's `Sidebar` | built with V-01 (one component) | — | T-D4.20 |
| A-02 nav "AI credits" | A 1821 (`item('ai', 'AI credits', 'sparkles', badge)`); titles and `r.ai` A 1825-1826 | System: Audit log, API reference, Settings | System: **AI credits** (`sparkles`, a badge with the pending requests), then the three; route `ai` | T-R5.20's nav list | R5; the page is new | — | R5 T-R5.35 (new); T-D4.19 corrects T-R5.20 |
| A-03 Audit log filters | A 681-724; `auditUiVals` A 1790-1802; `inRange`, `aFiltered`, `_auditEmpty` A 1836-1841 | A static search "Search person or entity" and a static "Last 7 days" chip | A live search (36px, `flex: 1 1 260px`, max 320) "Search person, action or entity" with `aria-label="Search audit log"` and a clear button, matching actor, action and entity; a "Date range" menu button (`calendar`, the label, chevron, `--shadow`): Today, Last 7 days, Last 30 days, All time, and "Custom range…" (`calendar-range`) opening a dialog From / To (date fields, "Must be on or after the start date") whose label reads "{d Mon} – {d Mon}"; "Reset" while a search or a non-default range is set; the empty row "No entries match these filters."; the pager counts the filtered rows | T-R5.12 (`range=7d\|30d\|year\|all`), T-R5.26 ("a range select … in place of the deck's static date button"; the search is "the deck's "Search person or entity""), DG-19 | R5: the contract's `range` set and the control change | S (R5) | R5 (T-D4.19 corrects T-R5.12, T-R5.26) |
| A-04 settings nav sticky | A 894 (`setNavPos`) | static | `sticky; top: 16px` from 768 | T-R5.28a | R5 | XS (R5) | R5 (T-D4.19) |
| A-05 AI credits page | A 1011-1125 (`r.ai`); `aiAdminVals` A 1666-1789; `AI_POOL` 200,000, `AI_DAY` 24, `AI_DAYS` 30, `AI_TEACHERS`, `AI_REQ0` A 1550-1559 | not drawn | Header "AI credits" / "Monthly allowances for AI features. Usage resets on 1 October." with "Add credits" (a dialog: Amount 50,000 / 100,000 / 250,000; "Added to the centre pool right away and billed on your next invoice. Added credits don’t carry over to next month."). A pool card: "Centre pool · September" and "Centre plan 200,000 (+ {n} added) · resets 1 October"; "{used}" (28px) "of {pool} credits used"; a 10px bar (accent; warning from 75%; danger from 90%) with a 2px projection tick ("Projected by 30 September"); three facts in a grid: "Left this month" ("{d} days to go"), "On pace for ≈ {n}" ("by 30 September, within the pool", or in warning ink "More than the pool. Add credits or lower allowances."), "Teacher allowances {sum}" ("Can add up to more than the pool, since few teachers use all of theirs"). Request cards (a `hand` tile in warning tones): "{name} asked for {n} more credits · {used} of {allow} used", the reason and age, "Decline" (toast "{name} will be told the request was declined"), "Approve {n}" (primary; raises the allowance; toast "{name}’s allowance is now {n} this month"). A Teachers card ("{n} teachers used AI this month · 3 haven’t yet"): rows sorted by used / allowance: avatar, name, email, "{used}" and "of {allow} (· custom)" over a 6px bar, a status pill with a dot (On track, Near limit from the warn percent, At limit, Paused), "Edit allowance" (pencil; a dialog "AI allowance · {name}": "{used} credits used so far this month.", Monthly allowance Default · {n} \| Custom, Credits per month ("Enter at least 1,000"; "Takes effect right away. Usage so far still counts."), "Pause AI for this teacher" / "AI buttons stay visible but can’t be used until you turn this off."). A Rules card (`flex: 1 1 320px`, sticky from a content width of 900): "For every teacher without a custom allowance."; Monthly allowance per teacher 20,000 \| 50,000 \| 100,000 ("Teachers see their balance in Settings → AI assistant."); Warn the teacher at 80% \| 90% ("They get a notification and an email."); When a teacher runs out: Pause until next month \| Let them ask for more ("Requests appear at the top of this page."); When the centre pool runs out: Pause AI for everyone \| Allow 10% extra ("Extra use is billed on the next invoice.") | nothing; no Admin console before R5 (T-R4.48 deletes the old `AdminLayout`, T-R5.20 builds the new one) | new (AI); "Allow 10% extra" and the two "billed" sentences contradict D21 and the spec's non-goals (DG+15) | server L (D4), web M (R5) | T-D4.31 (operations and the maintenance command); R5 T-R5.35 (the page) |
| A-06 Overview rows | A 1770-1773 | — | An "AI credits" row ("{used} of {pool} this month", accent) appended to the Storage meters and to Plan & usage; an attention item (`sparkles`, accent tones) "{n} teacher(s) asked for more AI credits" → `ai` while requests are pending | T-R5.18, T-R5.21, T-R5.28b | R5 + new | XS (R5) | R5 (T-D4.19) |
| A-07 fixtures | A 101-124, 1506-1507, 1521-1545, 1779 | — | Persona renames; the export-all email | — | nothing | — | — |

### 9. Student, Landing and Sign in, v2

Fixtures only (S 300-306, 335-341, 596, 855, 865, 1397-1412, 1439, 1748-1752, 1774; Landing
367-401; Sign in 91, 220-221): the teacher is "Giàu Bùi" with no honorific, the admin "Thường Võ",
the demo accounts `giau.bui@` and `thuong.vo@`. Nothing to build: the product says "Your teacher"
(DG-106) or the display name a teacher sets (T-R4.7).

### 10. Counts and what they mean for D4

| Class | Rows | Where they go |
|---|---|---|
| Reopens R4 | V-01, V-02 (nine R4 tasks), V-03 (a check), V-04, V-05, V-08, V-09, V-16; A-01 | Section F: T-D4.20 to T-D4.23; the four header buttons with their AI tasks |
| Inside a screen D4 already reopens | V-02's outline and V-09's title bar (T-R4.31a, which T-D4.2a reopens) | T-D4.20 runs first on the builder chain; T-D4.2a's comparison includes the scrolling outline |
| A later release's screen | V-14d, V-15, V-17, V-18, V-19; A-02, A-03, A-04, A-06 | T-D4.19 corrects `75-r5.md`, `78-r8.md`, `80-r10.md`; nothing is built early (R8's register needs sessions, R10's word list its schema, the Admin page R5's shell) |
| New capability | V-06, V-07, V-08, V-09, V-10, V-11, V-12, V-13, V-14 (five kinds); A-05 | Section G: T-D4.30 to T-D4.40 |
| Nothing | V-20, V-21, A-07 | T-D4.1's README notes |

**Deck ahead of the engine.** Every later-release row is a screen production lacks and will lack
when D4 ships; that is the standing state of the deck for R5 to R11, not the case the branch rule
guards (a shipped screen redrawn in the next revision). v2 adds thirteen such cases, the R4 rows,
all handled as before: the import lands on `work/deck-d4` and `develop` keeps the third import until
D4 merges.

### 11. Contradictions with recorded decisions, and their resolutions

| v2 draws | Recorded decision | Resolution |
|---|---|---|
| Cloud AI processing | Spec "Changes since v0.7": cloud and private processing "remain candidates … not enabled by default"; `17-word-import.md` §8; `18-word-import-ux.md` §4 | D19 supersedes it for the v2 features; the spec is edited by T-D4.40, in the release that ships the behaviour |
| "Allow 10% extra"; "Extra use is billed on the next invoice."; "billed on your next invoice" | D21 (never overspent without an approval); spec §1.3's non-goal on payments | One pool rule, "Pause AI for everyone"; "Add credits" is the approval and records the approver; the copy says the extra is paid for outside Quizzivy (DG+15) |
| "They get a notification and an email." | Email is R7 (D9) | In-app in D4; the email channel joins in R7 (T-R7.23 gains `ai.allowance_warning`) |
| "A question can have up to 6 options." | DG-63: 8 options | 8 (a DG-63 note) |
| "The variant is saved as a draft version" | One draft per test (T-R4.16) | A new draft test, "{title} · Variant", through the duplicate path (DG+16) |
| "Merge into one. Tests that used either question now share one copy." | No merge operation; frozen copies in published versions (`publish_snapshot_test.go`); `questions.LockForDraftUse` | Not built; "Open" and "Keep both" (DG+16) |
| "Add {n} to {section}", "Save as practice test", "Use score and comment" | The AI never writes a grade or a question on its own (D19 to D22, AGENTS.md) | The primary is the acceptance: the browser then calls the existing writes with the accepted content; the AI module has no content write path |
| "Scores against the four IELTS criteria." | Bands and rubrics are R6 (DG-61, DG-86) | D4 suggests a score in the card's set; the band form joins in R6 (T-R6.11 note) |
| "True / False / Not given" in Generate; "Change to Not given", "Add word limit", a matching finding in Check | `tfng`, `maxWords` and matching are R6 | D4 offers the five stored types and the fixes whose write exists; the rest joins with R6 (T-R6.10 note) |
| "Students see their own attendance." | T-R8.6: no student operation in R8 | R8 decides (DG+17) |
| "They count as absent until you mark them." | DG-92 / T-R8.6: `unmarked` is outside the rate | R8 keeps `unmarked` (DG+17) |
| No AI row in the roles matrix | 70 §4.1: every grantable key is a matrix row | `ai.use` and `ai.credits.manage` join the catalogue and the matrix; the rows are asked for (DG+18) |
| No "ask for more credits" control on the teacher side | — | A request form under the usage card, extrapolated from the Admin card (DG+19) |
| The fourth export's class fixtures | DG-01 | **Answered** (the card shows the full code, link, QR and the share buttons) |

---

## A. The import

T-D4.1 is the first pull request into `work/deck-d4`. It changes eight files of fifteen: the five
pages v2 touched, `github.md` (the fourth export's), and the manifest; the other seven are
byte-identical to the third import. No test reads a deck file (`grep "design/deck"` under `web/`
and `scripts/` finds the token comment, the CI plan test and the check script only), so the import
cannot turn a unit suite red. From its merge the engine on `work/deck-d4` is behind the deck of
record until section B ships, and the four R4 screens of section F and the AI surfaces of section G
until those ship. That state never reaches `develop`.

### T-D4.1 — Import the v2 export
**Depends on:** v0.10.0 released
**Touches:** `docs/design/deck/{Quizzivy Teacher.dc.html,Quizzivy Admin.dc.html,Quizzivy Student.dc.html,Quizzivy Landing.dc.html,Quizzivy Sign in.dc.html,github.md,MANIFEST.sha256}`, `docs/design/README.md`, `docs/design/gaps.md`, `scripts/check-design-deck.mjs`, `AGENTS.md` ("Redesign in progress"), `docs/plan/74d-d4-deck-update.md` (the gap ids)
**Size:** S
**Done when:**
- [ ] The seven files are copied byte for byte from `origin/uiux/update-uiux-v2` @ `26b60795`
      (Thuong's v2 export, 2026-10-10, 10:44, which carries the fourth export of 2026-10-04,
      17:31 in every file it did not touch), and hash to `184536f0…` (Teacher), `8c043d26…`
      (Admin), `dc6d6077…` (Student), `52196e86…` (Landing), `15ab7e2b…` (Sign in) and
      `bf0598f3…` (`github.md`); the full hashes are under "What the export changed" and "What
      the v2 export changed". The other seven files still hash to the manifest. `screenshots/`,
      `uploads/` and `.thumbnail` are not imported. Neither UIUX branch updated the manifest; this
      task regenerates it (Thuong's standing instruction).
- [ ] `MANIFEST.sha256` is regenerated with README's command, and
      `node scripts/check-design-deck.mjs` prints "Deck OK: 15 files, 7 pages".
- [ ] The check script reads `<dc-import name="X">` as a reference to `X.dc.html`: the Teacher
      page now mounts the Student page three times (T 4978, 5756, 5850) and nothing checked it.
      A page that imports a missing page fails the script; the PR shows that failure once.
- [ ] README, "Log": a fourth entry, dated the day of the import and naming the export's date,
      with the two hashes, the sizes (Student 123,460 to 173,426
      bytes, Teacher 781,532 to 794,309) and the screens: Student take test (one data-driven
      screen: sections rail, stimulus pane for a passage, a cloze passage or a recording, the
      phone drawer, the grid button and its sheet, the gap card, matching, the inline blank, the
      "Student view" chip); Teacher builder ("Shared content" in the group menu and under the
      group header, its 560px dialog), and the three previews that now embed the Student page.
      A fifth entry, dated the same day, names the v2 export (2026-10-10, 10:44; `26b60795`), its
      five pages with the hashes and sizes of "What the v2 export changed", and the screens: the
      AI assistant (the panel and its five kinds, "Suggest a score", "Explain results with AI",
      "Write with AI" and "Suggest wrong options", "Read with AI", Settings › AI assistant, Admin ›
      AI credits); the assignment detail's progress card; the grading pane's heights; the
      join-code card; sticky side columns and the collapsed sidebar's separator; the Attendance
      page and the class-detail Attendance tab; the Audit log's search and date range; the word
      list's progress card; and the persona renames.
- [ ] README, "Reading a page": the Teacher page mounts the Student page through `dc-import`
      with the props `exam` and `dark`; on the Student page `renderVals()` is `renderValsBase()`
      overlaid by `examVals()`, which holds every take-test rule.
- [ ] README, "Prototype chrome never ships" and the demo-data paragraphs gain: `previewExam`,
      `builderPreviewVals`, `sharedVals`' `pickFile` (always "part-1-asking-the-way.mp3") and
      `+plays || 99`; the Student fixtures `EXAMS`, `EX_RAW`, `EX_GROUP`, `EX_MATCH`, `EX_UL`,
      `EX_SIGN`, `AUD_LEN`, `MAP_CELLS`, including their answer keys (the seventh tuple entry
      and `ans`), which never reach a student payload; the 45:00 preview timer; the two toasts
      after Submit for s2 and s9; the dead branches `hasPaneTabs`, `dotsMode`, `hasRef` and
      `isSign`; the unread fixture fields `q.orig`, `q.stem`, `printed`, `multiSec`, `pauses`
      and `examBold` (S 1470, on q12); on the Teacher page the script values no markup reads
      any more: `wi.preview`, `wi.pvOptCols` (T 7427-7428), `td.preview`, `td.pvFoot`
      ("Showing 5 of … questions") and `td.pvOptCols` (T 7508-7509). And three slips of the
      prototype a reader must not copy: the arrow keys set the open question without going
      through `go()` (S 1616 against 1639), so they neither close the drawer or the sheet nor
      pause a recording when the section changes; Esc clears the old `sheet` state, not
      `exSheet` (S 1530); in a preview the header still says "All answers saved" and "Saving…"
      (S 961-963, `saved()` in `exPick`, S 1617). From v2: the AI plumbing (`aiAsk` calling a
      browser-side model with inline prompts, the `AI_*_DEMO` arrays shown when it fails,
      `AI_USE0` and `aiSpend` faking usage, `AI_LIMIT` 50,000, the per-kind costs, the 3.2s
      minimum run), the Admin fixtures `AI_POOL`, `AI_DAY`, `AI_DAYS`, `AI_TEACHERS` and
      `AI_REQ0`, the attendance fixtures `ATT_TODAY`, `ATT_ROOM` and `ATT_SES`, and the fixture
      persona (Giàu Bùi, Thường Võ, the demo accounts).
- [ ] README, "Not imported": "ten red-pen markups" becomes eleven, with one new row: "Student
      take test, a cloze: the gap's options cut in one row and the footer's count cut | Options
      wrap in a grid of 132px minimum; the count gives way to a key hint when the rail shows,
      and is a short form otherwise". Under the table, the two defects the drawing shows and
      the designer did not mark, both fixed in the export: the rail's "Grammar and vocabulary"
      overlapping its own second line (S 1011), and the header's "All answers saved" on two
      lines (the label is now `white-space: nowrap` and the leave button `flex: none`, S 952,
      963; the product already truncates the line and pins the button).
- [ ] README gains "Defaults taken with the fourth import (DG+1 to DG+14)", one line per row
      below, and "Defaults taken with v2 (DG+15 to DG+21)".
- [ ] `gaps.md`, existing rows (v2):
      - **DG-01**: **Answered, 2026-10-10**: the class-detail card shows the full code, the link,
        the QR and Copy code / Copy link / Download QR, and the New-code dialog says "Share the
        code, the link or the QR with your students."; the "Meanwhile" rule is deleted (T-D4.23
        builds the card as drawn)
      - **DG-22**, **DG-71**, **DG-106**: a note that the deck's teacher is now "Giàu Bùi" with no
        honorific; the rules do not change
      - **DG-63**: the question editor's AI panel says "up to 6 options"; the cap stays 8
      - **DG-100**: the testimonials now name the owner as "Centre manager"; placeholders still
        stand until consent
      - **DG-36**: the AI panel joins the list of dialogs with no Esc, no focus trap and no return
- [ ] `gaps.md`, existing rows:
      - **DG-80**: the row's first sentence becomes "Since the fourth import the deck draws a
        passage, a cloze passage and a recording as shared content, the sections navigator (a
        rail and a sheet), fill in the blank, an image above the prompt and the phone drawer."
        Its "Meanwhile" keeps every frame of inventory row S-35 and drops, when the D4 task
        for it ships and not before: the place of a shared recording (T-D4.8; its scope
        and sync lines, a group with several recordings and a material that mixes text and
        audio stay listed), "a fill-in with its blanks" (T-D4.6), "gaps
        that lead to their question" (T-D4.10), and the whole "Footer and sheet" sentence but
        "a sealed paper's footer" and "the F key" (T-D4.4)
      - **DG-60**: a note that the Student page now draws TFNG, matching, a two-pick multiple
        choice and the word limit; essay is still undrawn
      - **DG-63**: "MP3, M4A or WAV up to 25 MB" appears a third time (T 1700)
      - **DG-68**: the builder now draws a group's passage and recording; the bank's group
        screens are still undrawn
      - **DG-69**: a third drawing of "Plays", a select with "3 plays" (T 1714-1719)
      - **DG-111**: its "Students can pause is not built" default is reopened by DG+2
      - **DG-113**: the Student page's matching differs from R6's plan (DG+13)
      - **DG-115**: "The Student preview dialog was not redrawn" is answered (DG+7)
      - **DG-116**, **DG-36**: the Shared content dialog and the preview overlay join the list
        of dialogs with no name, no Esc and no focus trap
- [ ] `gaps.md`, new rows, numbered from the first id free in the file on the day of the pull
      request. DG+1 goes in "Decisions that override the current drawings" only if the owner
      confirms Q3; until then all fourteen are Open, with these "Meanwhile" texts:
      - **DG+1** Student › Take test, a recording at its play limit. The deck locks it ("No
        plays left", lock icon, "You have used all {N} plays"). Meanwhile: spec §11.4 stands;
        the button stays live, the status reads "You have used all {N} plays." with the
        product's "You can keep listening. Additional plays are recorded for your teacher to
        review." (T-D4.8). Please redraw the state, or Thuong changes §11.4.
      - **DG+2** "Students can pause" (Teacher: question media and Shared content; Student:
        the inert button and two status lines). Meanwhile: one boolean, `allowPause`, default
        on, on a question's audio and on a group's recording (T-D4.13, T-D4.9, T-D4.15); the
        intro fixture's "you can pause once" is read as a note, not a count.
      - **DG+3** A cloze is one item with n gaps on the Student page and one single-choice
        question per gap in the builder. The deck of record's import preview drew the
        product's model on a student-eye surface (a passage with gap chips and a member
        "23.1 Choose the word for gap 23.1." with its own options); this export removed it
        (T 4975-4980) and left its data in the script (T 7428). Meanwhile: one question per
        gap, each with its own number, square, points and result row; the engine shows a run of
        them as one screen, with the open member's own prompt and worth in the gap card, and a
        member that carries media is its own stop (T-D4.10).
      - **DG+4** Numbers and counts: "Question 23 of 31" against 35; a matching numbered
        "1–5" counts five, another numbered "31" counts one. Meanwhile: every question has one
        number; the cloze screen reads "Questions 23–27 of {total}", which is 34 on the fixture
        paper T-D4.3 builds (s9 without its matching) and 35 once R6 adds the matching.
      - **DG+5** The builder's group now carries shared content. Meanwhile: the deck's group
        is the product's section; shared content belongs to a shared-context group inside it,
        whose row carries the line and the menu item (T-D4.2a, T-D4.14).
      - **DG+6** The Shared content dialog against the product's group: a plain textarea, one
        passage or one recording, "[gap n]" appended at the end, any question as a target, one
        question on two gaps, unbound gaps saved, "MP3, M4A or WAV up to 25 MB", no "Show
        transcript after submitting" and no "Allow skipping ahead", no title for a recording.
        Meanwhile: the content editor, whose toolbar holds the dialog's only "Insert gap"
        (T-R4.63 puts it there for the `document` profile; the deck's separate 30px button is
        not built, so the dialog never has two), a gap at the caret, the product's targets and
        rules, DG-63's limits, both switches kept; a group holding more than the dialog shows
        opens the full pane; the recording's material is named after the file, a name the
        student never sees. The deck's sentence "Students see it on the left while they
        answer." (T 7029) is reworded in English to "Students see it beside the questions
        while they answer.", because a phone shows a drawer, not a left pane (T-D4.14).
      - **DG+7** The three previews embed the take-test screen, answerable, with a running
        45:00 timer. Meanwhile: T-D4.2c builds the bar and the frame around R4's read-only
        preview; T-D4.16 puts the engine in the frame with no saving, no integrity record and
        no play counting. A test has no time limit and no integrity policy: both belong to the
        assignment (`AssignmentInput.durationMinutes`, `IntegrityPolicy`), and `previewTest`,
        the builder draft and the review draft carry neither. So, against the deck: the timer
        pill is a placeholder of the same size reading "––:––", named "The time limit is set
        when the test is assigned"; the save line reads "Answers here are not saved." where
        the deck still says "All answers saved"; pasting is allowed and nothing is counted.
      - **DG+8** Student frames keyed by fixture ids, with no model: the sign card, the 60px
        word tiles, the image caption, the per-question hint, paragraph letters. Meanwhile:
        none is built; a notice is prompt content or an image, an underline is option content.
      - **DG+9** The phone drawer draws no Esc, no focus move, no `inert`, no reduced-motion
        form and no rule for how long the tip is remembered; on a phone the recording is out of
        sight while the student answers. The closed drawer is `aria-hidden` (S 1041, 1700),
        which hides it from a screen reader and leaves its gap buttons in the Tab order.
        Meanwhile: T-D4.7's additions, with `inert` in place of `aria-hidden`.
      - **DG+10** Navigation: the rail comes and goes between questions from 900 to 1179; the
        key hint names "A–D" on every question and omits E and F; the sheet has no title; the
        sheet's 42px squares, the rail's 34px, the 30px handle and the 38px blank are under the
        44px floor below 1024; the rail prints the part of a section title after " · ";
        the arrow keys bypass `go()`, so in the deck they do not close the drawer or the sheet
        and do not pause a recording, and Esc does not close the sheet. Meanwhile: T-D4.4's
        rules; every move, by key or by click, goes through one function.
      - **DG+11** Section instructions show above every question on the Student page while the
        builder says "above the group’s first question"; a group's own instructions are not
        drawn. Meanwhile: every question (T-D4.5); the builder's sentence follows.
      - **DG+12** What spends a play, and unlimited plays ("Play 1 of 99"). Meanwhile: a start
        with no play in progress spends one, a resume none; a play in progress is remembered
        while the tab lives, and a start whose place cannot be put back counts as a play;
        leaving a group pauses its recording however the student left; unlimited reads
        "Play {n}" (T-D4.8).
      - **DG+13** Matching on the Student page: chips per row, three key styles, the nouns
        "pick the {pick} for each {what}", 38px chips. Meanwhile: R6 (T-R6.10 as corrected by
        T-D4.19).
      - **DG+14** The sync that produced this export read two migrations (`github.md`). Please
        read `api/openapi.yaml` (`QuestionGroup`, `StudentGroup`, `AudioPolicy`),
        `web/src/features/take-test/**` and `web/src/features/question-groups/**` at the next
        sync, and add them to the Screen map.
      - **DG+15** Admin › AI credits, the pool's rules. The deck offers "When the centre pool runs
        out: Pause AI for everyone | Allow 10% extra", and says the extra and added credits are
        "billed on your next invoice". Meanwhile: D21 (the pool is never overspent without an
        Admin's approval) and the spec's non-goal on payments: the rule has one value, "Pause AI
        for everyone"; "Add credits" is the approval and records the approver; the dialog reads
        "Added to the centre pool right away. The extra is paid for outside Quizzivy." Please
        redraw the rule and the two sentences.
      - **DG+16** Teacher › "Make a variant" and "Clean up the question bank". The deck saves a
        variant "as a draft version" of the test, and merges two bank questions so that "tests
        that used either question now share one copy". Meanwhile: a test has one draft, so a
        variant is a new draft test named "{title} · Variant", owned by the teacher, with
        "Open the variant" where the deck says "Review it in Version history"; a near-duplicate
        card offers "Open" for each question and "Keep both", and nothing merges (a published
        version holds frozen copies; a merge would need an operation of its own). Tag suggestions
        apply through the bulk tag write.
      - **DG+17** Teacher › Class detail › Attendance (R8). The footer says "Students see their
        own attendance." and the save dialog "They count as absent until you mark them."
        Meanwhile: R8's decisions stand (T-R8.6: no student operation in R8; DG-92: `unmarked` is
        a status outside the rate); the two sentences are not taken until R8 decides.
      - **DG+18** Admin › Roles & permissions. The matrix draws no row for the AI, while the
        product adds two grantable keys. Meanwhile: "Use AI features" in Content (Admin and
        Teacher on) and "Manage AI credits" in System (Admin on), drawn in the matrix's own
        style. Please draw the two rows.
      - **DG+19** Teacher › Settings › AI assistant. The Admin page lists teachers' requests for
        more credits ("{name} asked for {n} more credits"), and the teacher side draws no way to
        ask. Meanwhile: "Ask for more credits" under the usage card (amount and reason, as the
        Admin card shows them), shown while the rule allows requests. Please draw it.
      - **DG+20** Teacher › the AI panel's states. Not drawn: the panel with the provider off
        (the entry points are hidden; the Settings section says "AI is not turned on for this
        school"), a teacher who is paused or out of credits (the deck says "AI buttons stay
        visible but can’t be used": a disabled button with the reason as its tooltip, and the
        sentence "Your allowance is used up. Ask for more in Settings." in the panel's footer), a
        refusal or a timeout (the warning callout, with "Try again", never sample content), a
        suggestion that fails validation, and Esc, a focus trap and focus return (DG-36).
        Meanwhile: those states, from deck parts.
      - **DG+21** Teacher › "Suggest a score", "Check this test", "Generate questions" against
        R6. The deck scores against the four IELTS criteria, offers "True / False / Not given",
        and fixes "Change to Not given", "Add word limit" and a matching heading. Meanwhile: D4
        suggests a score in the grading card's own set (0 to max in 0.5 steps); the band form,
        the TFNG type and the word-limit fix join in R6 (T-R6.10, T-R6.11).
- [ ] `gaps.md`'s preface gains the paragraph for DG+1 to DG+14, in the manner of the one
      for DG-108 to DG-116, and one for DG+15 to DG+21 (v2, 2026-10-10).
- [ ] This file: every `DG+n` label is replaced by the id its row took, in a commit that holds
      nothing else.
- [ ] AGENTS.md, "Redesign in progress": the bullet that says the design team's exports are not
      the deck of record is replaced by "Since the fourth import (D4, T-D4.1) the deck of record
      is the design team's v2 export of 2026-10-10, 10:44, which carries the fourth export of
      2026-10-04. D4 (`docs/plan/74d-d4-deck-update.md`) rebuilds the take-test engine, the
      builder's shared content and the three previews to it, and builds the AI assistant it
      draws."
- [ ] The PR says which screens changed. No file under `web/` or `server/` changes. The pull
      request targets `work/deck-d4`: `develop` keeps the third import until D4 merges.

### T-D4.19 — R5 to R10 plan corrections
**Depends on:** T-D4.1
**Touches:** `docs/plan/75-r5.md`, `docs/plan/76-r6.md`, `docs/plan/77-r7.md`, `docs/plan/78-r8.md`, `docs/plan/80-r10.md`
**Size:** S
**Done when (v2; the one-line head notes are on the branch `docs/d4-deck-v2-plan` already, so this
task completes each into the task's own bullets):**
- [ ] `75-r5.md`: **T-R5.12** `range` becomes `today | 7d | 30d | all | custom` with `from` and
      `to` dates (inclusive, in the viewer's zone) for `custom`, and `q` matches the actor's
      name, the action's sentence and the entity's label; **T-R5.26** draws the v2 filter row
      (the live search with a clear button and `aria-label="Search audit log"`, the Date range
      menu with Custom range… and its From / To dialog, Reset, "No entries match these
      filters."), and drops "a range select … in place of the deck's static date button";
      **T-R5.20** adds "AI credits" (`sparkles`, `ai.credits.manage`, the request badge from
      `GET /admin/ai/credits`) first under System; **T-R5.18** and **T-R5.21** add the "AI
      credits" meter row and the attention item; **T-R5.28a** notes the sticky nav; **T-R5.28b**
      adds the row to Plan & usage; a new **T-R5.35 — AI credits** (M) draws the page of
      inventory row A-05 on D4's `/admin/ai/*` operations, with DG+15's one rule.
- [ ] `76-r6.md`: **T-R6.10** gains "the AI's Generate offers "True / False / Not given" and its
      Check applies "Change to Not given" and "Add word limit" once the types exist (DG+21)";
      **T-R6.11** gains "Suggest a score takes the band form: the four criteria as reasons and the
      band as the score (DG+21)".
- [ ] `77-r7.md`: **T-R7.23** gains the kinds `ai.allowance_warning`, `ai.credit_request` and
      `ai.credit_request.decided` in the Email column.
- [ ] `78-r8.md`: **T-R8.12** describes v2's two screens (inventory rows V-17 and V-18): the
      Attendance page as the cross-class list of sessions in three cards, and the register as
      the Attendance tab of Class detail with the session picker, search, the progress row and
      its filters, selection and bulk marks, notes, the sticky save bar and the "not marked"
      confirm; **T-R8.6** gains a note on DG+17 (no student operation; `unmarked` stays).
- [ ] `80-r10.md`: **T-R10.12** draws the word list's progress card with its legend filters in
      place of the four tiles, and adds "Generate from text" opening the AI panel's `vocab` kind
      (a `POST /teacher/ai/words/generate` operation under `ai.use`, metered as section G
      describes); **T-R10.13** notes the sticky asides.
**Done when (fourth export, unchanged):**
- [ ] `76-r6.md`, the head. Old: "The student engine asks students to "Choose two" and counts
      words against a limit." New: "Since the fourth export (2026-10-04) the Student page draws
      True / False / Not given, matching, a multiple choice capped at two picks and a word
      limit on the take-test screen (DG-60, DG+13)." Old: "cloze groups (question groups,
      unchanged)". New: "cloze groups (question groups; the engine shows a run of gap-bound
      members as one stop, T-D4.10)".
- [ ] **T-R6.10**, TFNG bullet. Old: "under the header "True, false or not given"". New: "with
      the type label "True / False / Not given"".
- [ ] **T-R6.10**, "Choose N". Old: "the header reads "Choose two", with the number in words in
      both locales". New: "the type label stays "Multiple choice"; the hint line reads "Choose
      two answers.", with the number in words in both locales; one pick of two is "Partly" in
      the rail and the sheet (`progress`, T-D4.4)".
- [ ] **T-R6.10**, Matching. Old: "the options are listed once with their roman numerals, then
      one row per lettered item with an option select". New: "one card per item (1.5px border,
      primary once matched, radius 11) with its key, its text and a radiogroup of chips, one
      per option key (38 by 38px from 1024, 44px below it, radius 9); a chip used by another
      item is at 50% opacity, choosing it moves it, and tapping the chosen chip clears it;
      above the cards "{k} of {n} matched"; when the options have text, the question has no
      shared content and the engine is at least 1000 wide, a 220px card lists the options
      once, each with the item that uses it; keys are shown as authored (DG+13); a matching
      is one question, one number and one square, "Partly" until every item is matched".
- [ ] **T-R6.10**, last bullet. Old: "TFNG, "Choose two" and the word counter are compared with
      the deck's fixture paper. Matching and essay frames are listed for DG-80 and DG-60."
      New: "TFNG, the two-pick multiple choice, the word counter and matching are compared
      with the deck's papers s1, s2 and s9. The essay frame is listed for DG-80 and DG-60."
- [ ] **T-R6.10** gains: "`progress()` (T-D4.4) follows T-R6.6's rules for the new types; the
      Submit dialog's sentence becomes the deck's "Unanswered gaps, matches and questions score
      zero." once matching exists."
- [ ] **T-R6.9b**. Old: "The draft preview renders the new types through the engine's
      components." New: "The preview-mode engine (T-D4.16) renders the new types."
- [ ] **T-R6.5** gains: "the projection carries each matching item's and option's key as
      authored."
- [ ] `76-r6.md`, Open items: "True / false answers already marked 0" is removed (fixed:
      `attempts/domain/grading.go:100-110` reads `optionIds`). Added: "Matching on the Student
      page (DG+13): three key styles and the nouns "pick the {pick} for each {what}" have no
      model; default, keys as authored and no nouns." and "A per-question hint line and an
      image caption (DG+8) have no field; default, not built."
- [ ] `77-r7.md`, **T-R7.18**: Touches gains `SharedContentDialog.tsx`, and its read-only
      bullet names the dialog for a Can-use recipient. **O-R7.7** notes that "From Media" in
      that dialog is a third place that opens the media picker.

Also waiting for a later release, with no task here: the sign card, the word tiles, the image
caption and the per-question hint (DG+8, with DG-109's inline media in R6 if at all); a
transcript on the student's result as a drawn frame (DG-106, unchanged).

---

## B. The student engine, brought level with the fourth import

One chain, because every task but T-D4.6 and T-D4.11 edits `Paper` in `TakeTestPage.tsx`:
T-D4.3, T-D4.4, T-D4.5, T-D4.7, T-D4.8, T-D4.9, T-D4.10, then T-D4.12. T-D4.6 and T-D4.11 edit
`QuestionBody.tsx` and run beside the chain after T-D4.5.

Rules for every task in this section (AGENTS.md, "High-risk areas"):

- The five canaries and `pnpm test:unit tests/units/take-test tests/units/integrity
  tests/units/media` run on the branch point and on the last commit, both runs in the PR.
- Never edited: `web/tests/units/media/audio-player.test.tsx`,
  `server/internal/modules/attempts/application/tests/events_test.go`,
  `web/tests/units/api/client.refresh.test.ts`, `publish_snapshot_test.go`,
  `web/tests/integration/router-chunks.test.ts`.
- Frozen unless a task says otherwise: `take-test/{store,draft,strandedDraft,heldSession,groupPlayback,
  groupPlaybackDraft,saveStatus,openQuestion}.ts`, `take-test/useLeave.ts`,
  `features/integrity/**`, the
  four answer shapes of the contract (`choice`, `true_false`, `fill_blank`, `text`;
  `api/openapi.yaml` 2724-2737: a draft or an older attempt may still hold `true_false` with a
  boolean, which `answered.ts:18-19` and `grading.go` read, and `draft-compat.test.ts` is the
  proof it survives), `answered()`'s boolean rule, the flags key
  `sessionStorage['quizzivy.flags.<attemptId>']`, the held-session key
  `sessionStorage['quizzivy.session.<attemptId>']`, written by `getAttempt`,
  `startOrResumeAttempt` and `continueAttempt` and named on every attempt read
  (#337), the draft key
  `quizzivy.answer-draft.<attemptId>`, and the open-question key
  `sessionStorage['quizzivy.open-question.<attemptId>']`, which holds a question's id and is
  written from `TakeTestPage` on every move and read on load (#320, on `develop` since
  `49c9f0e1`). Their suites pass untouched: `open-question.test.tsx`, `draft.test.ts`,
  `draft-compat.test.ts`, `stranded-draft.test.ts`, `submit.test.ts`, `resume.test.ts`,
  `leave.test.tsx`, `leave-events.test.tsx`, `flags.test.ts`, `answered.test.ts`,
  `sections.test.ts`, `timer.test.ts`, `deadline.test.ts`, `end-states.test.tsx`,
  `load-failure.test.tsx`, `held-session.test.ts`, `session-claim.test.ts`,
  `superseded-load.test.tsx`, `deadline-lock.test.ts`, `sequence-anchor.test.ts`,
  everything under `tests/units/integrity/` (including `sequence.test.ts`,
  `shared-audio-record.test.ts` and `timeline-shared-audio.test.ts`).
  `clientSeq` uses an attempt-relative offset calibrated in `getAttempt` from
  `startedAt` and `serverTime`, never below the stored next sequence; it is
  monotonic within a tab, with no cross-tab chronology or uniqueness guarantee.
  The offset component caps at 2,000,000,000, but local increments can exceed it (#338).
  The timeline keeps its session order by earliest received time and session-id
  ties, resume first and takeover last, then differing non-null sequence values
  or event time and id. `integrity_timeline_test.go` and
  `timeline_sparse_test.go` preserve the session and sparse-sequence cases.
  A deadline lock rearms submission at the device's recorded deadline even after
  another refusal, without replacing a closed or superseded lock (#353, #337).
  A superseded tab removes a stored draft only if its own session wrote it,
  because the draft key is shared by every tab of the browser (#337).
- A test that pinned a frame the deck replaced is rewritten to the new frame; no case is
  dropped without a case that replaces it, and the PR lists the pairs.
- Strings vi first, en from the deck, listed in the PR. Removed keys go in their own commit.
- Compared with the deck in a browser at 360, 768, 1024, 1280 and 1440, light and dark, plus
  the boundaries the task introduces, on the fixture papers T-D4.3 adds (the deck's s1, s9 and
  s2 as product sessions). `pnpm e2e:live` (E2E 5 to 9) passes with assertions unchanged and
  only locators moved.
- A group fixture has the shape the server accepts, which ajv does not check: every recording's
  asset is an audio node of one of the group's materials, and every audio node has a recording
  (`group_validation.go:170-177`, 235, 243-246). A listening group therefore always has at
  least one material; "a group with recordings and no material" is not a product state.

### T-D4.3 — Engine: one paper source and one width source (no visual change)
**Depends on:** v0.10.0 released. It changes no drawing, so it does not wait for T-D4.1. It reads 768 only, which `viewport("phone" | "desktop")` answers, and it does not edit `tests/support/viewport.ts`
**Touches:** `web/src/features/take-test/pages/TakeTestPage.tsx`, `web/src/features/take-test/components/{QuestionCard,EngineHeader,Clock,GroupContext,QuestionAudio,SaveState}.tsx`, `web/src/features/take-test/{paper.ts,width.ts}` (new), `web/tests/units/take-test/{support.ts,paper-source.test.tsx (new),deckSession.ts,deck-fixtures.test.ts}`
**Size:** M
**Done when:**
- [ ] `Paper` and every component under it read the paper from one context: the title, the
      questions, sections and groups, the answers and `setAnswer`, the flags and `toggleFlag`,
      the lock and the submit state, the save status, the play counts, and for a shared
      recording `notePlay`, the pending-sync flag and `flush`. `TakeTestPage` fills it from
      `useTakeTestStore`, `useGroupPlaybackStore` and `useSaveStatus`. After this task no file
      under `components/` that `Paper` mounts imports a store: today `Clock`, `EngineHeader`,
      `GroupContext`, `QuestionAudio`, `QuestionCard` and `SaveState` do, and `Paper` itself
      (`TakeTestPage.tsx:347-353`). The stores are not edited. The open question's restore
      and its write (`openQuestion.ts`) stay in `TakeTestPage`, outside the source.
- [ ] The timer comes through the source too. `Clock` reads `deadlineAt`, `offsetMs` and the
      deadline lock from it, and its one-second tick calls a stable `timeLeft()` reader the
      source carries, so it still reads the live store on every tick and never a value
      captured at render (`Clock.tsx:47-63` today). The deadline-lock rule of #329 is not
      touched: `timer.test.ts`, `deadline.test.ts` and `header.test.tsx`'s "the timer" cases
      (373-672) pass with no edit.
- [ ] A question's audio reaches integrity through the source, not by itself: the source
      carries three callbacks per question, `onAudioPlay`, `onAudioEnded` and `onAudioBlocked`,
      which `TakeTestPage` fills with today's calls (`notePlay` and
      `recordAudioEvent(attemptId, "audio_play" | "audio_ended" | "audio_blocked", id)`,
      `QuestionAudio.tsx:34-46`). `QuestionAudio` no longer imports
      `features/integrity`. `question-audio-events.test.tsx` and `audio-events.spec.ts` pass
      with no edit.
- [ ] One hook answers a width question, `useEngineWidthAtLeast(px)`. Under `FocusLayout` it is
      `useMediaQuery`, so `viewport()` keeps working; a provider may replace the source with a
      measured container (T-D4.16). After this task 768 is the only threshold read.
- [ ] `deckSession.ts` gains the deck's other two papers as sessions the contract accepts,
      beside s1, so every later task has something to compare in a browser:
      **s9** without its matching: 34 questions in four sections, the cloze as a group with
      one material holding five gaps and five single-choice members bound to them (all five
      with the deck's one prompt, "Read the passage and choose the best option for each
      gap.", so T-D4.10's screen can be laid over the deck's), and the three statements as
      three-option single choice; **s2** as two listening groups, each with one material that
      holds only its audio node and one recording with `maxPlays` 2: part 1 with five
      single-choice members standing in for the five rows of the deck's matching (R6), so
      part 2 keeps the deck's numbers, 6 to 10: its four single-choice questions and its
      short answer. No fixture carries `ans` or a correct
      index. `deck-fixtures.test.ts` validates all three with ajv and asserts the server's
      group rule in TypeScript (each recording's asset is an audio node of a material, each
      audio node has a recording). Nothing renders them yet.
- [ ] No box, string, role or name changes. Every existing test under `tests/units/take-test`,
      `integrity` and `media` passes with no edit beyond the render helper in `support.ts`;
      `paper-source.test.tsx` renders `Paper` from a hand-built source with no store and
      asserts that an answer, a flag, a recording's play and all three audio callbacks of a
      question reach the source, and that the timer follows a deadline the source moves.
- [ ] `pnpm e2e` (`student-interactions`, `student-engine.phone`, `group-attempt`,
      `stranded-draft`, `integrity-leave`, `audio-events`) and CI's `pnpm e2e:live` are green
      with no spec edited.
- [ ] The engine is opened at 360 and 1280 before and after; the PR's two pairs of screenshots
      differ in nothing.
- [ ] The PR holds no other change (a behavioural change never rides in a refactor).

### T-D4.4 — Engine: sections rail, footer, question sheet and three-state squares
**Depends on:** T-D4.1, T-D4.3. Its tests use the numeric form of `viewport()` that T-R4.3a added (`web/tests/support/viewport.ts`, on `develop` from v0.10.0); this task writes no numeric form of its own
**Touches:** `web/src/features/take-test/components/Navigator.tsx` (rewritten), `web/src/features/take-test/components/SectionRail.tsx` (new), `web/src/features/take-test/pages/TakeTestPage.tsx` (`Paper`), `web/src/features/take-test/answered.ts` (adds `progress`), locales, `web/tests/units/take-test/{navigator,breakpoint,keyboard,submit-dialog,answered}.test.ts(x)`, `web/tests/e2e/{student-interactions,student-engine.phone}.spec.ts`
**Size:** L
**Frozen:** the key handler's rules (arrows stop at both ends; A to E choose; F flags; none
acts in a field, with a modifier, under a dialog or the sheet, or on a locked paper); every
move, by key or by click, goes through the one function that exists today, so nothing the deck's
arrow keys skip (S 1616 against `go()`, S 1639) is skipped in the product; Finish and the sealed
paper's footer; the sheet's focus rules, Esc included (the deck's Esc clears a state the new
sheet does not use, S 1530); `forgetRailWidth`; `answered()`; the header, which this task does
not change (`header.test.tsx` passes untouched).
**Done when:**
- [ ] The rail is the engine's own `<aside>` named "Sections" ("Các phần"): 236px, `--sidebar`,
      a right border, padding 16px 14px, gap 16px, scrolling by itself. It shows when the
      engine is at least 900 wide on a question with no shared content and at least 1180 on one
      with it, and never below 768 (DG+10). It imports neither `SideColumn`, `PageAside` nor
      `useColumnWidth`: `tests/units/student/side-column.test.ts` passes untouched.
- [ ] Per section: its title (12.5px, line-height 1.35; 600 `--fg` for the open question's
      section, 500 `--muted-fg` otherwise) and "n / m" answered (11.5px, tabular); a grid of
      five columns, gap 5px, of 34px squares (radius 8, 1.5px border, 12.5px 600). The title is
      the section's whole title, not the deck's part after " · ". A paper of one untitled part
      shows the grid with no heading.
- [ ] A square: answered is filled primary; partly answered is `--accent-soft` with a primary
      border; empty is `--card` with `--border`. The open question adds the `--accent-c` border
      and `0 0 0 2px var(--accent-soft)`. A flag is a 9px `--warning` dot with a 2px `--card`
      border at the top right. Names keep today's parts ("Câu 23", the part, "đang xem", "đã
      trả lời", "đã đánh dấu") and add "trả lời một phần".
- [ ] `progress(question, answer)` returns `none`, `partly` or `full`. Partly is a fill-in with
      at least one blank typed and not all. `answered()` is unchanged and still equals
      `progress === "full"`; `answered.test.ts` gains the partly cases and loses none.
      `liveAnsweredCount`'s SQL rule is not touched.
- [ ] Legend, pinned to the rail's foot and at the sheet's end, 11.5px: "Answered" (11px
      square, radius 3), "Partly" (11px, `--accent-soft`, 1px primary border), "Flagged" (9px
      round).
- [ ] The footer strip is gone at every width. With the rail: Previous, the key hint centred
      (two 22px `<kbd>` "←" "→", then "to move between questions · A–D to choose an answer", or
      "to move · A–D to choose" beside shared content; 12.5px muted, one line, ellipsis), Next.
      On a question with no options the hint stops at "to move". A to E and F keep working;
      the hint names what the deck names (DG+10).
- [ ] Without the rail, at every width: Previous (icon only below 768), the grid button (flex
      1, 44px, radius 10, 1px border, `--card`, 14px 500, the `layout-grid` icon at 16px), Next.
      Its label is "{no} · {answered}/{total}" beside shared content from 768 and below 420,
      otherwise "Question {no} · {answered} of {total} done"; it truncates with an ellipsis and
      never breaks onto two lines. A screen reader hears the long form and that it opens a
      dialog.
- [ ] The grid button opens the sheet at any width. Below 768: pinned to the bottom, radius 16px
      16px 0 0. From 768: a card 560px wide, centred, 76px from the bottom, radius 14px. Both:
      1px border, at most 80% of the height, padding 12px 16px 20px, gap 16px, the 40 by 4
      grabber. One block per section (title and "n / m"), six columns, gap 5px, squares 42px
      high from 1024 and 44px below it (the floor), 12.5px. No visible title or total: the
      dialog keeps the name "Questions" and the answered total for a screen reader. Focus
      starts on the open question's square, is trapped, Esc and the backdrop close, focus
      returns to the button unless a square took it.
- [ ] The sheet closes if the engine grows past the rail's threshold while it is open.
- [ ] The Submit dialog lists a partly answered question under "Go to", as it already does (it
      is not answered); its counts and sentences do not change.
- [ ] Strings. New: `takeTest.{sectionsRail,railDone,legendPartly,dotPartly,keysHint,
      keysHintShort,keysHintMove,gridLabel,gridLabelShort}`. Removed, in their own commit:
      `takeTest.{navPosition,navAnswered,navCount}`.
- [ ] Tests: `navigator.test.tsx` keeps every case of "the question sheet", "on a paper whose
      time is up", the sealed-paper cases and "the retired rail's remembered width"; its "the
      footer from 768" cases become rail cases at 900 and 1180 and grid-button cases at 768 and
      1024 beside shared content; `breakpoint.test.tsx` crosses 768, 900 and 1180 with an answer
      being typed and keeps it; `navigator.test.tsx:694`, "keeps the title, the save line and
      the timer in the header, and draws no rail and no shortcut hint", keeps its header
      assertions and has its last two inverted (a rail at 900, a hint in the footer);
      `keyboard.test.tsx` is unchanged but for the hint.
- [ ] Compared with the deck's papers s1 and s9 (T-D4.3's sessions) at the five widths and at
      419 / 420, 899 / 900 and 1179 / 1180, light and dark: the rail, its squares in the four states, the legend,
      the hint, the grid button in both label forms, the sheet in both shapes. In Vietnamese at
      320 the grid button stays on one line.

### T-D4.5 — Engine: question pane surface, section header, question line and stimulus head
**Depends on:** T-D4.4
**Touches:** `web/src/features/take-test/pages/TakeTestPage.tsx` (`Paper`), `web/src/features/take-test/components/{QuestionSheet,PassageBody,GroupContext,SectionInstructions (removed)}.tsx`, `web/src/features/take-test/{questionType,sections}.ts`, `web/src/components/shared/content/GroupMaterials.tsx`, `web/src/features/tests/components/PreviewGroupContext.tsx`, locales, `web/tests/units/take-test/{panes,group-context,navigator}.test.tsx`, `web/tests/units/builder/group-preview.test.tsx`, `web/tests/units/i18n/type-names.test.ts` (new)
**Size:** M
**Frozen:** `QuestionCard`'s join to the paper source, `useLanding` (where focus lands), the
gap callbacks of `GroupMaterials`, `PAPER_SURFACE`, `UnknownType`, the order of questions.
**Done when:**
- [ ] The question pane is on `--sidebar` only beside shared content from 768; alone, or on a
      phone, it is on `--bg`. Its column is at most 600px beside shared content and 720px alone.
- [ ] At the top of the question column, on every question: the section's title (12px, 600,
      muted, uppercase, letter-spacing .04em) over its instructions (13.5px, muted), 2px apart,
      12px of padding and a 1px border under them. It is left out on a paper of one section
      that has no instructions, and the instructions line when a section has none (DG+11).
      The first-question note and the headphones note go; `opensSection` and
      `SectionInstructions.tsx` are removed.
- [ ] That section header is drawn by `Paper`, above `QuestionCard`, never by `QuestionSheet`.
      `StudentPreview` stacks every question in one list and mounts `QuestionSheet` for each,
      so a header inside the sheet would repeat a section's instructions above every question
      of the teacher's preview; it keeps its own header, once per section
      (`StudentPreview.tsx:51-58`), until T-D4.16 replaces it. A unit test renders
      `StudentPreview` with a three-question section and finds its instructions once.
- [ ] The line reads "Question 4 of 8 · Single choice": the types are the builder's names. The
      engine keeps its own keys, `takeTest.type.*`, and their values become those of
      `questionEditor.type.*` as T-R4.31b's type menu left them: "Single choice", "Multiple
      choice", "True / False", "Fill in the blank" and "Short answer", with "Một đáp án" and
      the rest in Vietnamese. A value there that still differs from the deck's is set to the
      deck's in both groups. `type-names.test.ts` asserts the two groups are equal in vi and
      in en, so the engine and the builder's type menu cannot drift. Multiple choice keeps
      its instruction sentence as the hint line.
- [ ] The stimulus pane's kicker is the group's title (12.5px, 600, muted, uppercase,
      letter-spacing .04em). Its 22px heading is the title of the group's first material that
      has something to read; when that title equals the group's, the kicker is the section's
      title, so nothing is said twice. A material whose content is only audio nodes has no
      heading at any size: a listening group is a kicker and its recording, as the deck draws
      audio (S 1054-1056: the `h2` is inside `isPassage`), and the name a teacher or T-D4.14
      gave that material (the file's name) is never printed for a student. A group's
      instructions stay under the heading, or under the kicker when there is none (14px,
      muted). Further materials keep their own headings. `GroupMaterials` gains the
      audio-only rule; the audio node itself is still today's label until T-D4.8.
- [ ] `StudentPreview` takes the type names (it shares `QuestionSheet`) and the kicker and
      heading (`PreviewGroupContext` shares `PassageBody` and `GroupMaterials`);
      `builder/group-preview.test.tsx` and `group-preview.spec.ts` follow.
- [ ] The builder's Instructions dialog, which T-R4.31a built with the deck's "Students see
      this above the group’s first question." (T 7030), reads "Students see this above every
      question in the section." in this PR. It is one string in both languages and no builder
      logic (DG+11).
- [ ] `api/openapi.yaml`'s `StudentSection` description ("the engine shows it above the
      section's first question") is corrected in T-D4.13's pull request, or in a contract pull
      request of its own if the owner declines Q4 and T-D4.13 is not built.
- [ ] Tests: `panes.test.tsx`'s cases on the part eyebrow, the listening note and the type
      labels are rewritten; "is the deck's line in English" asserts the new labels;
      `group-context.test.tsx` gains "a material that is only a recording has no heading" and
      "a material with text and a recording keeps its heading", on the validated shape.
- [ ] Compared with s1 question 4, s9 question 5 (no shared content, 720px), s9 question 28
      (shared content, 600px; the deck's 24, four later because the cloze is five questions)
      and s2 question 6 (the kicker with no heading) at the five widths, light and dark.

### T-D4.6 — Engine: answer areas (choice rows and columns, the blank, the short answer)
**Depends on:** T-D4.5 (shares `panes.test.tsx`); otherwise beside the chain
**Touches:** `web/src/features/take-test/components/QuestionBody.tsx`, `web/src/features/take-test/components/blankInputs.ts`, locales, `web/tests/units/take-test/{renderers,panes}.test.tsx`
**Size:** M
**Frozen:** real `radio` and `checkbox` inputs inside their labels (the deck's `<button
aria-pressed>` is not taken); the four answer shapes, of which this file writes three
(`choice`, `fill_blank`, `text`) and must keep reading a stored `true_false` boolean as a
chosen option (`draft-compat.test.ts` and `stranded-draft.test.ts` hold one and pass
untouched); the ids `answer-blank-<id>` that a gap
lands on; the `<textarea>` (line breaks of saved answers); 16px fields below 1024; paste
blocking; the case-rule and worth lines (DG-80).
**Done when:**
- [ ] A chosen option's marker keeps its letter, in `--primary-fg` on `--primary`: no check
      icon. The multiple-choice marker's radius is 7px. Row line-height 1.4.
- [ ] Options are one column, or a grid when every option is plain text of at most 16
      characters and the question has no shared content: two columns below 768, `min(4, n)`
      from 768. An option with rich content forces one column.
- [ ] A blank is the deck's field wherever it stands: no box, a 2px bottom border (`--ring`,
      `--primary` once typed), `--muted` fill, radius 6px 6px 0 0, padding 0 8px, left-aligned,
      placeholder "type here" ("nhập ở đây"), `autocomplete="off"`, `spellcheck="false"`; 38px
      high from 1024 and 44px below; `min(100%, 340px)` wide when the question has one blank,
      128px when it has several or the blank is in a table. A fill-in prompt is 18px (20px
      larger), weight 500, line-height 2.1.
- [ ] The short answer gains `autocomplete="off"` and `spellcheck="false"`; "1 word" is
      singular in English.
- [ ] Larger text: the sentence 20px, options 17px, as drawn (`sentSize`, `optSize`).
- [ ] `renderers.test.tsx` asserts the letter in a chosen marker, the column rule at 359, 768
      and with one long option, the blank's two widths, and that a typed blank still writes
      `{type: "fill_blank", values}`; `draft-compat.test.ts` passes untouched.
- [ ] Compared with s9 questions 1 (four short options: four columns at 1280, two at 360), 5,
      21 and 31 (the blank; the deck's 27), and s1 question 5, at the five widths, light and
      dark. The sign
      card, the 60px word tiles and the sentence gap are known differences (DG+8, T-D4.11).

### T-D4.7 — Engine: the phone drawer
**Depends on:** T-D4.5
**Touches:** `web/src/features/take-test/pages/TakeTestPage.tsx` (`Paper`; `PaneSwitch` and `PaneTab` removed), `web/src/features/take-test/components/{StimulusDrawer.tsx (new),GroupContext.tsx}`, `web/src/features/take-test/panes.ts`, `web/src/index.css` (`qz-nudge`, `qz-tip`), locales, `web/tests/units/take-test/{panes,group-context,navigator,keyboard}.test.tsx`, `web/tests/e2e/student-engine.phone.spec.ts`
**Size:** L
**Frozen:** the integrity monitor and its listeners (the drawer adds pointer handlers on the
panes' container only and never touches `visibilitychange`, `blur`, `copy`, `paste`);
`useKeptScroll`'s promise that a hidden pane keeps its place; the rule that the keys answer and
flag only a question on screen.
**Done when:**
- [ ] Below 768, shared content is a drawer from the left, not a switcher: `min(88% of the
      width, 420px)` wide, over the question, `transform .32s cubic-bezier(.22,.8,.24,1)`,
      `--shadow-lg` while open, a backdrop that darkens to 45% of the overlay token (no literal
      colour: `no-raw-colours.test.ts` stays green).
- [ ] Closed, a handle at the left edge, 42% down: 30 by 84px, radius 0 12px 12px 0, `--card`,
      1px border without the left side, the `file-text` or `audio-lines` icon (15px), a 7px
      `--accent-c` dot while the group's recording plays, `chevrons-right` (14px, muted),
      `--shadow-lg`.
      It is named, in English as the deck has it, "Open the passage" or "Open the audio".
- [ ] Until a group's drawer has been opened once in this attempt, the handle nudges (7px,
      1.4s, three times, after 0.4s) and a tip shows beside it (`role="status"`, primary,
      12.5px, at most 220px): "The passage is here. Swipe right or tap to read it." or "The
      recording is here. Swipe right or tap to open it.". Remembered per group in
      `sessionStorage['quizzivy.stimulus-seen.<attemptId>']`, inside try/catch.
- [ ] Open, a sticky bar: "Swipe left to go back to the question" (12px, muted,
      `chevrons-left`) and a 34px close button.
- [ ] A swipe starts after 10px when it is more horizontal than 1.2 times its vertical travel;
      released past a quarter of the drawer's width it opens, and an open drawer closes when
      dragged back past 30%. `touch-action: pan-y`; a click within 60ms of a drag does nothing;
      a tap on an option is never swallowed (a unit test taps one right after a vertical
      scroll).
- [ ] What the deck does not draw (DG+9): opening moves focus to the close button and closing
      returns it to the handle; Esc and the backdrop close; the closed drawer is `inert` and
      stays mounted, so the place in the passage and a playing recording survive; under
      `prefers-reduced-motion` there is no nudge, no tip animation and no slide. The deck
      marks the closed drawer `aria-hidden` (S 1041, 1700); `inert` is taken instead because
      `aria-hidden` leaves the gap buttons inside it reachable by Tab, and DG+9 records the
      choice. The drawer's own background is `--bg`, stated on the pane (S 1041), since it now
      lies over the question.
- [ ] A gap tapped in the drawer closes it and lands on its question or blank, as today. A
      move by the arrows, a square or "Go to" closes it: all of them go through the engine's
      one move function, where the deck's arrow keys bypass `go()` and leave the drawer open
      (S 1616, DG+10). While it is open the keys that answer
      or flag do nothing. The footer stays where it is, under the backdrop.
- [ ] Crossing 768 with the drawer open shows both panes; the same pane element is used on both
      sides, so nothing inside it remounts.
- [ ] Strings. New: `takeTest.{openPassage,openRecording,drawerHint,drawerClose,tipPassage,
      tipRecording}`. Removed, in their own commit: `takeTest.{passage,paneSwitch}`.
- [ ] Tests: every case of `panes.test.tsx`'s "the panes below 768" is rewritten to the drawer
      (the PR lists each pair); `group-context.test.tsx`'s phone cases follow; pointer events
      drive the three thresholds; `student-engine.phone.spec.ts` swipes for real in
      mobile-chromium, and checks that no sideways scroll appears at 320.
- [ ] Compared with s9 question 23 and s1 at 360 and 767, light and dark: the handle, the tip,
      the open drawer, its bar and the backdrop.

### T-D4.8 — Engine: the recording card in the stimulus pane, and what spends a play
**Depends on:** T-D4.7
**Touches:** `web/src/features/media/components/AudioPlayer.tsx`, `web/src/features/media/playback.ts` (new), `web/src/features/take-test/components/{GroupContext,QuestionAudio}.tsx`, `web/src/features/take-test/panes.ts`, `web/src/features/take-test/pages/TakeTestPage.tsx`, `web/src/components/shared/content/GroupMaterials.tsx`, locales, `web/tests/units/media/{audio-player-card,audio-player-resume}.test.tsx` (new), `web/tests/units/take-test/{group-context,panes,question-audio-events}.test.tsx`, `web/tests/e2e/{audio-events.spec.ts,audio-plays.live.spec.ts,student-engine.phone.spec.ts}`, `docs/quizzivy-spec-v0.3.md` §11.3, §11.4
**Size:** L
**Frozen:** `toggle()`'s order in `AudioPlayer` (`element.play()` first, nothing awaited or set
before it); the player is never disabled by a count; the no-seek guard (`AudioPlayer.tsx:240-247`:
with seeking not allowed, a jump of 0.5s or more is put back and `onSeekBlocked` is called;
the canary's "puts the position back and reports it, because OS controls still seek"),
which keeps refusing every jump a student makes; `groupPlayback.ts` and `groupPlaybackDraft.ts`
(gesture ids, retries, monotonic counts); the `audio_play`, `audio_ended` and `audio_blocked`
records of #314; the one-length rule of #321; the expired-link retry; pausing on a lock.
A rejected play is read by its name: `NotAllowedError` says so under the player
that stays; `AbortError`, including a pause before the start, says nothing (#339).
`audio_seek` is recorded for a question's blocked seek (#339). A shared recording's
end and block carry `meta.recordingId`; known end duration is elapsed device wall
time since the latest start, not total listening time (#340). A resume replaces
that start, and an end or block clears it. The timeline consumes the latest matching
open play once, by question id first or otherwise recording id; valid numeric
0–86,400,000 ms durations are truncated, with missing, malformed or out-of-range
durations falling back to
clamped event-clock spans. Delayed counts or clock skew can leave an end unpaired;
overlapping sessions have no play-id matching. `timeline_shared_audio_test.go`
preserves those pairing rules.
`audio-player.test.tsx`, `audio-player-events.test.tsx`, `audio-player-length.test.tsx`,
`group-playback.test.ts`, `audio-plays.test.ts`, `audio-player-blocked.test.tsx`
and `shared-audio-events.test.tsx` pass untouched.

**The shape this task builds on.** A recording never comes alone: the server accepts a
recording only when its asset is an audio node inside one of the group's materials, and refuses
an audio node with no recording (`group_validation.go:170-177`, 235, 243-246;
`api/openapi.yaml` 2376-2381, "One explicit playback binding per audio asset used in the
group's materials"). So a listening group reaches the student with at least one material
(`tests/support/groupPreview.ts:41-64` is the shape), `hasPassage` is already true for it, and
today the passage pane prints that material's heading and the node's label
(`GroupContext.tsx:62`) while the player sits in the question pane (`GroupListening`).

**Done when:**
- [ ] The card is drawn at its audio node, in reading order, in the stimulus pane (the drawer
      below 768), in place of today's label line: `GroupContext`'s `renderAudio`, which
      already receives the node, the asset and the recording, returns the player.
      `GroupListening` leaves the question pane and its `stimuli.length === 0` branch goes.
- [ ] A material whose content is only audio nodes draws no heading (T-D4.5) and no label: a
      listening group is the kicker and the card, as drawn (S 1054-1056, 1074-1097). A
      material with text and audio keeps its heading and has the card where the node stands.
      A group with several recordings has each card at its own node, and only then is the
      node's label printed above its card (today's 14px line; "Recording {n}" when the label
      is empty), so two cards can be told apart; with one recording nothing is printed and the
      label is the card's accessible name. Never the same recording twice: no list of
      recordings is drawn before or after the materials.
- [ ] A recording no audio node names is a payload the server refuses; if one arrives anyway
      its card is drawn after the materials, so no payload can hide a recording.
      `hasPassage` becomes "has shared content" (`stimuli` or `recordings`), which is the same
      answer as today for every group the server accepts. One unit case keeps the refused
      shape, named for what it is.
- [ ] The shared player stays mounted across the group's questions and across 768 (the pane is
      keyed by the group and is the same element on both sides, T-D4.7). Leaving the group by
      any means (Previous, Next, the arrow keys, a square, "Go to", a gap) pauses its
      recording, because the pane unmounts; the deck pauses on a click that changes the
      section and not on the arrow keys, which is a slip of the prototype (DG+10, DG+12).
- [ ] `AudioPlayer` gains `layout="card"`, used by the engine and the teacher's preview only:
      padding 18px, radius 14, 1px border, `--card`, `--shadow`; a 52px round primary button
      with a 22px icon (play, pause, or `rotate-ccw` after a finished play); a 6px track on
      `--muted` filled `--accent-c`; under it the time "0:00 / 2:34" and, at the far end, "Play
      {n} of {N}" (12.5px, tabular), which reads 1 before the first play; then a status line
      with a 15px headphones icon (13px, muted). The same `toggle()` serves both layouts;
      `audio-player-card.test.tsx` repeats the same-tick assertion for the card.
- [ ] Status and names, as drawn: "Press play when you are ready.", "Playing", "Paused",
      "Finished · 1 play left" (plural in both languages); the button is "Play", "Pause",
      "Resume" or "Play again". With no limit the counter reads "Play {n}" and the finished
      line "Finished" (DG+12).
- [ ] The status line has a place for the blocked sentence (`media.playBlocked`).
- [ ] At the limit the button stays live (DG+1, spec §11.4, the canary's "still plays when
      the hint says none are left"): no lock icon, the status reads "You have used all {N}
      plays." and the product's "You can keep listening. Additional plays are recorded for your
      teacher to review." follows; the counter reads "Played {n}/{N} times".
- [ ] A play is spent when playback starts and no play is in progress; a resume after a pause
      spends none. `AudioPlayer` calls `onPlay` for the first and `onResume` for the second;
      both hosts count on `onPlay` only. With seeking allowed, moving inside a play in
      progress spends nothing.
- [ ] A play in progress and its position are remembered for the tab's life per recording or
      question (`playback.ts`, memory only, cleared with the attempt), so coming back to a
      group resumes where it stopped without a play; a reload forgets, and the next start
      counts. The restore must not meet the no-seek guard, which would put a remounted
      player back to 0:00 and so hand the student a replay that costs nothing:
      - the remembered position is applied when the element has its metadata (at mount if it
        is already there, otherwise on `loadedmetadata`), never inside `toggle()` ahead of
        `play()`;
      - the value the guard compares with is set to the remembered position before
        `currentTime` is, so the guard sees no jump, puts nothing back and does not call
        `onSeekBlocked`: a restore is never reported as a blocked seek;
      - if playback is running and the position is still more than 0.5s short of the
        remembered point (the restore was refused, or the metadata never came), the start is
        counted as a play through `onPlay` and the memory is cleared. A place that cannot be
        put back is never free.
- [ ] Starting one recording pauses any other that is playing.
- [ ] A question's own audio (`QuestionAudio`) takes the same card, in the question pane, with
      the same rules. A recording keeps "Allow skipping ahead": when it is on, the seek control
      lies over the 6px track. The scope line and the sync line with its Retry stay under a
      shared recording (DG-80).
- [ ] Spec §11.3 and §11.4 are changed in this PR: "Each play that starts the recording sends
      an `audio_play` event; resuming a paused play sends none", one instance per recording
      that keeps its place while its group is left, and the rule above for a place that
      cannot be put back. Version bumped per §18.
- [ ] Unit tests, which cannot see playback (jsdom plays nothing, and the existing tests stub
      `play`, `pause` and `paused` only): `audio-player-resume.test.tsx` (a pause and a resume
      call `onPlay` once; a second start after the end calls it twice; a remembered play
      resumes with `play()` in the click's tick and no `onPlay`; an empty memory counts; a
      restore does not call `onSeekBlocked`; a seek by the student with seeking off still
      does); `question-audio-events.test.tsx` gains "a resume records no second play";
      `group-context.test.tsx` gains the card at its node for an audio-only material, a mixed
      material and two recordings, all on the validated shape.
- [ ] `panes.test.tsx`: the fixture at 93-100 (`stimuli: []` with a recording) is retired as
      the listening fixture, because the server refuses it; its case "draws a group that has
      only recordings inside the question pane" becomes "draws a listening group's recording
      in the stimulus pane, with no heading", on T-D4.3's s2 group. The PR lists the pair.
- [ ] Browser proof, because the free replay cannot be seen in jsdom: `audio-plays.live.spec.ts`
      gains (a) a pause and a resume with the server's count still 1, and (b) with seeking
      off: start a group's recording, leave the group mid-play, come back, resume, and assert
      that `currentTime` is past the remembered point and the server's count is still 1. Its
      existing assertions are unchanged. `student-engine.phone.spec.ts` repeats (b) in
      mobile-chromium against the stubbed API, counting the `group-audio-play` requests.
- [ ] Compared with s2 question 1 (part 1's recording: T-D4.3's fixture gives it choice
      members) at the five widths, light and dark: the card at rest, playing, paused and
      finished. Question 6 is the deck's recording that cannot be paused and is T-D4.9's
      comparison. The lock state is a known difference (DG+1).

### T-D4.9 — Engine: a recording that cannot be paused
**Depends on:** T-D4.8, T-D4.13
**Touches:** `web/src/features/media/components/AudioPlayer.tsx`, `web/src/features/take-test/components/{GroupContext,QuestionAudio}.tsx`, locales, `web/tests/units/media/audio-player-pause.test.tsx` (new), `docs/quizzivy-spec-v0.3.md` §11.3
**Size:** S
**Frozen:** as T-D4.8.
**Done when:**
- [ ] `AudioPlayer` takes `allowPause` (default true). When false and playing, the button is
      `aria-disabled`, at opacity .45, and a click does nothing; the status reads "Playing ·
      this recording can’t be paused"; before the first play it reads "Press play when you are
      ready. It can’t be paused once it starts."
- [ ] If playback stops anyway (a headset button, another recording starting, leaving the
      group, a lock), the card reads "Paused" and "Resume" continues without a play. Nothing
      new is recorded: the product reports what it can see and does not pretend the pause
      could not happen.
- [ ] Both hosts pass `policy.allowPause`, which T-D4.13 makes required in the two student
      shapes; a payload without the field (a tab that loaded before the API) pauses freely.
      T-D4.3's s2 session gives part 2's recording `allowPause: false` in this PR, as the deck
      has it (`canPause: false`, S 1503).
- [ ] Spec §11.3 says what `allowPause` does, and what it cannot stop.
- [ ] `audio-player-pause.test.tsx` covers the inert button, the two sentences and the free
      resume; the canary passes untouched.
- [ ] Compared with s2 question 6 (the deck's recording that cannot be paused) at 360 and
      1280, light and dark: before the first play, playing with the inert button, and paused
      by leaving the group.

### T-D4.10 — Engine: a cloze as one stop
**Depends on:** T-D4.5, T-D4.7; the design team's answer to Q6 (DG+3), or the default once the team has had the question for two working days
**Touches:** `web/src/features/take-test/stops.ts` (new), `web/src/features/take-test/components/{ClozeBody.tsx (new),GroupContext.tsx,QuestionSheet.tsx,Navigator.tsx}`, `web/src/features/take-test/pages/TakeTestPage.tsx`, `web/src/components/shared/content/{GroupMaterials.tsx,content.css}`, locales, `web/tests/units/take-test/{stops.test.ts (new),cloze.test.tsx (new),group-context,keyboard,submit-dialog}.test.tsx`, `web/tests/e2e/group-attempt.spec.ts`
**Size:** L
**Frozen:** one `choice` answer per member question, keyed by its id (autosave, drafts, the
stranded-draft save, grading and the result are untouched); one square, one flag and one "Go
to" chip per question; the number of a question is its position; the integrity monitor's
`questionId` is the open gap's question; nothing a member carries is dropped from the screen.

**What a member is.** Every member of a group is a full question: `prompt` is required and not
empty, and it may carry rich prompt content, its own image or audio and its own points
(`GroupQuestionInput` and `QuestionInput`, `api/openapi.yaml` 2399-2413, 3100-3130;
`StudentQuestion` 2112-2144). The deck's cloze has one prompt for five gaps; the product has
five prompts.

**Done when:**
- [ ] `stops.ts` finds a cloze run: two or more consecutive `single_choice` members of one
      group, each bound by a `GroupQuestionGap` to a gap of the same material, and none with
      media of its own. A member that carries an image or audio ends the run and is a stop of
      one question, drawn by `QuestionSheet` as today (a player belongs to one mounted
      question, §11.3, and the gap card draws no media). Everything else is a stop of one
      question. `stops.test.ts` covers a run, a run broken by another type, a run broken by a
      member with an image and by one with audio, a lone bound member, a gap bound to a
      blank, and a paper with no groups.
- [ ] A run is one screen. Its line reads "Questions 23–27 of 34 · Cloze · 5 gaps" on
      T-D4.3's s9 session; the total is the paper's question count (35 once R6 adds the
      matching, DG+4). Then "The gaps are in the passage. Pick a gap here or in the
      passage." (12.5px, muted), a strip of chips (32px, radius 8, a 20px circle with the
      question's number, then the chosen word or "—"; the open gap is filled primary), and a
      card (`--sidebar`, 1px border, radius 12, padding 14px) with "Gap 23" (13.5px 600),
      "{k} of {m} gaps filled", and the open gap's options in `repeat(auto-fill,
      minmax(132px, 1fr))`, gap 8px, rows 46px with the round letter marker.
- [ ] Each member's own words stay on screen. When every member of the run has the same plain
      prompt and none has rich prompt content, that prompt is printed once above the hint at
      17px, as the deck draws its single prompt, and not again in the card. Otherwise nothing
      is printed above the hint, and the card shows the open member's own prompt (plain or
      rich, 15px) between its header and its options. In both cases the card ends with the
      open member's worth line, as `QuestionSheet` prints it (DG-80), so a run whose members
      are worth different points says so gap by gap.
- [ ] Choosing an option writes that member's `choice` answer and opens the next gap with no
      answer. Previous and Next and the arrows move by stop; a square, a "Go to" chip or a gap
      in the passage opens the run on that member's gap. A to E choose for the open gap; F
      flags its question.
- [ ] The open-question key keeps holding a question's id: on a run it is the open gap's
      member, written on every change of gap. A reload reopens the run on the gap of the
      stored question, and `cloze.test.tsx` has the case. `open-question.test.tsx` passes
      untouched.
- [ ] In the passage, a material that holds a gap is drawn in the deck's card (padding 18px
      20px, 1px border, radius 12, `--card`, line-height 2.3). A gap bound to a choice question
      is the chip: at least 76px wide, 30px high, radius 7, 1.5px border, the 20px primary
      circle with the question's number, then the chosen option's text (its plain text, with an
      ellipsis past 160px); empty `--muted` with a `--ring` border, filled `--accent-soft`,
      open `--card` with a primary border and `0 0 0 3px var(--accent-soft)`. A gap bound to a
      blank keeps today's underline and goes to its blank. The chip is drawn for a lone bound
      member too, and in the teacher's preview with no chosen word.
- [ ] The rail and the sheet keep one square per question; the squares of a run all open it,
      and the open gap's square is the current one. No square reads "Partly" for a cloze.
- [ ] The Submit dialog's counts and chips do not change (`submit-dialog.test.tsx` passes with
      one new case: five empty gaps are five unanswered).
- [ ] `cloze.test.tsx` covers both prompt cases (one shared prompt printed once; five
      different prompts, each shown with its gap; a rich prompt in the card), the worth line
      of the open member, and that choosing writes one `choice` answer under that member's id.
- [ ] Compared with s9 question 23 at 360, 768, 1024, 1280 and 1440, light and dark, with no
      gap, two gaps and all five chosen. Known differences, all DG+3 and DG+4: gap numbers
      are question numbers, the line reads a range and "of 34", five squares stand where the
      deck draws one, and each gap shows its own member's prompt and worth where the deck has
      one prompt for all five.

### T-D4.11 — Engine: a choice sentence with its gap
**Depends on:** T-D4.6
**Touches:** `web/src/features/take-test/components/QuestionBody.tsx`, `web/src/features/take-test/sentenceGap.ts` (new), `web/tests/units/take-test/{sentence-gap.test.ts (new),renderers.test.tsx}`
**Size:** S
**Frozen:** the prompt's stored text; the answer shape.
**Done when:**
- [ ] A `single_choice` or `true_false` question whose prompt is plain (no `promptContent`),
      one line, with exactly one run of three or more underscores and no other Markdown
      marker, is drawn as the deck's sentence: 18px (20px larger), weight 500, line-height 1.8,
      the run replaced by a box (at least 88px wide, padding 0 10px, radius 7, a 2px bottom
      border): empty on `--muted` with a `--ring` line, or the chosen option's plain text on
      `--accent-soft` with an `--accent-c` line. A screen reader hears "blank" or the chosen
      text in place.
- [ ] Any other prompt prints as written. `sentence-gap.test.ts` lists the refusals: two runs,
      a rich prompt, a prompt of several lines, multiple choice, a run of two underscores.
- [ ] Compared with s9 questions 5 and 12 at 360 and 1280, light and dark. This task is
      dropped without loss if the answer to Q17 is that the sentence is printed as written.

---

## C. Server and contract

**What needs nothing, with the evidence.** A passage, a cloze and a recording as shared
content: `StudentGroup` carries `stimuli`, `recordings` and `assets` (`api/openapi.yaml`
2146-2165). A gap bound to a question or a blank: `GroupStimulus.gaps` (2326-2371), frozen at
publish (`migrations/00039`) and validated on save (`group_validation.go:176-212`). Plays used:
`groupAudioPlays` and `recordGroupAudioPlay` (6837-6870, `x-permission: learning.take_tests`).
What spends a play is the client's decision; the counter and its description do not change. The
rail, the sheet, numbering, the cloze stop and the drawer read the same session. The preview:
`previewTest` and the browser's own mapping exist, and carry the paper only; a time limit and
an integrity policy belong to the assignment, so the preview shows neither and no field is
added for them (T-D4.16). The Shared content dialog: `updateQuestionGroup` replaces the graph
under a revision, through the group editor that already calls it. No operation is added by any
task in this file, so the isolation suite and the open list do not change.

**What is not built, and has no task:** a per-question hint, an image caption, a sign block,
the word-size option mode, matching's nouns (DG+8, DG+13); a hard play limit (DG+1: it
would make `recordGroupAudioPlay` refuse, which §11.4 forbids).

### T-D4.13 — `AudioPolicy.allowPause`
**Depends on:** T-D4.1 (DG+2's row); Q4: the owner's yes, or the default (a boolean, on by default) if no answer has come when the task starts
**Touches:** `api/openapi.yaml` (`AudioPolicy`, a new `StudentAudioPolicy`, `StudentQuestion.audio`, `StudentGroupRecording.policy`, the `StudentSection` description), `server/gen/openapi/`, `web/src/lib/api/schema.d.ts`, `migrations/NNNNN_add_questions_audio_allow_pause.sql`, `NNNNN_add_test_version_questions_audio_allow_pause.sql`, `NNNNN_add_group_recordings_allow_pause.sql`, `NNNNN_add_test_version_group_recordings_allow_pause.sql` (new; numbered at merge), `server/internal/modules/questions/{domain/question.go,http/{input,questions}.go,repositories/store.go}`, `server/internal/modules/tests/{domain/{draft,publish_validation}.go,http/{tests,preview_groups}.go,repositories/{group_assets,group_frozen,group_read,preview,preview_groups,publish_groups,publish_group_snapshot,publish_load,publish_snapshot,version_draft}.go}` and the group write, duplicate and group-copy paths, `server/internal/modules/attempts/{domain/paper.go,http/{attempts,admin_attempts}.go,repositories/{store,review_review}.go}`, the import commit's default, `seed/`, the student-shaped fixtures `web/tests/support/groupPreview.ts`, `web/tests/units/take-test/{panes.test.tsx,question-audio-events.test.tsx,deckSession.ts}`, `web/tests/units/builder/group-preview.test.tsx`, `web/tests/e2e/audio-events.spec.ts` and the MSW fixtures of a session, `server/internal/modules/tests/application/tests/allow_pause_test.go` (new), `docs/plan/20-data-model.md`, `docs/quizzivy-spec-v0.3.md` §11.1, §13, §15, `docs/plan/74-r4.md` and `docs/plan/70-redesign-overview.md` (their open items on "Students can pause")
**Size:** M
**Done when:**
- [ ] `make test-api-integration` and the Go unit tests of `questions`, `tests`, `attempts`
      and `media` run on the branch point and on the last commit, both runs in the PR
      (high-risk area). The integration run is the one that holds `publish_snapshot_test.go`,
      `attempts/application/tests/events_test.go`,
      `TestLockForVersionUseSerialisesAgainstDelete` and
      `TestLockForDraftUseSerialisesAgainstDelete`; the PR names the four as passed.
- [ ] The task adds a column to the rows these paths insert and changes no lock: every insert
      into `app.test_version_questions` still follows `media.LockForVersionUse`, and every
      insert into `app.test_section_questions` still follows `questions.LockForDraftUse`
      (AGENTS.md, "High-risk areas").
- [ ] Four migrations, one concern each, each with a working Down, goose up/down/up in CI:
      `questions.audio_allow_pause boolean` and `test_version_questions.audio_allow_pause
      boolean`, both nullable with `CHECK (audio_allow_pause IS NULL OR media_asset_kind IS NOT
      DISTINCT FROM 'audio')` and read as `COALESCE(…, true)`; `group_recordings.allow_pause`
      and `test_version_group_recordings.allow_pause`, both `boolean NOT NULL DEFAULT true`.
      A constant default and a nullable column: the previous binary keeps inserting during the
      rolling deploy, and none of the four rewrites or indexes a populated table. The
      biconditional `questions_audio_policy_iff_audio` is not widened, because the previous
      binary writes no value.
- [ ] The contract. A group has no separate read and write shape: `QuestionGroupBundle` is
      the response body (`StoredQuestionGroup`, 2412) and the request body (`GroupCreateInput`
      2451, `GroupUpdateInput` 2463), and one `AudioPolicy` (1694-1709, `additionalProperties:
      false`, three required fields) is referenced five times: `AdminQuestion.audio` (2031),
      `StudentQuestion.audio` (2141), `StudentGroupRecording.policy` (2175),
      `GroupRecording.policy` (2385) and `QuestionInput.audio` (3124), the last also inside
      every bundle's `GroupQuestionInput`. So "required in a response, optional in a request"
      cannot be said by adding an input schema, short of splitting the bundle in two (L). It
      is said this way instead:
      - `AudioPolicy` stays the authoring shape (2031, 2385, 3124) and gains `allowPause:
        boolean`, **optional**. Its description: the server always sends it; in a request an
        absent field means "do not change it".
      - A new `StudentAudioPolicy` has the same three fields and `allowPause`, all four
        **required**, and replaces the reference at 2141 and 2175. The engine never guesses.
      `make gen` is its own commit; MSW fixtures validate with ajv; `pnpm typecheck` passes.
      Tests are type-checked (`tsconfig.app.json:38`), so the student-shaped policy literals
      named in Touches gain the field or typecheck fails; the authoring-shaped ones
      (`contract/group-contract.test.ts`, `question-bank/{editor-audio-expiry,
      structure-validation}.test.ts(x)`, `question-groups/model.test.ts`) compile unchanged,
      and `QuestionValuesAreAcceptedByTheContract` (`questionSchema.ts:211-218`, an
      assignability check) still holds without an edit.
- [ ] What an absent field means, because `updateQuestion` and `updateQuestionGroup` replace
      the whole object and three clients send none: a tab loaded before the deploy, whose
      `z.object` parsers strip a key they do not name (`questionSchema.ts:14-18`); the same
      tab's device-recovery draft (`question-groups/recovery.ts:15-19`); and every tab until
      T-D4.15 teaches those parsers the field. **On create, absent is true. On update, absent
      keeps the stored value**: for a question by its id, for a member question by its id in
      the bundle, for a recording by its id; a recording or an audio policy that is new in
      the request is a create. "Absent means true" on update would silently turn a teacher's
      "cannot pause" back on at the next save from any of the three.
- [ ] The value is copied wherever `allowSeek` is: publish (both snapshots), restore as draft,
      duplicate, group copy, the import commit (true), the teacher's preview, the student's
      session for a question's audio and for a group's recording, the teacher's review.
- [ ] `allow_pause_test.go`: (a) publish a question and a recording with it off, edit both in
      the bank and the draft afterwards, and read the version's frozen false; (b) a version
      published before the migration reads true; (c) save false, then save the same question
      and the same group again with the field absent, and read false both times; (d) create
      with the field absent and read true; (e) a recording replaced by a new id in the same
      update reads true.
- [ ] No operation is added or changes its `x-permission`; `permissions.golden` and the
      isolation suite are unchanged, and the PR says so. The existing second-teacher cases for
      `updateQuestion` and `updateQuestionGroup` cover the field. No `x-resource` entry is
      added or changed: `allowPause` is a boolean, not an id.
- [ ] Leak review in the PR: `allowPause` is a rule a student is meant to read, not a key. The
      four forbidden keys are unchanged; `StudentAudioPolicy` holds none of them;
      `payload_test.go`, `openapi-contract.test.ts` and E2E 9 pass with their key lists
      untouched.
- [ ] `StudentSection`'s description reads "the engine shows it above each of the section's
      questions" (T-D4.5).
- [ ] Spec §11.1 lists the fourth control, its default and the keep-on-absent rule; §13 and
      `20-data-model.md` record the four columns and why two are nullable; §15 documents the
      two shapes.
- [ ] `74-r4.md`'s open item ""Students can pause", "Matching" and "Not given"" and 70 §10's
      item on the content editor say that "Students can pause" is built: both name
      `allowPause`, this task and DG+2. DG-111's row in `gaps.md` says the same.
- [ ] Rehearsed on a Neon branch of production; the timing goes in the release PR.

---

## D. The teacher builder's shared content, and the previews

R4 built these screens from the third import. Each task below that reopens one names the R4
task whose result it changes, and compares the screen with the deck again.

Untouched by the export, and not reopened: T-R4.1 to T-R4.29, T-R4.32, T-R4.33, T-R4.35 to
T-R4.38, T-R4.40 to T-R4.49, T-R4.53 to T-R4.57, T-R4.62, T-R4.63, T-R4.64, T-R4.66. The page's
tokens, shell, content editor, question media block and bank editor did not move.

Rules for T-D4.2a, T-D4.2b, T-D4.2c, T-D4.14 and T-D4.15 (AGENTS.md, "A debounced autosave
owes the user two flushes"). Each edits the builder or something its autosave holds: T-D4.2b's
`GapBindings` renders inside the group editor's form, with its bundle autosave and its
recovery draft, and T-D4.2c replaces the dialog the builder opens only after it has flushed
the open question and the outline. The five canaries, `tests/units/builder/*`,
`tests/units/question-groups/*` (`editor-save`, `material-transaction`, `model`) and
`tests/e2e/builder-group-recovery.spec.ts` run on the branch point and on the last commit, both
runs in the PR. `builder/autosave-unmount.test.tsx` and `autosave-flush.test.tsx` are not
edited.

### T-D4.2a — Builder: the shared-context group's row, its menu and its shared-content line
**Reopens:** T-R4.31a (the outline's group row)
**Depends on:** T-D4.1
**Touches:** `web/src/features/tests/components/{OutlineTree,OutlineGroupRow}.tsx`, `web/src/features/tests/pages/teacher/TestBuilderPage.tsx` (selecting the group from the line and from the menu items, nothing else), `web/src/features/tests/components/BuilderGroupPane.tsx` and `web/src/features/question-groups/components/GroupComposer.tsx` (focus on the title field when Rename asks; no save path changes), locales, `web/tests/units/builder/` (the outline's cases)
**Size:** S
**Done when:**
- [ ] The deck's "group" stays the product's section (DG+5). A section's "…" menu is R4's
      (Rename, Instructions, Move up, Move down, Remove group) and gains no "Shared content":
      the export's menu entry `mi('Shared content', 'panel-left', …)` (T 6986) is on every
      group, and a section has no shared content to hold.
- [ ] A shared-context group keeps its own row inside its section (`OutlineGroupRow`). The
      buttons R4 left on the row give way to a "…" menu: Rename, Shared content (the
      `panel-left` icon, the export's third item), Move up, Move down (disabled at the ends),
      Remove group. Move up, Move down and Remove group do what their buttons did; the row's
      drag, its keyboard equivalents and its focus ring are R4's.
- [ ] Rename is new on this row: R4 gives Rename to a section's menu only, and a group's
      title has one field, in `BuilderGroupPane` (`shared-group-title`). Rename selects the
      group and puts focus in that field with its text selected. The title is saved by the
      group's editor, and the outline calls no operation: a rename in the outline would be a
      second writer of the bundle (T-D4.14, "One writer").
- [ ] Under the row's header, while the group is open, the export's shared-content line
      (T 1581-1587, the button on `sec.editShared`; its text is the script's `sharedText`,
      T 6968) in the instructions line's geometry:
      margin 0 0 4px 26px, padding 4px 6px, radius 6px, 12px `--muted-fg`, a 13px icon, one
      line with an ellipsis. It reads `file-text` "Passage · {the material's title}", plus
      " · {n} gaps" when it has gaps; or `audio-lines` "Audio · {file name} · {n} plays" or
      "· unlimited plays", with "1 play" in the singular (the prototype prints "1 plays"). A
      group that holds more than one material, or both kinds, reads "{n} materials · {m}
      recordings". A group with no shared content shows no line.
- [ ] The line is a button. It and the "Shared content" item select the group, which opens
      `BuilderGroupPane`, until T-D4.14 opens the dialog. Selecting the group, from the line
      or from either item, saves what is pending in the open question first, as selecting a
      row does today.
- [ ] Unit tests: the line in each form (a passage with and without gaps, a recording with a
      limit, with one play and with no limit, more than one material, none); the menu's items
      and their disabled states; "Rename opens the group, focuses its title and sends no
      request"; "a pending edit is saved when the line is pressed".
- [ ] Compared with the deck at 360, 768, 1024, 1280 and 1440, light and dark, on a
      shared-context group holding one passage: the deck's fixture shows "Passage · Why cities
      need green space" under its first group (`INIT_SECS`, T 6632). Known difference: the line
      and the item are on the group's row, not on the section's (DG+5).
- [ ] Keys vi first, en from the deck, listed in the PR.

### T-D4.2b — Group pane: the gap row
**Reopens:** T-R4.34 (question groups, restyled)
**Depends on:** T-D4.1
**Touches:** `web/src/features/question-groups/components/GapBindings.tsx`, locales, `web/tests/units/content/gap-bindings.test.tsx`
**Size:** XS
**Done when:**
- [ ] `GapBindings` takes the row the export draws (T 1670-1693): a dashed chip "gap {n}" (at
      least 52px wide, 24px high, 1.5px dashed `--ring`, `--muted`, 12px 600), a 32px select
      whose first entry is "Choose a question…", a danger border while the gap is unbound, and
      a 28px "Remove gap". An unbound row is also named as unbound for a screen reader, so the
      state is not carried by the border alone.
- [ ] Above the rows: "Optional: add gaps for a cloze. Each gap is answered by a choice
      question or a blank in this group, shown at the gap." The export's sentence says "a
      single-choice question"; the product also binds a gap to a blank (DG+6).
- [ ] The rules do not change (DG+6; `model.ts:113-155`, `group_validation.go:176-212`): a gap
      is inserted at the caret by the editor's "Insert gap", a target is a choice member or one
      blank of a fill-in, a target is used once and shows disabled elsewhere, and a group with
      an unbound gap does not save. The export's "[gap n]" appended at the end of the text, any
      question as a target and a saved unbound gap are not taken.
- [ ] `gap-bindings.test.tsx` passes with only its locators moved, and gains the unbound row's
      name. `group-authoring.spec.ts`, `group-authoring.live.spec.ts` and
      `builder-groups.live.spec.ts` are green.
- [ ] Compared with the deck at 360 and 1280, light and dark, in the builder's group pane and
      in the bank's group editor, which share the component.

### T-D4.14 — Builder: the Shared content dialog
**Depends on:** T-D4.2a, T-D4.2b. It builds on what R4 left on `develop`: the builder (T-R4.31a, T-R4.31b), the group editor (T-R4.34), `useMediaUpload` (T-R4.35), the content editor (T-R4.63) and `AudioPolicyPanel` with "Choose from Media" (T-R4.65)
**Touches:** `web/src/features/tests/components/{SharedContentDialog.tsx (new),OutlineGroupRow.tsx,BuilderGroupPane.tsx}`, `web/src/features/tests/pages/teacher/TestBuilderPage.tsx`, `web/src/features/question-groups/{model.ts,sharedContent.ts (new),useGroupEditor.ts}` (the last only if `saveNow` must hand its outcome to the dialog; its queue, its base and its recovery writes are not changed), `web/src/features/question-groups/components/{GapBindings,MaterialContent,MaterialAssetDialog}.tsx` (reused, not restyled again), `web/src/features/media/components/AssetLibraryDialog.tsx` (an initial kind), locales, `web/tests/units/builder/shared-content-dialog.test.tsx` (new), `web/tests/units/question-groups/editor-save.test.tsx`, `web/tests/e2e/{builder-groups.live.spec.ts,builder-group-recovery.spec.ts}`
**Reuses:** `FormDialog`, `Segmented`, `ContentEditor` (profile `document`), `GapBindings`, `AudioPolicyPanel`, `AssetLibraryDialog`, `useMediaUpload`, `materialTransaction`, and the group's one writer: `useGroupEditor` behind `GroupRecoveryGate`, with `recovery.ts`'s draft
**Size:** M
**High risk:** this is the builder's autosave and its device-recovery draft (AGENTS.md, "A
debounced autosave owes the user two flushes"). Before and after, both runs in the PR: the five
canaries, `tests/units/builder/*`, `tests/units/question-groups/*` (`editor-save`,
`material-transaction`, `model`) and `tests/e2e/builder-group-recovery.spec.ts`.

**One writer.** A section-owned group already has one: `BuilderGroupPane` mounts
`useGroupEditor` (`BuilderGroupPane.tsx:85-101`), which owns the revision base and the
device-recovery draft (`useGroupEditor.ts:43-46`, 94-117, 121-136) and raises `copyRequired` on
a revision it did not write. A group's member questions are part of that bundle; they have no
question autosave of their own. T-D4.2a makes the menu item and the outline line select the
group, so the pane and its editor are mounted while the dialog is open. A dialog
that called `updateQuestionGroup` itself would move the revision under the editor: the
teacher's next keystroke in the pane would meet "This group changed elsewhere", and a recovery
draft holding the older materials could be offered back over what the dialog saved.

**Done when:**
- [ ] No operation is added, and the dialog calls none: it saves through the editor, whose
      `save` is today's `updateQuestionGroup` (`x-permission: [content.questions.write,
      content.tests.write]`; a group in a test section needs `content.tests.write`), already
      in the isolation suite with its `x-resource` entries. The PR says so.
- [ ] The dialog is rendered by `BuilderGroupPane`'s form, which holds the editor. "Shared
      content" in a shared-context group's menu, and its outline line, select the group and
      ask the pane to open it; the pane answers once its recovery gate has resolved, so a
      recovery draft is offered first, as today, and the dialog opens over the bundle the
      teacher chose.
- [ ] It is 560px (`min(560px, 100% − 24px)`): "Shared content for “{group}”", "A passage or
      recording shown beside every question in this group. Students see it beside the questions
      while they answer." (the deck's "on the left", reworded because a phone shows a drawer;
      DG+6), a scroll area (gap 12px, at most 56vh), Cancel and Save.
- [ ] Kind: a full-width Segmented, 30px, "None" (`circle-slash`), "Passage" (`file-text`),
      "Audio" (`audio-lines`).
- [ ] Passage: "Title" (36px, "e.g. Why cities need green space", required, 1 to 200
      characters); "Passage" is the content editor in T-R4.63's frame with the placeholder
      "Paste or type the passage students read" (DG+6), and the hint "Optional: add gaps for
      a cloze. …" over T-D4.2b's `GapBindings` rows. The editor's toolbar holds the dialog's
      only "Insert gap" (T-R4.63 shows it for the `document` profile, at the caret); the
      deck's separate 30px button beside the hint is not built, so there are never two
      (DG+6).
- [ ] Audio: the empty zone (1.5px dashed `--border`, radius 10px, padding 12px, `audio-lines`
      17px) reads "MP3 or M4A up to 50 MB and 5 minutes" (DG-63), with "Choose file" and "From
      Media"; the attached card shows the file name and "Replace". Uploads go through
      `useMediaUpload` with its progress and failure. "From Media" is `AssetLibraryDialog`
      opened on Audio: R4 built it to open on All from the question media block, its only
      entry point then (T-R4.65), so it gains an initial kind and the block keeps All. Then
      `AudioPolicyPanel`: "Plays", "Allow skipping ahead", "Show transcript after submitting",
      and "Transcript" with "Optional. Students see it after they submit." shown only while
      that switch is on. "Students can pause" appears under "Plays" when T-D4.15 merges; the
      panel is shared, so the dialog is not edited for it.
- [ ] What "Audio" stores is the shape the server accepts: one material whose content is the
      single audio node, and one recording bound to that asset (`group_validation.go:235`,
      243-246). The material's required title is the file's name without its extension. It is
      a teacher-side label: it shows in the outline line and in the full pane, and the student
      never sees it, because an audio-only material draws no heading (T-D4.5, T-D4.8).
- [ ] The dialog edits a group with at most one material and one recording of one kind. A
      group that holds more (an import, a copy from the bank, a material mixing text and
      audio) opens `BuilderGroupPane` instead, and nothing is dropped. Saving another kind
      over stored content asks first, naming what goes and how many gaps unbind. "None"
      removes the shared content and keeps the group and its members.
- [ ] The dialog works on a copy of `editor.bundle` taken when it opens. Save calls
      `editor.change(next)`, where `next` is the editor's current bundle with only `stimuli`
      and `recordings` replaced (so a member edited in the pane a second ago is in what is
      sent), then awaits `editor.saveNow()`. The revision, `expectedTestUpdatedAt` (through
      the pane's `coordinate`), the recovery draft and the conflict keep their one owner.
      Save is disabled, with one sentence, while a gap is unbound, the title is empty, the
      Audio kind has no file, or the editor's `copyRequired` is true (today's "This group
      changed elsewhere"). A failed save keeps the dialog open with the editor's message.
      Cancel discards the dialog's copy and nothing else; with edits it asks first.
      Publishing still flushes first, through the pane's `flushRef`.
- [ ] `shared-content-dialog.test.tsx` covers the three kinds, the refusals, the kept members,
      the group that opens the pane instead, that a transcript never reaches
      `question-groups/preview.ts`'s output, that Save sends exactly one request, and "an
      edit in the pane right after a dialog save is saved, with no conflict".
      `builder-group-recovery.spec.ts` gains: a dialog save, a reload before the server
      answers, and the recovery draft offered holds the dialog's content, not the older one.
      `builder/autosave-unmount.test.tsx` and `autosave-flush.test.tsx` stay green.
- [ ] Deck check at the five widths, light and dark; keys vi first, en from the deck except
      the one reworded sentence.

### T-D4.15 — Authoring: one "Plays" control, and "Students can pause"
**Reopens:** T-R4.65 (the question media block's "Plays", and "Students can pause is not built")
**Depends on:** T-D4.14; the four pause bullets also on T-D4.13. If the owner declines Q4, the task is its "Plays" bullet and the comparison
**Touches:** `web/src/features/question-bank/components/AudioPolicyPanel.tsx`, `web/src/features/question-bank/{audioPolicy.ts,questionSchema.ts}`, `web/src/features/question-groups/{model.ts,recovery.ts}`, locales, `web/tests/units/question-bank/editor.test.tsx`, `web/tests/units/question-groups/{recovery-compat.test.ts,recording-plays.test.tsx}` (new)
**Size:** S
**Done when:**
- [ ] "Plays" is one control everywhere (DG-69, Q25). T-R4.65 built "Once | Twice | Unlimited"
      for a question's audio, with a stored other value as a fourth option while it is the
      value. A group's recording shows the same control, in the group pane and in the Shared
      content dialog: `AudioPolicyPanel` is the one component for both, and if R4 left the
      group's recording on the older select (1, 2, 3, 5, Unlimited), this task moves it. The
      export's third drawing, a select with "3 plays" (T 1714-1719), is not taken. A unit
      case in the group pane: a recording stored with 3 plays shows "3" as a fourth option,
      and choosing "Twice" saves 2.
- [ ] `AudioPolicyPanel` shows "Students can pause" under "Plays", on by default, for a
      question's audio and for a group's recording; it saves `allowPause`. A policy read
      without the field shows the switch on.
- [ ] Both parsers carry the field instead of stripping it: `audioPolicySchema`
      (`questionSchema.ts:14-18`) and the recovery draft's `policy` (`recovery.ts:15-19`) are
      `z.object`, which drops a key it does not name, so each gains `allowPause` as an
      optional boolean. `QuestionValuesAreAcceptedByTheContract` still holds.
- [ ] A recovery draft stored before this task has no `allowPause`. When it is restored, a
      policy without the field takes the value the server's bundle holds for the same
      recording or member question (`GroupRecoveryGate` has `stored`), and true where the
      server has none, so the switch shows the truth and an old draft cannot switch pausing
      back on. If that merge were ever skipped, T-D4.13's rule (an absent field keeps the
      stored value) still holds the line. `recovery-compat.test.ts`, in the manner of
      `take-test/draft-compat.test.ts`, reads a draft written without the field (a literal in
      the test), restores it over a group whose recording has `allowPause: false`, and
      asserts the restored and the saved bundle both read false.
- [ ] A test saves it off for a question and for a recording and reads it back.
- [ ] The media block and the group pane's recording are compared with the deck again at 360
      and 1280, light and dark.

### T-D4.2c — Previews: the bar and the frame
**Reopens:** T-R4.31b (the builder's Student preview), T-R4.30 (Test detail's preview), T-R4.39 (the import's preview)
**Depends on:** T-D4.1
**Touches:** `web/src/features/tests/components/{DraftPreviewDialog,StudentPreviewPane}.tsx`, `web/src/features/tests/pages/teacher/TestDetailPage.tsx`, `web/src/features/tests/components/StudentPreview.tsx` (the compare outline and chips T-R4.30 added go), `web/src/features/imports/pages/teacher/ImportConfirmPage.tsx`, locales, `web/tests/units/builder/preview-and-leave.test.tsx`, `web/tests/units/tests/`, `web/tests/units/imports/`
**Size:** M
**Done when:**
- [ ] The builder's Preview opens a full-window overlay in place of R4's 720px dialog
      (T 5737-5761; the script's `builderPreviewVals`, T 6896-6901): `role="dialog"` named
      "Student preview", `--overlay`, padding 14px, gap 12px. Its bar is a card (radius 12px,
      `--shadow-lg`, padding 8px 10px 8px 14px) with the eye icon (16px), "Student preview ·
      {title}" (14px 600, ellipsis), the Segmented "Computer | Phone" (28px buttons, 12.5px,
      the icons `monitor` and `smartphone`) and a 34px "Close preview". Under the bar, a frame
      as tall as the space left: `min(1320px, 100%)` wide with a 12px radius for Computer,
      390px wide with a 28px radius for Phone.
- [ ] There is no Previous / Next. Pending edits are flushed before the overlay opens, as
      before R4's dialog. Esc closes it, focus is trapped and returns to Preview: the export
      draws none of the three (DG-36).
- [ ] Test detail (T 5846-5853, under `td.hasPreview`): the preview is a frame on `--sidebar`
      with 16px of padding. Computer is full width, 620px high, radius 12px; Phone is 390px
      wide, 720px high, radius 24px. "Changes from version {prev}" keeps its Added / Changed /
      Answer / Points rows above the frame, and nothing inside the frame is outlined or
      carries a "New" or "Changed" chip (Q23). The line "Showing {k} of {n} questions…" goes.
      The not-published state and `?version=&compare=1&device=phone&history=1` are R4's.
- [ ] The import's preview (T 4975-4980, the frame sized by `wi.pvW` and `wi.pvH`) is Test
      detail's frame, with the same two sizes. It holds the whole review draft through R4's
      adapter, never a fixture, with answers and notes left out. "Showing {k} of {n}
      questions…" goes (Q24).
- [ ] One frame component serves the three hosts (`StudentPreviewPane`); a host passes its
      sizes. In all three the frame holds R4's `StudentPreview`, scrolling inside it, until
      T-D4.16 puts the engine there (DG+7).
- [ ] Strings: "Close preview" and the bar's title are new, vi first. The keys of the retired
      dialog's sub line, Previous / Next, the two chips and the count line are removed in
      their own commit. The PR lists each test that pinned a retired frame beside the case that
      replaces it.
- [ ] Unit tests. `preview-and-leave.test.tsx`: an edit still pending is saved before the
      overlay opens (today's one preview case does not assert it); Esc closes the overlay and
      focus returns to Preview; "Computer | Phone" changes the frame's size. Under
      `tests/units/tests/`: with compare on, the rows stand above the frame and nothing
      inside it carries an outline or a chip; the frame's two sizes. Under
      `tests/units/imports/`: the frame holds the whole review draft, with no count line, no
      answer and no note.
- [ ] Compared with the deck at 1280 and 1440 (Computer) and with Phone selected, and at 360,
      768 and 1024 for the bar and the overlay, light and dark. Known difference, under DG+7:
      the frame's content.

### T-D4.16 — Preview: the take-test screen in the frame
**Depends on:** T-D4.2c, T-D4.3, T-D4.4, T-D4.5, T-D4.7, T-D4.8 (and T-D4.10 if it is built)
**Touches:** `web/src/features/take-test/components/{EnginePreview.tsx (new),EngineHeader.tsx,SaveState.tsx}`, `web/src/features/take-test/index.ts` (new), `web/src/features/tests/components/{DraftPreviewDialog,StudentPreviewPane,StudentPreview (removed)}.tsx`, `web/src/features/tests/pages/teacher/TestDetailPage.tsx`, `web/src/features/imports/pages/teacher/ImportConfirmPage.tsx`, `web/src/features/question-groups/preview.ts`, locales, `web/tests/units/builder/{group-preview,preview-and-leave}.test.tsx`, `web/tests/units/take-test/preview.test.tsx` (new), `web/tests/integration/router-chunks.test.ts` (run, not edited)
**Size:** L
**Frozen:** the live engine's wiring in `TakeTestPage` (store, autosave, drafts, integrity,
fullscreen, play counting); `preview.ts`'s stripping of keys and transcripts.
**Done when:**
- [ ] `EnginePreview` mounts `Paper` from a paper source held in component state and a width
      source that measures its frame (T-D4.3's two seams). It imports no store, starts no
      autosave, writes no draft and no open-question key, mounts no integrity monitor, asks
      for no fullscreen and counts no play; a unit test spies on each and finds no call.
- [ ] Its header has no leave button and no Submit, and shows the chip "Student view" (30px,
      radius 999, `--info-soft` / `--info-ink`, 12.5px 600, a 14px eye). Finish raises the
      info toast "This is a preview. Students submit here."
- [ ] No timer runs and no time is shown, because none of the three hosts has one: a test has
      no time limit, `durationMinutes` is the assignment's (`api/openapi.yaml` 3175, 3195;
      `Assignment` 2640-2645), and `previewTest` returns `version`, `questions`, `sections`
      and `groups` only (4391-4398), as the builder draft and the review draft do. In the
      timer's place stands a pill of the same geometry (36px, radius 999, the timer icon)
      reading "––:––", not a `timer`, named "The time limit is set when the test is assigned"
      ("Thời gian làm bài được đặt khi giao bài"), so the header keeps its width in the
      390px frame. `Clock` is not mounted. Listed under DG+7 beside the deck's 45:00.
- [ ] The save line does not claim a save. Where the engine says "All answers saved" the
      preview says "Answers here are not saved." ("Câu trả lời ở đây không được lưu."), with
      the eye icon, and no "Saving…" follows a pick; the deck's preview keeps the live save
      line (S 961-963, 1617), which DG+7 lists as a slip.
- [ ] The integrity policy is the assignment's too (`IntegrityPolicy`, 2615-2637), so the
      preview has none: pasting into an answer is allowed, nothing is counted, no strike line
      and no fullscreen bar are drawn.
- [ ] Answers can be chosen and are kept only while the preview is open (Q11).
- [ ] The three hosts mount it in the frames T-D4.2c built: the builder
      from the draft through `preview.ts`, Test detail from `previewTest` for the selected
      version, the import from the review draft's adapter. `StudentPreview.tsx` is removed
      when nothing imports it.
- [ ] At 390px the frame shows the phone engine (the drawer, the grid button) on a desktop
      window: the width comes from the frame, never from the viewport.
- [ ] The engine enters the teacher tree as a lazy chunk; `router-chunks.test.ts` passes
      untouched.
- [ ] The three previews are compared with the deck at 1280 and 1440 (Computer) and at 390
      (Phone), light and dark. Known differences, all under DG+7: the placeholder where the
      deck runs a 45:00 timer; the save line's sentence; content is the teacher's own, not a
      fixture.

---

## F. The R4 screens v2 redraws

Four tasks, all web, none touching a high-risk folder except T-D4.20's edit of the builder's outline
geometry (the builder chain's rule applies to it). Each reopens the R4 task named, compares the screen
with the v2 deck again at the five widths in two themes, and adds one closing line to that task in
`74-r4.md` through T-D4.12.

### T-D4.20 — Sticky side columns, and the collapsed sidebar's separator
**Reopens:** T-R4.4 (the sidebar), T-R4.31a (the outline), T-R4.27a, T-R4.30, T-R4.32, T-R4.33, T-R4.38, T-R4.42, T-R4.43 (their side columns)
**Depends on:** T-D4.1; runs first on the builder chain, before T-D4.2a
**Touches:** `web/src/layouts/shell/Sidebar.tsx`, `web/src/components/shared/StickyAside.tsx` (new), `web/src/features/tests/pages/teacher/{TestBuilderPage,TestDetailPage}.tsx`, `web/src/features/assignments/pages/teacher/AssignmentFormPage.tsx`, `web/src/features/question-bank/pages/teacher/{QuestionBankPage,QuestionEditorPage}.tsx`, `web/src/features/imports/pages/teacher/ImportReviewPage.tsx`, `web/src/features/classes/pages/teacher/ClassDetailPage.tsx`, `web/src/features/settings/pages/teacher/TeacherSettingsPage.tsx`, `web/tests/units/shell/sidebar.test.tsx`, `web/tests/units/shared/sticky-aside.test.tsx` (new), `web/tests/units/builder/` (the outline's geometry case)
**Size:** S
**Done when:**
- [ ] `Sidebar` draws a 1px `--border` `role="separator"` (margin 0 10px 4px) between nav groups
      while their labels are hidden (collapsed), in both consoles (V-01, A-01); the drawer and
      the expanded sidebar are unchanged.
- [ ] One `StickyAside` wraps every side column: `position: sticky; top: 16px; max-height:
      calc(100vh − 88px); overflow-y: auto; padding 1px 2px 2px; margin −1px −2px −2px` (the
      deck's allowance for focus rings), from a content width the host passes (800 for the
      wizard, 1000 for Class detail and the review, 900 for the question editor's aside, 768 for
      the settings nav), static below it; the version history keeps its own `top: 16px` and cap;
      the bank's filter aside gets `top: 16px` from 1024 and stays `top: 0` below (V-02).
- [ ] The builder's outline is sticky and scrolls inside itself whenever the builder splits
      (`outPos`, `outMaxH`, `outOv`, T 1633 and 8690); a section's "…" menu still escapes it
      (V-03): one unit case opens the menu on the last section of a long outline and finds it
      unclipped.
- [ ] No string changes; no behaviour changes (a behavioural change never rides in a refactor).
- [ ] Compared with the deck at 1024, 1280 and 1440 with a long page scrolled, light and dark,
      on the builder, the wizard, Test detail, the bank, the editor, the review, Class detail
      and Settings; the collapsed sidebar at 1024 in both consoles.

### T-D4.21 — Grading: the pane's heights
**Reopens:** T-R4.28
**Depends on:** T-D4.1
**Touches:** `web/src/features/attempts/pages/teacher/GradingPage.tsx`, `web/tests/units/attempts/grading-layout.test.tsx` (new)
**Size:** S
**Done when:**
- [ ] From 768 the page is `calc(100vh − 120px)` high with a 560px minimum; the queue aside is a
      column whose list scrolls (`flex: 1 1 auto; min-height: 0`), and the card's body scrolls
      inside it under the 3px progress bar (V-04, T 847-875). Below 768 everything is static and
      the queue keeps its 180px cap.
- [ ] `j` / `k` keep the active queue row in view inside the scrolling list; the card's score
      buttons and the footer stay reachable without scrolling the page.
- [ ] Unit cases at `viewport("desktop")` and `viewport("phone")`; compared with the deck at 768,
      1024 and 1280 with a long queue and a long answer, light and dark.

### T-D4.22 — Assignment detail: the progress card and the roster filters
**Reopens:** T-R4.25 (the four stat tiles)
**Depends on:** T-D4.1
**Touches:** `web/src/features/assignments/pages/teacher/AssignmentDetailPage.tsx`, `web/src/features/attempts/components/{Monitor.tsx,ProgressCard.tsx (new)}`, locales, `web/tests/units/assignments/detail.test.tsx`, `web/tests/units/attempts/monitor.test.tsx`
**Size:** M
**Done when:**
- [ ] The four `StatStrip` tiles give way to the card (V-05, T 607-631): "{n} of {total}
      submitted" ("All {n} submitted"; "Not assigned yet" with no target), an 8px two-segment bar
      (submitted `--success`, in progress `--info`), a legend of `aria-pressed` buttons (28px,
      radius 7, 12.5px) each with a dot or, for Flagged, the flag icon in danger ink, and a count
      in `--fg` (Flagged's in danger ink), then "Average {avg}" (15px 600) after a 1px divider.
      The counts are the monitor's whole-assignment counts, not the page's.
- [ ] A legend button filters the roster to its group (Submitted includes timed out, as the
      monitor groups them), opens the Students tab, resets the page and names the filter in the
      URL (`?roster=`, which T-R4.25's redirect already writes); pressing it again shows
      everyone; an entry hides at 0 unless it is the one selected; opening the page from the
      list clears the filter. The tooltip reads "Show only {group}" / "Show everyone".
- [ ] The average keeps T-R4.25's rule (the rounded mean of graded ratios; "—" with none).
- [ ] Strings: `assignmentDetail.progress.{submittedOf,allSubmitted,notAssigned,average,
      showOnly,showEveryone}`, vi first; the four tile keys go in their own commit if nothing else
      reads them.
- [ ] Unit tests: the three sentences, the bar's widths, a filter press and release, the hidden
      zero entries, the URL; compared with the deck at 360, 768, 1024, 1280 and 1440 on a live
      assignment with flagged papers, light and dark. Known difference: none expected.

### T-D4.23 — Class detail: the join-code card, v2
**Reopens:** T-R4.42 (the join-code card)
**Depends on:** T-D4.1
**Touches:** `web/src/features/classes/components/{JoinCodePanel,JoinQr,JoinQrDialog (new)}.tsx`, `web/src/features/classes/pages/teacher/ClassDetailPage.tsx` (members' "Joined via"), locales, `web/tests/units/classes/join-code.test.tsx`
**Size:** S
**Done when:**
- [ ] The card (V-16, T 4287-4345): a 112px QR button (`--qr-bg` / `--qr-fg`, radius 10, 1px
      border, `cursor: zoom-in`, named "Show full size for the projector", at opacity .4 while
      joining is paused) that opens a 420px dialog "Join {class}" with the QR at full size and
      "Show this on the projector. Students scan it or type the code." (or "Joining is paused, so
      this code won’t work until you turn it back on."); beside the button the full code (22px
      monospace, 600, letter-spacing .08em) with the join link under it (12.5px muted, ellipsis)
      and, while paused, a "Joining paused" chip (`pause`, warning tones) in place of the
      paragraph; under them a row of three outline buttons (32px, `flex: 1 1 auto`, ellipsis):
      Copy code, Copy link, Download QR; then Expires and Uses under a 1px divider; the footer
      "A new code makes the old code, link and QR stop working."; the New-code dialog's sentence
      "Share the code, the link or the QR with your students.". The switch, New code, Stop and
      the no-code state are T-R4.42's.
- [ ] Members' "Joined via" shows the full code in monospace where T-R4.42 showed "Code
      ••{hint}" (the code is always shown in full to its teacher, D5, DG-01).
- [ ] DG-01 is marked Answered in `gaps.md` and its "Meanwhile" rule deleted (T-D4.1 opens the
      row; this task closes it).
- [ ] Strings vi first; unit tests for the paused state, the dialog and the three buttons;
      compared with the deck at 360, 768, 1024, 1280 and 1440, light and dark, with joining on
      and paused.

---

## G. The AI assistant

Built under Thuong's decisions of 2026-10-10 (D19 to D22 in `70-redesign-overview.md` §1):
Anthropic's API with cloud processing on; data minimisation per feature; a $200 monthly pool that
is never overspent without an Admin's approval; built in D4. Everything below rests on them.

**Shape.** A new bounded context `server/internal/modules/ai/` (domain, application/{command,
query,ports,model}, repositories, http), wired in `core/wiring/ai.go`. It **reads** through ports
typed as other modules' handlers or adapters (a test's draft and default version, a question, a
grading item, item analysis and the new wrong-answer aggregate, the caller's own bank rows), and
**writes nothing a teacher sees**: every acceptance in the deck ("Use this", "Use score and
comment", "Add {n} to {section}", "Save {n} to the bank", "Save as practice test", "Add tags",
"{fix}") is the browser calling an existing write (`updateQuestion`, `createQuestion`, the
section unit writes, `duplicateTest`, the bulk tag write, `gradeAttempt`) with the content the
teacher accepted. The module's own writes are its ledger, reservations, requests and settings.
So the integrity rule holds by construction: the AI never writes a grade, a question or a comment,
and never concludes anything about a student (no feature reads integrity events, focus losses or
flags; the insights prompt asks for patterns in answers).

**The provider behind a port** (D19). `platform/anthropic` holds the client for the Messages
API. The module's port is `ports.Completer` (`Complete(ctx, Request) (Response, error)` with the
tier, a system prompt, user content blocks of text, images or a PDF document, a JSON schema for
the answer, and a `max_tokens`; the response carries the parsed object and the reported `usage`).
`core/adapters/ai.go` adapts the platform client and is **nil when `ANTHROPIC_API_KEY` is
unset**, as `adapters/media.go` is nil without storage: every AI operation then answers
`501 NOT_IMPLEMENTED`, `GET /public/status` says `ai: false`, the web draws no AI entry point, and
the import's "Read with AI" card is absent. The deck's "AI didn’t respond, so this is sample
output." is the **transient-failure** state (a timeout, a 429 after retries, a 5xx, a refusal, a
response that fails the schema): the panel shows the warning callout and "Try again", never
sample content. The key is read once by `platform/config`, is never logged, and the client
redacts the header from every error it returns.

**Dependency.** The official Go SDK for the API, with the reason the PR states: typed errors with
retry on 429 and 5xx and none on 4xx, streaming for the long generations, the image and document
content blocks the import reader needs, and the `usage` object the metering reads. The
alternative is `net/http` against one endpoint, as `platform/google` does, which saves the
dependency and costs a hand-written retry, error and streaming layer. Either way the client is one
file behind the port, and nothing outside `platform/anthropic` imports it.

**Tiers as configuration.** `AI_TIER_FAST_MODEL` and `AI_TIER_CAPABLE_MODEL` (the capable tier
must accept images and PDF pages), and `AI_TIER_{FAST,CAPABLE}_PRICE_{INPUT,OUTPUT,CACHE_READ}`
in USD per million tokens. The server refuses to start when the key is set and a tier or a price
is missing. **No model identifier is written in any document or committed file**; `.env.example`
lists the names with no values.

**What each feature sends (D20), its tier, and its cost.** One credit is **$0.001 of provider
list price**, so the $200 pool is 200,000 credits, the deck's `AI_POOL`. A call's credits are
`ceil((input × p_in + output × p_out + cache_read × p_cache) / $0.001)` from the response's
`usage` and the tier's configured prices. Estimates use indicative list prices of about $2 / $10
per million input / output tokens for the capable tier and $0.10 / $0.50 for the fast tier; the
product meters the real numbers, and the deck's costs are placeholders.

| Feature | Sent to the provider | Never sent | Tier | Tokens in / out | Credits | Ceiling reserved |
|---|---|---|---|---|---|---|
| Generate questions | The source text the teacher pastes or picks (a shared-content passage of the test, up to 8,000 characters), types, count, level, the explanation switch, the UI language | Anything about students | capable | 3k / 3.5k | ≈ 40 | 200 |
| Check this test | The draft's prompts, options, keys, explanations and shared passages; the chosen checks | Student data; assignments | capable | 12k / 2.5k | ≈ 50 | 300 |
| Make a variant | The default version's questions, options and keys; what to change; difficulty | Student data | capable | 10k / 4k | ≈ 60 | 300 |
| Write an explanation | Prompt, options, key, type, the language and length asked | — | capable | 0.6k / 0.2k | ≈ 3 | 20 |
| Suggest wrong options | Prompt, key, existing options | — | capable | 0.6k / 0.3k | ≈ 3 | 20 |
| Suggest a score | The question's prompt, type and max points, the accepted answers or the sample answer, the allowed score set, **the student's answer text** | The student's name, email, id, class; the attempt id; other answers | capable | 0.8k / 0.3k | ≈ 5 | 30 |
| Explain results | Per question: number, type, prompt excerpt, key, the correct rate (T-R4.13), and **the distinct wrong answer texts with their counts** (a new aggregate; the eight most frequent per question); the handed-in count | Any name, id or attempt; who wrote what | capable | 3k / 1k | ≈ 16 | 60 |
| Clean up the bank | The caller's own questions' prompts, types and tags, in chunks (`Own()` rows only, up to 2,000) | Shared questions; usage counts | fast | 90k / 8k | ≈ 13 | 200 |
| Read with AI | The uploaded exam's pages as images (rendered by `go-pdfium`, up to 40 pages) or the PDF document, and the recogniser's schema | Nothing else; the import page says so before upload (`18-word-import-ux.md` §4) | capable (vision) | 2k / 1.2k a page | ≈ 16 a page | 60 a page |
| Generate words (R10) | The text, count, level, meaning language | — | fast | 3k / 1.5k | ≈ 2 | 20 |

No feature sends a personal identifier; none needs one. The two that carry student-written text are
gated by the teacher's "Let AI read student answers" switch (Q33). A month in which ten teachers
each make 20 generations, 5 checks, 2 variants, 100 score suggestions, 10 insights, 50
explanations and import two 10-page scans costs about 23,000 credits, a tenth of the pool: the pool
guards against runaway use (scans, loops, a bulk import), not ordinary use. **The default
allowance is 20,000 credits ($20) a month** (Q30): ten teachers fill the pool exactly.

**Rules for every task in this section.** The module is a **high-risk area** (it reads student
answers and holds a paid key); every task runs the five canaries and `make test-api-integration`
on its branch point and its last commit, both runs in the PR. No task needs the key to pass CI:
every suite covers the nil adapter's 501 and the web's hidden entry points. A task that calls the
provider for real does so in a live smoke on the builder's machine and pastes the metered credits
into the PR, never the key or a response that holds student text. Prompts live in
`modules/ai/domain/prompts/` as Go constants with their JSON schemas beside them; a test asserts
every prompt's schema parses the deck's demo arrays (the shapes the UI draws).

### T-D4.30 — The provider adapter, its configuration and the tiers
**Depends on:** T-D4.1 (for nothing but the order); v0.10.0 released
**Touches:** `server/internal/platform/anthropic/{client.go,usage.go,errors.go}` (new), `server/internal/platform/config/ai.go` (new), `server/internal/core/adapters/ai.go` (new), `server/internal/modules/ai/application/ports/completer.go` (new), `server/internal/core/wiring/{ai.go (new),wiring.go}`, `server/internal/core/router/{modules.go,server.go}`, `server/go.mod`, `.env.example`, `docs/setup/operations.md` (the key, its rotation, the tiers), `server/internal/platform/anthropic/tests/client_test.go` (new, against an `httptest` server), `server/internal/core/tests/architecture_test.go` (the new packages' allowed imports)
**Size:** M
**Done when:**
- [ ] `platform/config` reads `ANTHROPIC_API_KEY`, `AI_TIER_FAST_MODEL`, `AI_TIER_CAPABLE_MODEL`
      and the six prices; with the key unset the whole block is off and nothing else is read;
      with the key set a missing tier or price refuses start-up with a sentence that names the
      variable. The key never appears in a log line, an error or a panic: `errors.go` wraps every
      SDK error and redacts the header, and a test asserts the redaction on a 401 body.
- [ ] `ports.Completer` carries a tier, a system prompt, content blocks (text, image bytes with a
      media type, a PDF document), a JSON schema and `max_tokens`, and returns the parsed object
      with the reported usage (input, output, cache read, cache write) and the provider's request
      id. The client streams when `max_tokens` exceeds 4,096, retries 429 and 5xx with back-off
      three times, never retries a 4xx, treats a refusal stop as a typed `ErrRefused`, and times
      out at 60s (120s for a document).
- [ ] `core/adapters/ai.go` returns nil with the key unset; the module's HTTP answers
      `501 NOT_IMPLEMENTED` for every operation then, with the envelope's code `AI_OFF`; `GET
      /public/status` gains `ai: boolean` (a read of configuration, never of the provider).
- [ ] `architecture_test.go` allows `platform/anthropic` to import the SDK and nothing else to;
      `modules/ai` imports no platform package but through its ports.
- [ ] `.env.example` gains the nine names with no values and a two-line comment; `operations.md`
      says how the key is set on Fly, rotated (set the new key, deploy), and that the prices are
      re-read at start-up when the provider's list price changes.
- [ ] Unit tests against `httptest`: a happy completion with usage, a 429 then a 200, a 5xx three
      times then the typed error, a refusal, invalid JSON, a timeout; the nil adapter's 501.

### T-D4.31 — Credits: the pool, allowances, reservations, the ledger and the maintenance command
**Depends on:** T-D4.30
**Touches:** `migrations/NNNNN_create_ai_settings.sql`, `NNNNN_create_ai_credit_periods.sql`, `NNNNN_create_ai_teacher_credits.sql`, `NNNNN_create_ai_reservations.sql`, `NNNNN_create_ai_usage_ledger.sql`, `NNNNN_create_ai_credit_requests.sql` (new; numbered at merge), `server/internal/modules/ai/{domain/{credits.go,feature.go},application/command/{reserve.go,settle.go,release.go,request_credits.go,decide_request.go,set_allowance.go,add_credits.go,set_rules.go},application/query/{usage.go,admin_credits.go},repositories/{credits.go,ledger.go,requests.go}}` (new), `server/internal/core/jobs/ai_reservations.go` (new), `server/cmd/maintenance/main.go` (`ai-credits`), `api/openapi.yaml` (`/admin/ai/*`, `GET /teacher/ai/usage`, `POST /teacher/ai/credit-requests`; the shapes ride in T-D4.32's contract PR), `server/internal/modules/notifications/` (three kinds), `server/internal/modules/ai/application/tests/credits_test.go` (new, integration), `docs/plan/20-data-model.md` (§12, the six tables), `docs/setup/operations.md` (the command)
**Size:** L
**Done when:**
- [ ] Six migrations, one concern each, Down working, all on new empty tables (none is
      `-- +goose NO TRANSACTION`):
      - `app.ai_settings` (one row, seeded by the migration: `pool_credits` 200,000 (D21),
        `default_allowance_credits` 20,000, `warn_percent` 80, `on_teacher_limit` `request`);
      - `app.ai_credit_periods (month date PK, pool_credits, added_credits, used_credits,
        reserved_credits)` with `CHECK (used_credits + reserved_credits <= pool_credits +
        added_credits)`: **the hard stop is a constraint**, so no code path can overspend the
        pool. `month` is the first day of the month in `APP_TIMEZONE`; the row is created on
        first use from `ai_settings.pool_credits`;
      - `app.ai_teacher_credits (month, user_id, allowance_credits NOT NULL, custom, paused,
        used_credits, reserved_credits, PK (month, user_id))` with the same CHECK against
        `allowance_credits`; the allowance is materialised from the default when the row is
        created and rewritten for every non-custom row of the month when the Admin changes the
        rule;
      - `app.ai_reservations (id uuidv7, month, user_id, feature, credits, created_at)`;
      - `app.ai_usage_ledger` (**append-only**: the app role has no `UPDATE` or `DELETE`, as on
        `audit_log`): one row per settled call with the actor, feature, tier, the four token
        counts, credits, the provider's request id, `created_at`;
      - `app.ai_credit_requests (id, user_id, month, amount_credits, reason text ≤ 500, status
        pending | approved | declined, decided_by, decided_at, created_at)`.
      Every insert names its owner (`user_id`); `20-data-model.md` §12 records the six with the
      reasoning (D-nn), and why the ledger is append-only.
- [ ] `Reserve(ctx, actor, feature, ceiling)`: one transaction locks the period row and the
      teacher row (`FOR UPDATE`, creating either if absent), adds the ceiling to both
      `reserved_credits`, and writes the reservation; a constraint violation (`pgerrcode`
      check violation) answers `402 AI_CREDITS_EXHAUSTED` with `detail.scope` `teacher` or
      `pool`, and a paused teacher `403 AI_PAUSED`. `Settle(ctx, reservation, usage)` moves the
      metered credits from reserved to used on both rows, writes the ledger row, deletes the
      reservation, and raises `ai.allowance_warning` once a month when the teacher crosses
      `warn_percent`. `Release` deletes the reservation and subtracts it. The three are one
      package the operations call; nothing else touches the balances.
- [ ] `core/jobs.ReleaseStaleAiReservations` releases reservations older than 10 minutes;
      `cmd/maintenance ai-reservations-sweep` runs it, under the maintenance role's grant in the
      same migration set.
- [ ] Operations (the contract rides in T-D4.32's PR; the code is here): `GET /teacher/ai/usage`
      (`ai.use`: the month's used, allowance, left, paused, and the five category sums the
      Settings card draws) and `POST /teacher/ai/credit-requests` (`ai.use`; amount 1,000 to
      100,000 and a reason; one pending request per teacher per month; `ai.credit_request` to
      every `ai.credits.manage` holder); `GET /admin/ai/credits` (`ai.credits.manage`: the pool,
      the facts, the teachers with their month, the pending requests, the rules), `PATCH
      /admin/ai/rules`, `POST /admin/ai/credits/additions` (an amount; records the approver),
      `PUT /admin/ai/teachers/{userId}/allowance` (custom or default, paused), `POST
      /admin/ai/credit-requests/{id}/approve` and `/decline` (approve raises the allowance by
      the amount; both notify the teacher with `ai.credit_request.decided`). Every admin write is
      one `audit_log` row through the data-modifying CTE pattern. The "On pace for" projection is
      `used / days elapsed × days in month`, computed in the query.
- [ ] `cmd/maintenance ai-credits {status,set-allowance,add-credits,approve,decline,pause,
      resume,set-rules}` calls the same application commands as the operations, audited as
      System with the operator named in `details`; it is Thuong's console until R5's T-R5.35
      (Q28). `operations.md` documents it.
- [ ] `credits_test.go` (integration): a reservation that would cross the pool is refused by the
      constraint and nothing is written; two concurrent reservations against a pool with room for
      one leave exactly one; settle moves exactly the metered amount; release returns it; a
      paused teacher is refused before any lock; the month rolls over in `APP_TIMEZONE`; the
      ledger refuses an `UPDATE` as the app role; a rule change rewrites non-custom allowances
      only; approve raises the allowance and writes the audit row.
- [ ] Rehearsed on a Neon branch of production; the timings go in the release PR.

### T-D4.32 — The teacher operations and the contract
**Depends on:** T-D4.30, T-D4.31. D4's second contract pull request; T-D4.31's and T-D4.33's shapes ride in it
**Touches:** `api/openapi.yaml` (`PermissionKey` gains `ai.use` and `ai.credits.manage`; the `/teacher/ai/*` and `/admin/ai/*` operations; `WordImport.readWithAi` and `createImport`'s field; `PublicStatus.ai`; `UserPreferences.ai`), `server/gen/openapi/`, `web/src/lib/api/schema.d.ts`, `migrations/NNNNN_add_ai_permissions.sql` (new), `server/internal/platform/httpx/permissions.go` (the two keys' trees), `server/internal/core/router/{ratelimits.go,tests/testdata/permissions.golden}`, `server/tests/isolation_cases_test.go`, `server/internal/modules/ai/{domain/prompts/*.go,application/{query/{generate,check,variant,explanation,distractors,score,insights,cleanup}.go,ports/{tests,questions,grading,analysis}.go}},http/ai.go}` (new), `server/internal/core/adapters/ai_reads.go` (new: the ports over the tests, questions, attempts and assignments modules), `server/internal/modules/attempts/{application/query/wrong_answers.go,repositories/analysis.go}` (the aggregate), `server/internal/modules/ai/application/tests/{prompts_test.go,operations_test.go}` (new), `docs/plan/70-redesign-overview.md` §4.1 (two rows; on the branch already)
**Size:** L
**Done when:**
- [ ] The contract. `PermissionKey` gains `ai.use` ("Use AI features", Content, Admin and
      Teacher) and `ai.credits.manage` ("Manage AI credits", System, Admin); the migration
      inserts both into `app.permissions` (`in_matrix` true, DG+18) and grants `ai.use` to the
      built-in Teacher role; the Admin wildcard holds both. `httpx.PermissionRequirements` lets
      the `/teacher` tree take `ai.use` and the `/admin` tree `ai.credits.manage`.
      `permissions.golden` is regenerated once, in this PR. Operations, all `ai.use`, all `POST`,
      all answering a suggestion object and never writing content:
      - `/teacher/ai/questions/generate` `{source | testId + stimulusId, types ⊆ the five stored
        types, count 3 | 5 | 8, level, explanations, language}` → `{items: [{type, prompt,
        options?, answerKey, explanation}]}` (`x-resource`: body `testId` kind `test`);
      - `/teacher/ai/tests/{id}/check` `{checks}` → `{findings: [{severity, questionId, number,
        type, title, issue, fix: {kind: option_text | accepted_answer | explanation | none,
        payload}}]}` (path `id` kind `test`);
      - `/teacher/ai/tests/{id}/variant` `{changes, difficulty}` → `{changes: [{questionId,
        kind, before, after, payload}]}` (kind `test`);
      - `/teacher/ai/questions/{id}/explanation` `{language, length}` → `{text}`;
        `/teacher/ai/questions/{id}/distractors` → `{items: [{text, why}]}` (kind `question`);
      - `/teacher/ai/attempts/{attemptId}/answers/{questionId}/score-suggestion` → `{score,
        confidence, reasons, comment}` with `score` snapped to the card's set (`attempt`,
        `question`; the attempt must be in the caller's scope as the grading reads are); refused
        with `403 AI_READ_OFF` while the teacher's `readStudentAnswers` is off;
      - `/teacher/ai/assignments/{id}/insights` → `{basis, items: [{kind, title, body,
        action}]}` (kind `assignment`; the same preference gate);
      - `/teacher/ai/bank/cleanup` `{checks}` → `{duplicates: [{a, b, why, usage}], tags:
        [{questionId, prompt, level, tags}]}` over the caller's own rows, in chunks, capped at
        2,000 questions and a 60s budget (`x-resource-list`).
      Every operation declares its `x-resource` kinds and has its isolation-suite entry; another
      teacher's id answers as a missing one. `make gen` is its own commit; MSW fixtures validate
      with ajv; `pnpm typecheck` passes.
- [ ] `PrincipalRateLimits()` gains every AI operation at 10 a minute and 100 an hour per actor,
      the clean-up at 3 an hour, the credit request at 5 an hour; `credential_limits_test.go`'s
      form of assertion lists them.
- [ ] Each operation reserves its ceiling (the table above) before the call, settles the metered
      usage after it, and releases on failure; the response carries `usage: {credits, left}` for
      the panel's footer. With the adapter nil: 501. With the budget gone: 402 as T-D4.31 says.
      A provider failure after retries: `503 AI_UNAVAILABLE` with `Retry-After`, and the
      reservation released.
- [ ] Data minimisation is tested, not described: `operations_test.go` drives each operation
      against a fake `Completer` that records the request, and asserts that no student name,
      email, user id, attempt id or class name appears in any content block; the score and
      insights cases run with a fixture whose student is named in every other field.
- [ ] The wrong-answer aggregate (`attempts/application/query/wrong_answers.go`): per question of
      an assignment's default version, the distinct normalised answer texts that did not earn
      full points with their counts, the eight most frequent, over handed-in attempts in scope;
      choice answers as option letters; `x-permission` through the insights operation only.
- [ ] Prompts: one Go constant per feature with its schema; every prompt names the UI language
      for any student-facing text; the insights prompt asks for patterns and actions, never a
      judgement of a student; `prompts_test.go` parses the deck's demo arrays against each
      schema and asserts the snapping of a suggested score to the allowed set.
- [ ] Leak review in the PR: no AI response reaches a student; the four forbidden keys are not
      involved; `payload_test.go` and `openapi-contract.test.ts` pass with their lists untouched.
- [ ] Live smoke on the builder's machine (the key in `.env`): one generation from the deck's
      passage, one check of a seeded test, one score suggestion on a seeded short answer; the
      metered credits and the ledger rows pasted in the PR, the responses not.

### T-D4.33 — "Read with AI" in the import worker
**Depends on:** T-D4.30, T-D4.31; T-D4.32 merged (its contract PR carries `readWithAi`)
**Touches:** `migrations/NNNNN_add_word_imports_read_with_ai.sql` (new), `server/internal/modules/imports/{domain/{source.go,pdf.go},application/command/{create.go,process.go},repositories/imports.go,http/imports.go}`, `server/cmd/import-worker/`, `server/internal/core/adapters/{import_pdf.go,import_ai_reader.go (new)}`, `server/internal/platform/pdftext/` (page rendering through `go-pdfium`), `server/internal/modules/imports/application/tests/ai_reader_test.go` (new), `docs/plan/17-word-import.md` (§1, a checkpoint; §8's "not enabled by default" superseded by D19), `docs/plan/18-word-import-ux.md` §4, `docs/setup/word-import-worker.md`
**Size:** L
**Done when:**
- [ ] `createImport` takes `readWithAi: boolean` (default false; refused with 422 when the
      adapter is nil or the caller lacks `ai.use`); `WordImport` carries it; the column has a
      constant default, so the v0.10.0 binary keeps inserting.
- [ ] With it on, a PDF that fails the text-layer check (`PDF_NO_TEXT`) is not refused: the
      worker renders its pages (up to 40; a longer file is refused with `PDF_TOO_LONG_FOR_AI`
      naming the limit), **reserves 60 credits a page before the first call** (a refusal is the
      import's failed state "Not enough AI credits for {n} pages", never a half-read file), sends
      each page to the capable tier with the recogniser's schema, and feeds the returned
      candidates into the existing pipeline as **untrusted candidates** (17 §8): every block
      still passes validation, review and the transactional commit; a page the model cannot read
      is a finding on that page, not a silent gap. A `.docx` or a PDF with a text layer is read
      as today whether or not the switch is on.
- [ ] The processing log names the reader ("Read 12 pages with AI · 190 credits"); the history
      row's meta shows it; the privacy callout on the import page (`18-word-import-ux.md` §4) says
      what left the system ("The pages of this file were read by an AI service outside Quizzivy.").
- [ ] Nothing of a student leaves: the import holds exam documents; the task adds a check that
      refuses a file whose metadata or first page names it as a student's submission is **not**
      attempted (a false sense of safety); the page's sentence carries the responsibility.
- [ ] `ai_reader_test.go`: a scanned fixture (the corpus of 17 §1.1 has 49 two-page PDFs; one is
      rendered to images in the test) goes through a fake `Completer` and yields candidates the
      recogniser accepts; the reservation precedes the first call; a mid-file failure releases the
      remainder and fails the import; the 40-page refusal; the switch off keeps today's
      `PDF_NO_TEXT`.
- [ ] Live smoke: one two-page scan from the corpus, read for real; the credits in the PR.

### T-D4.34 — The AI panel and its entry points: the builder, the bank, Test detail
**Depends on:** T-D4.32; T-D4.2a and T-D4.14 merged (the builder's title bar is shared); T-D4.20
**Touches:** `web/src/features/ai/` (new: `components/{AiPanel,AiPanelField,AiItemCard,AiRunSteps,AiCreditNote}.tsx`, `kinds/{generate,check,variant,cleanup}.ts`, `api.ts`, `useAiAvailable.ts`), `web/src/features/tests/pages/teacher/{TestBuilderPage,TestDetailPage}.tsx`, `web/src/features/question-bank/pages/teacher/QuestionBankPage.tsx`, `web/src/app/router.tsx` (nothing routes: the panel is a dialog), locales, `web/tests/units/ai/` (new), `web/tests/integration/router-chunks.test.ts` (run, not edited: `features/ai/` must stay out of the entry chunk)
**Reuses:** `Sheet` (side, 560px), `Segmented`, `Switch`, `Callout`, the deck's chip and pill primitives, `BulkTagDialog`'s write, `duplicateTest`, the section unit writes
**Size:** L
**Done when:**
- [ ] `AiPanel` is the deck's drawer (V-14) on the `Sheet` primitive from the right,
      `min(560px, 100%)`, with the three stages, the field types, the run steps and skeleton, the
      item cards and the footer's credit note from the operation's `usage`; and the states the
      deck lacks (DG+20): Esc, a focus trap and return (DG-36); "Try again" on `503`; the 402
      sentence "Your allowance is used up. Ask for more in Settings." (or "The school's AI pool
      is used up this month."); `403 AI_PAUSED` as "AI is paused for your account."; a response
      that fails the schema as the warning callout. No demo content ever renders.
- [ ] Entry points. The builder's title bar gains "AI check" and "Generate" before Preview
      (V-09) and is re-measured for one row at 768 collapsed, 1024, 1280 and 1440 in both
      languages (T-R4.31a's rule); Preview still flushes pending edits first, and so do both AI
      buttons. The bank's header gains "Clean up" and "Generate" (V-08). Test detail gains "Make a
      variant" (V-12), disabled without a published version. Every entry point is drawn only when
      `useAiAvailable()` (the status read and `ai.use`) is true, and disabled with a tooltip when
      the usage read says paused or exhausted (Q34).
- [ ] Generate: the source chips offer the test's shared-content passages by title ("Passage 1 ·
      …") and "Start empty"; types are the five stored types (TFNG joins in R6, DG+21); "Add {n}
      to {section}" creates the accepted items as questions in the open question's section
      through the existing writes, with the "AI draft" tag when the preference is on, and opens
      the first; in the bank "Save {n} to the bank"; the toast is the deck's.
- [ ] Check: findings as drawn; "{fix}" is one click only for a fix whose write exists (option
      text, accepted answer, explanation) and calls `updateQuestion` with the payload; otherwise
      "Open question" and "Dismiss" only (DG+21).
- [ ] Variant: the before / after cards; "Save {n} changes as a draft" calls `duplicateTest` into
      "{title} · Variant" and applies the accepted changes to the copy's draft questions through
      `updateQuestion`; the toast reads "Variant saved as a draft. Open the variant." with a link
      (DG+16).
- [ ] Clean up: a duplicate card offers "Open" for each question and "Keep both"; no merge
      (DG+16); "Add tags to {n} questions" is the bulk tag write.
- [ ] Every write a primary makes goes through the existing mutation hooks and their autosave
      flushes; the panel itself calls only `/teacher/ai/*`. A unit test spies on the API module
      and finds no write from `features/ai/`.
- [ ] Strings vi first, en from the deck, listed in the PR; `features/ai/` is a lazy chunk;
      `router-chunks.test.ts` passes untouched.
- [ ] Compared with the deck at 360, 768, 1024, 1280 and 1440, light and dark: the panel in its
      three stages for each kind, and the three headers. Known differences: DG+16, DG+20, DG+21.

### T-D4.35 — Question editor: "Write with AI" and "Suggest wrong options"
**Depends on:** T-D4.32, T-D4.34 (shares `features/ai/api.ts` and the credit note)
**Touches:** `web/src/features/question-bank/components/{QuestionEditor,ExplanationField,OptionsEditor,AiExplanationPanel (new),AiDistractorsPanel (new)}.tsx`, locales, `web/tests/units/question-bank/editor-ai.test.tsx` (new)
**Size:** M
**Done when:**
- [ ] On both pages that mount `QuestionEditor` (the bank and the builder, DG-108): the
      Explanation label row gains "Write with AI" (28px; "Writing…"; "Write again" after), the
      field keeps its `<label for>` (DG-138: the deck's `aria-label` is not taken); the panel
      "Suggested explanation" with "Use this" (writes the text into the field through the editor's
      own change handler, so autosave sees it), "Shorter", "In Vietnamese" / "In English", and
      "Discard" (V-10). The explanation is written in the preference's language.
- [ ] For single and multiple choice: "Suggest wrong options" beside Add option; the panel
      "Suggested wrong options" with up to four rows, "Add" fills the first empty option or
      appends while under the cap of 8 (DG-63; the deck's 6 is not taken), "Added" after; the
      error line on 503.
- [ ] Both guarded by "Write the question first" / "Mark the correct answer first" as toasts;
      both hidden without `useAiAvailable()`; both disabled with the reason when paused or
      exhausted.
- [ ] Unit tests on both hosts; compared with the deck at 360 and 1280, light and dark.

### T-D4.36 — Grading: "Suggest a score"
**Depends on:** T-D4.32, T-D4.34
**Touches:** `web/src/features/attempts/components/{GradingCard,AiScoreBox (new)}.tsx`, locales, `web/tests/units/attempts/grading-ai.test.tsx` (new)
**Size:** S
**Done when:**
- [ ] Between the student answer and Score, while the teacher's `readStudentAnswers` is on and
      AI is available: the box of V-07 with its idle, running and done states; the confidence
      pill; "{score} / {max}"; the reasons; the comment; "Use score and comment" sets the score
      button and the comment field through the card's own handlers (nothing is saved until the
      teacher's Save & next, as today) and toasts the deck's sentence; "Try again". The idle
      sentence is the short-answer one; the band sentence waits for R6 (DG+21).
- [ ] With the preference off the box is absent and a line under the answer reads "AI
      suggestions are off in Settings › AI assistant." (one sentence, from deck parts).
- [ ] Unit tests: the gate, the fill, that no request carries the student's name (the fixture
      names one); compared with the deck at 360 and 1280, light and dark.

### T-D4.37 — Assignment detail: "Explain results with AI"
**Depends on:** T-D4.32, T-D4.34, T-D4.22
**Touches:** `web/src/features/assignments/pages/teacher/AssignmentDetailPage.tsx` (the Questions tab), `web/src/features/assignments/components/AiInsightsPanel.tsx` (new), locales, `web/tests/units/assignments/insights.test.tsx` (new)
**Size:** S
**Done when:**
- [ ] The Questions tab's header row gains the button and the panel of V-06; "Review answers"
      opens Grading narrowed to the assignment; "Make a practice set" opens the Generate panel
      with the insight's types preselected and the primary "Save as practice test", which calls
      `duplicateTest`'s create path for a new draft test "{title} · Practice" with the accepted
      questions. "Based on {n} attempts" is the operation's `basis`; "Hide insights" closes the
      panel for the visit.
- [ ] Hidden without `readStudentAnswers`; disabled with the reason when exhausted; "Try again"
      on 503.
- [ ] Unit tests; compared with the deck at 360, 1024 and 1280, light and dark.

### T-D4.38 — Settings › AI assistant, and the credit request
**Depends on:** T-D4.32 (the usage read, the request operation, `UserPreferences.ai`)
**Touches:** `web/src/features/settings/sections/AiAssistant.tsx` (new), `web/src/features/settings/pages/teacher/TeacherSettingsPage.tsx` (`/teacher/settings/ai`), `web/src/features/auth/preferences.ts` (`ai` defaults), locales, `web/tests/units/settings/ai-assistant.test.tsx` (new), `web/tests/e2e/settings-navigation.spec.ts`
**Size:** M
**Done when:**
- [ ] The sixth section (V-13): the usage card from `GET /teacher/ai/usage` (used of allowance,
      left, the bar, the five category rows) with "Usage resets on {1 Month}" computed in
      `APP_TIMEZONE`; the Preferences card (`UserPreferences.ai {language, tagDrafts,
      readStudentAnswers, readImports}`, saved through `PATCH /auth/me` as the other
      preferences are; defaults en-or-vi from the locale, on, on, off) with the deck's three
      switches and the language segment.
- [ ] "Ask for more credits" under the usage card (DG+19): amount and reason, `POST
      /teacher/ai/credit-requests`; "Request sent. Your admin will be told."; the pending request
      shown until decided; hidden while the rule is "Pause until next month".
- [ ] With AI off (501): the section shows one sentence, "AI is not turned on for this school."
      (DG+20), and the nav keeps the item so the teacher knows where it would be.
- [ ] The `handle.permission` of the section is `ai.use`; a teacher without it gets the 403 page
      as the other sections do (T-R5.20's guard pattern, built here for the teacher tree if it is
      not there yet).
- [ ] Unit tests and the settings E2E spec's navigation case; compared with the deck at 360, 768
      and 1280, light and dark.

### T-D4.39 — Imports: the "Read with AI" card
**Depends on:** T-D4.33, T-D4.38 (the preference's default)
**Touches:** `web/src/features/imports/components/SourceIntake.tsx`, `web/src/features/imports/pages/teacher/ImportNewPage.tsx`, locales, `web/tests/units/imports/intake.test.tsx`
**Size:** S
**Done when:**
- [ ] The toggle card of V-11 above the slots, prefilled from `preferences.ai.readImports`,
      switching the accepts line; the privacy callout gains the sentence of T-D4.33 while it is
      on; `createImport` carries `readWithAi`. Absent without `useAiAvailable()`; disabled with
      the reason when exhausted (the 402 would otherwise arrive after upload).
- [ ] The processing and history screens show the reader's log line and meta (T-D4.33).
- [ ] Unit tests; compared with the deck at 360 and 1280, light and dark.

### T-D4.40 — Documents for the AI assistant
**Depends on:** T-D4.32 merged (the behaviour exists); runs before T-D4.12
**Touches:** `docs/quizzivy-spec-v0.3.md` (§1.3, §2, §8, §12, §13, §15, §16, §17, the "Changes since" head; version per §18), `AGENTS.md` ("Non-negotiable", "High-risk areas", "Repository map", "The composition root"), `docs/plan/70-redesign-overview.md` (§4.1 is on the branch; §10), `docs/plan/17-word-import.md` §8, `docs/plan/18-word-import-ux.md` §4, `docs/design/gaps.md` (DG+15 to DG+21 "what was built"), `docs/setup/operations.md`
**Size:** S
**Done when:**
- [ ] The spec: §2 names the provider behind `platform/anthropic` and the tiers as
      configuration; §8 gains the teacher AI surfaces and Settings › AI assistant, and the Admin
      "AI credits" page with "Availability: R5"; §12 records the panel's rules (the acceptance
      rule, the disabled-with-reason rule, the credit note); §13 the six tables and the
      append-only ledger; §15 the operations; §16 D4's scope; §17 a decision "cloud AI processing
      is on for the assistant's features (D19), with per-feature minimisation (D20) and a
      pool-constrained budget (D21)"; the "Changes since v0.7" sentence is marked superseded by
      D19 for these features. Version bumped per §18, after #429's v0.64.
- [ ] AGENTS.md: "Non-negotiable" gains "The AI assistant never writes a grade, a question or a
      comment: a suggestion is stored only when a teacher accepts it through an existing write;
      no AI operation sends a student's name, email or id; `ANTHROPIC_API_KEY` is never logged";
      "High-risk areas" gains `server/internal/modules/ai/` and `web/src/features/ai/`; the
      composition-root table gains `adapters/ai.go`'s nil rule and `wiring/ai.go`; the repository
      map gains the module and `platform/anthropic`.
- [ ] `17-word-import.md` §8 and `18-word-import-ux.md` §4 record D19 (cloud approved; the
      admin-managed configuration is the environment and `ai_settings`; the page says what is
      sent); 70 §10 keeps the open item on Q35's two remaining pieces.
- [ ] `gaps.md`: DG+15 to DG+21 say what was built.

---

## E. Close-out

### T-D4.12 — Suites, browser verification and documents
**Depends on:** T-D4.2a to T-D4.2c, T-D4.3 to T-D4.11, T-D4.13 to T-D4.16, T-D4.20 to T-D4.23, T-D4.30 to T-D4.40, as far as each is in the release
**Done when (v2):**
- [ ] The four redrawn R4 screens (section F) and every AI surface (section G) are compared with
      the v2 deck at 360, 768, 1024, 1280 and 1440, light and dark, with the sidebar collapsed
      where the deck draws a separator; the differences listed are DG+15 to DG+21's and no other.
- [ ] `74-r4.md`: T-R4.4, T-R4.25, T-R4.26, T-R4.27a, T-R4.28, T-R4.30, T-R4.32, T-R4.33,
      T-R4.36, T-R4.38, T-R4.42 and T-R4.43 each gain one closing line naming the T-D4 task that
      reopened them (T-R4.31a's line names T-D4.2a, T-D4.20 and T-D4.34).
- [ ] With the key unset on a clean checkout, `pnpm e2e` and `make test-api` are green and no AI
      entry point is drawn; with the key set on the builder's machine, the live smoke of the exit
      criteria runs once and its metered credits go in the PR.
**Done when (fourth export):**
**Touches:** `web/tests/e2e/{student-interactions,student-engine.phone,group-attempt}.spec.ts`, `AGENTS.md` ("Design"), `docs/quizzivy-spec-v0.3.md` (§9 S-05, S-06, S-08; §11; §12; version per §18), `docs/plan/73-r3.md` (T-R3.9b, T-R3.9c), `docs/plan/74-r4.md` (the six reopened tasks), `docs/design/gaps.md`, `docs/design/README.md`
**Size:** L
**Done when:**
- [ ] The fixture papers are T-D4.3's (s1, s9 with 34 questions, s2 with both parts); this
      task adds none and checks that none gained an answer key on the way.
- [ ] Every engine frame is compared with the deck at 360, 768, 1024, 1280 and 1440 and at 419
      / 420, 899 / 900 and 1179 / 1180, light and dark, sizes measured with
      `getBoundingClientRect`, the dev server confirmed fresh. Every frame of DG-80's list is
      looked at in Vietnamese at 320, 360, 768 and 1280: nothing scrolls sideways, overlaps or
      is clipped.
- [ ] On a real phone and in mobile-chromium: a recording plays from inside the drawer, keeps
      playing with the drawer closed, and the handle's dot shows it. On the real phone, with
      seeking off: leave the group mid-play, come back and resume; the recording goes on from
      where it stopped and the count has not moved (T-D4.8's rule, on iOS Safari, where
      metadata may not load before the first play).
- [ ] `pnpm e2e:live` passes E2E 2 and 5 to 9, E2E 9's key list unchanged.
- [ ] The teacher side is compared with the deck at 360, 768, 1024, 1280 and 1440, light and
      dark: the builder with a shared-context group holding a passage with two gaps and one
      holding a recording; the Shared content dialog in its three kinds; the question media
      block and the group's recording with "Plays" and "Students can pause"; the three
      previews as Computer and as Phone, with the engine in the frame. The differences listed
      are DG+5, DG+6 and DG+7's, and no other.
- [ ] AGENTS.md's engine paragraph is rewritten: the rail at 900 and 1180, the grid button and
      sheet elsewhere, the drawer below 768, shared content in the stimulus pane, the cloze
      stop, no strip. Its sentence "so the strip can leave a gap between parts" becomes "so the
      rail and the sheet can head each part".
- [ ] Spec S-05 says a section's title and instructions show above each of its questions;
      S-06 and S-08 describe the rail and the sheet; §12 records the thresholds.
- [ ] `73-r3.md`: T-R3.9b's and T-R3.9c's "As built" each gain one closing line naming the
      T-D4 tasks that replaced the switcher, the strip, the first-question instructions and the
      shared recordings' place. Their text is not rewritten.
- [ ] `74-r4.md`: T-R4.31a, T-R4.31b, T-R4.30, T-R4.34, T-R4.39 and T-R4.65 each gain one
      closing line naming the T-D4 task that reopened it and what it replaced. Their text is
      not rewritten.
- [ ] AGENTS.md, "Redesign in progress": T-D4.1's bullet leaves; the engine paragraph under
      "Design" carries the rules.
- [ ] `gaps.md`: DG-80's list is trimmed as T-D4.1 says; the rows DG+1 to DG+13 say what was
      built.

### T-D4.18 — Release v0.11.0
**Depends on:** T-D4.1 to T-D4.16, T-D4.19 to T-D4.23, T-D4.30 to T-D4.40
**Touches:** `web/package.json`, `docs/plan/74d-d4-deck-update.md` (the checklist), `docs/plan/70-redesign-overview.md` (§2: D4 released), `docs/plan/75-r5.md` to `docs/plan/80-r10.md` (their titles take the versions 70 §2 names, Q29), release notes
**Size:** S
**Done when:**
- [ ] The release checklist below is complete, with Thuong's go before `main`
- [ ] `75-r5.md` names the release R5 follows: under "Schema changes", "the v0.10.0 binary
      keeps inserting" reads "the D4 binary (v0.11.0)", and T-R5.1's "Depends on" reads "D4
      (v0.11.0) in production". The steps are still the ones R4 owes; D4 adds none.
- [ ] `ANTHROPIC_API_KEY` and the tier variables are set as Fly secrets before the API deploys
      (Thuong, `docs/setup/operations.md`); without them the release is still complete and the
      AI is off.

---

## Release checklist

**v0.11.0, the deck update and the AI assistant** (Q29, decided 2026-10-10)

- [ ] Every task is ticked on `work/deck-d4`. `develop` is merged into it, and verification is
      re-run after that merge. Then `work/deck-d4` → `develop` (`--no-ff`), and
      `release/0.11.0` is cut from `develop` at a named commit.
- [ ] Every CI step runs on the release pull request, in order, judged by exit code (70 §8.1):
      - `make lint`, `make gen-check`
      - `make test-api`, and goose up/down/up
      - `pnpm lint`, `pnpm typecheck`, `pnpm format:check`
      - `pnpm test:unit`, `pnpm test:integration`, `pnpm build`
      - `pnpm e2e` (chromium and mobile-chromium), `pnpm e2e:content`
      - `pnpm e2e:live`, required because take-test, integrity and media changed
      - `node scripts/check-design-deck.mjs`, which prints "Deck OK: 15 files, 7 pages"
- [ ] The five canaries pass, and their files are as they were at `v0.10.0`:
      `git diff --stat v0.10.0 release/0.11.0 -- <the five>` is empty.
- [ ] Each task that touched a high-risk area has its before-and-after suite runs and its deck
      comparison in its PR: T-D4.3 to T-D4.11, T-D4.13, T-D4.2a, T-D4.2b, T-D4.2c, T-D4.14,
      T-D4.15, T-D4.16. T-D4.12's matrix is complete.
- [ ] Security: the AI operations are the only operations added and the open list is the same;
      `permissions.golden` changed once (T-D4.32) and holds the two AI keys; R2's isolation suite
      (every `/teacher/ai/*` entry included) and escalation tests pass; E2E 9 passes with its key
      list unchanged (`allowPause` is a rule a student reads, not a key); `payload_test.go`
      walks `StudentAudioPolicy`; no AI response reaches a student payload; `operations_test.go`'s
      minimisation cases pass; the key appears in no log of the rehearsal.
- [ ] Migrations: T-D4.13's four, T-D4.31's six, T-D4.32's one and T-D4.33's one are rehearsed
      on a Neon branch of production. The timings and the up/down/up run go in the release PR.
- [ ] Credits: on the Neon branch, a deliberate reservation past the pool is refused by the
      constraint; `ai_settings` reads 200,000 and 20,000; the maintenance command's `status`
      prints the month.
- [ ] Rolling deploy: the API deploys first, then Pages. A tab of v0.10.0 keeps saving a
      question's and a group's audio policy, and a "cannot pause" set from a new tab survives
      that save: an absent field keeps the stored value (`allow_pause_test.go`, case c).
- [ ] Attempts in progress at the deploy keep their answers, flags and play counts. The draft
      and flag formats and the open-question key did not change (`draft-compat.test.ts`,
      `flags.test.ts`, `open-question.test.tsx`). A play in progress at the reload costs one
      play (T-D4.8), and the release notes say so.
- [ ] A read-only review of everything since `v0.10.0`, by three reviewers (a student in the
      middle of a test and the answer key; listening and its counts; access and the contract),
      finds no blocker.
- [ ] Exam window: a query of production assignments open during the planned window returns
      none, or Thuong picks another window.
- [ ] `web/package.json` reads 0.11.0, set on the release branch.
- [ ] Release notes, vi first:
      - the test screen shows the parts and their questions at the side on a computer, and
        from one button elsewhere
      - on a phone the passage or the recording opens from the left edge
      - a recording has its own card, and pausing it no longer costs a play
      - a fill-in-the-gaps passage is answered on one screen
      - tests in progress keep their answers; a recording that was playing at the reload
        counts one more play
      - for teachers: a group's passage or recording is edited in "Shared content"; the
        preview is the student's own screen, on a computer or a phone; a teacher can stop
        students pausing a recording
      - for teachers: the AI assistant: questions from a passage, a check of a test, a variant,
        explanations and wrong options, a score suggestion, an explanation of a class's results,
        scanned tests read on import; every suggestion is yours to accept or edit; your usage and
        preferences in Settings › AI assistant; the school's monthly pool and your allowance
      - for the owner: `cmd/maintenance ai-credits` until the Admin page (R5)
      - the assignment's progress card filters the roster; the join-code card shows the QR for a
        projector
- [ ] Deploy and smoke:
      - Thuong's go, then `release/0.11.0` → `main`
      - `ANTHROPIC_API_KEY` and the tier variables set as Fly secrets (or deliberately left unset:
        the AI is then off and the release is still whole)
      - deploy the API (the release command migrates), then Pages
      - smoke `/healthz`, `/livez`; `/version.json` reads 0.11.0; a sign-in as an admin and as
        a student
      - a student takes a short test with a passage, a cloze and a recording in the owner's
        test class, once on a phone and once on a laptop; the recording is paused and resumed
        and its count moves once
      - the owner opens a group's Shared content, saves a passage with one gap, and previews
        the test as Computer and as Phone
      - with the key set: the owner generates three questions from the test's passage, asks for a
        score suggestion on one short answer, and reads the ledger with `ai-credits status`; the
        three calls are metered and the pool moved by their sum
      - tag `v0.11.0`, and back-merge `main` to `develop`
- [ ] The documents changed with the code (70 §3): AGENTS.md's engine paragraph, its
      "Redesign in progress" bullet and T-D4.40's three additions, the spec's S-05, S-06, S-08,
      §11, §12, §13 and §15 and T-D4.40's sections, `20-data-model.md`, `gaps.md`,
      `docs/design/README.md`, `docs/setup/operations.md`, `.env.example`, and the closing lines
      in `73-r3.md` and `74-r4.md` (T-D4.12).

## Open items

Every question below has the default this file builds, what the other answer costs, and who
answers. "Owner" is Thuong; "design" is the design team. Nothing waits for an answer, with two
exceptions: the play-limit lock (Q3) is not built without the owner, and T-D4.10 gives the
design team two working days on Q6, counted from the day the question is sent, which need not
wait for T-D4.1.

### Decided

**Q1. Which phase is "this phase"?** Decided by Thuong, 2026-10-04: R4. The deck update is D4,
a phase of its own after v0.10.0, and R4 is not amended to the export while it is in flight.

**Q2. Do the fixes already on `develop` wait for the deck update?** Decided by Thuong,
2026-10-04: no, and they get no release of their own. They ship with v0.10.0; R2's contract
steps ship as T-R4.49 if v0.9.1 has not shipped before R4.

**D19 to D22 (D-AI-1 to D-AI-4). The AI assistant.** Decided by Thuong, 2026-10-10, recorded in
`70-redesign-overview.md` §1: Anthropic's API for every AI feature, cloud processing approved
and on, the adapter behind a module port that answers 501 with the key unset (D19); data
minimisation per feature, no personal identifier to the provider unless a feature cannot work
without it, and none does (D20); a $200 monthly pool, exceeded only when an Admin approves extra
credit that Thuong pays for, a hard stop, a credit unit metered from reported usage (D21); built in
D4, the release after R4 (D22). The spec records them in the task that ships the behaviour
(T-D4.40), not on the planning branch, because #429 takes v0.64.

### Blocking, or close to it

**Q3. May a recording lock at its play limit?** (owner, then design)
- The deck: lock icon, "No plays left", "You have used all {N} plays", and the button does
  nothing (S 1677-1681).
- Against it: spec §11.4 ("Over-limit plays are reported to the teacher, not enforced"), the
  contract (`AudioPolicy.maxPlays`: "never blocked"; `recordGroupAudioPlay`: "never block
  playback or submission"), AGENTS.md ("the teacher judges; the app reports"), and the canary
  case "still plays when the hint says none are left" (`audio-player.test.tsx:105-112`).
- Default: the decision stands. The new card is built; at the limit the button stays live and
  the status adds the product's "You can keep listening. Additional plays are recorded for your
  teacher to review." (DG+1).
- Cost of the other answer: the owner changes §11.4 and that canary case himself; a student
  whose play was counted twice, or counted while offline, is locked out of a recording they
  did not hear, and only a teacher's intervention reopens it. The lock in the browser alone is
  S; a limit the server enforces is a new refusal on two operations.

**Q4. "Students can pause": build it, and as what?** (owner, then design)
- Now drawn on the builder's question media, on Shared content, and acted on by the Student
  page. The intro fixture says "you can pause once"; the script keeps an unread `pauses` count.
- Default: one boolean, `allowPause`, on by default, on a question's audio and on a group's
  recording (T-D4.13, T-D4.9, T-D4.15). When off, the button is inert while playing; if
  playback stops anyway (a headset button, leaving the group, a lock) the student resumes
  without spending a play, and nothing new is recorded. A save that does not send the field
  (a tab loaded before the release, or a draft recovered from the device) keeps the stored
  value, so it cannot switch pausing back on.
- When: T-D4.13 is D4's server task. It builds the default if no answer has come when it
  starts, and T-D4.15 adds the switch R4 left out (T-R4.65).
- Cost of "do not build": T-D4.13 and T-D4.9 go, T-D4.15 keeps only its "Plays" bullet and
  its comparison, DG-111's default stands, and the deck's switch is left off two teacher
  surfaces. Cost of "a number of pauses": a counter the server must own as
  it owns plays (a column, a ledger, an operation with its permission and isolation entry), L,
  and a student rule nobody has drawn.

**Q5. What spends a play?** (owner)
- Today every press of play counts, a resume included, so one pause costs a play
  (`AudioPlayer.tsx:108-110`). The deck spends one only on a start from 0:00.
- Default: a start with no play in progress spends one; a resume spends none; a play in
  progress and its position are remembered while the tab lives, so coming back to a group
  resumes; a reload forgets, and the next start counts. If the place cannot be put back (the
  browser refuses, or the file's length never loads), that start counts as a play: with
  seeking off the player undoes any jump (`AudioPlayer.tsx:240-247`), and a restore that
  failed silently would be a replay from 0:00 that costs nothing. Proven in a browser, not in
  jsdom (T-D4.8). With no limit the counter reads "Play {n}" (the deck's preview reads "Play
  1 of 99"). Spec §11.3 and §11.4 change with it.
- Cost of the other answer: keeping today's rule makes the drawn "Paused · Play 1 of 2" false
  and keeps the defect. Surviving a reload needs the server to know a play is in progress: a
  contract change, M, in a do-not-guess area.

**Q6. Is a cloze one question with n gaps, or n questions?** (design, then owner)
- Student page: one item, "Question 23 of 31 · Cloze · 5 gaps", one square, sub-ids 23.1 to
  23.5. Teacher page: "Each gap is answered by a single-choice question in this group", which
  is the product's model. The builder's own preview never passes a gap to the Student page.
  The deck of record drew the product's model once on a student-eye surface, in the import
  preview (a passage with gap chips, then "23.1 Choose the word for gap 23.1." with its own
  options); this export removed that preview (T 4975-4980).
- Default: one question per gap, each with its number, square, flag, points and result row.
  The engine shows a run of them as one screen, "Questions 23–27 of 34 · Cloze · 5 gaps" on
  the fixture paper (35 once R6 adds the matching), with the gap strip and the gap card as
  drawn (T-D4.10). No contract change.
- Every member is a full question with its own required prompt, and may have media and its
  own points. So: a prompt all members share is printed once, as the deck draws its one
  prompt; otherwise the open gap's card shows that member's own prompt; the card always ends
  with that member's worth; a member that carries an image or audio is not part of the run
  and is shown as a question of its own.
- Cost of the other answer: a new question type with its own answer shape, marking, result,
  authoring and import mapping, across every high-risk area: an R6-sized track. Tests already
  published with gap-bound members would keep the old shape beside it.

**Q7. How is a multi-part question numbered and counted?** (design)
- One matching is "Questions 1–5" and counts five (s2); another is "31" and counts one (s9);
  the cloze counts one; the Teacher import counts the same paper as 35.
- Default: every question has one number and counts once. A matching (R6) is one question,
  "Partly" until every item is matched.
- Cost of the other answer: a display label per question in the contract, and the count
  changing on Home, the intro, the Submit dialog, the roster and the result together.

**Q8. Is the builder's "group" now the shared-context group?** (owner, then design)
- The deck puts "Shared content" on every outline group, which R4's plan mapped to the
  product's Section.
- Default: two levels stay. A Section keeps Rename, Instructions, Move, Remove. "Shared
  content" and its outline line belong to a shared-context group's row inside a section
  (T-D4.2a, T-D4.14). A bank question cannot be dragged into a shared group.
- Cost of the literal answer: a section-wide group and two new operations that turn a bank
  question into a group member and back, through the draft lock, the publish snapshot and the
  deal order: L, high risk, with permissions and isolation cases. A middle answer, "Shared
  content" on a section's menu creating a group in it, is S more in T-D4.14, but its sentence
  "shown beside every question in this group" would be untrue of the section's other questions.

### Design decisions with a default

**Q9. One passage or one recording per group?** (design)
- In the product a recording is always an audio node inside a material (the server refuses
  one without the other), so "Audio" in the dialog stores a material holding that one node,
  named after the file; the student never sees that name.
- Default: the dialog edits a group with at most one material and one recording of one kind.
  A group that holds more (imports, bank copies: up to 16 of each, or a material that mixes
  text and audio) opens the full group pane, and nothing is dropped. On the student side a
  recording's card stands where its audio node is: a material that is only a recording shows
  the kicker and the card, with no heading, as drawn; a material with text and audio keeps
  its heading and has the card in reading order; several recordings each stand at their own
  node, and only then is each labelled. None of the last two is drawn.
- Cost of the other answer: a dialog that is the only editor would lose content on save, or
  needs a drawing for several materials.

**Q10. The passage: a plain textarea, or the content editor?** (design)
- Default: the content editor (T-R4.63's `document` profile) with the deck's labels and
  placeholders, a gap inserted at the caret, targets limited to choice questions and blanks,
  each used once, and no save with an unbound gap. The editor's toolbar already has "Insert
  gap", so the deck's separate button is not built and the dialog has one, not two. The
  dialog saves through the group's own editor, so its autosave, its recovery draft and its
  conflict message stay one mechanism.
- One English sentence is reworded: the deck's "Students see it on the left while they
  answer." becomes "Students see it beside the questions while they answer.", because on a
  phone it is a drawer (DG+6).
- Cost of the other answer: stored passages hold tables, images and gaps a textarea cannot
  show; the deck's looser rules ("[gap n]" at the end, any question, unbound gaps saved) are
  refused by the server today (`group_validation.go:176-212`).

**Q11. Is the preview meant to be answered, with a running timer?** (owner, then design)
- A test has no time limit: `durationMinutes` belongs to the assignment, and neither
  `previewTest`, the builder draft nor the import's review draft carries one. The same holds
  for the integrity policy. So the deck's 45:00 has nothing to read, running or not.
- Default: answerable, kept only while it is open; no saving, no integrity record, no play
  counting, pasting allowed; "Student view" chip; Finish says "This is a preview. Students
  submit here." In the timer's place, a pill of the same size reading "––:––", named "The
  time limit is set when the test is assigned"; in the save line's place, "Answers here are
  not saved." (the deck's preview still says "All answers saved"). T-D4.2c builds the bar and
  the frame around R4's read-only preview first (DG+7).
- Cost of the other answer: read-only saves little, because the engine must run from a source
  other than the store either way. A timer with a number needs an invented limit, or a
  "typical time" field on the test, which nothing else uses.

**Q12. The sections rail returns and the footer strip goes: intended?** (design; owner informed,
since it reverses what shipped today)
- Default: as drawn. The rail from 900 with no shared content and from 1180 with it, so it
  comes and goes between questions from 900 to 1179; the grid button and its sheet everywhere
  else.
- Cost of the other answer: keeping R3's strip turns T-D4.4 from L to S (the third state and
  the label), and stays a recorded departure.

**Q13. The phone drawer: is swipe required, and may the recording be out of sight while a
student answers?** (design)
- Default: the handle and the close button are the controls and swipe is an extra; focus,
  Esc, `inert` (the deck marks the closed drawer `aria-hidden`, which leaves its gap buttons
  reachable by Tab), reduced motion and a once-per-group tip are added (DG+9). The handle
  is named "Open the audio" in English, as the deck has it. The handle's dot is
  the only sign of a playing recording on the question.
- Cost of the other answer: keeping the switcher drops T-D4.7 (L) and leaves a recording on a
  phone with no drawing. A small player on the question needs one.

**Q14. Section instructions above every question?** (design)
- Default: yes, with the section's title, except on a one-section paper with no instructions.
  A group's own instructions stay under the stimulus title. The builder's dialog sentence
  becomes "Students see this above every question in the section."
- Cost of the other answer: none; today's first-question note stays and S-09 is a departure.

**Q15. The engine's type labels become the builder's names?** (owner, for the Vietnamese)
- Default: "Single choice", "Multiple choice", "True / False", "Fill in the blank", "Short
  answer", with the same Vietnamese names as the builder's type menu.
- Cost of the other answer: none; the labels stay instructions ("Choose one").

**Q16. Options in columns: what counts as short?** (design)
- Default: every option plain text of at most 16 characters, and no shared content; two
  columns on a phone, up to four from 768. Rich option content forces one column.
- Cost of the other answer: one column always, as built.

**Q17. A choice sentence with a gap that fills with the chosen option: what marks the gap?**
(design)
- Default: a plain one-line prompt with exactly one run of three or more underscores, on a
  single-choice or true/false question (T-D4.11). Anything else prints as written.
- Cost of the other answer: T-D4.11 is dropped, or a field marks the gap (a contract change).

**Q18. Sign cards, 60px word tiles, the hint line and the image caption: what in the data
triggers each?** (design)
- All four are keyed to fixture ids or fixture fields with no model.
- Default: none is built (DG+8). A notice is an image or bold prompt content; an underline
  comes from the option's rich text.
- Cost of the other answer: a notice block in the content model (contract, editor, engine,
  result), an option-size rule, a hint field and a caption field: R6 work.

**Q19. The fill-in field: 340px and 38px everywhere?** (design)
- Default: the underline style everywhere; 340px wide only when the question has one blank,
  128px otherwise and in tables; 38px from 1024 and 44px below. The case-rule and worth lines
  stay.

**Q20. The key hint names "← →" and "A–D" only.** (design)
- Default: printed as drawn, without "A–D" on a question that has no options; A to E and F
  keep working; on the cloze card the letters choose for the open gap (in the deck they do
  nothing there).

**Q21. The sheet has no title, and four targets are under 44px on a phone.** (design)
- Default: the name "Questions" and the total stay for a screen reader only; the sheet's
  squares and a blank are 44px below 1024; the 34px rail squares (never below 900) and the
  30px drawer handle are as drawn.

**Q22. Multiple choice: capped at two, or "Choose all the answers that are correct"?** (design)
- The Student script caps it; the Teacher preview's hint says the opposite (T 6907).
- Default: no cap until R6's `selectCount`; then R6's rule.

**Q23. Test detail's compare mode lost its outlines and "New" / "Changed" chips.** (design)
- Default: followed. The "Changes from version {n}" list stays above the frame; nothing is
  marked inside it.

**Q24. The import's preview lost "Showing 7 of 34 questions. Answers and your notes are hidden
from students."** (design)
- Default: the whole draft is previewed and the sentence goes. The deck shows a Student
  fixture there, not the import.

**Q25. "Plays" is drawn three ways; the audio line reads "MP3, M4A or WAV up to 25 MB" a third
time; the dialog has no "Show transcript after submitting" and no "Allow skipping ahead".**
(design)
- Default: one control everywhere, T-R4.65's "Once | Twice | Unlimited" with a stored other
  value as a fourth option; DG-63's limits (MP3 or M4A, 50 MB, 5 minutes); both switches kept,
  and "Students see it after they submit." shown only while the first is on.

### Process

**Q26. Import now, and amend R4's six tasks now?** (owner)
- Decided for the import and for every build task: not while R4 is in flight. The export is
  imported by T-D4.1, after v0.10.0.
- Still asked: whether the documents alone (the import, and the amendment of T-R4.31a, 31b,
  30, 39, 34 and 65) may go ahead before those six are built. This file is written for "no",
  and its head says what changes on a "yes".
- Cost of "no": those six are built to drawings the design team has replaced (the Previous /
  Next preview, the "New" / "Changed" chips, "Showing k of n", an outline without the
  shared-content line), then redone by T-D4.2a, T-D4.2b, T-D4.2c and T-D4.15.

**Q27. For the design team: what the next sync should read.**
- This export's sync read `migrations/00037` and `00002` only (`github.md`). Pause is in no
  migration; `allow_seek`, `show_transcript_after_submit` and the `blank` gap kind are in 00037
  and were not drawn; the contract and the engine were not read.
- Request: read `api/openapi.yaml` (`QuestionGroup`, `StudentGroup`, `AudioPolicy`),
  `web/src/features/take-test/**` and `web/src/features/question-groups/**`, and add them to
  the Screen map. Recorded as DG+14.
- Inconsistencies inside the export to send back with it: the cloze counts one and the
  listening matching five; the builder binds a question per gap and the Student fixture is one
  item; multiple choice is capped at two under a hint that says "all"; "you can pause once"
  beside a boolean; the hint says "A–D to choose" on a card where the letters do nothing;
  `previewExam` passes no gaps, media or rich content; `hasRef`, `dotsMode`, `hasPaneTabs` and
  `isSign` are dead branches, `examBold` is a dead field, and `wi.preview`, `td.preview`,
  `td.pvFoot` and the two `pvOptCols` are dead values on the Teacher page; the arrow keys
  bypass `go()`, so they do not close the drawer or the sheet and do not pause a recording;
  Esc does not close the new sheet; the preview's header says "All answers saved"; the preview
  runs a 45:00 timer for a test that has no time limit; the handle says "Open the audio"
  beside a tip that says "The recording is here."; the dialog's "on the left" is not true on a
  phone. From v2: the pool rule "Allow 10% extra" and the two "billed" sentences; a variant
  "saved as a draft version"; "Merge into one"; "up to 6 options"; "Students see their own
  attendance" and "They count as absent until you mark them"; a matrix with no AI row; a request
  inbox with no request form; the AI panel without Esc, a focus trap or an off, paused or failed
  state; the demo output shown as if it were a result.

### v2 (2026-10-10): Thuong's decisions

Asked on 2026-10-10 with the Principal's recommendations; Thuong answered the same day ("good
catch; I agree with the proposals above"). Q28 to Q34 are **Answered 2026-10-10**, each as
recommended; Q35 is answered for its default and stays open for two items.

**Q28. Where does "AI credits" live in D4, when the Admin console is R5's?** (owner)
- T-R4.48 deletes the old `AdminLayout`; T-R5.20 builds the new one in R5.
- **Answered 2026-10-10: (a).** D4 ships the server side in full (`/admin/ai/*` under
  `ai.credits.manage`: the pool, the rules, the allowances, the requests) and `cmd/maintenance
  ai-credits` for Thuong, in the manner of DG-42's maintenance windows; the page is R5's new
  T-R5.35, drawn against those operations.
- Not taken: pulling T-R5.20 (M) into D4 and building T-R5.35 (M) there (2M in D4 and a one-item
  console for a release).

**Q29. Is D4 v0.11.0?** (owner)
- **Answered 2026-10-10: (a), v0.11.0.** A release with a new module, nine operations, two
  permission keys and twelve migrations is a minor by 70 §3's own rule; R5 to R10 move one minor
  each and R11 stays v1.0.0. 70 §2 carries the new versions; the release files' titles take them
  in T-D4.18.
- Not taken: v0.10.1, the first minor-sized release labelled a patch.

**Q30. The default per-teacher allowance.** (owner)
- **Answered 2026-10-10: (a), 20,000 credits** ($20) a month, the deck's lowest rule value; ten
  teachers fill the pool exactly, and a month of ordinary use is about 2,300 credits a teacher
  (section G). The rule still offers 50,000 and 100,000; changing the default is one row in
  `ai_settings`.

**Q31. The pool's rule.** (owner; follows D21)
- **Answered 2026-10-10: the default.** One value, "Pause AI for everyone"; "Add credits" is the
  approval (DG+15). The deck's "Allow 10% extra" contradicts D21 and is not offered.

**Q32. A variant and a merge.** (owner, then design)
- **Answered 2026-10-10: (a).** A variant is a new draft test; the clean-up merges nothing
  (DG+16). Not taken: a variant that overwrites the test's draft, or a merge operation (L, high
  risk: the publish snapshot and the draft lock).

**Q33. "Let AI read student answers": a per-teacher switch, on by default?** (owner)
- **Answered 2026-10-10: (a), as drawn.** D20 is satisfied either way (no identifier leaves); the
  switch gates Suggest a score and Explain results.

**Q34. Paused or out of credits: visible and disabled, or hidden?** (owner)
- **Answered 2026-10-10: (a).** Visible and disabled with the reason, as the deck says ("AI
  buttons stay visible but can’t be used"); hidden only when the provider is off (DG+20).

**Q35. What students and parents read about cloud processing.** (owner; product copy about
minors' data)
- **Answered 2026-10-10: (a) for its default.** The import page's privacy callout says what left
  the system (T-D4.33), and the centre's privacy notice (R11, D18) names the provider.
- **Still open, two items:** the student-facing sentence on the Test intro waits for Thuong's
  wording and is not built in D4 until he gives it; and the provider account's data-retention
  setting is his to confirm. 70 §10 keeps both.

### The gap rows T-D4.1 opens

Fourteen new rows in `docs/design/gaps.md`, all Open. DG+1 moves to "Decisions that override
the current drawings" only if the owner confirms Q3's default. Their ids are assigned when
T-D4.1 runs, the first one being the next free id after the last row `develop` holds then: R4
has DG-120 to DG-122 and may use more. T-D4.1 replaces the labels below, and every `DG+n` in
this file, with the ids it took. If a `develop` sync later brings a row with the same id, the
D4 row is renumbered at that sync. The full "Meanwhile" texts are in T-D4.1.

| Label | Where | What the row records | Question | Built by |
|---|---|---|---|---|
| DG+1 | Student › Take test, a recording at its play limit | The deck locks the recording; the product keeps it live and reports extra plays | Q3 | T-D4.8 |
| DG+2 | Teacher › Question media and Shared content; Student › the recording card | "Students can pause" as one boolean, `allowPause`; "you can pause once" is read as a note | Q4 | T-D4.13, T-D4.9, T-D4.15 |
| DG+3 | Student › Take test, a cloze | One item with n gaps on the Student page, one question per gap in the builder; the product keeps one question per gap | Q6 | T-D4.10 |
| DG+4 | Student › Take test, numbers and counts | "Question 23 of 31" against 35; a matching counts five or one; every question has one number | Q7 | T-D4.10 |
| DG+5 | Teacher › Test builder, the group | The deck's group carries shared content; the product keeps a section and a shared-context group | Q8 | T-D4.2a, T-D4.14 |
| DG+6 | Teacher › Test builder, the Shared content dialog | A textarea, one passage or one recording, loose gap rules, "MP3, M4A or WAV up to 25 MB", two switches missing; "on the left" reworded | Q9, Q10, Q25 | T-D4.2b, T-D4.14 |
| DG+7 | Teacher › the three previews | An answerable take-test screen with a running 45:00 timer and "All answers saved"; the product shows no time and claims no save | Q11 | T-D4.2c, T-D4.16 |
| DG+8 | Student › Take test, frames with no model | The sign card, the 60px word tiles, the image caption, the per-question hint, paragraph letters | Q18 | not built |
| DG+9 | Student › Take test, the phone drawer | No Esc, focus move, `inert`, reduced-motion form or tip rule; the recording is out of sight while answering | Q13 | T-D4.7 |
| DG+10 | Student › Take test, navigation | The rail comes and goes from 900 to 1179; the key hint; a sheet with no title; four targets under 44px; the arrow keys bypass `go()`; Esc does not close the sheet | Q12, Q20, Q21 | T-D4.4 |
| DG+11 | Student › Take test, section instructions | Shown above every question, while the builder says "above the group’s first question"; a group's own instructions are not drawn | Q14 | T-D4.5 |
| DG+12 | Student › Take test, what spends a play | A start spends one and a resume none; unlimited plays read "Play 1 of 99" | Q5 | T-D4.8 |
| DG+13 | Student › Take test, matching | Chips per row, three key styles, the nouns "pick the {pick} for each {what}", 38px chips | Q7 | R6 (T-D4.19) |
| DG+14 | `github.md`, the sync behind the export | The sync read two migrations; the contract, the engine and the group editor were not read | Q27 | the design team's next sync |
| DG+15 | Admin › AI credits, the pool's rules | "Allow 10% extra" and "billed on your next invoice" against D21 and the non-goal on payments; one rule, pause everyone; "Add credits" is the approval | Q31 | T-D4.31, R5 T-R5.35 |
| DG+16 | Teacher › Make a variant; Clean up the bank | A variant "saved as a draft version"; "Merge into one"; the product makes a new draft test and merges nothing | Q32 | T-D4.34 |
| DG+17 | Teacher › Class detail › Attendance (R8) | "Students see their own attendance"; "They count as absent until you mark them"; R8's decisions stand | — (R8) | R8 |
| DG+18 | Admin › Roles & permissions | No AI row; two keys join the matrix | — | T-D4.32, R5 T-R5.24 |
| DG+19 | Teacher › Settings › AI assistant | No request form while the Admin page lists requests; a form under the usage card | — | T-D4.38 |
| DG+20 | Teacher › the AI panel's states | Off, paused, exhausted, failed and invalid states; Esc, focus trap and return | Q34 | T-D4.34 |
| DG+21 | Teacher › the AI against R6 | The band form, TFNG, the word-limit fix and the matching finding wait for R6 | — | T-D4.34, T-D4.36, R6 |

T-D4.1 also edits sixteen existing rows: DG-80 (what the export now draws, and which frames stay
listed), DG-60, DG-63, DG-68, DG-69, DG-111, DG-113, DG-115, DG-116 and DG-36 for the fourth
export; DG-01 (Answered), DG-22, DG-71, DG-106, DG-100 and DG-63 (a second note) for v2.

### Other open items

- **The documents ahead of R4.** The lead has asked Thuong whether the import and the
  amendment of six R4 tasks may go ahead before those tasks are built. This file's head says
  what changes on a yes.
- **A newer export.** D4 starts weeks after the export it describes. If the design team sends
  another before T-D4.1 runs, the inventory is redone against it and this file is corrected
  before any task starts.
- **Q6 and Q7 decide the largest task.** T-D4.10 (L) builds the default. If the design team
  answers "one question with n gaps", T-D4.10 is not built and the cloze becomes an R6-sized
  track with its own plan.
- **The preview's 45:00 timer.** A test has no time limit until it is assigned, so no answer
  from the design team alone makes the deck's timer buildable; it needs a field on the test
  (Q11).
- **"Students can pause" in the other documents.** `70-redesign-overview.md` §10 says it is
  unbuilt until D4, and `74-r4.md`'s open items say R4 does not build it. T-D4.13 corrects
  both when it merges. If the owner declines Q4, 70 §10 records the D1 exception instead.
- **Migration numbers.** T-R4.49 (R2's contract steps), R4's lane and T-D4.13 all number above
  `develop`'s last file. Numbers are assigned at merge.
- **Line numbers of code.** They are those of `develop` at `fd9c763a`. The fixes for #315 and
  #320 have moved the contract and `TakeTestPage.tsx` since, and R4 rewrites the builder,
  `StudentPreview` and the media block, so a D4 builder finds each place by the identifier
  quoted beside the number.
- **A newer export after v2.** This file now describes v2 (`26b60795`). If the design team sends
  another before T-D4.1 runs, it is read against sections 1 to 11 first and this file is
  corrected before a task starts, as before.
- **The design team's next sync should read `server/internal/modules/ai/` and
  `web/src/features/ai/`** once they exist, and draw the states DG+20 lists and the rows DG+18
  and DG+19 ask for.
- **Builders.** The engine chain takes one builder for its whole length and, at times, a
  second for T-D4.6 or T-D4.11; the server task, the builder chain and the preview chain take
  one each. Never more than four work at once (70 §3).
