# Word import — teacher workflow and interaction specification

Status: direction accepted for implementation, 2026-09-22. Companion to
[the delivery plan](17-word-import.md); teacher pilot validation remains pending.
The interaction mockup uses synthetic content and demonstrates review decisions;
it is not a working importer or evidence of recognition quality.

R4 (v0.10.0) rebuilt every import screen on the design deck, under
`/teacher/imports`, and added the pasted-text source. §§1 to 11 are the direction
of 2026-09-22; §12 records the screens as built and wins where it differs: the
route table in §2 is corrected in place, and §12.7 lists what the deck drew that
the product keeps, what it gates and why.

## 1. Design direction

Use Quizzivy's current zinc/charcoal components, divided settings-style rows,
restrained borders/shadows and clear action hierarchy. The reference dashboard's
organization informs spacing and navigation, not decorative charts or extra KPIs.
The workspace uses the available width for comparison. Reading line length is
bounded inside each pane, rather than leaving an unused third of the page.

The default experience asks the teacher to resolve meaningful decisions. Technical
details, parser/model versions and retry diagnostics belong in an optional
processing-history panel. No confidence percentage, “AI magic” decoration, fake
progress, automatic answer invention or blanket “accept all errors” action.

Vietnamese first; every shipped label/help/error has an English key. All status
states have text as well as icons/color. Use “Cần xử lý”, “Cần xác nhận”, “Thông
tin”, “Đã lưu”, “Chưa lưu được”, and “Đáp án chưa có trong tài liệu”. A file that
finished processing says “Sẵn sàng rà soát”, not “Nhập thành công”.

## 2. Information architecture

| Route | Purpose | Entry / primary action |
| --- | --- | --- |
| `/teacher/imports` | Searchable import history | Secondary entry from tests; "Import from Word/PDF" and "Paste a test" |
| `/teacher/imports/new` | Upload or paste, optional key | Tests list split action beside "Tạo đề"; "Bắt đầu xử lý"; `?source=paste` opens the paste mode |
| `/teacher/imports/:id` | Current processing state or resumable import | "Tiếp tục rà soát" when ready; committed state links to resulting test |
| `/teacher/imports/:id/review` | Source-linked correction workspace | "Xem trước & hoàn tất" |
| `/teacher/imports/:id/confirm` | Learner preview and validated summary | "Tạo bản nháp đề" |

There is no `/compare/:runId` route (WU-05): a newer machine draft that arrives
while the teacher has edits is offered inside the review, beside the teacher's
version, and applied with `adoptWordImportReprocessed` (§12.5). The old
`/admin/imports*` paths redirect to these (T-R4.5).

Do not add many permanent global sidebar items. The tests area exposes imports
and history clearly; the dedicated review route can collapse global navigation
while preserving a labeled way back. A direct URL must resolve to a helpful state
for processing, failed, cancelled, stale or committed imports.

These entries follow `GET /teacher/imports/capabilities`:

- **Intake off.** The tests list shows neither entry, and every `/teacher/imports`
  route explains that import is not enabled (`ImportsGate`).
- **Intake on, processing off.** The history, review and commit stay available.
  Upload, retry, reprocess and "start over" are replaced by a calm note saying
  why. A queued import says it waits for processing to return.

Imports create a new test initially. Starting from an existing builder flushes
the existing draft, then clearly states that import creates a separate draft;
it never silently replaces an open exam. Append/merge into an existing test is
not implicitly added to this milestone.

## 3. Screen WU-01 — history

Columns: filename/title, creator, created time, status, unresolved issue counts,
resulting test and row actions. Search and status filters live in the URL and
survive review/back navigation. A row's primary action follows its state:
processing → view progress; needs review → continue; failed → view cause/retry;
committed → open test. Do not auto-start a second import from the same row.

“Tạo đề khác từ tệp này” is a distinct action explaining that it creates a new
import identity. Retrying resumes the current job policy. Bulk removal, where
available, lists eligible items and per-row failures; active/committing/referenced
items cannot be swept away. Archive/delete follows the approved retention policy
(`17-word-import.md` §1.38). The upload page states it, and a reviewable import
warns that it closes after the idle period. Once files are removed, the detail
and review pages say so calmly, without download or review links.

Empty states differ: no imports offers upload; no filtered results offers clearing
filters; unavailable service preserves controls and offers retry. History remains
useful when the AI provider is disabled.

## 4. Screen WU-02 — upload and setup

Use a page, not a long modal. The essential fields are visible without an
accordion: exam file; optional answer-key file; optional audio attachment/library
selection. Show filename, actual validation status, size, role and remove/replace
action. Drag-and-drop has an equivalent labeled file picker. Input limits appear
before upload and authoritative server errors appear beside the affected file.

Recognition: “Tự nhận diện” by default; “Dùng cấu hình đã lưu” and a downloadable
optional template remain secondary. Advanced settings cover only explicit source
conventions: numbering restarts, answer markings and score defaults. A profile
must show a concrete example and its scope before applying; never expose a raw
prompt editor to teachers.

If AI processing is configured, show where processing occurs and what data is
sent in plain language. This uses an admin-managed approved configuration. Teachers
do not choose infrastructure/model knobs on every import. Private mode must not
silently use cloud when unavailable.

Preflight handles password protection, mismatched extension, corruption, macros,
scans and limits distinctly. The exam and the key may each be `.docx` or a PDF
with a text layer; the limits line says a scanned PDF is refused. A PDF that
turns out to be a scan, locked, broken or too long fails processing with its own
message and asks for another file (plan 17 §1.39). Replacing a rejected file preserves the other valid
inputs. A partially uploaded companion key never appears as fully received.

### 4.1 Pasted text (decided 2026-10-03, built in R4)

The same page also takes a test pasted as text in place of the exam file. A
"Source" switch chooses between uploading a file and pasting. The paste mode
has a plain text box with a character count against the server's limit, a
quick count of what was found so far, and no answer-key slot. The layout is in
§12.3; the copy and tests are in `74-r4.md` T-R4.57, the source and its
recognition rules in T-R4.55 and T-R4.56, and the scope and what was built in
`17-word-import.md` §1.41.

What the teacher is told holds on every screen:

- **Plain text only.** No screen says formatting was kept, because none is.
  The source is named by a localized "Pasted text" label taken from its
  format, never from a stored file name.
- **The page states the conventions the recognizer reads**, and no others:
  put a heading on each part, number each question, put each option on its
  own line, and mark the answer with `*` after the correct option, an
  "Answer: B" line or one "Answer key: 1-B 2-C" line.
- **The quick count follows the server's rules** for text sources, so it
  cannot contradict the review. It is a hint; the full check runs in
  processing.
- **Later screens name the text, not a file.** Processing checks the text
  rather than the files, review shows the one source as pasted, and the
  original downloads as a `.txt`. A failed pasted import offers a new import,
  which opens the paste mode again.

## 5. Screen WU-03 — durable processing

Compact real stages: “Kiểm tra tệp” → “Đọc nội dung” → “Nhận diện cấu trúc” →
“Đối chiếu đáp án” → “Kiểm tra đề”. Show elapsed time and current stage. Use a
percentage only if the stage has a measurable denominator; otherwise use a plain
activity indicator, without an invented time remaining.

Copy states that the teacher can leave and return from history. Closing the tab
does not cancel a run. “Hủy xử lý” is an explicit action; explain that saved source
and review data remain according to retention. A cancellation racing with commit
shows the authoritative result rather than claiming cancellation unconditionally.

A failure card identifies what failed, retained progress and a specific action:
replace an invalid file, retry a transient service failure, or return later when
the provider is disabled. Processing history is expandable and avoids raw document
text or stack traces. Completion opens review on an explicit action, preserving
the teacher's current work if processing finished in another tab.

## 6. Screen WU-04 — linked review workspace

### 6.1 Layout

Top: filename/test name, concise “X câu · Y phần”, save state and next-step action.
Below: section/group selector, issue filter and previous/next finding. Main space
has two adjustable panes: source (~42%) and reconstructed editable exam (~58%).
Issue details are inline with the selected field. Do not permanently stack a
global sidebar, outline rail, source pane, editor and findings sidebar together.

At wide desktop widths, an optional outline can open in the shared side-column
mechanism; it is collapsed by default if comparison would become cramped. Use
existing resize controls, visible focus and keyboard resizing. Persist widths
separately from other page panels.

Source pane: exam/key selector, “Nội dung trích xuất” / “Bản xem trang” when an
actual rendition exists, and a labeled original download. Clicking a question or
finding locates and highlights the corresponding source. Clicking a source block
reveals its mapped item(s), metadata use or exclusion. Do not force synchronized
scroll while the teacher is independently reading another passage.

Source highlights cite stable block/run/table-cell references. Show page numbers
only from actual rendering and make conversion/font differences visible. When
exact text-to-page highlighting is unavailable, select the known page/region and
retain precise block evidence; do not fabricate a bounding box.

### 6.2 Question editing

The selected card includes original printed label, normalized position, type,
points, prompt, options/blanks, answer controls and evidence. A small toolbar
supports the approved formatting marks, table actions and asset insertion.
Question type changes warn about affected answer data and preserve source evidence.
No heavy rich-editor instance is mounted for every question in a large exam.

Use visibly different labels for explicit source data, inferred structure, defaults
and teacher edits. Reveal detailed provenance on demand. The teacher can edit a
value directly, but the UI never describes that edit as an extracted source fact.
Choice options keep stable local IDs when reordered, so keys follow the option.

Question/group actions: split, merge, move, exclude with reason, restore exclusion,
and attach/detach shared material with dependency checks. Dragging has explicit
“Chuyển đến nhóm” and move up/down alternatives. Splitting/merging previews the
resulting boundaries, answer associations and points before applying.

### 6.3 Resolve issues rather than dismiss warnings

| Finding | What the teacher sees | Concrete resolution |
| --- | --- | --- |
| Missing answer | “Tài liệu chưa có đáp án cho câu này” and searched evidence | Enter explicit teacher key, attach/map key file or exclude; never “AI trả lời giúp” |
| Conflicting keys | Both values with exam/key source links | Select evidence or enter corrected key; decision records actor and affected revision |
| Ambiguous numbering | Candidate section/question matches | Choose mapping with both contexts visible |
| Default points | Proposed value and affected item count | Set/confirm an explicit score, reviewing total impact |
| Unsupported interaction | Preserved original content and incompatibility | Intentionally transform with preview/confirmation or exclude; type relabel alone is insufficient |
| Lost formatting/object | Original rendition and missing semantic element | Repair supported representation, replace asset or explicitly exclude dependent content |
| Missing audio | Affected group and asset requirement | Upload/select usable audio or intentionally change the assessment with a recorded decision |
| Unassigned content | Exact source block and nearby context | Attach to question/material/instructions, classify metadata or exclude with reason |

Selecting a finding focuses the field and source evidence. Resolving it updates
counts but does not yank the teacher to a different question; “Tiếp theo” advances
when ready. Editing an answer-relevant field invalidates affected confirmations.

Batch confirm exists only for equivalent low-risk findings, e.g. applying the same
points to a visible listed set. The confirmation lists exact items, decision and
total change. Blockers, uncertain answer mappings and unsupported content have no
blanket dismissal. Filtered-out items are never silently included.

### 6.4 Save and conflict behavior

Keep text local while typing, serialize autosaves and show “Đang lưu” only when
a request is in flight. “Đã lưu” means server acknowledgment for that revision.
A save failure keeps the editor and unsent edits, shows retry and disables commit.
Route/question changes await a flush or remain on screen with an actionable error.

A stale revision shows “Bản nhập đã thay đổi ở nơi khác” with compare/recover,
never a blind reload. Saved revisions survive tab closure; unsent-outbox recovery
must explicitly show its local status and ask which revision to apply on conflict.
No correctness claim depends on `beforeunload` completing an async write.

## 7. Screen WU-05 — reprocess comparison

New source/model/profile processing creates a candidate beside the teacher's
existing draft. Default to preserving manual changes. Compare added/removed/moved
questions, changed keys/content/material links and source coverage. Every applied
change is scoped to a current draft revision. New or conflicting evidence can
invalidate an old confirmation but cannot silently replace teacher text.

Offer per-item apply, keep current and inspect source. Equivalent low-risk changes
may be selected as a visible set. Applying reruns relevant validators. A cancelled
or failed reprocess leaves the old draft usable.

## 8. Screen WU-06 — preview, summary and handoff

Preview uses the production learner renderer, with desktop/phone viewing modes,
group materials, accessible tables, cloze links, images and shared audio placement.
Use a teacher-authorized learner-safe projection; do not merely hide admin keys
with CSS. Preview does not consume real student playback allowances.

The summary lists included/excluded questions, sections/groups, points, answer
completeness, assets, defaults accepted and unresolved decisions. Every unresolved
count links back to its filtered review set. Commit is enabled only for the current
saved and validated revision; explanatory text identifies the remaining work.

“Tạo bản nháp đề” creates a draft, not a publication. During commit, show a stable
pending state; disable duplicate clicks but also rely on server idempotency.
On lost response, recover by import identity instead of offering a fresh commit.
Success names the new draft and offers “Mở trình soạn đề” plus history. Do not
unexpectedly navigate away while the teacher is reading the final summary.

## 9. Learner and existing-builder parity

The normal builder gets the same content editor and section → group → question
structure, inline material preview and explicit shared audio policy. Restoring old
versions copies the complete graph; bank insertion cannot orphan passage children.
The builder now offers whole-group bank insertion with a destination selector and
an independent save-to-bank action that retains the current editor. Empty groups
stay selectable. Group navigation collapses to a content selector when the editor's
available width is narrow; material/member content keeps its usable editing width.

During an attempt, the current group's material remains available across member
navigation. Desktop offers a readable material area alongside the question;
phone places it above the question with an explicit expand/collapse control and
remembered state. Do not force repeated reading or destroy an in-progress answer
when changing the material view. Group audio stays scoped to that group, with
counts stable across navigation/reload. Answer controls remain stationary.

Check long passages, wide tables, picture captions, underlined option fragments,
ordered cloze gaps and results with hidden answers. Student presentation must not
expose source filenames/key documents, provenance, teacher notes or answer markings.

Results keep the group's material above its first visible child when filters
change; following a material gap restores all questions and focuses its target.
Teacher paper review keeps context while navigating members. Grading by question
shows that question's complete material without combining different students'
listening counts. Transcript disclosure is explicit and keyboard-operable;
learners receive only released transcripts. Review playback never changes an
attempt's counters.

## 10. Responsive, keyboard, motion and performance rules

- At ≥1280px: two full comparison panes, optional outline; avoid fixed empty space.
- At 768–1279px: collapsed app navigation, switchable source/result views when two
  readable panes cannot fit; preserve active edit, selection and scroll anchors.
- Below 768px: follow the app's admin support policy with accessible status/history
  and clear desktop/tablet guidance; don't promise dense editing on a phone without
  testing it. The interaction mockup still reflows to 320px for conversation use.
- Learner rendering remains fully supported from 320px. Long labels wrap; tables
  scroll within their own region without causing page-wide overflow.
- All actions work by keyboard. Native tab order, visible focus, labeled toolbar,
  movable dialogs, and no shortcut interception inside content editors. Prefer
  visible Previous/Next issue controls; add shortcuts only with discoverable help
  and tests against existing app bindings.
- Announce stage changes/save failures/validation results politely. Do not announce
  every keystroke or move focus after every background validation. Test actual
  screen-reader use and 200% zoom, not only automated accessibility scores.
- Use 150ms ease-out for pane/selection feedback and light shadows for raised
  controls. No page-wide entrance cascade, springing text, pulsing error badges or
  animation that moves an answer control. Honor reduced motion.
- Load source blocks/rendition pages by region and window large result lists;
  active editor stays mounted outside virtualization eviction. Keyboard navigation
  loads the target and restores focus. Mount one rich editor per active context.

## 11. Teacher usability and acceptance scenarios

Use authorized real documents for the pilot, synthetic documents for repeatable
interaction tests. Record task success, active time, mistakes, recovery steps and
teacher feedback. Do not claim a satisfaction score without collecting it.

1. Import a clean free-form exam without reading a template guide; reach learner
   preview and explain the difference between draft and published test.
2. Resolve contradictory same-file/companion answers with restarted numbering;
   point to both pieces of evidence and choose the intended question/key.
3. Fix a pronunciation underline and confirm it survives option editing and the
   phone learner preview without carrying teacher-only answer formatting.
4. Correct a shared reading/cloze group; move a child, undo/repair a broken gap,
   preserve the passage and explain which shuffle settings are compatible.
5. Attach one audio file to a group; see one allowance across its child questions.
6. Lose network during an edit, attempt to leave, retry, reload and recover; repeat
   with a second tab causing a revision conflict.
7. Reprocess a changed source after manual corrections; keep the corrections,
   inspect a conflict and selectively apply new evidence.
8. Exclude unsupported matching content with a reason; verify changed counts and
   totals, then restore it and see the blocker return.
9. Use only keyboard and a screen reader to navigate findings, correct a key and
   create a draft; repeat at 768px/200% zoom with Vietnamese and English labels.
10. Double-click commit or lose its response; recover the same draft without
    duplicates. Return from its builder and retain history/review filter state.

The first prototype demonstrates contradictory keys, score-default confirmation,
source/result switching and the commit gate. Upload/reprocessing/audio/split-merge
and genuine persistence require the later implementation and acceptance tests;
do not infer their completion from the prototype.

The supplied corpus adds four concrete walkthrough cases: missing source
underlines (offer correction without guessing), a 31-label exam with 35 answer
targets (show the five-child group explicitly), a companion key containing
several exams (confirm the matching exam and show unmatched entries), and an
annotated homework containing learner answers plus teacher corrections (classify
the evidence before accepting a key). Treat source defects separately from
recognition defects in both wording and measurements. These are acceptance
requirements, not functionality implemented by the initial source inspector.

## 12. R4: the screens as built on the deck

R4 (T-R4.36 to T-R4.39 and T-R4.57) rebuilt the five import pages in
`TeacherLayout`, on the deck in `docs/design/deck/` and its tokens, with
`data-scale="deck"` on each page root, in light and dark. Every string is a vi
key first, then en. The behaviours below are read from
`web/src/app/router.tsx` and `web/src/features/imports/`; the task notes in
`74-r4.md` have the measurements and the departures (DG-12 to DG-18, DG-21,
DG-73, DG-142, DG-147, DG-148).

### 12.1 Frame

| Route | Content width | Notes |
| --- | --- | --- |
| `/teacher/imports` | 1320 | The history |
| `/teacher/imports/new` | 860 | Upload and paste |
| `/teacher/imports/:id` | 720 | Processing |
| `/teacher/imports/:id/review` | full, sidebar collapsed | Entering the review collapses the sidebar for the visit and leaving restores it (route handle `sidebar: "collapsed"`); it is no longer collapsed permanently |
| `/teacher/imports/:id/confirm` | 1320 | Preview and create |

All five sit under `ImportsGate`, which answers from `GET
/teacher/imports/capabilities` (§2). Another teacher's import answers as a
missing one on every page. The breadcrumb names Tests, then Imports, then the
page (the import's title on the review and the confirm page).

### 12.2 History

- **Header.** "Word and PDF files and pasted text you are turning into tests.
  Each import keeps its original content, its progress and your review, so you can
  come back at any time." Then the outline action "Paste a test"
  (`/teacher/imports/new?source=paste`) and the primary "Import from Word/PDF".
  Both show only while a new import can start, which is when intake and processing
  are both on; with processing off the page keeps its list and a calm note says
  why.
- **Filters.** One segmented control, a group of `aria-pressed` buttons and not a
  tablist (DG-17), in the order All, Processing, Ready for review, Couldn't
  process, Draft created, Cancelled, each with its count. The counts are the
  history's six facets, which follow the search and ignore the selected status.
  Processing selects `awaiting_sources`, `queued`, `processing` and `committing`.
  The search ("Search file or test name", 200px at least, 280px at most) matches
  the title and the current filenames, accents ignored; a pasted import is found by
  its title only. Status, search, page and size are in the URL, and a status change
  resets the page.
- **Pager.** The sizes are 20, 30 and 50, and real pagination shows whenever the
  list has a total.
- **Grid and cards.** The list is not a `DataTable`. A container query on the list
  itself (not the viewport) switches it: from 960px of its own width a six-column
  grid with table roles, `minmax(200px,2.4fr) minmax(160px,1fr) minmax(150px,1.4fr)
  minmax(110px,1fr) minmax(150px,1fr) 32px`, the deck's tracks with the fixed ones
  taken as minimums so longer Vietnamese labels fit (DG-21); below it each import is
  a stacked card with the tile, name, wrapping meta and the menu, then the status
  pill and the review chips, then the test link and the primary action. Nothing
  scrolls sideways at any width, and a row's controls exist once in the
  accessibility tree. At 1280px with the sidebar expanded the list is 976px wide,
  so a vertical scrollbar of more than 16px turns it into cards (DG-18).
- **Cells.** File: the tile, the name and "{creator} · {when} · with {key}". Status:
  the eight import statuses on the deck's five pills. To review: "{n} need action"
  and "{n} to confirm" from the review counts (an import whose draft was never
  assessed shows no counts). The test link. The primary action follows the status:
  continue the review, view progress, view the cause, open the test. The menu has
  Download original only.
- **A pasted import** shows the clipboard tile, the localized name "Pasted text" in
  place of the stored filename, and "{creator} · {when} · {n} characters".
- **States.** The skeleton, a filtered empty state with "Clear filters", the
  no-imports state with a hint that names both ways in, and a failed load with a
  retry that keeps the last list on screen. Active imports are polled only while
  the tab is visible and the user is active.

### 12.3 Upload and paste

A "Source" control under the header, "Upload a file | Paste text", chooses the mode
(`?source=paste`; anything else is the file mode). The options size to their
content rather than the deck's fixed 168px (DG-21). One intake stays mounted across
the switch, so chosen files, the pasted text and a typed title survive it.

- **File mode.** The Files card with Exam (required) and Answer key (optional), the
  deck's preflight messages and the server's limits, Test title prefilled from the
  exam file's name. "How to read the content" offers automatic recognition only.
  The privacy callout, "Where your content goes.", names files and pasted text and
  says only what the capabilities report.
- **Paste mode.** The page reads "Paste a test" and the "Test content" card takes
  the Files card's place, with no answer-key slot. The box is a plain monospace
  textarea at least 340px high. The counter reads "{n} of {max} characters", n
  counting code points of the NFC-normalised text (the text sent), max
  `ImportLimits.pasteMaxCharacters`; over the limit the counter and the border turn
  danger. "Paste from clipboard" replaces the text and moves focus to the box, and an
  empty or refused clipboard shows the deck's two toasts; "Clear" shows only with text;
  "Copy example" ships. The kind pill has one state, "Plain text · mark answers with *
  or "Answer:"", and nothing reads the clipboard's HTML (DG-12).
- **Found so far.** While the box has text: Sections, Questions, With an answer and
  Need an answer, then the notes (over the limit, no numbered question, which
  questions have no answer, a number seen twice in one part, images and audio are
  not pasted). The count is `paste.ts`, the server's grammar for text sources, run on
  a deferred value, never the deck's script (DG-13); `api/testdata/pasted-text-counts.json`
  holds it and the server to the same numbers.
- **"How to lay out pasted text"** is open while the box is empty: the deck's four tips
  beside the example (a container query on the card, 594px wide, DG-142), stacked
  below it otherwise. The example is a case of the shared fixture with nothing
  missing (DG-15).
- **Title.** The first non-blank line when it reads as one (at most 120 characters and
  not a part heading, a question or an option); a typed title wins; a blank title
  starts as "Pasted test · {date}" in the user's zone.
- **Footer.** Cancel and Start processing, which is disabled for an empty box, a text
  over the limit and a text with no numbered question, each with the deck's hint. When
  ready it reads "{n} questions found." and then the file flow's sentence, "Processing
  takes about a minute. You can leave this page.", until T-R4.51 measures the cold
  worker at 10 s or less for the deck's sample (DG-16). Ctrl or Cmd+Enter in the box
  starts, never during an IME composition.
- **Start** runs the file flow's steps with the text in place of the exam file: the
  capability re-check, `createWordImport`, `pasteImportSource`, `processWordImport`. A
  retry reuses the create and upload identities; an edited text takes a new
  `uploadId`; a 409 re-reads the import; a 429 and an intake refusal show the
  server's message.
- **Leaving.** Pasted text that has not started asks first (a router blocker and
  `beforeunload`); switching the Source does not ask, nor does the navigation after a
  start. The file mode has no guard: leaving mid-upload stops the intake quietly.

### 12.4 Processing

A 720px frame with the file tile (the clipboard tile for a paste), the name,
"Started {when}" and "with {key}". The running card (`role="status"`) draws the deck's
five rows from the run's stage: Check the files, Read the content, Recognise the
structure and Match the answers, Check the test. Elapsed time, "attempt n of m" and
waiting-to-retry stay, with the callout "You can leave this page." Ready reads "We
found {q} questions with {a} answers in {s} sections" with pills for "{n} need action",
"{n} to confirm" and "{n} notes", each shown above zero, "Start review →" and Later.
Failed names the stage, the error code in mono and the action: replace the exam file
when the import accepts a new source, otherwise start a new import. Cancel asks first
and states the configured retention; Cancelled is final; committing and committed are
kept. A pasted import says "Check the text" and "{n} characters checked", then "Plain
text · no formatting"; its failed state offers "Start a new import", which opens the
paste mode; the cancel copy says "the pasted text". Its Ready card's first pill is
"Every question has an answer" when nothing blocks, and the deck's "need an answer" pill
is not built (DG-14). The processing history is absent until run events are exposed.
The spinner pauses on hover and focus and is static under reduced motion.

### 12.5 Review

One card the height of the viewport less 128px, at least 560px. The top bar has the
title (an input, because it is part of the saved draft, DG-147), "{q} questions · {a}
answers · {s} sections", the save state ("Saved", "Saving…", or "Couldn't save your
last change" with Retry, `role="status"`), the severity pills and "Preview & finish →".
The toolbar has the Section select, All / Needs action / To confirm / Info with counts,
a Source | Test switch below 980px of content width, and the navigator ("{i} of {n}
open", Previous, Next). From 980px the source pane and the test pane share a `SplitPane`
(30 to 65 percent, 42 by default, arrow keys step 2, remembered under
`quizzivy.importReview.split`). The source pane has Exam | Answer key, Extracted | Page
view (Page view is off until a page renderer exists), Download original and the block
tags. A pasted import has one option, "Pasted text", and one view, "As pasted". The test
pane has the whole-test card, findings with their resolutions as radio cards, the Exclude
dialog (a reason of up to 500 characters is required) and the Points dialog. The open
question's prompt is the content editor of T-R4.63 in the import profile: "Insert gap"
for a fill-in-the-blank, no Rich text | Markdown switch (import content is never
Markdown), no Image or Audio. One card is open at a time, so one editor is mounted.
A phone (below 768) is read-only with the deck's dismissible note. The stale reload
and "Use the new result" notices, the leave guard and the lost-response commit copy stay.

### 12.6 Preview and create

`/teacher/imports/:id/confirm`: "Preview and create a draft" and "← Back to review". Two
columns, `minmax(0,1fr) 360px`, from 1000px of the page's own width. The preview is the
student's paper through `StudentPreview` with Computer | Phone and the answers hidden, and
reads "Showing {k} of {n} questions…" when a type the paper cannot draw is left out
(DG-148). The summary lists the answers included, sections and groups, answers set (with
Fix), points (with Confirm), audio and images ("Images and audio are not imported. Add
them in the test builder.") and what is still to resolve (with Review); only a row that
blocks the draft is in danger ink. "Create draft test" is 44px and `aria-disabled` with
its reason while off; a lost response replays the same request. The created draft's card
offers "Open in test builder →" and "Back to imports".

### 12.7 What DG-67 kept and what it gated

The deck did not draw several behaviours the product has, and drew several controls the
product has no backend for (DG-67). R4 keeps the first in the deck's styling and does not
build the second. Their backend and UI are built in R6 beside T-R6.13.

| Kept, in the deck's styling | Gated, absent until built in R6 |
| --- | --- |
| Elapsed time, attempt n of m and waiting-to-retry on the processing page | The audio slot |
| The stale reload notice and "Use the new result" | Saved recognition profiles |
| The committed card and banner, and the lost-response commit copy | Source conventions |
| The leave guard on the review (and, for pasted text, on the paste box) | The template download |
| The no-imports empty state, with a hint that names both ways in | Move, Split and Merge in the review (Exclude and Restore stay) |
| Cancel as a final state; Download original in the row menu | The processing history; "Page view" in the source pane (shown, off) |
| Phones read-only, with the deck's dismissible note | "Create another test from this file", "Delete import" and "Process again" |

"Paste a test" is not one of the gated controls: R4 builds it (§12.3) with its departures
in DG-12 to DG-16. Its "How to read the content" card is the file flow's, so the same
controls stay gated there.
