# Design gaps — requests to the design team

The deck in `docs/design/deck/` is the single source of truth for the UI. This register lists
what the deck does not draw, what it draws in two incompatible ways, and where a decision taken
with Thuong on 2026-09-26 overrides it. Each entry says what we build until the design team
answers ("Meanwhile") and which release needs the answer (`docs/plan/70-redesign-overview.md`).

When a new export answers an entry, mark it **Answered** with the export date, and delete the
"Meanwhile" rule once the product matches the drawing.

Status: **Open** — needs the design team. **Decided** — Thuong decided; the deck should be
redrawn to match. **Answered** — resolved by a later export.

## Decisions that override the current drawings

| ID | Where | What changes | Status | Needed by |
|---|---|---|---|---|
| DG-01 | Teacher › Class detail, join code card | Codes are stored encrypted and **always shown in full** to their teacher and to admins. Redraw the card without "The full code is shown only once" and the "Copy it now… only the last 4" dialog: full code, copy code, copy link, QR and download always available. Pick one link format (`/join/K7QM-2PXA` keeps the dash on cards but not in the new-code dialog). | Decided | R4 |
| DG-02 | Sign in › Join, Student › Join dialog, Landing | The public preview **never shows a student count** ("{n} students" goes); schedule and room may stay. Sample codes must use the §6 alphabet (`ABCDEFGHJKLMNPQRSTUVWXYZ23456789`, which has no 0, O, 1 or I): `ABCD-1234` becomes e.g. `K7QM-2PXA`. Draw the invalid-format state (the Landing input silently does nothing today). | Decided | R1, R8 |
| DG-03 | Admin › Roles & permissions | Add the row **"Create student accounts"** (Admin and Teacher on). The Teacher Students page has "Add student", but "Add and disable users" is Admin-only. | Decided | R2 |
| DG-04 | Admin › Users, Roles | **Assistants are attached per class** (class staff). Draw where a teacher or admin adds an assistant to a class, and what an assistant's sidebar shows. Until then Assistant is hidden from role pickers. | Decided | R5 |
| DG-05 | Admin › Users import, password resets | Emails carry a **one-time set-password link**, never a temporary password. Draw the set-password page (signed out, from the link) and reword "Email sign-in details". | Decided | R5 |
| DG-06 | Landing › Consultation | Add an explicit **consent checkbox** and a linked **privacy notice page**; leads are kept 12 months. | Decided | R11 |
| DG-07 | Admin › System settings › Data & privacy | Retention choices drive a nightly sweep. "Delete disabled accounts after" **anonymises** the account (results stay, identity is removed) rather than deleting rows; please reword the hint. | Decided | R5 |
| DG-08 | Teacher › Messages | Direct messages are readable by their participants only; admins never open them. Nothing to redraw unless an admin surface is added. | Decided | R7 |
| DG-09 | Teacher › Media | A published test version never changes. "Replace file" reaches drafts and bank questions only, and a file used by a published version cannot be deleted. The Replace and Delete copy says otherwise; please redraw both, including the "used in published versions" case. | Decided | R4 |
| DG-10 | Admin › API reference | The API has no `/v1` prefix. The base URL reads `https://api.quizzivy.com`. The "Try it runs as you and is recorded" note is false: the docs session holds no bearer token. | Decided | R5 |
| DG-11 | Admin › Audit log | The diff shows join codes by their last four characters only, never in full. | Decided | R5 |

## Open — needed before or during the release named

### Language and copy

| ID | Gap | Meanwhile | Needed by |
|---|---|---|---|
| DG-20 | Only Landing and Splash carry Vietnamese; Admin, Teacher, Student, Sign in and System pages are English only, and Settings › Language defaults to English. | We write vi first from the existing glossary, en from the deck, and Vietnamese is the default. Please review the glossary: workspace, admin console, roles & permissions, audit log, shared with me, can use / can edit, gradebook, attendance, flashcards. | R1 |
| DG-21 | Fixed-width labels clip longer Vietnamese strings: navigator text (min-width 92px), change-kind chips (70px), a 170px status column, 110/100px access and date columns. | Columns size to content with a minimum. | R3, R4 |
| DG-22 | Avatar initials differ: lists, sharing and Landing use given + family name (Hoàng Thương → TH), the sidebar avatars use HT / TQ. | Given + family (TH) everywhere. | R1 |

### Foundations and accessibility

| ID | Gap | Meanwhile | Needed by |
|---|---|---|---|
| DG-30 | No focus states: inputs only change border colour and buttons have none. The light `--ring` (#9aa3a5) is about 2.6:1 on white, under the 3:1 a focus indicator needs. | A 2px `:focus-visible` ring from a darker ring token. | R1 |
| DG-31 | `--border` #e4e7e8 on white is about 1.25:1, so input outlines fall below 3:1 non-text contrast. White text on `--danger` is about 4.47:1. | Inputs use a darker border token; danger buttons use a darker fill. The token test measures both. | R1 |
| DG-32 | Toasts always show a green success check, even for "Pick a score first". | Error, warning and info tones with their own icons. | R1 |
| DG-33 | Continuous motion (infinite marquee titles, the pulsing live dot) conflicts with WCAG 2.2.2. | Marquee only on overflow, paused on hover and focus; both static under reduced motion. | R1 |
| DG-34 | Hard-coded colours: switch knob #fff, danger button text, badge text #1b2123, QR tile, the white "paper" of the import page view, theme preview swatches, the serif used for quotes (Georgia/Times). The "Match device" swatch is a gradient. | Tokens for each, including a `--font-serif`; the swatch is split, not a gradient. | R1 |
| DG-35 | Dark mode is not drawn for content surfaces: transparent question images, the import page view, rich-text tables, the Landing product mocks. | Content keeps a light "paper" surface in dark mode. | R1 |
| DG-36 | Keyboard: the share dialog has no Esc or focus trap, the palette has no arrow keys or no-results state, menus lack roving focus, the toast has no live region. | Built to the WAI-ARIA patterns. | R4 |
| DG-37 | "Compact tables" has no drawn effect. | Rows 32px instead of 40px, cell padding halved. | R4 |

### Front door and system

| ID | Gap | Meanwhile | Needed by |
|---|---|---|---|
| DG-40 | Auth states not drawn: Google callback and its errors, 429 rate-limit copy, disabled account, a signed-in user opening `/join/:code`, enrolment failing after sign-in. | Deck primitives on the auth layout, listed for sign-off. | R1 |
| DG-41 | Forgot password shows the front-desk phone and hours, but Admin › Organization has no field for them (nor contact email and address the Landing shows). | Fields added to Organization settings. | R1, R5 |
| DG-42 | Maintenance: no Admin control to schedule a window, no "all systems normal" status page, no rule for how long tests closing during maintenance are extended. | Windows are scheduled from the command line; in-progress attempts and closing tests extend by the window length. | R1 |
| DG-43 | Home targets on system pages all go to the Student app; teachers and admins need their own, and signed-out visitors go to sign in. | Home = the caller's console. | R1 |

### Roles, users and the Admin console

| ID | Gap | Meanwhile | Needed by |
|---|---|---|---|
| DG-50 | The matrix has no rows for imports, courses, vocabulary, messages, schedule, gradebook, reports, data export, API reference or leads. | Each maps to an existing row or to "any teacher permission" (listed in `70-redesign-overview.md`). | R2 |
| DG-51 | Admin's "Take tests and view own results" cell is editable while every other Admin cell is locked. | Kept: admins may take tests if an admin turns it on. The built-in Student role cannot lose it. | R2 |
| DG-52 | Custom roles have no delete action; what happens to their members? | No delete in v1. | R5 |
| DG-53 | A plain teacher (no Admin console) and custom roles: which nav items and workspace switcher entries show. | Nav items and the switcher follow permissions. | R4 |
| DG-54 | Users: status precedence (disabled / must change password / never signed in / active), what the sidebar badge "4" counts, and what "active" means for Last active and the daily chart. | Disabled > must change > never signed in > active; the badge counts must-change; active = an authenticated request that day. | R5 |
| DG-55 | Plan & usage shows a plan, seats (400) and a teacher limit (15). Billing is a non-goal. | Values are configuration, shown but not enforced. "Attempts kept 18,420 · 46%" needs a denominator. | R5 |
| DG-56 | No ownership transfer when a teacher leaves, and no view of a disabled teacher's content. | An Admin "Transfer content" action in the user sheet. | R5 |
| DG-57 | No leads inbox, no field for who is notified of new leads, and no maintenance or term management in the Admin console. | Terms in Admin settings (T-R8.13); the leads list and its recipients field under Admin, built from deck parts (T-R11.9); maintenance windows from the command line (DG-42). | R8, R11 |
| DG-58 | "Signed-in devices" shows "MacBook Air" and a city. A user agent does not give the device model. | "Mac · Chrome · Ho Chi Minh City". | R4 |
| DG-59 | The Admin console has no language control, so a user whose only workspace is the console cannot change language or profile. | An interim "Language" item in the Admin account menu writing the user's locale preference. | R5 |

### Teaching

| ID | Gap | Meanwhile | Needed by |
|---|---|---|---|
| DG-60 | Question types differ by screen: the builder has Matching, the editor has True/False and Short answer, the bank lists Open answer and "Audio · labelling", grading has Essay with a 0–9 band, import has True/False/Not given and Cloze group, the student engine has "Choose two" and word-limited answers. | One canonical set: single, multiple (optional exact count), true/false/not given, fill in the blank, short answer (optional word limit), essay (band), matching, cloze group. Please draw the authoring side of each. | R6 |
| DG-61 | Band scores and rubric tiles (Task achievement, Coherence, Vocabulary, Grammar) have no model. | Rubric criteria defined per question; the band is their mean, rounded to 0.5. | R6 |
| DG-62 | Levels and skills: forms offer A1–C1, fixtures use Pre-A1 and ranges, filters group A1–A2. | Pre-A1…C2 per question; skills: grammar, vocabulary, reading, listening, writing, speaking. | R4 |
| DG-63 | Option limits: the builder allows 6 options (A–F), the editor 8 (A–H). Media limits: upload 50 MB for every type, Replace limits images to 10 MB. | 8 options; audio 50 MB, images 10 MB. | R4 |
| DG-64 | Grading: does "Save & next" follow the grouping or global order? Does grading the last item release scores, or is there a release action? | Follows the grouping; scores release when the last answer is graded, as today. | R4 |
| DG-65 | Assignment settings: which groups stay editable while live; results visibility disagrees (wizard: after submitting, live Results group: after the window closes); what "Save draft" persists. | Only Test & timing locks; release follows the stored rule; drafts persist every wizard field. | R4 |
| DG-66 | Test detail draws a per-version diff and change notes with no model. | Diff = questions added, removed and changed between versions; the note is free text at publish. | R4 |
| DG-67 | Imports: current behaviours not drawn (elapsed time, retries, stale reload, "Use the new result", the committed banner, the leave guard, the no-imports empty state); the audio slot, saved profiles, source conventions, template download, Move / Split / Merge, the processing history, "Create another test from this file", "Delete import" and "Process again" have no backend. | R4 keeps current behaviour with deck styling and gates the controls without a backend; their backend and UI are built in R6 beside T-R6.13. Entering review no longer collapses the sidebar permanently. | R4, R6 |
| DG-68 | The existing question-group (passage/cloze) bank screens and the full attempt review page are not in the deck. | Restyled with deck primitives. | R4 |
| DG-69 | Media card "Play limit" against the per-question "Plays allowed": which wins? | The question's setting wins; the card's value is the default for new questions. | R4 |
| DG-70 | Dashboard and Reports range selectors are cosmetic. | They drive the data: 7 days, 30 days, this term. | R4, R9 |
| DG-71 | The Student deck's Test intro draws a note from the teacher ("Note from Ms Thương"); the Teacher deck has no field to write it. | `assignments.student_note` (≤500 characters) with a plain "Note to students (optional)" field in the wizard's Rules step and the Settings tab. | R4 |
| DG-72 | The Question bank's Import dialog (CSV / XLSX) is drawn, but no import of bank questions exists. | Built in R6 after T-R6.3. | R6 |

### Take test and student

| ID | Gap | Meanwhile | Needed by |
|---|---|---|---|
| DG-80 | Only a reading passage is drawn. Missing: audio (plays left, the recorded-extra-plays note), image, fill in the blank, question groups, the sections navigator, timer at zero, the focus dialog for the `warn` and `auto_submit` policies, fullscreen enter/return, offline, session takeover, session expiry, review before submit. | Extrapolated from deck primitives; each frame listed for sign-off. | R3 |
| DG-81 | Student routes have no URLs. | `/app`, `/app/classes`, `/app/assignments/:id`, `/app/attempts/:id(/result)`, `/app/settings/:section`, `/app/messages/:id`, `/app/week`, `/app/grades`, `/app/learn/…`. | R3 |
| DG-82 | What the Home badge "3" counts; the bell's red dot is always on; no "Mark all read" or full list for student notifications; student notification switches have one channel while teachers have In app / Email. | Badge = due within 7 days; dot only when unread; mark all read in the popover; student switches control both channels. | R3, R4 |
| DG-83 | Missing empty, loading and error states on most new screens (Messages, Calendar, Attendance, Gradebook, Reports, Courses, Vocabulary, Shared with me, Admin Users, Roles, Audit, Overview). | One sentence + one action, from deck primitives. | Each release |
| DG-84 | The result page draws a "Comment" callout from the teacher with no model behind it. | Only per-answer comments show; a single-essay result shows its comment. | R3 |
| DG-85 | Result variant 2 draws topic tiles (Present perfect 4/5, Past simple 4/5) for a one-part, one-skill paper; nothing models topics. | Not built: a one-part, one-skill paper shows no tiles. | R6 |
| DG-86 | Essay grading draws scores 3–8 in nine values; bands run 3–9 in half steps. | 3–9 in half steps; please redraw the score row. | R6 |

### Collaboration, schedule, insights, learn

| ID | Gap | Meanwhile | Needed by |
|---|---|---|---|
| DG-90 | Sharing entry points are drawn for tests only; Shared with me lists four kinds. Draw sharing for question sets, word lists and courses, and what a "question set" is in the bank. Also: what a Can-use view allows, who is shown as author when a Can-edit recipient publishes, and where the share message appears. | Question set = a named selection made from the bank's bulk bar; Can use = read-only + make a copy; the owner stays the author, the publisher is recorded; the message appears in-app and by email. | R7 |
| DG-91 | Messages: is the attempt-event card in a DM teacher-only? Class-thread attachments render as a "me" bubble. No teacher-to-teacher messaging. | Teacher-only; attachment bubbles follow the sender; no staff DMs. | R7 |
| DG-96 | Data & privacy has no row for how long messages are kept; no screen shows an address that bounced or complained. | Messages are kept; anonymising an account deletes its direct messages. A suppressed address shows in the Admin user sheet from deck parts. | R7 |
| DG-97 | Student Grades draws one "Grammar & vocab" tile; skills are tagged separately. The command palette omits Reports. | Skills shown separately; the palette follows the deck. | R9 |
| DG-92 | Schedule: are sessions generated from the class schedule, from "Repeat every week", or both? Is an Excused or Unmarked attendance status needed? Per-date exceptions on the teacher side. | Sessions come from a weekly series; statuses present / late / absent, plus unmarked. | R8 |
| DG-93 | Gradebook: no place to set an assignment's category and weight, "homework" is undefined, terms have no management screen. | Category and weight in the wizard's Schedule step; terms managed in Admin settings. | R8, R9 |
| DG-94 | Courses: no lesson editor for reading, video or listening; video source undefined; reorder grips have no behaviour; unit "On a date" prints a raw ISO date. | Lessons edited with the existing rich editor; video by upload; reorder by drag; dates formatted per locale. | R10 |
| DG-95 | Vocabulary: the source of "Play pronunciation" / "Listen" audio. | Uploaded audio per word, falling back to the browser's speech synthesis. | R10 |

### Landing

| ID | Gap | Meanwhile | Needed by |
|---|---|---|---|
| DG-100 | Below 960px the section links disappear with no menu; testimonials need consent from real people; the footer "Status" link has no target; SEO / Open Graph content; the page defaults to English. | A menu button below 960px; placeholder testimonials until consent; Status → the status page; vi default. | R11 |
| DG-101 | The consultation form has no submitting, server-error or rate-limited states. | Deck primitives, listed for sign-off. | R11 |
