# Word import — teacher workflow and interaction specification

Status: direction accepted for implementation, 2026-09-22. Companion to
[the delivery plan](17-word-import.md); teacher pilot validation remains pending.
The interaction mockup uses synthetic content and demonstrates review decisions;
it is not a working importer or evidence of recognition quality.

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
| `/admin/imports` | Searchable import history | Secondary entry from tests; “Nhập đề từ Word” |
| `/admin/imports/new` | Upload, optional key/audio, recognition setup | Tests list split action beside “Tạo đề”; “Bắt đầu xử lý” |
| `/admin/imports/:id` | Current processing state or resumable import | “Tiếp tục rà soát” when ready; committed state links to resulting test |
| `/admin/imports/:id/review` | Source-linked correction workspace | “Xem trước & hoàn tất” |
| `/admin/imports/:id/compare/:runId` | New recognition versus manual draft | “Áp dụng các thay đổi đã chọn” |
| `/admin/imports/:id/confirm` | Learner preview and validated summary | “Tạo bản nháp đề” |

Do not add many permanent global sidebar items. The tests area exposes imports
and history clearly; the dedicated review route can collapse global navigation
while preserving a labeled way back. A direct URL must resolve to a helpful state
for processing, failed, cancelled, stale or committed imports.

These entries follow `GET /admin/imports/capabilities`:

- **Intake off.** The tests list shows neither entry, and every `/admin/imports`
  route explains that import is not enabled.
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
scans and limits distinctly. Replacing a rejected file preserves the other valid
inputs. A partially uploaded companion key never appears as fully received.

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
