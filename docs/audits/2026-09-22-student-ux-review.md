# Independent student UX review

Date: 2026-09-22. Reviewed revision: `85d4e5c` on `work/issue-remediation`.
Status: the user approved the proposals after this review. The implementation
is tracked in `../plan/92-student-ux.md`; validation is in progress. PR #98 remains
a draft, with no merge or deployment. The observations below describe the
reviewed revision before those changes.

This review evaluates whether the interface supports the student's task. Matching
the mockup and passing the existing tests are insufficient acceptance criteria.
The findings below supplement, rather than invalidate, the earlier functional
regression results in `2026-09-22-remediation.md`.

## Method and limits

Inspected the student route tree, layouts, controls, question renderers, public
entry flow, result policies, and error states. Exercised the local production
preview against the local API with the existing development student account.
Viewport checks used 320, 360, 1024, the normal 1375, 1440, and 1920 CSS pixels;
these were targeted checks, not every page at every width. No physical mobile
device, mobile keyboard, or screen reader was used in this pass.

Only the existing local listening attempt was resumed. Its answers were not
changed, audio was not played, and the submission confirmation was cancelled.
Flagging and panel resizing were restored after keyboard checks. A temporary
unsaved profile-name edit was used to demonstrate the responsive state-loss bug;
the profile was not submitted. No passwords, Google links, or enrolments changed.

No build, test worker, or agent was started for this review. Available RAM was
checked repeatedly (approximately 2.5–3.0 GiB). The existing local API and preview
processes exited with code 143 during review; the API logged a graceful shutdown.
They were restarted from the existing build. A failed navigation to login during
that outage is not counted as a product defect: login rendered after recovery.
The cause of the process termination was not established.

## Why the right side looks empty

The shell fills the viewport, but its content does not:

1. `StudentLayout.tsx:60` caps the main wrapper at `max-w-4xl` (896px), without
   centring it in the available main column.
2. A `PageAside` consumes a separate full-height column. The default is 320px;
   the current browser's remembered width is 366px. Home creates this column even
   when it contains only one class and a join link.
3. The width preference is shared with every `panel` in the application through
   `quizzivy.column.panel`, including teacher screens and the test navigator.
4. Result pages add a second 720px cap inside the already capped wrapper. This
   centres the paper within 896px, not within the actual remaining main area.

Measured bounds on Home, with the current 366px panel:

| Viewport | Main column | Content bounds      | Panel starts | Content-to-panel gap |
| -------- | ----------- | ------------------- | ------------ | -------------------- |
| 1375px   | 1009px      | x=32–928, width=896 | x=1009       | 81px                 |
| 1920px   | 1554px      | x=32–928, width=896 | x=1554       | 626px                |

The gap includes the intended gutter, but grows far beyond it as the viewport
widens. The largely empty full-height panel adds to the impression. This is a
layout problem, not missing data and not solely a screenshot/emulation artefact.

The test engine's centred 720px question column is a different case: that is a
useful reading width. Stretching a question or password form edge to edge would
not resolve the underlying information hierarchy.

## Findings to address before release

Priority means repair order for this review, not a claim of production severity.
P1 blocks a reliable student workflow; P2 affects usability or a narrower case.

### UX-01 — P1: Home hides additional active attempts

Source-confirmed: `StudentHomePage.tsx:73–74,157` takes one active assignment with
`find`, then excludes **every** active assignment from the remaining list. With
two or more active assignments, only the first receives a resume control. The
other assignments disappear from Home. The summary count also excludes active
assignments, so its wording does not describe all open work.

Proposal: render every active attempt in a clearly labelled group, sorted by
deadline; then render work not yet started. Name counts consistently. Do not
depend on an unstated rule that students can have only one active assignment.
Acceptance: two active attempts plus one unopened assignment expose all three
actions, with accurate counts and a deterministic order.

### UX-02 — P2: Fixed main cap and sparse side panels waste width

Browser-confirmed by the bounds above. Sources: `StudentLayout.tsx:59–67`,
`StudentHomePage.tsx:104–110`, `StudentClassesPage.tsx:108–127`,
`StudentSettingsPage.tsx:62–89`, and `useColumnWidth.ts`.

Proposal: make the everyday student shell fluid. Put classes/join in the Classes
page, account metadata in Settings, and upcoming work beside or within the Home
list according to available space. Use a side panel only when it helps the task.
Keep readable widths at the paragraph/form level. Give the exam navigator a
separate preference from teacher panels if it remains resizable.
Acceptance: no unexplained strip between content and a panel at 1440/1920px;
one class does not require a full-height side column on Home or Settings.

### UX-03 — P2: The exam footer clips its primary action at 320px

Browser-confirmed on the last question. Viewport width was 320px, document width
333px. The final button extended from x=219.43 to x=332.97. The screenshot showed
the right edge of “Xem lại & nộp” clipped and a horizontal scrollbar.
Source: `TakeTestPage.tsx:395–405,496–510` and the shared button's non-wrapping,
non-shrinking defaults.

Proposal: use compact labelled navigation that fits the content width: a 44px
previous button, a 44px question-list icon button with an accessible label, and a
flexible primary next/review button. Alternatively put the list control on its
own row. Do not shrink the touch targets or text to make three long labels fit.
Acceptance: all actions stay visible at 320/360px in Vietnamese and English,
including the last question, with no document-level horizontal scrolling.

### UX-04 — P2: Phone touch controls are inconsistent

Browser measurements at 360px:

| Control                                      | Current height |
| -------------------------------------------- | -------------- |
| Detail back button                           | 32px           |
| Save profile / change password / link Google | 32px           |
| Language options                             | 28px           |
| Result filter chips                          | 28px           |
| Sign out in Settings                         | 36px           |
| Main exam navigation / flag / answer input   | 44px or larger |

Source inspection also finds 36px confirmation buttons in `ReviewScreen`, a
32px close button and 36px review button in `NavigatorSheet`, and a 36px integrity
acknowledgement. These are not covered by simply making the normal next button
44px. Sources include `StudentLayout.tsx:115,130`,
`SettingsSections.tsx:134,223,293,352`, `ResultPage.tsx:183`, and the navigator,
review, and integrity components.

Proposal: apply a student touch-control convention consistently, with at least
44px targets below 1024px; keep the desktop density separate. Increase the hit
area without inflating every icon. Check width as well as height: the question
grid uses a 36px minimum track even though its phone buttons are 44px high.

### UX-05 — P2: Crossing 1024px discards unsaved form input

Browser-confirmed: type a temporary name in Settings at 360px, switch to 1024px,
and the field returns to the saved name; Save becomes disabled. Changing widths
also reset the selected result filter during this review.

Both `StudentLayout.tsx:36` and `StudentSettingsPage.tsx:35` switch between
different trees, remounting the page/forms. Proposal: keep form ownership and
the route outlet stable across layout changes, or explicitly preserve the form
state outside the responsive branches. Keep the required code-level breakpoint
without making it discard the task in progress.
Acceptance: edited name and selected result filter survive 1023↔1024px without
saving, losing focus unnecessarily, or making a network mutation.

### UX-06 — P2: Classes is a dead end for finding work

Browser-confirmed: the class card shows “8 bài đang mở” but neither the name nor
the count leads to those assignments. Phone cards omit these counts entirely.
Source: `StudentClassesPage.tsx:83–103,134–170`.

Proposal: add a clear “Xem bài của lớp” action that opens Home with that class
filter applied. Put “Tham gia lớp” in the page header and open the existing join
flow. Use an adaptive card grid rather than forcing two columns as soon as the
viewport reaches 1024px. Empty states should offer one clear join action.

### UX-07 — P2: Multiple-choice questions lack a visible multiple-selection cue

Source-confirmed: `QuestionBody.tsx:92–159` changes the hidden native input from
radio to checkbox but renders the same letter tiles, selected treatment, and
generic answer-options label. There is no automatic visible “Chọn nhiều đáp án”
instruction. A teacher might put this in the prompt, but the UI cannot rely on
every author doing so.

Proposal: show a neutral question-type instruction and visibly distinguish
single selection from multiple selection. Do not disclose the number of correct
answers. Keep native keyboard semantics and the existing large clickable rows.

### UX-08 — P2: Result filters have no empty state and can imply hidden grades

Browser-confirmed: choosing “Sai 0” removes all question cards and leaves only
the unrelated explanation-policy message. Source: `ResultPage.tsx:173–209`.
Source-confirmed conditional case: filters are always rendered, while `verdict`
returns unknown when scores are withheld, yielding a misleading “Sai 0”.

Proposal: add contextual empty text plus “Xem tất cả”; hide filters whose data
the review policy does not reveal. Put the result/score summary near the title,
then the filters and answers. Keep the explicit provisional-score explanation.
Acceptance: zero matches differs visibly from loading, failure, and withheld
grading. No new answer-key or score disclosure.

### UX-09 — P2: Entry and completion actions do not describe the next step well

Home's “Bắt đầu làm bài” only opens the introduction
(`StudentHomePage.tsx:294–296`); the same label on the introduction starts the
clock. Use “Xem bài”/“Xem chi tiết” on Home and reserve the start label for the
action that creates the attempt. Keep “Tiếp tục làm bài” for active attempts.

After submission, `SubmittedScreen.tsx` only offers Home. A student must then
find the completed assignment to inspect their paper. Proposal: offer “Xem bài
đã nộp” as the main action, respecting the existing result policy, with Home as
a secondary link. This completion-state finding is source-reviewed in this pass;
no additional attempt was submitted to reproduce it.

### UX-10 — P2: Signed-in join flow loses account context and a route home

Browser-confirmed: following “Tham gia lớp” while signed in shows “Đã có tài
khoản? Đăng nhập”, with no in-app back/home control. `JoinPage.tsx:69–74` renders
that footer regardless of authentication. The centred join form itself is clear.

Proposal: show the current account and a route back to Classes for signed-in
students; keep the login link for signed-out visitors. Do not alter the approved
authentication or direct-link enrolment policy as part of this layout change.

### UX-11 — P2: Secondary states can mislead a student during an attempt

Browser-confirmed: a resumed paper already containing an answer says “Chưa có gì
để lưu”. The audio control says “Còn 0 lượt nghe” but still offers Play.
Sources: `SaveState.tsx:29–31`, `QuestionAudio.tsx:27–45`.

The latter follows the existing policy that records over-limit playback; this is
not a proposal to block audio or change integrity enforcement. Clarify the copy:
distinguish recovered server answers, pending local changes, and acknowledged
saves; state what additional playback means rather than implying a hard limit.
Preserve synchronous play initiation and the existing recovery behaviour.

### UX-12 — P2: Ordinary deadlines receive warning treatment

Browser-confirmed: assignments due in 15 days have the same amber clock badge
as closer deadlines. `StudentHomePage.tsx:259` always chooses `warning`.
Every open card also carries the same strong primary start button.

Proposal: reserve warning styling for genuinely near deadlines, use a neutral
due-date treatment otherwise, and visually prioritise ongoing work. Changing
colour alone is insufficient; name and order the groups and actions clearly.

## Screen-by-screen direction

| Screen / state                         | Keep                                                                                      | Change                                                                                                                                              |
| -------------------------------------- | ----------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| Login                                  | Centred phone form, explicit labels, 44px main actions                                    | Add a direct join entry and consider a password visibility control; do not stretch the form across desktop width                                    |
| Forced password change                 | Focused form and clear required action                                                    | Consider password visibility; source-reviewed, credentials were not changed                                                                         |
| Join code / confirmation               | Single card, class and teacher identity, one main confirmation                            | Signed-in account context and route back; confirmation success/error variants were source-reviewed                                                  |
| Home                                   | Ongoing-work priority, deadlines, completed history                                       | Show all active attempts, add class/status filtering, remove sparse permanent side panel, use available width for an adaptive list/card arrangement |
| Classes                                | Teacher and class identity                                                                | Make the next action explicit; join action in header, adaptive cards, route to class assignments                                                    |
| Assignment introduction                | Rules before the clock starts, separate before/after-submit policy                        | Keep title, facts, rules, and start action in one coherent reading sequence; the start action should not feel detached in a far-right column        |
| Test engine                            | 720px reading measure, stable clock/save feedback, large answer rows, dedicated navigator | Fix 320px footer; explain question type and audio/save states; isolate navigator width from admin panels                                            |
| Question list                          | Grouping by section, current/answered/flagged state, accessible names                     | Ensure both dimensions of phone targets are 44px and make close/review controls consistent                                                          |
| Pre-submit review / confirmation       | Unanswered and flagged links, explicit final confirmation                                 | Consistent phone targets; keep final action reachable on long papers without hiding the unanswered summary                                          |
| Submitted                              | Clear final state and submission metadata                                                 | Direct link to the submitted paper, Home secondary                                                                                                  |
| Results                                | Policy-aware answers, provisional score distinction, readable question column             | Summary near title, usable filters and empty states, full title available on phone, no misleading zero for withheld grades                          |
| Settings                               | Separate profile/password/provider/language responsibilities                              | Remove sparse account rail, use a comfortable form column or adaptive grid, preserve edits across breakpoints, consistent phone actions             |
| Integrity / session takeover / offline | Calm language, visible clock, explicit save/lock feedback                                 | Improve touch targets and state copy; failure variants were source-reviewed, not all re-triggered in this pass                                      |
| 403 / 404 / unexpected error           | Clear retry/home actions; normal phone actions are already 44px                           | Keep recovery contextual; do not misclassify local preview outages as application failures                                                          |

## Proposed layout rules

- Use a fluid student shell for Home and Classes. Apply comfortable reading
  widths to text within cards, and adapt the number of cards to available space.
  Do not leave an uncentred 896px wrapper inside a much wider column.
- Use a centred, readable detail flow for the introduction, results, and settings.
  Keep the summary and action connected to the content. Whitespace around a
  centred form is intentional; a large strip on only one side is not.
- Keep the exam as a focused workspace. A question navigator justifies an aside;
  a single class name or account summary usually does not. Retain the 720px paper.
- Keep actions semantically stable: view details → start clock → resume work →
  review → confirm submit → view submitted paper. Labels should name the actual
  next step, and the main action should not move arbitrarily between columns.
- Apply one phone interaction standard, including sheets and error states. Check
  real content width and long Vietnamese labels, not merely the viewport class.

These are proposals. Adopting them requires updating the student shell guidance
in the spec/plan and the relevant mockup-derived expectations. The neutral
palette, semantic colours, accessibility, privacy, integrity, and auth contracts
remain applicable.

## Validation for the next implementation pass

Run small, targeted checks with one worker. Add regression coverage for multiple
active attempts, 320px final-question navigation, state preservation at the
breakpoint, policy-aware result filters, and class-to-assignment navigation.
Test the five question types, long titles, several classes, long question lists,
empty/error/loading states, and Vietnamese/English labels. Review screenshots
and actual focus/navigation behaviour in addition to assertions. Physical
mobile keyboard and screen-reader checks remain separate validation work.

During this pass, F toggled a flag, the final right arrow opened review, and arrow
keys on the panel separator resized the panel without navigating questions.
The resize was reversed and the flag cleared. Earlier automated shortcut and
autosave results remain useful but do not cover all the UX findings above.
