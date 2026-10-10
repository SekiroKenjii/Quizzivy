# Quizzivy — Frontend Portal & Data Model Specification

**Version:** 0.69 · **Owner:** Thuong · **Audience:** AI coding agent + future contributors
**Scope:** web frontend (admin + student portals) and the PostgreSQL data model. Go backend implementation is a separate spec; the API surface in §15 is the contract both sides implement.

**Changes since v0.68**

R4, notification producers and due-time items (T-R4.10b):

- §15 Who is told, and when. A notification is written after the change that caused it has
  committed, under a budget of its own that the end of the request does not cancel; a
  failure is logged and never fails the request. The teachers are the assignment's creator
  and the teachers of a target class the student belongs to (`visibility.PaperReaders`, the
  rule that lets a teacher see a paper, read the other way), each once, so a teacher of
  another class is told nothing about this student. `attempt.submitted` merges the papers
  handed in on one assignment into one notification per fifteen minutes, a timed-out paper
  being a hand-in; `attempt.flagged` is written once, when the integrity policy flags a
  paper, and never for a teacher's own flag or under `warn`; `class.joined`, which has no
  switch, on a join by code. The student is told `assignment.extended` when `notify` is
  true on an extension (everyone whose close it moved) or on an override that moves a
  close (those it gave time past the assignment's own close), and `result.ready` when a
  teacher finishes grading and the review policy shows the score at that moment. A
  `result.ready` notification carries the title and the attempt, never a score.
- §15 Due-time items. `GET /me/summary` and `GET /me/notifications` first write what the
  passing of time has earned the caller, at most once in five minutes per caller in the
  serving process; there is no timer and no job. A moment older than seven days is not made
  up, a row the caller already holds is left as it is, read or not, and a switch that is
  off writes nothing. For a student: `assignment.opened` (when the assignment opened or was
  published, whichever is later), `assignment.due_soon` a day and an hour before the
  student's own close, override and early close included, while they have handed nothing in
  and have an attempt left and the test was available at that moment, the day's only while
  the hour's is not yet due (a student who first reads inside the last hour gets the
  hour's alone), and `result.ready`
  at the close of an `after_close` release for a paper with nothing waiting for a mark. For
  a teacher: `assignment.closing` an hour before the assignment's close, with the number of
  students they reach who have not handed in and whose own close is the assignment's. A
  close that moves earns its reminders again.
- §9 The Test intro says "We will notify you when it opens." under a test that has not
  opened for a student whose "Test due soon" switch is on, and "You can start once it
  opens." otherwise.

**Changes since v0.67**

R4, duplicate, item analysis and the results export (T-R4.13):

- §15 `POST /teacher/assignments/:id/duplicate` (`teaching.assignments.write`) takes
  `{classIds}` and creates a draft that pins the same version and keeps the original's
  time limit, attempts, shuffle flags, review options, integrity policy and note to
  students. Its window opens now and lasts as long as the original's; it is assigned to
  the classes named and to no student by name, and is owned by the caller. An original the
  caller does not reach answers `404`, one whose version the caller may not assign answers
  `409 TEST_NOT_PUBLISHED`, and a class the caller does not teach answers `400` on
  `classIds`.
- §15 `GET /teacher/assignments/:id/item-analysis` (`teaching.grading`) answers
  `{handedIn, items}`: for each question of the pinned version, hardest first, how many
  papers answered it and the share that earned its full points. The papers are, for each
  student the caller reaches, the latest attempt that is handed in and not voided. A
  question left unanswered counts against it, a manually marked answer not yet marked
  counts for neither side, and a question no paper has a mark for has a null rate and
  comes last.
- §15 `GET /teacher/assignments/results.csv?ids=` (`teaching.grading`, 1 to 50 unique ids)
  downloads a UTF-8 CSV with a byte-order mark: a row for every student of each assignment
  whom the caller reaches, a student who has not started included, with the attempt the
  monitor shows for them. Its times are in the caller's own time zone, and its headers and
  status words follow `Accept-Language`. A cell a spreadsheet would run as a formula is
  written with a leading `'`. An assignment the caller does not reach answers `404` for the
  whole request, and more than 20000 rows answer `422 VALIDATION_FAILED`. A caller may
  download 10 files a minute and 60 an hour.
- §15 `GET /teacher/assignments` takes `q` (the test's title or the name of a target class
  the caller reaches, ignoring accents) and `classId` more than once (any of the classes),
  and its facets follow both. Every assignment carries `questionCount`.

**Changes since v0.66**

Characters the brand font draws (FONT-1, DG-141):

- §12 Typography: the italic faces are loaded for every upright weight. Every character
  the UI draws is in the loaded font faces; a symbol the font lacks (arrows, ⌘, ⏎, ⇧, ≤,
  ≥) is an icon, or a character it has, such as the en dash for a range.
  `font-coverage.test.ts` holds the source and the locales to it.
- §12 Typography: content is shown in NFC. Be Vietnam Pro's subsets lack the combining
  circumflex, breve and horn, so decomposed Vietnamese would draw in a fallback font. The
  rich content editor composes what it writes; a plain field, the Markdown source and a
  student's answer are stored as typed and composed where they are drawn (F-37 proposes
  composing stored text on the server).

**Changes since v0.65**

R4, extensions and per-student overrides, second half (T-R4.12b):

- §13 An attempt in progress follows a change to its window. `POST
  /teacher/assignments/:id/extend`, `PATCH /teacher/assignments/:id` (a later close),
  `POST /teacher/assignments/:id/reopen` and `PUT
  /teacher/assignments/:id/student-overrides` set the deadline of each attempt in
  progress to the earlier of its start plus its student's time limit and its student's
  close, in the transaction that changed the window, and only where that is later than
  the deadline it has. A deadline never moves earlier: removing an override (`DELETE`),
  setting a shorter time limit or a shorter close, and closing early leave a running
  attempt where it is, and so does an attempt whose deadline has already passed (judged on
  the database's clock), which is over whether or not it has been swept and is not revived.
  Each attempt moved is audited as `attempt.extended` with the old and new deadline and the
  cause. An override moves only the attempts of the students it
  names.
- §13 An attempt starts under the window as it stands when it commits. A start that is
  racing an extension stores the extended deadline, or is itself lengthened by the
  extension, whichever commits first; it is never left on the close it read before.
- §15 `closesAt` of an override must be ahead of the database's clock, not the API
  server's.
- §15 `MonitorRow.extendedTo` carries the time a student's own override closes when that
  is later than the assignment's effective close, and is absent otherwise. An override
  that closes sooner than the assignment says nothing. Closing an assignment early does
  not take an override away: a student whose override reaches past the close stays open.
- §13.9 The maintenance window's extension also moves the close of each student override of
  a published assignment that falls inside the window, by the window's length, and writes
  an `assignment.override_extended` audit entry per override. The operator's role needs
  `SELECT, UPDATE (closes_at)` on `assignment_student_overrides` (release checklist).
  `extendAssignment` and the two override writes take the same advisory lock (73819, 40)
  shared, as a start does.

**Changes since v0.64**

R4, extensions and per-student overrides, first half (T-R4.12a):

- §13 New table `assignment_student_overrides`, one row per assignment and student, that
  gives one student a later close, a longer time limit (1 to 600 minutes) and up to 10
  more attempts, with a required reason (1 to 500 characters). It cascades with the
  assignment and with the student, and a row must change something. Migration 00101.
  A missing row is the assignment as everyone has it.
- §15 `POST /teacher/assignments/:id/extend` moves the close later for everyone
  (`teaching.assignments.write`) and answers `409 ASSIGNMENT_CLOSED` for a closed
  assignment, which is reopened instead. `PUT`, `GET` and `DELETE` on
  `/teacher/assignments/:id/student-overrides[/:studentId]`
  (`teaching.attempts.intervene`) set, list and remove one student's override. A request
  names only students of the assignment the caller reaches; a student who is not one
  answers `422 VALIDATION_FAILED` for the whole request. A field the request names
  replaces the stored one and a field it leaves out keeps it. Every write is audited with
  the old and new values.
- §9 The student's card, intro, start and result read the student's own window: the later
  of the assignment's close and their override's (which lifts an early close), their time
  limit, and the attempts they were given. An assignment that closed for everyone else
  reads open to a student whose override is still ahead. A result released `after_close`
  is withheld until that student's own close; the class average still waits for the
  assignment's. Nothing in a student's payload says an override exists or why.
- An attempt already in progress kept its deadline in this version; v0.66 (T-R4.12b) recomputes it.

**Changes since v0.63**

R4, assignment review options, the note to students and the live lock (T-R4.11):

- §13 `assignments` gains `review_release` (`on_submit` or `after_close`, default
  `on_submit`), `review_show_class_average` (default false) and a nullable
  `student_note` (1 to 500 characters once trimmed). Migrations 00099 and 00100; every
  default is a constant, so nothing is rewritten.
- §14 `Assignment.review` carries `release` and `showClassAverage`, and `Assignment`
  carries `studentNote`. On a write the three are partial: omitted, they keep what is
  stored, and `studentNote: null` clears the note.
- §15 While an assignment is `open`, a PATCH that changes `testVersionId`,
  `durationMinutes` or `maxAttempts` answers `409 ASSIGNMENT_LOCKED`; nothing else locks.
- §9 A result released `after_close` is withheld until the assignment's effective close
  (score, marks, answer key, explanations and the grader's comments) and carries
  `releasesAt`. With the teacher's switch on, a released result of a closed assignment
  carries `classAverage` once at least 3 students qualify. The Test intro draws the
  teacher's note and says in its score sentence when the score comes.

**Changes since v0.62**

R4, the content editor's Rich text and Markdown modes (T-R4.64, DG-110):

- §7.1: rich prose is how a prompt and an explanation are written, no longer
  an opt-in behind `VITE_RICH_QUESTION_EDITOR`, which is removed. A field opens
  in the form it is stored in, and a field with no text opens as rich text;
  the builder's and the group composer's starter prompt is stored as Markdown
  and opens as Markdown until T-R4.31b and T-R4.34 create it as rich text.
  "Switch to Markdown" and "Apply conversion" are the only ways the stored form
  changes, each behind its confirmation; Markdown is refused while a
  fill-in-the-blank prompt holds a gap. Markdown gains GitHub's tables and
  `~~strikethrough~~`, for the teacher and for the student, and no other GFM
  syntax. Default, not yet confirmed by Thuong.

**Changes since v0.61**

R4, alt text for a question's image (T-R4.62a):

- A bank question whose media is an image carries optional alt text of 1 to 1000
  characters, returned to teachers as `mediaAlt` and frozen into the published
  version with the rest of the question, so a later bank edit never reaches a
  version. Students receive it beside `media` in the paper and in the result; it
  is a description for a screen reader, not part of the answer key.
- A value for a question whose media is not an image is refused with a field
  error on `mediaAlt` (400 `VALIDATION_FAILED`). A write replaces the stored
  value, so an update that leaves it out clears it. Restore as draft and a
  duplicated group carry it, and the version diff counts a changed alt text as a
  change to the question's media. Migrations 00097/00098 add nullable columns
  without backfill (`docs/plan/20-data-model.md` D-33, §37).

**Changes since v0.60**

R4, content editor frame (T-R4.63, DG-115):

- §7.1: a formatted paste leaves the images inside copied content out, unloaded,
  and counts them in its preview, instead of refusing the whole paste. Every
  other refusal stays; a paste holding only images, and a pasted or dropped
  file, are refused with a notice. Default, not yet confirmed by Thuong.

**Changes since v0.59**

R4, question metadata and authoring compatibility (T-R4.15):

- Teacher question reads and authoring carry nullable level and skill; draft test
  lists expose distinct sorted skills. Published metadata stays frozen through
  later bank edits and draft restoration.
- New authoring accepts at most eight choice options. Stored reads, bodyless
  copies, publication and restoration preserve legacy longer arrays. Migrations
  00087/00088 add nullable metadata without backfill. Local PG18 up/down/up,
  isolated app/migrate behavior and unchanged critical canaries pass. A separate read-only Neon data copy reports zero oversized
  questions among 64 bank and 23 frozen rows; no cloud DDL was executed.

**Changes since v0.58**

R4, import processing presentation (T-R4.37):

- The teacher processing page follows the deck for file and pasted-text sources,
  preserving real run stages, revision guards, cancellation, retention and review
  states. Text failure titles and recovery copy do not guess which source failed.
  Paste intake editing remains T-R4.57; run-event history remains unavailable.

**Changes since v0.57**

R4, account preference adoption (T-R4.7):

- Server-authoritative account preferences, reactive date displays and same-page
  actor/cookie/cleanup ordering are complete. Teacher wall-clock inputs remain
  fixed-HCM until T-R4.43; full profile controls and photo operations remain
  T-R4.43 and T-R4.8. This task completion is not a production release.

**Changes since v0.56**

R4, private caller profile and preferences (T-R4.7, staged):

- §7, §15 Public users gain optional display name/avatar; only the caller gains
  phone, locale, zone and preferences. Self profile PATCH preserves omitted keys
  and supports explicit clearing of display name/phone. Preferences merge top-level
  keys with bounded, closed payloads and atomic audits.
- §8 The Dashboard zone port now reads the caller's stored valid zone, defaulting
  only a stored NULL to Vietnam time. §13 records migrations00085/00086.
- Frontend account preference adoption, actor/cookie ordering and reactive date
  display remain the staged companion's acceptance gates. Existing datetime
  input parsing stays fixed to Vietnam time; full profile controls/photo follow
  T-R4.43/T-R4.8. No completed frontend or release is claimed by this record.

R4, private pasted-text sources (T-R4.55):

- §16 Adds plaintext exam intake, source character counts and text evidence using
  the existing private upload lifecycle. Recognition conventions and paste editing
  remain separate T-R4.56 and T-R4.57.

**Changes since v0.55**

R4, import history read model (T-R4.21):

- §16 History adds search-scoped status facets and nullable counts from the
  current review body. Repeated statuses are OR-ed; Processing includes all
  four active statuses. Legacy uncomputed counts remain unknown, and a pending
  machine candidate never replaces the counts of the teacher's current edits.

**Changes since v0.54**

R4, import history and source upload presentation (T-R4.36):

- §16 History uses one responsive card/grid with URL-backed filters and real
  pagination. File intake keeps selected files and the title across Source mode
  changes. Facets, text metadata and paste intake follow in T-R4.21, T-R4.55 and
  T-R4.57; their data and operations are not synthesized by the presentation.

**Changes since v0.53**

R4, read-only grading queue (T-R4.14):

- §8, §15 Teachers can read pending manual answers grouped by student or frozen
  question, with complete counts and a deterministic first200-item prefix.
  Historical paper reach and the existing explicit Finish operation are preserved.

**Changes since v0.52**

R4, teacher home and shell counts (T-R4.19):

- §5 Both home operations use the caller's own teaching scope, admins included;
  repository-wide reads and the attempt list retain their existing scope.
- §8, §15 The dashboard adds live taking counts, range-bound calendar submissions,
  today's assignment events and recent activity; the nav gets its own summary.
  Grading is permission-aware, calendar queries use the actor-zone port and
  application clock, and nullable graded scores retain their existing meaning.

**Changes since v0.51**

Round 5, the merged fixes and their limits:

- §9, §15 A tab names its held session on each attempt read; a superseded
  answer stays read-only and preserves another session's local draft (#337).
- §9 A deadline lock rearms automatic submission without replacing a closed
  or superseded lock (#353, #337).
- §10.1, §10.6 A question's blocked seek records `audio_seek` (#339).
- §10.4, §10.6 Sequence numbers use a server-calibrated offset and local
  increments; cross-tab order and uniqueness are not guaranteed. The timeline
  follows its session and sequence comparator (#338).
- §10.6 A shared recording reports its end and block; duration is elapsed
  device wall time since the latest start, with pairing limits (#340).
- §11.3 A blocked play keeps its player, and an aborted start shows no
  expired-link card (#339).
- §15 The body-limit reader answers `408 REQUEST_INCOMPLETE`; the earlier
  body-key reader can still hide an incomplete request (#342).
- §15 Every JSON-body operation declares the validator's 400 response (#346).
- §5.2 Own sign-out removes `next` in that tab's guard; the second-tab expired
  card remains open work (#355, #375).

**Changes since v0.50**

R4, "Teacher workspace" (v0.10.0, `docs/plan/74-r4.md`):

- §11.1 Audio may be 50 MB; an image stays at 10 MB, and the length stays at
  five minutes (DG-63). A teacher's media library has a quota, 5 GB by
  default. A file can be renamed in the library, audio can carry a default
  play limit, and an image's size in pixels is recorded at upload. §15 lists
  the search, the "not used" filter and `PATCH /teacher/media/:id`
  (T-R4.17a).

**Changes since v0.49**

R4, "Teacher workspace" (v0.10.0, `docs/plan/74-r4.md`):

- §6.1 At each start-up the API replaces every join code held only as its
  SHA-256, and neither revoked nor expired, with a sealed one that keeps its
  expiry and use cap, and tells the class's teacher once in the app. The old
  code then answers as any replaced code does (T-R4.22).

**Changes since v0.48**

A code nothing sent leaves the contract, and the sections are brought level
with six fixes to the web app:

- §15 `ALREADY_ENROLLED` is removed from `ErrorCode`: no server version ever
  sent it (#315).
- §15 A path outside the contract answers `404 NOT_FOUND` and a path under a
  method it does not serve answers `405 METHOD_NOT_ALLOWED` with `Allow`, both
  in the envelope. `HEAD` is answered as `GET` without a body on the operations
  that need no token (#308).
- §9 A draft a closed tab left behind is sent under the session that wrote it
  before "Tiếp tục làm bài" gives the student a new one. A refusal that can
  never become a save drops it; any other failure keeps it, and the attempt is
  not resumed (#330).
- §10.6 Leaving the engine while the attempt goes on sends the buffered events
  through the beacon and keeps the sequence (#331).
- §10.6 A question's audio player records `audio_ended` and `audio_blocked`
  beside `audio_play`. §10.4 A play with no recorded end shows no duration and
  is not called ongoing (#314).
- §10.1, §11.4 A question's play is counted by
  `POST /app/attempts/:id/audio-play`; the three audio events report it and
  change no count.
- §11.3 The total a player shows is the server's probed length when it is
  known (#321).
- §11.3 The player's file is
  `web/src/features/media/components/AudioPlayer.tsx`.
- §9 Once the server has said the time is up, the timer and the Submit
  dialog's time left read 00:00 (#329).
- §5.2 A sign-out closes the "Vui lòng đăng nhập lại" overlay if it was raised
  while the sign-out was on its way (#328).

R4, "Teacher workspace" (v0.10.0, `docs/plan/74-r4.md`):

- §5 In the teacher workspace the six content lists (tests, questions,
  question groups, media, imports, assignments) hold the caller's own rows for
  every caller, the Admin included: `scope.all` no longer widens them. It
  still reads and edits by id, and still widens the lists of classes, students
  and attempts and the dashboard. §16 says the same of the import history
  (DG-53, T-R4.54).

**Changes since v0.47**

The contract's descriptions, brought level with four fixes:

- §15 An error's `message` follows `Accept-Language` on every operation; the
  field sentences and publish violations the rules word do not yet (#284).
- §15 `publishTest` reports a question whose media file was deleted as a
  violation, not a 500 (#287).
- §15 A JSON body that repeats a member name is refused, unless the server
  filled a default into it (#288).
- §15 `createDraftFromTestVersion` answers `RESOURCE_REFERENCED` when a
  question of the draft's groups is still used elsewhere (#296).

R4, "Teacher workspace" (v0.10.0, `docs/plan/74-r4.md`):

- §6.5 The signed-in operations that hand out a credential are limited per
  signed-in user, after the permission check, and no longer per address: staff
  behind one address stop sharing a budget. §5.5 says the same of
  `openDocsSession` (T-R4.53).

**Changes since v0.46**

R3, "Student console" (v0.9.0, `docs/plan/73-r3.md`), the screens and the engine
as built (T-R3.5 to T-R3.11):

- §5.2 A refusal ends only the session its request was sent under, and a 401
  at app load keeps a sign-in that finished while the request was out (#317).
- §6.2 A signed-in student joins from the Join dialog on Classes and Home. A
  member is told so from the preview's `classId`; a member's join answers 200,
  and nothing sends `ALREADY_ENROLLED`. A lookup's answer is remembered for 30
  seconds.
- §6.5 The preview returns three fields, `classId` among them.
- §9 The student shell branches at 768px, with bottom tabs below it. The
  `/app` rows describe Home, the intro, the result, Classes (cards that are not
  links) and Settings (Profile, Sign-in, Appearance) as built. The engine has
  panes, a number strip or a question sheet, and the Submit dialog in place of
  the review page. Keys are A–E, F and the arrows, which stop at either end.
  §7.3's sentence on learner material says the same: one pane at a time on a
  phone, with no collapse state.
- §10.1 The clipboard listeners are on `document`.
- §10.2 The focus dialog's body is the deck's, without the seconds away or
  "the timer keeps running". The intro's start or resume click enters
  fullscreen; Home's resume card does not, and the engine's bar offers it. The
  engine leaves fullscreen once the attempt is submitted and records no
  fullscreen change after that. A blocked copy or paste raises a toast. The
  timer turns red under five minutes.
- §10.6 The monitor owns every listener that records or stops a signal. Other
  hooks listen to the same events and do neither: `useClipboardNotice` for the
  toast, `useSaveStatus` for the save line, `useVersionWatch` for a newer build.
  A question's audio player records `audio_play` through `recordAudioEvent`.
- §12 The student rules: one breakpoint, the page widths, the 44px floor and
  the controls that keep the deck's size, the dialog frame, the timer's digit
  cells, larger text in tests. The integrity UI follows the deck on the
  student side. The teacher console is the only one that still forces light.
- §7, §13.3, §15 v0.9.0 ships without R2's contract steps. The legacy `role`,
  the `NOT VALID` owner constraints with their fill triggers and the `/admin`
  alias stay until v0.9.1 (T-R3.1 to T-R3.3), no earlier than 2026-10-10.

**Changes since v0.45**

R3, "Student console" (v0.9.0, `docs/plan/73-r3.md`), the contract (T-R3.4 and
three fixes):

- §15 A student's assignment card carries `liveAnsweredCount`, and the result
  carries the paper's `sections` with a `sectionId` on every question
  (T-R3.4).
- §15 `startOrResumeAttempt` accepts `resume`; a Continue never starts an
  attempt (#237).
- §15 The monitor's `answeredCount` counts by the same rule as
  `liveAnsweredCount` (#234).
- §15 A student's assignment card carries `classIds` (#240).
- §15 A logout the maintenance gate or the limiter refuses, or whose revoke
  fails, still clears the session cookies (#188, #256).
- §5.2 A sign-in stores its refresh token under the user's lock and is refused
  if the account changed meanwhile; the rotations, reuse detections and
  logouts of one user run one at a time (#248, #279).
- §5.4 An individual target who is already on an assignment stays on it when
  disabled (#210).
- §15 `deleteQuestionGroup` answers `RESOURCE_REFERENCED` (#213).

**Changes since v0.44**

R2, "Access" (v0.8.0, `docs/plan/72-r2.md`):

- §1.1 Roles are data, one per user. The personas are the Admin (the owner),
  Teachers and Students; the Assistant role exists with no class staff yet.
- §1.3 One organization with several teachers. Every class, test, question,
  question group and media asset has an owner, and every Word import its
  creator.
- §5 Every operation that requires the bearer token declares `x-permission`,
  which `RequirePermission` enforces on every request from a 10-second
  principal cache; the seven open operations declare none. Another teacher's
  id answers as a missing one does.
- §5.2 The session epoch: disabling a student or resetting their password ends
  their live sessions at once, and a disabled user is refused on the next
  request.
- §5.3 Sign-in returns `CurrentUser`.
- §5.4 The web guards read the user's workspaces, not the role. The guards
  that are not permissions: strict student targets, the subset rule,
  disabling, shared students (`403 STUDENT_SHARED`) and the last Admin.
- §5.5 The API reference needs `system.api_reference`, and its cookie carries
  the session epoch.
- §6.1 New join codes are stored encrypted under `JOIN_CODE_KEY` and read back
  for their teacher. Codes issued before v0.8.0 stay hashed until R4. The key
  and its rotation are described.
- §6.4 The controls belong to the class's teacher, and to an Admin through
  `scope.all`.
- §6.5 Limits are sized for a classroom: 120/min and 600/h per address, 200/h
  per code, and the login, refresh and logout budgets. Keyed bodies take at
  most 8 KiB (`413`). Lookup is by a keyed hash.
- §7 Adds `CurrentUser`, `PermissionKey` and `Workspace`; `Role` is legacy.
- §13.2 `roles` is a table, not an enum. §13.3 adds `permissions`, `roles`,
  `role_permissions`, `users.role_id`, `session_epoch` and `created_by`, the
  owner columns and the join-code columns. §13.5 describes sealed join codes
  and the read-only access tables. §13.7 allows reference data in a migration.
- §15 Teaching operations move to `/teacher/*`, and the v0.7.0 `/admin/*`
  paths answer through an alias until R3 (v0.9.1, since v0.47). `/admin/*` keeps `deleteUser` and
  the docs session. Adds `PATCH /auth/me`, `getJoinCode`, the guards' 403s
  and `RESOURCE_REFERENCED`'s `details.referencedBy`.
- Word import: an import, its sources, review and commit belong to its
  creator, and `scope.all` reaches every import.

**Changes since v0.43**

R1, "Foundations and front door" (v0.7.0, `docs/plan/71-r1.md`):

- §4 The organisation's name and front-desk details come from build-time config.
- §5.2 A refused session keeps a signed-in user's page under a "sign in again"
  overlay. A refresh that fails without refusing is not a lost session.
- §5.4 New passwords need 8 characters and a number or symbol, and must differ
  from the current one (`PASSWORD_UNCHANGED`). `/forgot-password` exists.
- §6.1 The alphabet prose now matches the alphabet, which contains `L`.
- §6.2 Join previews the class inline on `/join/:code`, and ends on a Joined
  state. `/join/:code/confirm` redirects.
- §9 Covers `/forgot-password`, the system pages, the boot splash and the
  overlays (maintenance, sign in again, a newer build).
- §10.2 Integrity pauses under the maintenance and sign-in overlays. The
  engine adopts the server's deadline from every autosave.
- §12 Records the deck's foundations as built: tokens, Be Vietnam Pro, dark
  mode, `data-scale="deck"`, the accessibility tokens and the motion list.
- §13.9 Adds maintenance windows.
- §15 Adds `GET /public/status` and the 503 during a window, plus
  `MAINTENANCE_SCHEDULED`, `DEADLINE_NOT_REACHED`, `PASSWORD_UNCHANGED` and
  `saveAnswers.deadlineAt`.

**Changes since v0.42**

- §16 Word and PDF import run in production (O-24, decided 2026-09-25). The
  image ships the import worker, and `fly.toml` runs it as its own process group
  on a 1 GB Machine, which the API wakes over Fly's private network.

**Changes since v0.41**

- §16 Word import also takes PDF (D-10, decided 2026-09-25):
  - Only a PDF with a text layer can be read. A scan fails with `PDF_NO_TEXT`;
    there is no OCR.
  - Underline, bold and colour are not read from a PDF, and every PDF draft says
    so. Keys come from a key file or explicit `1. A` lines.
  - PDFium reads the file inside a WebAssembly sandbox in the worker. No
    conversion runs.
  - `ImportSource.format` and `ImportLimits.formats` gain `pdf`.

**Changes since v0.40**

- §16 Word import has a retention policy (D-08, approved 2026-09-25):
  - Original files and the review draft are kept 30 days after the test is
    created and 7 days after cancellation.
  - An import untouched for 60 days is closed and its files removed.
  - History rows stay.
  - The API applies the policy at start-up and then daily. It exists wherever
    import storage does, with or without a worker.
  - `WordImport.filesRemovedAt` records the removal. The source download, the
    source view and the review then answer 410 `IMPORT_FILES_REMOVED`.
  - `GET /admin/imports/capabilities` states the three periods.

**Changes since v0.39**

- §16 Word import states whether it can run on this deployment:
  - `GET /admin/imports/capabilities` reports `intakeEnabled` and
    `processingEnabled`.
  - Processing requires `IMPORT_PROCESSING_ENABLED`, set beside a worker. Without
    it, `processWordImport` answers 503 `IMPORT_PROCESSING_UNAVAILABLE` and
    queues nothing.
  - The client hides or explains whatever cannot run.
  - Production keeps import off until O-24 is decided (#138).

**Changes since v0.38**

- §5.5 adds a docs session: the API reference (`/docs`, `/docs/openapi.json`)
  opens only with a fifteen-minute, docs-only cookie that an admin gets from
  `POST /admin/docs-session`. Its token is signed with its own key for its own
  audience and is never an access token. Local development may opt out with
  `DOCS_PUBLIC`, which production refuses. The page loads Scalar by SRI under a
  CSP that admits only that bundle and its own inline script (#135).

**Changes since v0.37**

- W-11b connects private storage, isolated conversion, chunked source extraction
  and deterministic recognition in a separate worker process. Completed stages
  survive retries; source/artifact checksums are verified before parsing. Run
  envelopes contain lineage IDs, with source and answer content in private artifacts.
  Public processing controls and full review/domain validation remain subsequent work.

**Changes since v0.34**

- W-13a adds an internal isolated legacy Word converter and private source-page
  rendition. One physical container slot, offline safe document loading, resource
  limits, cancellation and independent timeout protect processing capacity. Outputs
  retain conversion lineage and require review. Public legacy intake and durable
  processing integration remain disabled pending the complete pipeline.

**Changes since v0.33**

- W-12a adds ordered, source-bound extraction with Unicode fragment locations,
  nested/merged table evidence and explicit review reasons for private/ambiguous
  content. Bounded raster normalization retains images privately. These internal
  tools do not yet enable recognition, review or draft creation; original evidence
  is never a learner payload.

**Changes since v0.32**

- W-11 adds an internal durable processing queue with immutable source/pipeline
  identity, bounded attempts, leases, fenced writes and append-only run events.
  Cancellation and completion serialize on the import. A context-aware runner
  records safe operational metadata and preserves retryable work after shutdown.
  The production processor, supervisor and public processing controls are not
  wired yet; this checkpoint does not enable recognition or review.

**Changes since v0.31**

- W-10a adds disabled-by-default private native DOCX intake, teacher-only history,
  immutable source sets and retry identities. Storage writes have durable reservations,
  quota accounting and revision guards. Original downloads require teacher access and
  short-lived attachment URLs from a separate private bucket. Legacy conversion,
  processing, review, commit, retention and production acceptance remain later work.

**Changes since v0.30**

- W-07h integrates section → group → question authoring into the test builder.
  Outline and group writes share a serialized enclosing-test revision; acknowledged
  moves update only the editor's own revision baseline. Whole-group bank insertion,
  independent bank copies, complete learner preview and local recovery are available.
  The import pipeline and pilot acceptance remain release gates.

**Changes since v0.29**

- W-07g adds the independent group bank and a complete-group editor: shared
  materials, authorized image/audio insertion, stable gap links, learner preview,
  revision-checked autosave and explicit local recovery. Archived groups support
  bulk restoration/deletion; copies remain on the list with an indicator.
  Mixed builder editing and the import pipeline remain release gates.

**Changes since v0.28**

- W-07f saves complete mixed outlines under the test revision, moving whole owned
  groups between sections while retaining every existing group exactly once.
  Group removal remains explicit. Legacy saves clear obsolete unit rows after
  the last group is removed. Builder integration remains a release gate.

**Changes since v0.27**

- W-07e exposes teacher-only complete-group read/write/copy/archive operations,
  bounded bank summaries and ordered draft units. Writes check aggregate and
  enclosing-test revisions. Media-link outages return a retryable asset state
  without obscuring committed content. Builder integration remains a release gate.

**Changes since v0.26**

- Draft totals/tag filters include group members. Legacy question-only outline
  writes refuse grouped drafts atomically. Archived tests must be restored before
  draft restoration/default-version changes. Assignment introductions include
  shared recording policies from the assigned version and explain their scope.

**Changes since v0.25**

- W-09f supplies frozen shared context in learner results and both teacher
  grading modes. Transcript release follows each recording's policy independently
  of score/key/explanation flags. Review playback does not add attempt plays.

**Changes since v0.24**

- W-09e renders shared learner materials beside the question on wide screens and
  above it on phones with remembered collapse state. Stable recording players and
  idempotent gesture recovery preserve shared counts across member navigation,
  reload and takeover. Result/review context and authoring remain gated.

**Changes since v0.23**

- W-09d freezes the delivery algorithm on each published version. Historical
  standalone papers keep `section_v1`; group-aware snapshots use `group_v1`.
  Attempt reload, takeover and results select the frozen marker, never the
  current test default. Unsupported formats fail closed.

**Changes since v0.22**

- W-09c persists shared-recording counters and gesture receipts, deduplicates
  retries atomically with timeline events, and includes shared excess plays in
  teacher monitoring. Group authoring/player/result UI remain gated.

**Changes since v0.21**

- W-09b adds learner-safe shared context to start/resume/read attempt responses.
  Frozen media bindings require an attempt owned by the learner. Shared playback,
  result/review context and group authoring remain gated.

**Changes since v0.20**

- W-08c previews frozen group context and authorized assets without grading keys,
  follows the selected default version, and offers a 320px learner preview inside
  the supported admin shell. Group authoring and live learner delivery stay gated.

**Changes since v0.19**

- W-08b restores and duplicates complete independent context graphs, preserves
  mixed unit order and removes owned draft graphs when an unreferenced archived
  test is deleted. Grouped preview/draft UI and delivery remain gated.

**Changes since v0.18**

- W-08 defines independent frozen group membership, materials, gap targets and
  recording policies alongside existing version questions. Historical snapshots
  are not backfilled; group authoring stays unavailable until readers are ready.

**Changes since v0.17**

- W-07d adds revision-checked full-group editing, independent copy materialization,
  bank archive/restore/delete and section removal. These remain internal operations
  until complete draft, snapshot and learner readers are available.

**Changes since v0.16**

- W-07c adds atomic internal group persistence, owned-member barriers and protected
  media references. No group endpoint or authoring affordance is enabled yet.

**Changes since v0.15**

- W-07b adds relational group ownership, ordered units, stable cloze targets and
  protected material/recording bindings. Existing question ownership remains
  null; new group authoring is still unavailable until lifecycle and readers ship.

**Changes since v0.14**

- W-07a defines the bounded, independent group graph: ordered members, rich
  materials, explicit choice/blank gap targets and shared recording identities.
  Validation and detached copying precede persistence and delivery; group writes
  remain unavailable until snapshots, protected references and readers are ready.

**Changes since v0.13**

- Rich authoring supports bounded structured clipboard conversion with an explicit
  preview, atomic insertion and undo. Unsupported source content is refused in full;
  it is never silently reduced to plain text. Original clipboard HTML stays local.

**Changes since v0.12**

- Rich fill-blank prompts bind stable gap identities to answer rows. Moving gaps
  keeps answers attached; copies remap both ends; published versions preserve the
  frozen graph. Legacy Markdown and answer payloads remain compatible.

**Changes since v0.11**

- Manual/internal question writes and publication share interaction validation.
  Single-choice and true/false require one key; true/false requires two options.
  Question points are positive exact hundredths within numeric(8,2); publication
  sums exact hundredths and rejects empty exams or an overflowing total.

**Changes since v0.10**

- W-05b adds bounded semantic prompts and explanations with exact plain projections,
  immutable snapshots and policy-gated explanation delivery. Legacy Markdown remains
  on its existing reader; no old content is rewritten. Fill-blank prompts retain
  Markdown until stable gap binding is integrated.

**Changes since v0.9**

- Inline option formatting now has additive storage, snapshot/restore and learner
  readers (§7.1, §13.3). New authoring is a pilot opt-in; legacy options remain
  literal text and existing published rows are not backfilled.
- Option documents contain only marked text and line breaks; their plain text
  must match the stored `text`. Missing rich content in a legacy write cannot
  silently erase formatting. See `docs/plan/19-content-contract.md`.

**Changes since v0.8**

- The Word foundation defines an additive `ContentDocument` contract (§7.1).
  Existing question writes, stored Markdown and published versions are unchanged.
- Thuong approved independent copies of groups/materials and per-account local
  recovery for unsent edits, with seven-day expiry and logout clearing (§16).
- New grouped exams will shuffle whole groups within their section, retain member
  order, and share a recording's play allowance across its group (§7.2, §11.5).

**Changes since v0.7**

- The Word exam import milestone is accepted for implementation (§16). Its full
  scope includes semantic content, shared materials and groups, durable review,
  safe publication/delivery and operational release gates. It is not yet released.
- Both cloud and private AI processing remain candidates for measured evaluation;
  provider selection and external exam-data processing are not enabled by default.

**Changes since v0.6**

- Dashboard work cards lead into a full-width assignment table and compact activity below.
- Student discovery has counted status filters and prominent resume cards.
- Teacher and student settings share Profile, Security and Preferences routes,
  persistent forms, section navigation and separated form rows.
- Subtle shadows, hover feedback and 150ms section transitions respect reduced motion.

**Changes since v0.5**

- Admin lists retain filters, support bulk removal and duplicate in place. The
  question bank exposes attached outlines in a nested table (§8).
- Version history supports safe draft restoration, default selection and unused
  version deletion without changing assigned snapshots or reusing version numbers.
- Assignment limits support custom durations, attempt counts and immediate
  integrity submission with retained answers and recorded violations (§10).

**Changes since v0.4**

- The approved independent UX review replaces the student mockup's sparse right
  panels with fluid discovery pages and centred reading/form pages (§9, §12).
- All live attempts remain discoverable; class/status filters, result empties,
  explicit selection/audio/save states and direct submitted-paper navigation are
  part of the student flow. Responsive changes preserve local form/filter state.
- Phone actions use 44px touch targets, including navigation and dialog controls;
  the final-question footer must fit at 320px in Vietnamese and English.

**Changes since v0.3**

- Pending student answers survive a failed save/reload within the same session;
  manual submission waits for the final save. Keyboard editing/navigation rules
  are explicit in §9.
- §12 pins the mockup type scale and phone input/touch exceptions.
- Approved O-23 defines thirteen-month integrity retention, retained audit logs
  and manual structured-identity anonymization in §13.3.

**Changes since v0.2**
- **OQ-2 answered: yes** — students self-join with a class code. New §6, new `/join` flow, join-code lifecycle in the schema. This adds a public, unauthenticated surface that did not exist before; read §6.5 on the security consequences.
- **OQ-3 answered: yes** — audio (listening) questions ship in v1, with upload and a custom player. New §11, new `media_assets` table, object storage added to the stack.
- **OQ-4 answered: yes** — `sample_answer` on `short_answer` questions, visible to the admin during grading only.
- All open questions are now closed. §17 lists the decisions that were made *inside* these answers and are worth a second look.

---

## 1. Product context

Quizzivy is a web app supporting a private English-teaching practice. Students take tests assigned by their teacher; the teacher/admin builds tests, assigns them, monitors attempts, grades open answers, and reviews results.

**v1 ships one capability end-to-end: test-taking.** Later versions add vocabulary practice, homework, pronunciation drills, and class management. v1 must not paint us into a corner for those, but must not build them either.

### 1.1 Personas

| Persona | Built-in role | Description |
|---|---|---|
| Owner | Admin | Runs the practice and teaches in it. Holds every permission ("Take tests" only when turned on) and reaches every teacher's data (`scope.all`). Works in the teacher console until R5 builds the Admin console. **Desktop/tablet only.** |
| Teacher | Teacher | Builds tests, manages students and classes, grades, reads results, over the classes and content they own. **Desktop/tablet only.** Data-dense UI is fine. |
| Student | Student | Joins a class with a code, takes assigned tests, sees own results. Often on a phone. Needs a calm, focused UI. |

Roles are data (`app.roles`), one per user (`users.role_id`). The four built-in roles are Admin, Teacher, Assistant and Student; their grants from the permission catalogue are in `docs/plan/70-redesign-overview.md` §4.1, and §5 says how they are enforced. The Assistant role exists with the deck's grants; until the deck draws class staff, no class has an assistant and no role picker offers the role (D14, 70 §1). No screen creates a role or a staff account before R5.

### 1.2 Goals (v1)

1. A student can join a class and complete an assigned test on a phone without losing work on refresh, tab close, or brief network loss.
2. The admin can author a test with mixed question types — including listening — in under 15 minutes without reading docs.
3. Every attempt is reviewable: answers, timing, score breakdown, and an integrity timeline.
4. Codebase is structured so the next feature (e.g., vocabulary sets) is a new folder under `features/`, not a rewrite.

### 1.3 Non-goals (v1)

- **Payments and subscriptions.**
- **Open public sign-up.** Self-signup exists but is gated behind a class join code (§6). There is no "create an account" entry point without one.
- **Multi-tenant / multiple schools.** One organization with several teachers. Each class, test, question, question group and media asset has an owning teacher, and each Word import its creator (§5); the Admin reaches them all. No org/tenant scoping in the schema.
- **Hard proctoring** — webcam, screen recording, screen-lock, remote inspection. §10 covers browser-signal monitoring only; see §10.5 on its limits.
- **Audio transcoding, trimming, or waveform editing.** Upload, validate, serve. Nothing more (§11.2).
- **Speaking/recording questions.** Students listen in v1; they do not record. That is a separate feature with its own storage, consent, and grading model.
- **Rich analytics dashboards.** Per-test and per-student score tables only.
- **Native mobile apps.** Responsive web only.
- **Real-time collaborative authoring.** One person edits at a time.

---

## 2. Tech stack

Fixed unless Thuong approves a change. Do not swap libraries silently.

| Concern | Choice | Notes |
|---|---|---|
| Build | Vite + React 19 + TypeScript (strict) | `pnpm` |
| Routing | React Router v7 (SPA mode) | Route-level code splitting via `lazy` |
| Server state | TanStack Query v5 | All API reads/writes go through query/mutation hooks |
| Client state | Zustand (minimal) | Auth session, test-taking engine, UI prefs only |
| Forms | react-hook-form + zod | Zod schemas are the single source of validation and TS types |
| Styling | Tailwind CSS v4 + shadcn/ui (neutral/zinc base) | See §12 |
| Icons | lucide-react | One icon set only |
| i18n | i18next + react-i18next | `vi` default, `en` secondary. All strings via `t()` from day one. |
| Dates | date-fns + date-fns-tz | `Asia/Ho_Chi_Minh` default |
| HTTP | native `fetch` wrapped in `src/lib/api/client.ts` | No axios |
| Google auth | Google Identity Services | Authorization Code + PKCE, exchanged server-side (§5.3) |
| Audio | native `<audio>` element + custom React controls | No wavesurfer.js, no howler. See §11.3 |
| Object storage | Cloudflare R2 (S3-compatible) | Audio and image assets. `aws-sdk-go-v2/s3` on the backend |
| Markdown | react-markdown + rehype-sanitize | Never `dangerouslySetInnerHTML` with server content |
| Drag & drop | `@dnd-kit/core` + `@dnd-kit/sortable` | Builder reordering only; lazy-loaded |
| Testing | Vitest + Testing Library; Playwright e2e; MSW mocks | See §14 |
| Lint/format | ESLint (typescript-eslint, react-hooks, jsx-a11y) + Prettier | CI fails on lint errors |

**Single SPA, three route trees.** `/admin/*`, `/app/*`, and a small public tree (`/login`, `/join/*`). Split at the route level so a student never downloads admin code and an anonymous visitor downloads neither.

---

## 3. Repository layout

```
web/src/
├─ app/                      # shell: providers, router, error boundaries, guards/
├─ features/
│  ├─ auth/
│  ├─ join/                  # NEW: public class-code join flow
│  ├─ tests/                 # admin: authoring
│  ├─ question-bank/
│  ├─ media/                 # NEW: upload widget + asset picker
│  ├─ assignments/
│  ├─ attempts/              # admin: monitoring + grading
│  ├─ integrity/
│  ├─ students/
│  ├─ classes/
│  ├─ take-test/             # student: the engine
│  └─ results/
├─ components/
│  ├─ ui/                    # shadcn primitives
│  └─ shared/
├─ layouts/                  # AdminLayout, StudentLayout, FocusLayout, PublicLayout
├─ lib/                      # api/, i18n/, utils/, config.ts
├─ hooks/  stores/  styles/  main.tsx
```

**Feature folder convention** (`src/features/<name>/`): `api.ts`, `schemas.ts`, `components/`, `pages/`, `store.ts` (optional), `index.ts` (public exports only). Features import each other only via `index.ts`. `components/shared` never imports from `features/`.

---

## 4. Naming and branding

Display name **Quizzivy**, sentence case. Package `quizzivy-web`; Go module `quizzivy`. DB `quizzivy`, schema `app` (not `public`, §13.2). R2 bucket `quizzivy-media`. The literal string appears once, in `.env`; everything else reads `config.appName`.

The organisation's own details are build-time config: `VITE_ORG_NAME`, `VITE_ORG_FRONT_DESK_PHONE` and `VITE_ORG_FRONT_DESK_HOURS` (`web/src/lib/config.ts`). An unset value hides what would show it, such as the front-desk row on `/forgot-password`. From R5 the admin's organisation settings take precedence, and these values stay as the fallback.

---

## 5. Auth

Signing in (§5.1–§5.3) says who a user is; the user's role (§1.1) says what they may do. The permission catalogue, the built-in grants, the Admin wildcard, the hidden keys and the pseudo-keys (`self`, `workspace.teacher`, `workspace.admin`) are in `docs/plan/70-redesign-overview.md` §4.1. `PermissionKey` in `api/openapi.yaml` equals the rows of `app.permissions`, and the server refuses to start when the database lacks a key it was built with.

- **Enforcement.** Every operation that requires the bearer token declares `x-permission` in `api/openapi.yaml`: a key, a pseudo-key, or a list met by any one of its keys (70 §4.2). `httpx.RequirePermission` enforces it on every request, after `RequireAuth`; the path is not the gate. The server refuses to start when an operation declares nothing, declares a value outside the catalogue, or declares one that does not belong to its path tree (`/teacher/*`, `/admin/*`, `/app/*`, `/auth/*`, `/me/*`).
- **Open operations** do not require the bearer token, declare no permission and pass untouched. There are seven: login, Google sign-in, refresh, logout, `POST /join/preview`, `GET /public/status` and the integrity beacon (`POST /app/attempts/:id/events`).
- **Refusals.** No valid token, an unknown or disabled user, or a token older than the user's session epoch (§5.2) → `401 UNAUTHORIZED` with `WWW-Authenticate`, which the single-flight refresh settles. An unmet permission → `403 FORBIDDEN`, with no detail.
- **The principal.** Each gated request resolves the caller's role, permissions, `disabled_at` and session epoch through an in-process cache that keeps a user for 10 seconds. A write on the same machine forgets the entry at once, so the change applies on that machine's next request and on any other within 10 seconds. Polling does not query the database on every request.
- **Scope is separate from permission.** Repositories filter by `access.Scope`: another teacher's id answers as a missing one does, and their rows never appear in a list or a count. A student is visible to every teacher who reaches them: a member of a class the teacher teaches, an account the teacher created, or an individual target of an assignment the teacher created. `scope.all`, which only the Admin holds, lifts the filter for a read or a write by id, and in the lists of classes, students and attempts. It does not lift it in the teacher workspace's six content lists (`listTests`, `listQuestions`, `listQuestionGroups`, `listMedia`, `listWordImports`, `listAssignments`, with their facets, tags and counts): those hold the caller's own rows for every caller, the Admin included, and for assignments that is those the caller created or that target a class the caller teaches (DG-53, T-R4.54). Both teacher home operations, `getDashboard` and `getTeacherSummary`, pass `.Own()`, so their figures cover only the caller's teaching, admins included; the repository retains its wide `scope.all` reading for the future Admin console. The Admin reaches another teacher's content by id and, from R5, in the Admin console.

### 5.1 Methods

Two, both landing on the same `users` row keyed by verified email:

1. **Email + password** — student accounts created by staff (`people.students.create`), and staff sign-in.
2. **Google Sign-In** — the primary path for students, and the **only** path for self-join (§6.3).

A user may have both. Linking rule: a Google sign-in whose ID token carries `email_verified: true` matching an existing user links to that user. An **unverified** email is rejected outright — no link, no create. This closes an account-takeover path.

### 5.2 Session model

- Access token: JWT, ~15 min, held **in memory** (Zustand). Never localStorage, never sessionStorage.
- Refresh token: opaque, rotating, `httpOnly; Secure; SameSite=Lax; Path=/auth` cookie. Stored server-side as a hash (§13.5).
- On 401 the client calls `POST /auth/refresh` once and retries. The refresh is single-flight.
- A **refused** refresh (401 or 403), or a second 401, ends the session.
  - A refusal ends only the session its request was sent under. When the store's access token is no longer the one the refused request carried (a sign-in or a refresh replaced it, or the user signed out), nothing is expired or cleared, and the 401 returns to its caller.
  - A user who was signed in keeps the page: the access token is dropped, the user stays, and the "Vui lòng đăng nhập lại" overlay covers the page (§9).
    - A sign-out closes that overlay if it was raised while the sign-out was on its way.
    - After the user's own sign-out, that tab's guard sends a visitor to
      `/login` without `next` until its next sign-in. The marker is in memory,
      so a reload does not preserve it. An expired session keeps its return
      path. A second tab left open after sign-out can still offer `next`
      through its expired-session card; that path remains unresolved.
  - "Đăng nhập" clears the session, keeps the answer drafts, and goes to `/login?next=<path>`.
  - While that overlay is up, a 401 returns to its caller without another refresh.
- A refresh that **fails without refusing** (a network failure, a 503 or another 5xx) is not a lost session. Waiting requests fail retryably and nobody is signed out.
- App load: `GET /auth/me`, under the boot splash (§9).
  - 401 → signed out, unless a sign-in finished while the request was out.
  - A network failure (no HTTP response at all, on `/auth/me` or on the refresh it triggers) → the splash's offline state, which retries.
  - Any other failure (a 5xx, a 503 other than `MAINTENANCE`, or a 4xx other than 401 and 403) → on `/` and the signed-in routes, the unexpected-error page with the server's `requestId`; on the public routes the splash fades out and the page renders as usual.
  - 503 `MAINTENANCE` → the maintenance overlay.
- Every request sends `Accept-Language` set to the app's locale, so server messages match the UI rather than the browser.
- Reuse detection: presenting an already-rotated token revokes the whole family and forces re-login.
- **Races.** A sign-in stores its refresh token under a lock on the user and compares the account with what it read: if the password, the session epoch or the disabled state changed meanwhile, a password sign-in answers `INVALID_CREDENTIALS` and a Google sign-in reads the account again. A rotation and a logout take the lock an update of the user takes, so one user's rotations, reuse detections and logouts run one at a time and none overlaps a reset, a disable or a password change: a session never survives the event that should have ended it.
- **Session epoch.** `users.session_epoch` ends live sessions at once. Sign-in, Google sign-in and refresh put the user's current epoch in the access token (claim `sep`; a token without it reads as 0), and a token older than the user's epoch is refused (§5). In R2 two writes move it, each in the same transaction that revokes every refresh family the student has: disabling a student (`updateStudent` with `disabled: true`) and resetting a student's password (`resetStudentPassword`). Enabling the account again does not bring those sessions back. R5's sign-out-everywhere, role changes and set-password links move it too (70 §4.2).
- **A disabled user is refused on the next request.** Every gated request reads `disabled_at` through the principal cache (§5): at once on the machine that made the change, within 10 seconds on any other. Refresh refuses a disabled user and revokes the family, so the client's refresh ends the session.

### 5.3 Google flow

1. Frontend loads GIS and requests an **authorization code** (not implicit ID-token) with PKCE.
2. `POST /auth/google` `{ code, codeVerifier, redirectUri, joinCode? }`.
3. Backend exchanges with Google, verifies the ID token (`iss`, `aud`, `exp`, signature via JWKS), reads `sub`, `email`, `email_verified`, `name`, `picture`.
4. Resolution order:
   - identity exists → log in;
   - verified email matches a user → link identity, log in;
   - no match **and** a valid `joinCode` is present → create account + enroll (§6.3);
   - no match, no join code → `403 ACCOUNT_NOT_PROVISIONED`.
5. Returns `{ accessToken, user }`, where `user` is the `CurrentUser` (§7), and sets the refresh cookie.

`VITE_GOOGLE_CLIENT_ID` is public config. The client secret lives only in the backend.

### 5.4 Guards and edge cases

- `RequireSession` → `/login?next=<path>` when unauthenticated.
- The web guards read the signed-in user's `workspaces` and `permissions` (`CurrentUser`, §7), never `role`, through `features/auth/permissions.ts` (`can` and `useCan`, `hasWorkspace` and `useWorkspace`). They decide what the SPA shows; the server enforces the permissions on every request (§5).
  - `TeacherWorkspace` holds `/admin/*`, the teacher console until R4, for the `teacher` workspace. Anyone else gets a **403 page**, not a redirect (a redirect hides the misconfiguration).
  - `StudentArea` holds `/app/*` for the `app` workspace. A user without it who has the `teacher` or `admin` workspace is redirected to `/admin`; a user with neither gets the 403 page.
  - `homePathFor` sends the `teacher` or `admin` workspace to `/admin` and anyone else to `/app`. An Admin with "Take tests" turned on (R5) lands on `/admin` and may also open `/app`.
  - `learnsOnly` (the student app is the user's only workspace) keeps v0.7.0's rules on `/join` and the Settings role label (§6.2).
- `mustChangePassword: true` → all routes redirect to `/change-password`. Google-only users never hit this.
- Logout: `POST /auth/logout` (revokes refresh token), clear store, `queryClient.clear()`, → `/login`.
- The frontend applies accepted server locale/theme/display zone/larger
  text over browser mirrors, with omitted defaults `vi`/`light`/`Asia/Ho_Chi_Minh`/false
  and no default-materialization PATCH. Anonymous choices remain local. Authenticated
  controls preview pending changes, restore acknowledged presentation on failure and
  retain an explicit Retry. A browser-unsupported account zone retains its exact
  server value, reports compatibility and temporarily presents Vietnam time without
  saving that fallback. Actor departure invalidates stale asynchronous effects.
- The frontend orders refresh/login/Google-login/logout cookie sends and global
  draft cleanup before replacement admission. Refresh remains single-flight. A20s
  transport/admission UI deadline and10s cleanup deadline show pending status without
  abandoning raw ownership; late timed-out login never auto-admits. Permanent hangs
  may keep admission closed. Cross-tab ordering is not promised by this design.
- Password reset in v1: a holder of `people.students.reset_password` sets a temporary password from the student detail page, under the shared-student rule below. No self-service email flow (§17.1).
- New passwords have three rules:
  - at least 8 characters;
  - at least one number or symbol (`[\p{N}\p{P}\p{S}]`);
  - different from the current password (the temporary one, while `mustChangePassword` is set).

  The contract enforces the first two (`400 VALIDATION_FAILED`) and the server the third (`400 PASSWORD_UNCHANGED`). Existing passwords are never re-validated. `/change-password` shows the rules and a strength meter.
- `/forgot-password` makes no request. It tells a Google user that no password is needed, and everyone else to ask the front desk (§4, when configured) or their teacher.
- **Guards that are not permissions** (70 §4.3), enforced by the server. A guard on a student runs after the student is found in the caller's scope, so another teacher's student answers `404`, never `403`.
  - **Strict student targets.** Every student read and write (the Students list and record, update, reset and delete, class membership, individual assignment targets, the dashboard's counts and `maintenance anonymize-student`) accepts only a role in `app.student_like_roles`: the built-in Student, or a custom role granted nothing but `learning.take_tests`. An Admin with "Take tests" turned on is never a student target. For an assignment's individual targets the rule applies to a target being added: a student who is already a target stays on the assignment when they are disabled or their role stops being student-like, so the teacher can still publish, edit and close it (#210).
  - **The subset rule.** `updateStudent`, `resetStudentPassword` and `deleteUser` need the target's permissions, without `learning.take_tests`, to be a subset of the caller's (`access.CanActOn`); otherwise `403 FORBIDDEN`. 70 §4.3 lists the R5 operations it will also cover.
  - **Disabling.** `updateStudent` with `disabled`, either value, also needs `people.users.manage`; otherwise `403 FORBIDDEN`.
  - **Shared students.** A reset, or a new `email`, by a caller without `people.users.manage` needs a student no one else reaches: every class they are in, archived ones included, is the caller's; no other account created them; no other account's assignment targets them individually; and a student in no class was created by the caller. Otherwise `403 STUDENT_SHARED`, and nothing is written. Sending the address the student already has is not a change. A new address could take the account over through Google sign-in, which links by email (§5.1).
  - **The last Admin.** The `users_last_admin` trigger refuses any demotion, disable or delete that would leave no active Admin, whatever the path. No R2 operation can reach an Admin account; `409 LAST_ADMIN` arrives with R5.
  - The built-in Student role cannot lose `learning.take_tests`.

### 5.5 API reference session

- `/docs` and `/docs/openapi.json` are served by the API beside the contract and open only to a holder of `system.api_reference`, a hidden key only the Admin holds (70 §4.1). A browser navigating there sends no bearer token, so they check a separate cookie instead.
- `POST /admin/docs-session` (`openDocsSession`, `x-permission: system.api_reference`, rate-limited per signed-in user, §6.5) sets `quizzivy_docs`: `Path=/docs; HttpOnly; Secure; SameSite=Strict; Max-Age=900`. Its value is a JWT for the `docs` audience that carries the caller's session epoch, signed with a key derived from the access-token key: a docs token is never accepted as an access token, and an access token never opens the docs.
- The gate resolves the cookie's user through the same principal cache as the API (§5). A missing, tampered or expired cookie, an unknown or disabled user, or a cookie older than the user's session epoch → `401`; a role without `system.api_reference` → `403`, both in the error envelope with no page or contract leaked. `POST /auth/logout` clears `quizzivy_docs` together with the refresh cookie, so signing out also ends an open docs session.
- The SPA's admin settings open the reference: the new tab is opened synchronously in the click, `opener` is cleared, and only then is the session requested and the tab pointed at `/docs`.
- `DOCS_PUBLIC=true` skips only the cookie check, for local development. The server refuses to start with it when `APP_ENV=production`; SRI, the page's CSP and the rate limit apply everywhere.

---

## 6. Class join codes (self-signup)

### 6.1 Model

A join code belongs to a class and is a **bearer secret**: whoever holds it can enrol. Treat it accordingly.

- Format: 8 characters from an unambiguous alphabet `ABCDEFGHJKLMNPQRSTUVWXYZ23456789` (no `0/O`, `1/I`). Displayed grouped `XXXX-XXXX`; accepted with or without the dash, case-insensitive.
- Generated from a CSPRNG. Never sequential, never derived from the class ID.
- Per-code controls: `expires_at` (default 30 days), `max_uses` (default null = unlimited), `uses_count`, `revoked_at`.
- One **active** code per class at a time. Rotating issues a new code and revokes the old one; previously enrolled students are unaffected.
- Stored encrypted, so it can be read back for its teacher and for admins (D5), and found by a keyed hash (§6.5). A code issued from v0.8.0 on is sealed under the server's `JOIN_CODE_KEY`, which the database never holds (§13.3). A code issued before v0.8.0 is held only as its SHA-256: it still redeems but cannot be read back, and R4 rotates every such code. At each start-up from v0.10.0 the API replaces every such code that is neither revoked nor expired with a sealed one that keeps its expiry and use cap and starts unused, leaves the class's self-join setting as it is, audits the change as the System and tells the class's teacher once in the app; the old code then answers as any replaced code does, "revoked" where self-join is open and as an unknown code where it is closed or the class archived.
- The key is standard base64 of exactly 32 random bytes, kept as a Fly secret with an offline copy. The API refuses to start without it and never logs it. HKDF-SHA256 derives three values from it, each under its own label: the AES-256-GCM key, the HMAC-SHA256 lookup key and a 16-bit key id; a key whose id derives to 0 is refused.
- Rotating the key: one deploy sets `JOIN_CODE_KEY_PREVIOUS` to the old key and `JOIN_CODE_KEY` to the new one, and codes under either key redeem and read back. `maintenance rekey-join-codes -apply` then re-seals every code under the old key with the new one, and the old key is unset (`docs/setup/operations.md`, "Join-code key"). A lost key leaves every code sealed under it unreadable and unredeemable: rotate every class's code.

### 6.2 Student flow

```
/join                → enter code; at 8 valid characters the class is previewed inline
/join/:code          → deep link (QR / message): code prefilled and previewed
                     → preview card: class name + teacher, "Tham gia {class}"
                     → signed out: sign in (Google → POST /auth/google {code, joinCode};
                       or password → POST /app/classes/join after sign-in)
                     → signed in: POST /app/classes/join at once
                     → Joined: "Bạn đã vào lớp {class}", "Đến lớp của tôi" → /app/classes
/join/:code/confirm  → redirects to /join/:code
/app, /app/classes   → "Tham gia lớp" opens the Join dialog for a signed-in student:
                       the same field and preview, then POST /app/classes/join
```

The preview exists so the student sees **which class they are joining** before authenticating. Never create an account and enrol in one blind tap.

- **Typing.** The field uppercases, drops spaces and dashes, and shows a dash after four characters. A character the alphabet never uses (`0`, `O`, `1`, `I`) marks the field invalid and sends nothing.
- **Lookup.** One lookup runs per complete code, after a 250 ms debounce. Every failure (invalid, expired, exhausted, revoked) shows the same message, and nothing about any class reaches the page. An answer is remembered for 30 seconds, a refusal as much as a class, so retyping a code sends nothing. An edit does not abandon a lookup already out, which the server has counted. A lookup that could not be made is sent again only when the code is retyped.
- **Signed out.** The join context (code, class name, teacher) is kept for 30 minutes in `sessionStorage` across sign-in. A must-change-password account goes through `/change-password` first, and the join then continues.
- **In the app.** A signed-in student joins without leaving the page. "Tham gia lớp" on Classes, and on Home for a student in no class with nothing assigned, opens the Join dialog: the same field and lookup, the class found (name and teacher) and "Tham gia", which calls `POST /app/classes/join`. On success the class and assignment lists refetch, a toast says "Bạn đã vào lớp {class}" and the dialog closes; it waits for the class list first, for at most three seconds. A 429 shows the server's message with the wait from `Retry-After`. The typed code and the state of a join are forgotten when the dialog closes. `/join` and `/join/:code` stay for links and QR codes.
- **Already a member.** The dialog compares the preview's `classId` with the student's own classes (`GET /app/classes`): a member reads "Bạn đã ở trong lớp này rồi." and cannot join. The server checks the code's state before membership, so a member who types a dead code reads the failure message. The enrolment is idempotent: a member's join answers 200 with the class and counts no use of the code, so on `/join` it reads as joined.
- **Outcomes.** On `/join` an enrolment failure shows its message on the Joined page. In the dialog a code that died after the preview shows the failure message, and any other failure "Không thể tham gia lớp. Vui lòng thử lại." An account that is not a student is told that joining is for student accounts.

### 6.3 Why Google-only for self-join

Self-signup with email + password requires a verified email, which requires transactional email infrastructure (provider, domain auth, deliverability, bounce handling) — a real dependency v1 does not have. Google hands us a verified email for free and is one tap on a phone, which is exactly the self-join case.

So: **self-join requires Google.** Password accounts remain staff-created. If Thuong later wants password self-signup, add an email provider and a `email_verifications` table; the join flow itself does not change. This is a v1 scoping decision, not a permanent constraint (§17.1).

### 6.4 Teacher controls

A class's code and members are managed by its teacher (`classes.teacher_id`), and by an Admin through `scope.all` (plan 70 §4.1). Another teacher's class answers 404, as a missing class does. The code operations and member changes need `teaching.classes.write`; the member list needs `people.students.read`.

On `/admin/classes/:id` (R4 moves the screen to `/teacher/*`):
- Show the active code, with copy button, QR code, expiry, and uses count. `getJoinCode` reads the code back in full (§6.1); until R4 the screen shows it once when issued and then as its hint.
- **Rotate code** (confirm dialog: "Mã cũ sẽ ngừng hoạt động ngay").
- **Disable self-join** toggle — revokes the code without issuing a new one.
- Member list shows `joined_via` (`admin` / `join_code`) and `joined_at`, so the teacher can spot unexpected enrolments.
- Remove member (revokes access; attempts are retained, not deleted).

### 6.5 Security requirements — non-optional

A leaked code lets a stranger into the class. Mitigations, all required:

- **Rate limit** the sign-in and join operations, and return `429` with `Retry-After`. Without this, an 8-char code space is still worth probing at scale. The budgets let a class of 40 behind one school address make three tries at each step (T-R2.15):
  - `POST /join/preview`, `POST /app/classes/join` and `POST /auth/google`: 120/min and 600/h per address; 200/h per code, from any address. Google sign-in counts a code only when the body carries a `joinCode`.
  - `POST /auth/login`: 120/min and 600/h per address; 10/min per address and email; 20/h per email.
  - `POST /auth/refresh` and `POST /auth/logout`: 120/min and 1,200/h per address.
  - A code is counted after normalization, so respelling it buys no fresh allowance. Guessing stays at 600 codes an hour per address on each operation that checks one.
  - The signed-in operations that hand out a credential are limited per signed-in user, after the permission check and wherever the user connects from, so staff behind one address do not share a budget (T-R4.53): `resetStudentPassword` 5/min and 30/h, `createStudent` 30/min and 300/h, `rotateJoinCode` 10/min and 60/h, `getJoinCode` 60/min and 600/h, `openDocsSession` 5/min and 30/h. A caller with no session or without the permission is answered `401` or `403` and spends no budget.
- **Bounded bodies.** The four operations with a bucket keyed on a body field (login, Google sign-in, the preview and the in-app join) accept at most 8 KiB and answer `413` before any handler, so padding cannot hide the key from its bucket.
- **Constant-time comparison** on code lookup; look up by a keyed hash (HMAC-SHA256) of the normalized code, not by plaintext equality. A code issued before v0.8.0 is found by its SHA-256 until R4 rotates it.
- No audit row or log line carries a code, its ciphertext or its hash.
- `POST /join/preview` returns exactly three fields: the class's id, its name and the teacher's display name. Never student names, counts, a member list or assignment titles. The id is a uuidv7: it reveals only when the class was created and grants nothing by itself. The Join dialog uses it to tell a student who is already a member (§6.2). It is an unauthenticated endpoint.
- Log every enrolment (`class_id`, `user_id`, `ip`, `user_agent`, `at`) to the audit table.
- Expiry defaults to 30 days precisely so an abandoned code stops working on its own.

**Deliberately not built:** an approval queue. For a small practice, rotate-and-remove is sufficient and one less state machine. Revisit if enrolment volume grows (§17.2).

---

## 7. Domain model (frontend types)

Mirrors §13. IDs are UUID strings; timestamps ISO 8601 UTC.

```ts
type Role = 'admin' | 'student';        // legacy: 'student' for a student-like role (§5.4), 'admin' for any other; v0.9.1 (T-R3.1) removes it

interface User {
  id; email; fullName; role: Role;
  displayName?: string;
  avatarUrl?: string;
  hasPassword: boolean;                 // false for Google-only accounts
  linkedProviders: ('google')[];
  mustChangePassword: boolean;
  createdAt;
}

type PermissionKey = 'content.tests.write' | /* … */ 'system.leads';  // the 22 keys of 70 §4.1, in catalogue order
type Workspace = 'teacher' | 'admin' | 'app';

interface CurrentUser extends User {    // the signed-in user only; a payload about anyone else carries User
  permissions: PermissionKey[];         // the role's effective keys, in catalogue order
  workspaces: Workspace[];              // derived from permissions; the web guards read these (§5.4)
  phone?: string;
  locale?: 'vi' | 'en';
  timeZone?: string;
  preferences?: UserPreferences;
}

interface UserPreferences {
  theme?: 'light' | 'dark' | 'system';
  compactTables?: boolean;
  largerTestText?: boolean;
  assignmentDefaults?: {
    durationMinutes?: number;
    shuffleQuestions?: boolean;
    showScore?: boolean;
    blockCopyPaste?: boolean;
    requireFullscreen?: boolean;
  };
}

interface Class {
  id; name; description?;
  studentCount: number;
  selfJoinEnabled: boolean;
  joinCode?: { hint: string; expiresAt: string; maxUses: number | null; usesCount: number } | null; // teacher responses only; the code itself comes from getJoinCode (§6.4)
  createdAt;
}

interface MediaAsset {
  id; kind: 'image' | 'audio';
  url: string;                          // short-lived signed URL
  mimeType: string; bytes: number;
  durationMs?: number;                  // audio only
  originalFilename: string;
}

type QuestionType =
  | 'single_choice' | 'multiple_choice' | 'true_false'
  | 'fill_blank' | 'short_answer';

type QuestionLevel = 'pre_a1' | 'a1' | 'a2' | 'b1' | 'b2' | 'c1' | 'c2';
type QuestionSkill = 'grammar' | 'vocabulary' | 'reading' | 'listening' | 'writing' | 'speaking';

interface AudioPolicy {
  maxPlays: number | null;              // null = unlimited
  allowSeek: boolean;                   // default false for listening
  showTranscriptAfterSubmit: boolean;
}

interface Question {
  id; type: QuestionType;
  level: QuestionLevel | null;
  skill: QuestionSkill | null;
  prompt: string;                       // Markdown, rendered sanitized
  media?: MediaAsset;
  mediaAlt?: string;                    // image only; frozen at publish; read to students by a screen reader
  audio?: AudioPolicy;                  // present iff media.kind === 'audio'
  transcript?: string;                  // admin-authored; student sees it only per policy
  options?: { id; text; isCorrect: boolean }[];
  blanks?: { id; ordinal: number; acceptedAnswers: string[]; caseSensitive: boolean }[];
  points: number;
  explanation?: string;
  sampleAnswer?: string;                // short_answer; ADMIN ONLY, never in a student payload
  tags: string[];
}

interface Section { id; title; instructions?; questionIds: string[] }

interface Test {
  id; title; description?;
  sections: Section[];
  totalPoints: number;                  // server-computed
  status: 'draft' | 'published' | 'archived';
  currentVersion: number;
  createdAt; updatedAt;
}

interface IntegrityPolicy {
  requireFullscreen: boolean;
  blockCopyPaste: boolean;
  maxFocusLoss: number;                 // -1 = none allowed; 0 = unlimited
  onLimitExceeded: 'warn' | 'flag' | 'auto_submit';
}

interface Assignment {
  id; testId; testVersionId; testVersion: number;
  targets: { classIds: string[]; studentIds: string[] };
  window: { opensAt: string; closesAt: string };
  durationMinutes: number;              // server-enforced
  maxAttempts: number;
  shuffleQuestions: boolean; shuffleOptions: boolean;
  review: {
    showScore: boolean; showCorrectAnswers: boolean; showExplanations: boolean;
    release: 'on_submit' | 'after_close';   // when a result is released; on a write, omitted keeps the stored value
    showClassAverage: boolean;              // whether the result may carry classAverage; omitted on a write keeps the stored value
  };
  integrity: IntegrityPolicy;
  studentNote: string | null;           // the teacher's note on the Test intro; plain text, trimmed, at most 500 characters (counted before trimming on a write); null clears
  status: 'scheduled' | 'open' | 'closed';
}

type AttemptStatus = 'in_progress' | 'submitted' | 'timed_out' | 'graded' | 'voided';

interface Attempt {
  id; assignmentId; studentId; testVersionId; attemptNo: number;
  status: AttemptStatus;
  startedAt; deadlineAt; submittedAt?; gradedAt?;
  answers: Record<string, Answer>;
  audioPlays: Record<string /*questionId*/, number>;   // server-authoritative (§11.4)
  score?: { earned: number; total: number; pendingManual: number };
  integrity?: { focusLossCount: number; flagged: boolean };
}

type Answer =
  | { type: 'choice'; optionIds: string[] }
  | { type: 'true_false'; value: boolean }
  | { type: 'fill_blank'; values: Record<string, string> }
  | { type: 'text'; value: string };
```

**Invariant:** an attempt references `testVersionId`. Editing a published test creates a new version; in-flight attempts keep rendering the version they started with. In the student flow, test content is fetched **only** via `GET /app/attempts/:id`.

---

### 7.1 Word milestone content contract (foundation)

Every question write and publication uses the same interaction invariants:
nonempty prompt and option text, supported type, choice/key cardinality,
nonempty accepted blank answers, unique blank ordinals matching the prompt,
media/audio consistency and content projection validity. Single-choice and
true/false require exactly one correct option; true/false has exactly two options.
Points must be greater than zero, no greater than 999999.99 and have no more than
two decimal places. Do not silently round source or teacher-entered points.
Publication requires a nonempty exam and an exact total within numeric(8,2).

Teacher question metadata is nullable: level is `pre_a1`, `a1`, `a2`, `b1`,
`b2`, `c1` or `c2`; skill is grammar, vocabulary, reading, listening, writing
or speaking. Bank and shared-group authoring requests cap choice arrays at eight;
existing stored arrays remain readable, copyable, publishable and restorable.
The stored editable contract is separate from the capped request contract.
Import assessment also refuses an included question above the authoring cap;
excluded questions do not prevent commit. Publishing freezes both metadata
fields, including grouped questions, and restoring a version preserves them.
Draft test-list skills are distinct, sorted and omit null values.

Question-bank filtering is OR within level or skill and AND across dimensions.
`tagMatch=any|all` preserves today's any default. Each metadata facet omits only
its own dimension while retaining the other filters and normal owner scope;
tag enumeration omits tag filtering. These reads do not promise a new atomic
cross-query snapshot or measured performance.

`ContentDocument` is an application-owned discriminated union: `legacy_markdown_v1`
retains the exact historical Markdown string; `semantic_v1` contains typed
paragraphs, three heading levels, lists, tables, asset references and inline
text/marks, safe links, line breaks and stable gaps. Its normative structural
schema is in `api/openapi.yaml`; aggregate and cross-node rules are described in
[the content contract](plan/19-content-contract.md) and enforced by both domain
validators. Unknown fields, answer metadata, raw HTML nodes and editor-specific
JSON are refused. Text is not rewritten or Unicode-normalized during validation.

The general content components and value object provide the foundation for the
inline option and question prose profiles below. Each new profile needs matching
persistence, publication, restoration and delivery before enabling its writes.
Media/group integration is pending.
The renderer never resolves an asset ID
without an authorized media binding. Existing question payloads above remain the
active contract with the additive option and prose rollouts below.

**Inline option rollout (W-05a).** Choice options may carry optional `content`
using the `OptionContent` subset: exactly one semantic paragraph of text marks
and line breaks. Existing option text is literal, not Markdown, and stays exact.
`text` is the deterministic plain projection when `content` is present. Preserve
both fields in bank copies, version snapshots, restored drafts, previews,
attempts, results and teacher review. The document cannot contain links, gaps,
assets, source metadata or answer keys; grading remains in normalized columns.
An omitted document on update may retain existing formatting only when the
current option ID and text match. Stale or changed legacy writes fail atomically;
explicit `content: null` removes formatting. `VITE_RICH_OPTION_EDITOR=true`
enables the pilot affordance to format plain options; readers and editing
existing rich options remain available. This is not an import release or final
editor acceptance. Grouped content and shared audio remain on their existing paths until their
separate integration gates pass.

**Question prose rollout (W-05b).** Optional `promptContent` and
`explanationContent` support semantic paragraphs, headings, lists and tables with
text marks, breaks and safe links. `QuestionContent` excludes assets and gaps;
`QuestionPromptContent` additionally allows bound gaps for fill-blank questions. `prompt`/`explanation`
are exact plain projections when their document exists; otherwise they retain
legacy Markdown semantics. Rich fill-blank prompts require a bijection between gap node IDs and blank
`gapId` values. Labels never bind answers. Legacy `{{n}}` prompts retain ordinal
binding; rich explanations support all five types.
Snapshots, restoration, bank duplication and all relevant readers preserve these
fields. Active attempts and learner previews never contain explanations; results
release both explanation fields only when `review.showExplanations` permits it.
Legacy writes omitting a rich field must leave its companion string unchanged;
otherwise the update fails atomically. Explicit null clears a document. A
field is written in the form it is stored in: a document in the rich editor, a
Markdown string in the Markdown editor; a field with no text is rich text,
except that the builder's and the group composer's starter prompt is stored as
Markdown and opens as Markdown until T-R4.31b and T-R4.34 create it as rich
text. The
form changes only through "Switch to Markdown" (which keeps text, bold, italic,
strikethrough, headings, lists, tables and links, and drops underline,
superscript and subscript) and "Apply conversion", each after its confirmation.
"Markdown" is unavailable while a fill-in-the-blank prompt holds a gap, because
the gaps bind the accepted answers. Markdown, for the teacher and the student,
reads GitHub's tables and `~~strikethrough~~` and no other GFM syntax.
Conversion from Markdown is explicit, validates the supported subset
and refuses unsupported structures (for example a table's column alignment,
raw HTML, code, images, quotations and links that are not HTTPS) without
changing the original.

Formatted clipboard content follows the same principle: parse locally into the
allowlisted semantic vocabulary, preview the complete resulting field, and apply
only after confirmation. Preserve supported marks, list starts, table spans and
safe links; adapt fonts, colors and spacing to the application's design. Reject
files, active/hidden content, unbound gaps, unsupported styles or incomplete
structure as one paste. The one exception is an image inside copied content
(`img`, `picture`, `svg`, `video`, `canvas`): it is left out without being
loaded, and the preview says how many were left out; a paste holding nothing
else is refused like a file. No fallback to text without the teacher explicitly using
plain-text paste. The converter cannot infer answer keys from visual formatting.
Cancel/stale preview leaves current edits unchanged; one undo reverses insertion.
Field profiles and aggregate budgets apply to the whole resulting document.

### 7.2 Word milestone group ordering (approved, not yet enabled)

For the new group-aware delivery version, `shuffleQuestions` shuffles complete
question groups and standalone questions as units within each section. Sections
retain their authored order. Members of a shared passage/listening/cloze group
retain their authored order even when the groups move. With shuffle disabled,
all units retain authored order. These rules were approved by Thuong as D-03.

Snapshot group membership and member order at publication. Persist the delivery
algorithm version with the assigned snapshot/attempt so reload, takeover and
restoration cannot choose a new deal; preserve stable answer IDs. Historical
versions/attempts continue using their existing section-scoped algorithm and
seeds. `test_versions.delivery_version` stores this marker: existing standalone
versions default to `section_v1`; prerelease group snapshots are classified as
`group_v1` without changing their content or identities. New publications
explicitly write `group_v1`, whose standalone ordering matches `section_v1`.
Rollback refuses to remove the marker while group-aware snapshots exist.
A choice member with semantic references to its option labels declares
`optionOrder: fixed`; assignment with option shuffling is rejected for that member.
Other members use `shuffle`; this never changes the member order inside a group.

### 7.3 Word milestone group graph (contract foundation, not yet enabled)

`QuestionGroup` owns ordered members and materials, optional rich instructions,
and explicit recording bindings. A material's gap targets either a choice member
or the stable `blankGapId` of a rich fill-blank member. Printed labels and mutable
answer-row IDs cannot bind responses. Each material gap has exactly one target;
a response appears only once across that group's material gaps. Gap names are
local to their material or question. Empty groups are valid drafts but cannot
publish; existing sections are not reinterpreted as groups.

The complete resolved context is bounded to 200 members, 16 materials, 16 shared
recordings and 4 MiB, with per-document content limits still enforced. Materials
may repeat an audio asset but resolve it to one recording binding in the group;
the same asset cannot also grant a per-question allowance inside that group.
Distinct member recordings can keep their existing per-question policy. Asset
kind, existence, authorization and deletion locks remain required at persistence.

Copying remaps group, question, material, answer, gap and recording identities,
preserving all grading data, labels, order and content. Immutable media IDs are
reused through new protected bindings. The teacher-only group API and independent
bank editor and mixed builder are available on the milestone branch; release
still requires import and pilot acceptance gates. Relational ownership uses a nullable section owner for each group
(null means an independent bank group) and explicit ownership/order on its member questions.
Section units distinguish standalone questions from groups. Material gaps and
media references have relational bindings; cross-group response/playback links
are rejected. Owned children cannot be archived separately. Restrictive owner
foreign keys require explicit whole-graph cleanup; existing section deletion
cannot leave questions detached from their context. Internal graph creation and
copy materialization are atomic with their audits. Legacy question operations
hide or refuse owned children; active and archived groups protect referenced
media under the same asset lock used by deletion. Full-group editing checks an
aggregate revision; section groups also check the enclosing test revision. Edits
replace content and member order atomically. Bank archive/restore/delete checks
the same revision; permanent deletion requires archival and keeps audit history.
Removing a section-owned group deletes its complete draft graph and compacts unit
order. Copy materialization checks an observed source revision and creates new
editable identities. Draft/snapshot/delivery readers preserve the complete graph.
The builder inserts bank groups as complete independent copies into an explicitly
chosen section, and can save the current group graph as an independent bank copy.

Draft totals and tag filters include owned members alongside standalone questions.
Listening counts count each question once when it has its own audio or a shared
group recording. Independent bank groups are not part of a test's totals. The
legacy question-only whole-outline writer refuses a draft containing groups with
`GROUP_OUTLINE_REQUIRED` before changing metadata or structure; metadata-only
updates remain supported. The group-aware writer accepts complete mixed units and
moves whole groups transactionally. The builder keeps new section client identities
until server IDs arrive, preserving edits and order during acknowledgement. The
outline distinguishes group and standalone IDs, moves empty groups as units and
keeps continuous learner numbering across member questions. Group and outline
writes share a serialized enclosing-test revision. Only acknowledged own moves
advance the active group's revision baseline; external conflicts remain visible.
Preview and publication flush pending content and outline writes. Group findings
open the owning group editor instead of the standalone question endpoint.

The independent group bank supports search, remembered URL filters, newest-first
updates, inline/menu duplication and bulk archive/restore/permanent deletion.
Copies keep the teacher on the list with a small indicator. The group editor has
one active rich editor, ordered materials/members, explicit gap-target selectors,
authorized image/audio picking and upload, HTTPS links and shared recording policy.
Historical Markdown material remains editable. Material preview preserves undo
history; full desktop/phone preview uses the learner renderer without answer keys,
explanations or transcripts. Archived groups must be restored before editing.

Full-group autosave serializes writes with the observed revision, keeps newer
keystrokes when an earlier write is acknowledged, and flushes before route exit.
Incomplete edits remain locally recoverable. Quota/storage errors are visible;
only a server acknowledgement marks the matching revision saved. Recovery from a
stale revision never overwrites current data: the teacher can edit the recovered
content and save a fully remapped independent bank copy.

Publication freezes the entire group into version-owned rows, including ordered
units, member order, materials, stable gap targets and explicit recording policy.
Frozen context points only to frozen question/blank identities and immutable media;
editing or deleting a source group cannot alter it. The app role cannot update
these new snapshot graph rows. Deleting an unreferenced version removes its owned
graph together. Restoring a version replaces the current owned draft graph with
fresh editable copies and remaps both ends of material/blank gap links. Duplicating
a draft preserves mixed unit order and independently copies every group; standalone
bank references retain their existing duplication semantics. Permanent test deletion
removes its owned draft groups after the assigned-version reference checks pass.
Published previews resolve the selected default when no version is specified,
read one coherent frozen version, and expose ordered sections/groups plus safe
material bindings. Their projection never selects answer keys or transcripts;
media URLs come only from relational bindings on that version. Group material
appears before its first member, and material gaps link to the corresponding
question. Teacher playback consumes no student allowance. Desktop and 320px phone
preview modes run inside the existing admin shell (minimum supported width 768px).
Existing version questions and historical attempts are unchanged.

Archived tests must be restored before creating a draft from a version or changing
their default version. Both flat and grouped tests return `TEST_ARCHIVED` without
rewriting content. An unused, non-current version may still be deleted while its
parent is archived; existing reference checks continue protecting assigned history.

Start, resume and read-attempt responses include the same safe frozen group
projection. Membership comes from the attempt's version, regardless of the test's
current default. The tests module owns the reader, invoked through an application
port after attempt authorization. Shared media is reachable only through protected
relational bindings on a version the learner has an attempt on; assignment
targeting alone grants no media access. A missing context reader fails explicitly
instead of serving grouped questions without their materials. Learner material
is drawn by the renderer the teacher's preview shares: in a passage pane beside
the question from 768px, and below 768px behind the "Ngữ liệu | Câu n" switcher,
which shows one pane at a time, the question first (§9). Shared recordings are
drawn in the question pane at every width, never in the passage, so their
controls show whenever the question does; below 768px they are out of view while
the passage is showing, and stay mounted. Stable gap targets navigate to the
question or blank input without losing pending answers. Group players stay mounted
across child navigation.

Learner results retain the frozen material when filtering questions and expose
only transcripts released by each recording's `showTranscriptAfterSubmit` flag.
The score, answer-key and explanation flags do not override that release policy.
Teacher paper review includes the complete shared transcript and that attempt's
recording counts. Grading by question includes only the selected question's
group, without an aggregate playback count across students. Material links focus
the matching question where navigation is available. Review playback is unlimited
and does not append attempt playback events or change recorded counts. Legacy
flat results/reviews keep their existing payload and require no group reader.


---

## 8. Screens & routes — admin (`/admin/*`)

`AdminLayout`: sidebar + top bar. Collapsible sidebar ≤1280px. Minimum supported width 768px.

| Route | Screen | Key behaviour |
|---|---|---|
| `/admin` | Dashboard | Legacy figures remain; the teacher home API also supplies live taking counts, calendar submissions, today's assignments and recent activity in the caller's own teaching scope. |
| `/admin/tests` | Tests list | Title, status, #questions, total points, updated. Filter by status. Create / duplicate / archive; permanent delete for unreferenced archived tests. |
| `/admin/tests/new`, `/admin/tests/:id/edit` | Test builder | Left: outline with drag-to-reorder. Right: question editor incl. **audio attach** (§11.1). Autosave debounced 1.5s. **Publish** validates: `points > 0`; choice questions have ≥1 correct option; `fill_blank` has ≥1 accepted answer per blank; audio questions have a processed asset; no empty sections. |
| `/admin/tests/:id` | Test detail | Student-eye preview of any version, with history in the shared right sidebar. Restore a snapshot into a draft, select the default for future assignments, or delete an unused non-default version. |
| `/admin/question-bank` | Question bank | Type/tag filters + full-text search. CRUD. Audio badge + inline preview. CSV import (P1). |
| `/admin/question-bank/groups`, `/admin/question-bank/groups/:id` | Group bank/editor | Independent complete-group copies, materials and member editing, shared recording policies, revision-safe autosave and account-scoped local recovery. |
| `/admin/media` | Media library | Uploaded audio/images: filename, duration, size, where used. Delete blocked if referenced by any published version. |
| `/admin/assignments` | Assignments list | Test, targets, window, status, `submitted/total`, flagged count. |
| `/admin/assignments/new` | Create assignment | Published test, targets, window, duration, attempts, shuffle, review policy, integrity policy (§10.3). |
| `/admin/assignments/:id` | Monitor | Per-student: not started / in progress (live remaining, live focus-loss) / submitted / graded. Poll 15s while `open`. Extend deadline, reset, void — all with confirm + reason. |
| `/admin/attempts/:id` | Review & grading | Per question; auto-graded shown; `short_answer` gets points input + comment + **sample answer panel**. Audio questions show plays used vs allowed. **Integrity timeline tab** (§10.4). "Finish grading" → `graded`. |
| `/admin/students` | Students | Table + create/edit. Linked providers, `joined_via`. Reset password. CSV import (P1). |
| `/admin/classes`, `/admin/classes/:id` | Classes | CRUD, members, **join-code panel** (§6.4). |
| `/admin/settings/:section?` | Settings | Profile (default), security (password and Google) and preferences (language), with desktop section navigation and a mobile select. |

The teacher home API (`getDashboard`) accepts `range=7d|14d|30d`, defaulting
to `14d`. Only `submissions` depends on this range: one zero-padded calendar day
per entry in the actor's zone, oldest first with today last, counting `submitted`,
`timed_out` and `graded` papers by `submitted_at`. The total is their sum. The
integer mean percentage rounds half up over graded papers with a valid score;
graded papers without scores still count, and the mean is null when none has a
score. The existing nullable score columns and historical rows are unchanged.

`takingNow` counts distinct students and assignments with an `in_progress` paper
whose deadline is strictly in the future. `today` holds up to 20 published
assignment openings and effective closings in the actor's calendar day, earliest
first; an early close uses the earlier of `closed_at` and `closes_at`.
`notSubmitted` counts distinct enabled targets the caller reaches without a
handed-in paper; a former target's submission does not subtract from that roster.
`recentActivity` holds the ten newest events: one state per non-voided attempt
(started while in progress, submitted once handed in), and student-like joins
through a taught class's code. Attempt flags carry through; joins are unflagged.
These new readings contain no score, band or answer.

`getTeacherSummary` supplies live assignments matching the caller's open list,
unmarked manual answers in reachable handed-in papers, and the caller's unread
notifications. The grading count is null and its query is not run without
`teaching.grading`; live assignments is currently always a number. Both home
operations use `.Own()` even for an Admin, while `listAttempts` keeps its wider
scope. The application supplies the new queries' clock and resolves the IANA
calendar zone through `ports.Zones`; T-R4.7 wires the identity effective-zone
query. Only a stored NULL defaults to `Asia/Ho_Chi_Minh`; an invalid stored zone
or ineligible account propagates an error. A nil port retains the default for
existing callers. Frontend date displays react to the caller's accepted display
zone; existing local datetime inputs and their rules preview remain fixed-HCM
until T-R4.43. Legacy readings retain their
SQL clocks. An absent notifications summary port returns 501.

The grading queue requires `teaching.grading` and preserves the existing assignment
and paper scope. It includes saved, unmarked manual answers in submitted or timed-out
papers; a manual score of zero is marked. Student groups use student IDs; question
groups use the assignment and frozen question IDs together. Counts cover the complete
filtered set before the deterministic first200 items, with oldest submissions first
and null times last. Optional assignment and student filters apply to counts and items.
Group labels use current names or the global one-based frozen question number;
assignment titles are current, while question keys, rich content and group context
come from the published version. The aggregate, groups and item prefix share one SQL
statement snapshot; later immutable enrichment does not claim a whole-request
repeatable-read snapshot. Reading or exhausting the queue never finishes grading;
the teacher still invokes Finish explicitly. The rebuilt web queue is T-R4.28.

Admin list behaviour (approved change request, 2026-09-22):

- Tests, questions, media, classes, students and assignments support explicit multi-selection and confirmed bulk removal. Class rosters support bulk membership removal. Failed items remain selected with an individual explanation; successful items are not retried. Attempt history, grading records and integrity/audit events remain retained.
- Tests and questions expose duplication beside the row menu and inside it. A duplicate leaves the teacher on the list and marks the source and newly created row until navigation.
- List search, filters and pagination survive navigation to a child and back for the signed-in session. Explicit shared URLs take precedence. The question bank shows updated time and orders newest-created questions first.
- The question bank's usage count expands a nested table of currently attached test outlines, fetched on demand, with links to their builders. Frozen published snapshots are excluded from this count and remain unaffected by bank edits.
- Archive tests/classes or disable student accounts before permanent deletion. Assignment deletion requires a draft or closed assignment without attempts. Foreign-key references to assigned work block deletion. Student actor references in retained audit history also block deletion; the existing manual anonymization policy remains separate.
- Published snapshots are never edited in place. Editing an old version copies its frozen content into new bank questions and replaces the editable draft after confirmation. Existing assignments retain their version. Switching the default does not reset the monotonically increasing publication counter. Only unused, non-default versions can be deleted.
- Empty builder groups accept question drops; double-clicking a group title opens rename. Switching questions flushes the current editor and uses the returned saved record. Fresh starter text disappears on focus without scheduling an invalid empty save. Tags suggest recent and matching existing names.
- Assignment duration offers 30, 45 and 60 minutes plus a custom minute input. Attempt count is a number input defaulting to 1. Focus loss offers unlimited, none allowed, or a custom positive count.

## 9. Screens & routes — student and public

`StudentLayout` is the deck's student shell: a 60px top bar, no sidebar,
safe-area padding, and one route outlet that stays mounted at every width. It
branches at 768px. From 768 the bar holds the destinations beside the logo
(Home, Classes). Below 768 they are a bottom tab bar (Home, Classes, Me), and a
detail screen (the intro, the result, Settings) swaps the logo for a back arrow
and its title and hides the tabs. A destination whose module has not shipped is
absent: Learn, Grades, Messages and the bell in v0.9.0. Home's item carries a
badge, the papers still to do that close within seven days (DG-82). The avatar
button opens the account menu: Settings, the light or dark theme, Sign out.
Each page is one centred column with its own maximum width (§12). The focus
engine has no navigator column and no stored width. `AuthLayout` is the brand frame for sign-in, forgot password, the Google callback, change password and join. The brand panel shows from 900px, except on join and its Joined state, which stay one column at every width. The system pages use `SystemFrame`.

| Route | Layout | Key behaviour |
|---|---|---|
| `/join`, `/join/:code` | Auth (one column) | §6.2: inline preview, then Joined. Every failed lookup shows one plain message, with no hint about which classes exist. `/join/:code/confirm` redirects to `/join/:code`. |
| `/login` | Auth | Password form + "Tiếp tục với Google". One message for an unknown email, a wrong password and a disabled account; a 429 shows the server's message. In a join context the subtitle names the class and a successful sign-in continues the join. |
| `/forgot-password` | Auth | Static help (§5.4); no request. |
| `/app` | Student | Home. A greeting for the time of day and one line about what is next. The resume card for the attempt in progress that closes soonest; "Tiếp tục làm bài" resumes it at once. Coming up: every other paper still to do, each row a link to its intro, with a pill (in progress, due today or tomorrow inside 24 hours, open now, opens …). Recent results, newest first: the score if allowed, "being graded" or "submitted", each a link to its result. No filters. Only the intro starts the clock. A student in no class with nothing assigned is offered the Join dialog (§6.2). |
| `/app/assignments/:id` | Student | Intro: class and title, three facts (time limit, questions, attempts used of allowed), and "Trước khi bắt đầu": **the rules stated plainly**, generated from the stored policy and dates (availability, the timer, fullscreen, copy and paste, leaving, the audio plays per §11.4, what the result shows). "Bắt đầu làm bài" asks "Bắt đầu ngay?" first; "Tiếp tục làm bài" resumes at once. A test not yet open, closed or with no attempts left says so in the button's place. A start the server refuses says why above the button: `409 MAINTENANCE_SCHEDULED`, for a start that would run into a maintenance window, with the server's message. When the teacher wrote a note for the students, it is drawn between "Trước khi bắt đầu" and the button: the teacher's initials, "Ghi chú từ {name}" and the text, as plain text with its line breaks; with no note nothing is drawn. The score sentence says "sau khi nộp bài" for a result released on submit and "sau khi bài đóng" for one released after the close. |
| `/app/attempts/:id` | Focus | The engine (below). §10, §11.3. |
| `/app/attempts/:id/result` | Student | The summary card (the score ring if allowed, the class, the title, one sentence on where grading stands), tiles, then every answer honoring `review.*`, with the transcript if `showTranscriptAfterSubmit`. While answers wait for the teacher the ring shows the score so far and the tiles say what waits. A line names what the policy hides. A result released after the close is withheld until then: the ring is a lock, no score, mark, answer key, explanation or grader's comment is sent, and the line and the summary say when it is released (`releasesAt`). When the server sends `classAverage`, one muted line under the summary sentence reads "Điểm trung bình của lớp: {n}%." Filters All / Wrong / Waiting: Wrong only when scores are shown, Waiting only when something waits; an empty filter says so and offers all questions. The phone header reads "Kết quả". |
| `/app/classes` | Student | The classes joined, as cards: name, description, teacher, and "Next", the paper that comes next in that class. A card is not a link, and nothing here leaves a class. One action, "Tham gia lớp", opens the Join dialog (§6.2). |
| `/app/settings/:section?` | Student | Profile (default: name, email read-only, language), Sign-in (password, Google) and Appearance (theme, "Chữ lớn hơn khi làm bài"), under a segmented switcher. Every section stays mounted, so a form survives a change of section or of width. The old slugs redirect: `security` to `sign-in`, `preferences` and any unknown slug to Profile. |

Shared:

- `/change-password` (§5.4).
- `/403`, and the 404 page for any unknown route.
- The unexpected-error page, with a copyable error ID (the server's `requestId` when there is one).
- The maintenance page.

"Home" on these pages is the caller's console, or `/login` when signed out. The system pages (403, the 404 page, the unexpected-error page and the maintenance page) ship in the entry chunk, so they render when a lazy chunk cannot load; `/change-password` is a lazy route.

**Boot splash.** The app starts behind the deck's splash. Before the bundle runs, `index.html` paints the mark, the track with the first step's 12% fill and the version line; the step label stays empty until the bundle writes it.

- It shows three steps, and the bar moves only when a step is done, with no minimum duration:
  1. "Đang kiểm tra phiên đăng nhập…" — refresh and `/auth/me`.
  2. "Đang tải lớp học của bạn…" — the first route's code.
  3. "Sắp xong rồi…" — the hand-over.
- **Slow:** eight seconds without a step shows "Tải lâu hơn bình thường" and "Tải lại".
- **Offline:** a session restore that got no HTTP response shows "Mất kết nối" and retries after a ten-second countdown, or as soon as the browser is back online.

**Overlays** stand over a mounted page, which becomes inert; focus moves into the overlay. Over the teacher console, which forces light until R4 rebuilds it, the overlays are light too.

- **Maintenance** closes when `GET /public/status` reports no window under way. It checks on "Kiểm tra lại", and every minute while the user is active. Closing refetches everything.
- **Sign in again** (§5.2).
- **A newer build.** `/version.json` is checked when the tab becomes visible and every ten minutes while the user is active. A different build shows "Quizzivy vừa được cập nhật" at the next navigation, never on the engine's route. A route whose code cannot load shows it at once.
- **Precedence:** maintenance, then sign in again, then a newer build.

---

**The engine** shows one question at a time.

- **Header.** The ✕ ("Thoát khỏi bài làm"), from 768 the title over the save
  line, the timer, and "Nộp bài". The ✕ asks first, except on a locked paper,
  which it leaves at once and without saving. Leaving saves what is
  unsaved and goes to `/app`; a save that fails keeps the student on the paper.
  Below 768 the header has no save line. A strip under it carries one only when
  a save has failed or the device is offline with an answer unsaved, never for
  a save on its way, and carries the strike count where the assignment sets a
  limit (§10.2). A locked paper (taken over, out of time, ended) says why in a
  bar under the header at every width. Once the server has said the time is
  up, the timer and the Submit dialog's time left read 00:00. A deadline lock
  rearms automatic submission at the device's recorded deadline, whichever
  request was refused and even after a second refusal. Submission can still
  fail and retry. A deadline refusal never replaces a closed or superseded lock.
- **Panes.** A question whose group has something to read shows a passage pane
  beside the question pane from 768, each scrolling by itself. Below 768 a
  "Ngữ liệu | Câu n" switcher shows one at a time, the question first and again
  after every move.
- **Footer.** Under the question pane: Previous, the questions, and Next, which
  is "Hoàn tất" on the last question. From 768 the questions are a strip of
  numbered squares. Below 768 a button ("4 / 8 · đã trả lời 3") opens them in a
  bottom sheet. A square shows answered, current and flagged, and on a paper of
  several parts the strip and the sheet keep the parts apart.
- **Submitting.** "Nộp bài" in the header and "Hoàn tất" open the Submit
  dialog, the only way a student hands a paper in: answered, flagged, time left, and a
  "Đến câu" chip for each unanswered question. There is no review page.
  Submitting saves what is unsaved first. A submission that fails says so in
  the dialog, which stays open. The submitted screen offers Home and the
  submitted paper.
- **An unknown type.** A question of a type the page has no renderer for shows
  a block that names it and offers a reload, and takes no answer.
- **Local drafts.** Student answers awaiting server confirmation are cached
  locally per student, attempt and session until saved or closed. They may be
  restored before the server deadline after a reload. A draft a closed tab left
  behind is sent under the session that wrote it before the student is given a
  new session by "Tiếp tục làm bài" on Home or the intro. The server saves it
  if that session is still the attempt's. If it is refused with one of the
  attempt's own codes (`SESSION_SUPERSEDED`, `DEADLINE_PASSED`,
  `ATTEMPT_CLOSED`) or as a body that can never be accepted
  (`VALIDATION_FAILED`), it is dropped: a superseded session must not overwrite
  a newer session's answers. On any other failure the draft is kept and the
  attempt is not resumed. Explicit sign-out clears the local cache.
- **Held session.** A tab names the session it holds each time it reads an
  attempt. An in-progress paper taken over elsewhere is answered as
  `superseded`: it stays read-only after a reload or refetch, reads no local
  draft, and removes a draft only if its own session wrote it. It writes
  again only when the student continues the attempt there. A tab that holds
  no session is given the current one. Such a read still replaces the
  attempt's beacon token, and two tabs of one browser can share a session.
- **Keys.** A–E choose an option; a sixth has no key, because F flags the
  question. The arrows move between questions and stop at either end. Esc
  closes a dialog or the sheet. None acts in a text field or another control
  that owns the key, with Ctrl, Alt or the command key held, while a dialog or
  the question sheet is open, or on a locked paper.

## 10. Integrity monitoring (proctoring-lite)

First-class requirement. Lives in `src/features/integrity/`, consumed by `take-test` (capture) and `attempts` (review).

### 10.1 Signals

Every signal produces an append-only event `{ kind, occurredAt, clientSeq, questionId?, meta? }`.

| Kind | Source | Notes |
|---|---|---|
| `tab_hidden` / `tab_visible` | `document.visibilitychange` | Primary tab-switch signal; pair to compute away-duration. |
| `window_blur` / `window_focus` | `window` blur/focus | Catches alt-tab to another **application** — the Visibility API alone misses this. |
| `fullscreen_enter` / `fullscreen_exit` | Fullscreen API | Only when `requireFullscreen` is on. |
| `copy` / `cut` / `paste` | listeners on `document` | Always recorded; **blocked** only when `blockCopyPaste` is on. |
| `context_menu` | `contextmenu` | Recorded; blocking off by default (it breaks assistive tooling). |
| `network_offline` / `network_online` | `navigator.onLine` + fetch failures | Distinguishes cheating from bad wifi. Matters for fairness. |
| `audio_play` / `audio_ended` / `audio_blocked` / `audio_seek` | player (§11.4) | Gives the teacher listening behaviour. `audio_seek` is a jump the player put back on a recording that may not be skipped. The events only report: a question's play is counted by `POST /app/attempts/:id/audio-play`. |
| `resume` | server-side | Re-entry into an `in_progress` attempt: reload, crash, device change. |
| `session_takeover` | server-side | Attempt opened in another tab/device. |
| `page_hide` | `pagehide` | Best-effort final flush via `navigator.sendBeacon`. |

**Deliberately excluded:** devtools-detection heuristics (window-size deltas, `debugger` timing). They false-positive on zoom, split-screen, and extensions, and are bypassed in seconds. Do not implement them.

**Away-duration over count.** A 2-second blur is a notification; a 90-second blur is a search. Store both endpoints so duration is visible, and only count a strike when an away episode exceeds `minAwayMs` (default 3000ms).

### 10.2 Student-facing behaviour

Announced, visible, never silent.

- The intro page states the active rules in plain Vietnamese before starting, generated from the stored policy (§9). If `requireFullscreen` is on, the intro's click that starts or resumes the attempt is what enters fullscreen (browsers require a gesture): "Bắt đầu" in the "Bắt đầu ngay?" dialog, or the intro's "Tiếp tục làm bài". Home's "Tiếp tục làm bài" (§9) goes straight to the paper and does not ask for fullscreen: the engine opens outside it and shows the fullscreen bar described below. A browser with no fullscreen is told so and takes the test as normal.
- Each counted absence opens the deck's `alertdialog` when the student returns: "Bạn vừa rời trang làm bài", one body and one button, "Quay lại bài làm". The body says where the student stands and what happens next, and only what the server does:
  - within the allowance, "Lần này được tính là lần n trong m lần được phép…";
  - past it under `flag`, that the teacher has been told and the answers are safe (D8);
  - past it under `warn`, the count and that the answers are safe, never naming the teacher;
  - with no absence allowed (`-1`), that leaving is not allowed, with no number;
  - under `auto_submit`, that the test is submitted after the allowance, and at the last one that the next absence submits it;
  - with no limit, that leaving is recorded, once a sitting.

  The body does not state the seconds away or that the timer keeps running: the timer is on screen. The dialog does not close from the backdrop and has no close button; the button and `Esc` acknowledge it.
- A small persistent indicator shows remaining strikes when a limit is set: after the save line from 768, in the strip under the header below it (§9). Past the allowance it reads "Giáo viên đã được báo" under `flag`. `maxFocusLoss = 0` retains the unlimited default; `-1` permits no counted departure; positive values permit that many departures.
- `onLimitExceeded`: `warn` = dialog only; `flag` = attempt marked for the admin, student told; `auto_submit` = immediate submission on exceeding the count, retaining answers for grading and recording the violation. There is no cancellation or extra strike. The final answer/event batch is saved before the server grades and closes. While offline, the attempt is locked locally, pending answers are retained and submission is retried with a visible notice.
- Fullscreen exit shows a bar under the header with a "Quay lại toàn màn hình" button, or one sentence on a browser with no fullscreen. It is a bar, never a dialog. Never trap the student: `Esc` always works and there is always a visible way to leave and submit.
- Once the attempt is submitted on an assignment that requires fullscreen, by the student, the timer or an auto-submit, the engine leaves fullscreen itself. That exit is the app's, so from then on no `fullscreen_enter` or `fullscreen_exit` is recorded, the student's own on the submitted screen included. A paper that is locked without a submit (taken over, already ended) stays as it is.
- Under `blockCopyPaste`, a copy, cut or paste is stopped and recorded, and a danger toast says "Sao chép và dán đã bị tắt trong bài này": one toast, however often.
- Under five minutes the timer takes the danger tones (D8). A polite live region says the time left once when it passes 5:00 and once when it passes 1:00.
- While the maintenance or sign-in-again overlay (§9) covers the engine, no focus change is recorded. An away episode already open when one appears is dropped, so time on an overlay never counts against the student.
- The engine adopts the server's deadline from every autosave (`deadlineAt`). It also adopts it from a timer submit that came too early (`409 DEADLINE_NOT_REACHED`; the server allows 5 s of grace). So a teacher's extension, or a maintenance window's, reaches an open attempt on its next autosave.

### 10.3 Policy defaults (per assignment)

`requireFullscreen: false`, `blockCopyPaste: true`, `maxFocusLoss: 0`, `onLimitExceeded: 'flag'`. Conservative on purpose — the teacher opts into stricter modes per test.

### 10.4 Admin review

`/admin/attempts/:id` → **Integrity** tab: ordered timeline with event kind, wall-clock time, offset from attempt start, duration for paired events, and the question on screen. Summary strip: total away-time, away episodes ≥ `minAwayMs`, paste count, resume count, audio replays. Neutral text — no red banners, no "CHEATING DETECTED". The teacher judges; the app reports. An audio play with no recorded end shows no duration and is not called ongoing.

Sessions are ordered by their earliest `receivedAt`, with `sessionId` breaking
ties. Within a session, `resume` comes first and `session_takeover` last.
Within each priority, two events with different non-null `clientSeq` values
compare by `clientSeq`; otherwise by `occurredAt` and then event id. This order
is not a guarantee of wall-clock chronology across tabs or sessions.

### 10.5 Honest limits — say this in the UI help text

Browser monitoring detects *this tab* losing focus. It cannot see a second device, a phone beside the laptop, a person in the room, or a printed sheet. The signals are **evidence for a conversation**, not proof. Do not build auto-zero or auto-ban features that assume otherwise.

### 10.6 Client implementation

- One `useIntegrityMonitor` hook owns every DOM listener that records a
  signal or stops one, registered and torn down in a single `useEffect`.
  Four signals are recorded outside it, by a question's audio player through
  `recordAudioEvent` (§11.4): `audio_play` from its play callback,
  `audio_ended` when playback reaches the end, `audio_blocked` when the browser
  refuses to start it, and `audio_seek` when the player puts a jump back.
  None changes a play count. A shared recording's play is written by the
  server (§11.5); the browser records its end and block with `scope: group`
  and `recordingId` in `meta`, without a question id. A known end duration is
  elapsed device wall time since the latest start, clamped to zero if the
  clock moved backwards; it is not total listening time. A resume overwrites
  that start; an end or block clears it. The timeline pairs the latest open
  play once, by question id first or otherwise by recording id. It accepts
  numeric `durationMs` from 0 through 86,400,000, truncating fractions to
  whole milliseconds; missing, malformed or out-of-range durations fall back to the
  nonnegative span between event clocks. Delayed play counts or clock skew
  can leave events unpaired. Overlapping sessions have no play-id matching.
  Other hooks listen to some of the same events and record and stop nothing:
  `useClipboardNotice` has its own `copy`, `cut` and `paste` listeners on
  `document`, only to show §10.2's toast; `useSaveStatus` listens to `online`
  and `offline` on `window`, only for the save line (§9); `useVersionWatch`
  listens to `visibilitychange` on `document`, only to look for a newer build
  (§9). The monitor records and stops whether or not they run.
- Events buffer in memory + `sessionStorage`, flush with the autosave batch, and immediately on `pagehide` via `sendBeacon`. Leaving the engine while the attempt goes on sends what is buffered through the same beacon and keeps the sequence in `sessionStorage`, so a return in the same session numbers on from it; the buffer is forgotten once the attempt has ended.
- `clientSeq` uses an attempt-relative millisecond offset, calibrated by
  `getAttempt` from `startedAt` and `serverTime`, and is never below the tab's
  stored next sequence. It increases within a tab. The offset component caps
  at 2,000,000,000; subsequent local increments can exceed that value.
  Response latency, clock changes and local increments can cause reordering
  or collisions across tabs, even for events in different milliseconds.
  Cross-tab chronology and uniqueness are not guaranteed. Older builds still
  send counters from zero, which the server accepts. The append-only dedup
  key remains `(attempt, session, clientSeq)`; a repeated key is treated as a
  retry and not stored.
- Failed background event flushes do not block answering or manual submission.
  The immediate `auto_submit` policy retries its final answer/event batch before
  confirming submission so the violation is retained with the answers.
- An event transport failure alone does not block input. The explicit `auto_submit` policy locks further answering once exceeded, while preserving and retrying the pending answers.

---

## 11. Audio / listening questions

### 11.1 Admin: upload and attach

- In the question editor, an audio question has: file upload, transcript textarea (admin-authored, optional), and the `AudioPolicy` controls (`maxPlays` default 2, `allowSeek` default **false**, `showTranscriptAfterSubmit` default true).
- Accepted: `audio/mpeg` (.mp3) and `audio/mp4` / `audio/aac` (.m4a). **Reject everything else** with a plain message. These two cover every current browser without transcoding; supporting `.ogg`/`.wav`/`.webm` means either transcoding or a Safari support matrix, and neither is worth it in v1.
- Limits: audio 50 MB and 5 minutes; an image (PNG, JPEG or WebP) 10 MB (DG-63). Validate **server-side** by sniffing magic bytes and probing duration — never trust the `Content-Type` header or the file extension. The file is streamed to disk, never held in memory, and an image's size in pixels is read from its header.
- A teacher's library holds at most 5 GB unless the deployment sets another quota (`MEDIA_OWNER_QUOTA_MIB`). An upload that would take it past the quota is refused with `MEDIA_QUOTA_EXCEEDED`. A deleted file no longer counts, and neither does a replaced one.
- In the library a file can be renamed, and audio can be given a default play limit: unlimited, or 1 to 3. The stored file and its original filename never change. The limit is stored with the file for the editor to start a question's own limit from; it changes no question by itself.
- Upload goes **through the Go backend** in v1 (low volume, and the backend must validate anyway). Presigned direct-to-R2 upload is the P1 optimisation; design the API so switching does not change the client contract beyond the upload call.
- Assets are **immutable**. Re-uploading creates a new `media_assets` row; it never overwrites an existing key. This is what lets `test_version_questions` reference an asset without copying the file.
- A library file can be replaced only by the same kind. Replacement creates a new
  immutable asset owned by the old file's owner, with its current display name and default
  play limit. In one database transaction, the retained current graph of live editable
  questions and bank or draft-owned group media of that owner points to the new asset;
  each changed group gains one revision.
  Published versions, import review drafts and other owners' bindings retain the old
  readable asset. The response counts distinct changed questions/groups and other owners'
  live editable questions/groups left on the old asset; historical versions and imports
  are excluded. Late same-owner bindings may retain the old asset and are excluded from
  the left counts. The old file leaves the library and quota, but remains available to existing
  bindings and later publication/import materialization. An internal error can follow a
  commit, or its outcome can be unknown; both retain the new object. A retry against the
  replaced old library ID can answer 404. Replacement does not promise idempotent retries
  or an atomic transaction with object storage.
- Client-side pre-check before upload: read duration via an `<audio>` element and reject early, so a student's teacher does not wait 50 MB to be told no.

### 11.2 Storage and delivery

- Bucket `quizzivy-media`, key `audio/{asset_id}.{ext}`. Bucket is **private**; no public listing, no public read.
- Served via **short-lived signed URLs** (10 min), minted per request by the backend. The frontend treats `MediaAsset.url` as expiring and refetches on `403`.
- `Cache-Control: private, max-age=600` — long enough to survive a replay, short enough that the signed URL does not outlive its cache entry.
- Honest note: `controlsList="nodownload"` and hiding the URL are **not** protection. A determined student can save the file. Signed short-lived URLs raise the cost slightly; do not invest further. If a listening file must never leak, do not put it online.

### 11.3 Player component

`web/src/features/media/components/AudioPlayer.tsx`. Custom controls over a native `<audio>`; no third-party audio library.

- Controls: play/pause, elapsed/total time, a progress bar that is **display-only when `allowSeek` is false**, and a plays-remaining indicator. The total shown is the server's probed length when it is known; the file's own length drives the track and the seek.
- `preload="metadata"` so duration renders without downloading the file.
- **Autoplay is impossible** — browsers block audio without a user gesture. The first play is always a tap. Do not attempt to auto-start; do not treat the block as an error state.
  A play refused with `NotAllowedError` says so under the player, which stays
  in place. `AbortError`, including a pause before playback started, says
  nothing. Other playback rejections or a file-load failure show the
  expired-link card.
- iOS Safari: only one audio element plays at a time, and playback must originate from a gesture handler (not an async continuation). Call `.play()` synchronously in the click handler; do not `await` anything before it.
- `allowSeek: false` implementation: no `<input type="range">`, and an `onSeeking` handler that resets `currentTime` to the last known position. Note that OS-level media controls can still seek in some browsers — record an `audio_seek` event rather than pretending it cannot happen.
- Accessibility: real `<button>` elements, `aria-label` on each, keyboard-operable, `aria-live` announcement of plays remaining. When `showTranscriptAfterSubmit` is on, the transcript appears on the result page — this is also the accessibility fallback for hard-of-hearing students.
- One player instance per question. Navigating away pauses and releases it.

### 11.4 `maxPlays` — server-authoritative

The obvious client-side counter resets on reload, which makes the limit meaningless. So:

- Each `play` calls `POST /app/attempts/:id/audio-play`; the server increments `attempt_audio_plays (attempt_id, question_id, plays)` and the count is returned in `GET /app/attempts/:id` as `audioPlays`. The `audio_play` event the player records beside it (§10.6) only reports the play and changes no count.
- Client renders remaining plays from the server value, optimistically decrements on play, and reconciles on the next fetch.
- Playback is **optimistic**: a failed event POST does not block the audio. A student who goes offline to farm replays will show a gap in the event log, which is exactly what the integrity timeline is for. Blocking playback on a network round-trip would punish bad wifi far more often than it would catch anyone.
- On submit, the server rejects nothing based on play count. Over-limit plays are reported to the teacher, not enforced retroactively.

### 11.5 Word milestone shared recordings (approved, not yet enabled)

D-03 extends the existing audio contract to a recording shared by a group. All
child questions consume the same configured play allowance. Navigating between
children, reloading or resuming the same attempt on another device never grants
a fresh allowance. Two groups may reference the same immutable file and still
have independent counters. The counter identity therefore includes the versioned
group recording binding; the asset ID alone is insufficient.

Freeze that binding and its audio policy in the published version, and keep its
counter scoped to the attempt. Preserve synchronous gesture playback, optimistic
accounting, retry deduplication, teacher reporting and transcript visibility rules
from §11.3–11.4. The future shared player must use this scope consistently in
navigation, event writes and state reconciliation. Its API, relational references,
concurrency tests and student-payload tests must precede enabling shared audio.
Existing per-question audio remains a separately supported historical contract.

W-09c adds a separate shared-play endpoint with `recordingId`, current `sessionId`
and a stable per-gesture `playId`. It follows the existing autosave writable-session
checks and verifies the recording's material binding belongs to the attempt's
version before writing. Retrying the same gesture/recording returns its current
counter; reusing the gesture ID for another recording is a conflict. The counter,
append-only receipt and server `audio_play` event commit together. Counts survive
reload/takeover and are returned as `groupAudioPlays`, keyed by recording ID.
Teacher monitor and timeline excess-play totals include this ledger. New attempts
start with their own allowance. Network acknowledgements and `maxPlays` never gate
the actual browser play call. The browser persists unconfirmed gesture IDs per
learner/attempt until the attempt deadline, clears them on logout and serializes
bounded retries. Confirmed extensions update the recovery deadline. A pending
sync label is distinct from answer save state. A response lost after commit does
not consume a second allowance on retry; late responses cannot mutate another
session. Counts remain monotonic during refetch. Submission attempts a bounded
three-second telemetry flush but remains available during network accounting
failure. Closed/expired attempts reject late telemetry; offline closure cannot
guarantee complete listening evidence. Results and teacher paper review report
the confirmed counts without creating new playback receipts.

The assignment introduction derives listening presence, transcript permission and
the strictest finite allowance from both question audio and shared recordings on
the assigned version. A newer default does not change these facts. The displayed
minimum is not presented as every recording's allowance; individual players show
their own limits. A shared-audio notice explains that member navigation and reload
do not grant a new allowance.

---

## 12. Design guidelines

Deliberate. Do not "improve" them with trendy defaults.

**As built in R1 (v0.7.0).** The deck (`docs/design/deck/`) is the source.

- **Tokens.** `web/src/index.css` carries the deck's light and dark sets. `tokens.test.ts` measures them rather than pinning values:
  - every themed token has a dark value;
  - the neutrals stay under 0.03 chroma and `--primary` stays charcoal;
  - each colour family stays in its hue band, with only info allowed to be blue;
  - text pairs reach 4.5:1, and the focus ring and input borders 3:1, in both themes;
  - `@theme` routes every utility colour through a variable.

  `no-raw-colours.test.ts` refuses a Tailwind palette class, or a literal hex, `rgb()`/`rgba()` or `oklch()` colour, in the `.ts`/`.tsx` files under `src/` (except `GoogleMark.tsx`); the literal values live only in `src/index.css`.
- **Accessibility tokens beyond the deck** (DG-30, DG-31), each measured in both themes:
  - `--focus`, the 2px `:focus-visible` ring on every control, 3:1 or more on `--bg`, `--card`, `--sidebar` and `--muted`;
  - `--input`, input borders, 3:1 or more;
  - `--danger-solid` / `--danger-solid-fg`, danger buttons, 4.5:1 or more.
- **Deck geometry that would change an existing primitive** (control heights, `--radius` and the sm/md/lg radii, the badge and card shapes) applies only inside `data-scale="deck"`, which rebuilt surfaces set on their root. The deck's type steps are global tokens, but the pre-R1 steps keep their values, so an old screen does not shift. Sizes that exist only in the deck have their geometry everywhere (button `md`, `xl` and `icon-xl`, the new avatar sizes, the toast).
- **The teacher console** (the admin layout) keeps its layout and forces light (`useForcedLightTheme`) until R4 rebuilds it. The student and focus layouts are deck surfaces since R3 and follow the theme. R5 makes the deck geometry the default and removes `data-scale` (T-R5.30).

**The student app and the engine, as built in R3 (v0.9.0).** The source is `Quizzivy Student.dc.html`. What the deck does not draw, and where the product departs from it, is in `docs/design/gaps.md` (DG-80 to DG-89, DG-102 to DG-107, DG-119).

- **One breakpoint, 768px.** The shell and the engine change their chrome there and nowhere else: destinations in the top bar or bottom tabs, side-by-side panes or the pane switcher, the number strip or the count button with its sheet. A page keeps its state across it.
- **One centred column per page:** Home and Classes at most 960px, the test intro 720px, the result 820px, Settings 760px. No page has a side panel. In the engine the passage column is at most 640px and the question column 600px.
- **Classes** are cards in a grid that fills the row, with 280px as a card's least width. A card is not a link.
- **Touch targets.** Below 1024px the student surfaces put a 44px floor on buttons. A control the deck draws keeps the deck's size: the header's 36px ✕, the 32px flag toggle, the strip's 34px squares, the dialogs' 42px buttons (46px for the one button of the "you left the test" alert). The engine's Previous and Next, the count button and the question sheet's squares are 44px. An option row is at least 52px high.
- **Dialogs** use the deck's frame: 440px (420px for the "you left the test" alert) or the width less 24px, no close button, the actions at the right; the alert has one button, as wide as the dialog. A dialog with a field sits 12% from the top below 768, so the keyboard does not cover it (DG-103).
- **The timer** is a pill centred in the header's free space. Each digit sits in a cell one zero wide, because Be Vietnam Pro has no tabular figures and the pill would otherwise change width every second. Under five minutes it takes the danger tones.
- **Text in a test.** The passage is 16px on a 1.75 line and cannot be selected, the prompt 17px, an option 15px. "Chữ lớn hơn khi làm bài" raises them to 18, 19 and 17px; the signed-in choice is an account preference mirrored in this browser; anonymous choices remain local.
- **Content keeps a light paper surface in dark mode** (DG-35): images and rich tables in the engine and on the result.

The rules below carry over from v0.43, restated in the deck's tokens, except that the shadow and radius limits now follow the deck. Typography, motion, the front door, dark mode and the lime accent are new in R1.

- **Neutral by default.** The deck's teal-grey scale (`--bg`, `--card`, `--muted`, `--border`, `--fg`, `--muted-fg`) and 1px borders. Deck surfaces use the deck's shadows (`shadow-card` at rest); floating surfaces go up to `shadow-lg`. The old consoles keep shadcn's shadows until their rebuild.
- **Primary action: the deck's charcoal `--primary` with `--primary-fg`.** Not blue, not purple, not indigo. The lime `--accent-c` (with its soft and ink tones) marks progress, counts and current states, never a primary button.
- **Semantic color only where it carries meaning:** green = correct/success, red = incorrect/error/destructive, amber = warning/time-low. Never decorative.
- **Forbidden:** decorative gradients, glassmorphism/backdrop blur, pulsing rings, glow effects, radii beyond the deck's (the old consoles keep `rounded-md` controls and `rounded-lg` cards), emoji in UI chrome.
- **Typography:** Be Vietnam Pro, self-hosted (latin, latin-ext and vietnamese subsets, 400–700, upright and italic; the CSP allows no font host). Every character the UI draws is in the loaded font faces; a symbol the font lacks is an icon (`KeyGlyph` for ⌘, ↵, ↑ and ↓), or a character it has, such as the en dash for a range. Content is shown in NFC: the rich content editor composes what it writes, and other text is composed where it is drawn. The deck's scale: 3xs 10.5, 2xs 11, caption 11.5, xs 12, meta 12.5, sm 13, ui 13.5, base 14, body 14.5, md 15, title 16, lg 17, stat-sm 18, xl 20, stat 22, h1 24, h1-student 26, kpi-sm 28, kpi 30, display 34px. Phone text inputs stay at 16px to avoid input zoom; student touch targets follow the R3 rule above; seek tracks have a 44px hit area. `leading-relaxed` in the test view.
- **Icons:** lucide-react, 16px dense / 20px nav, consistent stroke, `aria-hidden` unless standalone.
- **Density:** admin tables dense (~40px rows). Student pages are the centred columns of the R3 rules above. The teacher's settings use a 192px local navigation column beside a form column capped at 768px, with divided rows and light shadows, until R4 rebuilds them. The student test view stays spacious: one question at a time, beside its passage from 768px when it has one.
- **Action hierarchy:** resume is primary; opening an assignment's intro is secondary. Deadline pills turn amber only within 24 hours. The 320px test footer has previous as an icon, the count button that opens the question sheet, and Next or Finish. In the Submit dialog the two buttons stay in reach while a long list of unanswered questions scrolls. Submission confirmation offers the submitted paper directly.
- **Explicit states:** single- and multiple-choice instructions identify selection behaviour without revealing the key. The save line says saved, saving, offline or failed; exhausted audio allowances explain continued playback is recorded. Login exposes class joining and a reversible password visibility toggle. Signed-in join screens identify the current account and provide a way back.
- **Motion.** The deck's keyframes, and what the old consoles still carry:
  - 150ms control feedback, and the settings panels' 150ms ease-out `settings-enter` entrance.
  - A 2px hover lift on cards, on pointer devices (ours; the deck changes the border instead).
  - The splash's .35s fade and its bar.
  - `qz-breath`, `qz-indet` and `qz-shimmer`: the splash, and skeletons on deck surfaces. Elsewhere the skeleton keeps shadcn's `animate-pulse`; spinners use Tailwind's `animate-spin` (`qz-spin` is defined and not yet used).
  - `qz-pulse`, for the live dot.
  - `qz-marquee`, only when a title overflows, paused on hover and focus.
  - The vendored primitives' enter and exit (dialog, popover, select, dropdown menu), standing in for the deck's popover entrance.
  - The test builder outline's drag-and-drop reorder slide (dnd-kit); the deck draws a drop line instead.

  Under `prefers-reduced-motion` the marquee, live dot, breath, shimmer and indeterminate bar each have a static state, not a shorter one: an ellipsis, a solid dot, a still mark, flat `--muted`, a fixed 40% bar. Everything else is cut to 0.01ms by the global reduced-motion block. Exam inputs remain stationary. The rebuilt consoles (R3–R5) replace the old consoles' motion with the deck's.
- **Audio player:** monochrome. A filled `--primary` play button, a thin `--secondary` track with a `--primary` fill. No waveform visualisation, no equaliser animation, no colored accents.
- **Front door:** sign-in, forgot password, join and change password share the brand frame, with the brand panel from 900px, except join, which stays one column. On join, the class name is large and there is one primary button. This is the first thing a new student sees — it should look calm and legitimate, not like a marketing page.
- **Dark mode:** in scope from R1.
  - The preference is light, dark or system, stored in `localStorage['quizzivy.theme']`.
  - `web/public/boot.js` applies it before paint, since the CSP allows no inline script.
  - Components read colour tokens rather than hard-coding colours. The exceptions: the brand art, which shows its on-dark files in dark because an image cannot read a token; `GoogleMark`, in Google's own colours; and the vendored primitives in `components/ui/`, which keep their `dark:` variants.
  - Content surfaces keep a light `--paper` surface in dark mode (DG-35). The tokens exist, and each surface adopts them when it is rebuilt.
- **Empty states:** one short sentence + one primary action. No illustrations.
- **Vietnamese first.** Design for longer Vietnamese strings; avoid fixed-width labels.
- **Integrity UI follows the deck** (D8, decided 2026-09-26). On the student side since R3: the red timer under five minutes, the "Bạn vừa rời trang làm bài" dialog with its eye-off tile, "Giáo viên đã được báo" past the allowance under `flag`, and the danger toast for a blocked copy or paste (§10.2). The words stay plain: no sentence a student reads says "violation" or "cheating", and the product never concludes that a student cheated. The teacher's roster follows in R4; until then it keeps the calm rule: plain text, no alarm iconography.

---

## 13. Database design (PostgreSQL 18)

### 13.1 Binding references

Two authorities govern all schema and query work. Where they conflict, the PostgreSQL docs win.

1. **Neon `postgres-best-practices` agent skill** — <https://github.com/neondatabase/postgres-skills>. Install before writing any DDL:
   ```
   npx skills add neondatabase/postgres-skills
   ```
   An Agent Skill package (`skills/postgres-best-practices/SKILL.md` + `references/schema-design.md`), curated by Postgres practitioners including a PostgreSQL Core Team member. Its schema-design, indexing, and migration guidance is binding for this project.
2. **PostgreSQL 18 official documentation** — <https://www.postgresql.org/docs/18/index.html>. Version-specific behaviour is checked here, not against training-data recollection. Target **PostgreSQL 18**; do not write DDL that silently degrades on 16/17.

**Rule for the agent:** load the skill before proposing DDL or non-trivial SQL, and cite the specific docs section for any PG18-specific construct used.

### 13.2 Conventions

- Schema `app`, not `public`. `REVOKE CREATE ON SCHEMA public FROM PUBLIC`.
- Tables plural snake_case; columns snake_case; no Hungarian prefixes.
- PKs: `uuid PRIMARY KEY DEFAULT uuidv7()` — PG18's built-in time-ordered UUID avoids the B-tree fragmentation random v4 causes on insert-heavy tables. Use `uuidv4()` only where hiding creation time genuinely matters.
- `timestamptz` everywhere; never bare `timestamp`. Store UTC, render `Asia/Ho_Chi_Minh`.
- `text` over `varchar(n)` unless a real business constraint exists; enforce with `CHECK` when it does.
- Scores: `numeric(8,2)`. Never `float`.
- PG enums where the set is genuinely closed (`attempt_status`); lookup tables otherwise, such as `roles` (R2), which custom roles keep open. Adding an enum value is easy, removing one is not.
- Every FK has an explicit `ON DELETE`. Default `RESTRICT`; `CASCADE` only for true owned children.
- Every FK column used in a join or filter gets an index — Postgres does not index FKs automatically.
- `created_at timestamptz NOT NULL DEFAULT now()`; `updated_at` via trigger, not application code.
- Soft delete only where history matters (`questions`, `tests`, `media_assets`); everything else deletes for real.

### 13.3 Core tables

Sketch, not final DDL. The agent produces real migrations after loading the skill.

```sql
CREATE SCHEMA app;

CREATE TYPE app.user_role      AS ENUM ('admin','student');
CREATE TYPE app.test_status    AS ENUM ('draft','published','archived');
CREATE TYPE app.question_type  AS ENUM ('single_choice','multiple_choice','true_false','fill_blank','short_answer');
CREATE TYPE app.attempt_status AS ENUM ('in_progress','submitted','timed_out','graded','voided');
CREATE TYPE app.media_kind     AS ENUM ('image','audio');
CREATE TYPE app.join_source    AS ENUM ('admin','join_code');

CREATE TABLE app.users (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  email         text NOT NULL,
  full_name     text NOT NULL,
  role          app.user_role NOT NULL DEFAULT 'student',  -- legacy; v0.9.1 (T-R3.1) drops it
  role_id       uuid NOT NULL REFERENCES app.roles(id) ON DELETE RESTRICT,
  password_hash text,                              -- NULL = Google-only account
  must_change_password boolean NOT NULL DEFAULT false,
  disabled_at   timestamptz,
  session_epoch integer NOT NULL DEFAULT 0 CHECK (session_epoch >= 0),
  created_by    uuid REFERENCES app.users(id) ON DELETE SET NULL,  -- NULL = unknown or self-joined
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_key ON app.users (lower(email));

ALTER TABLE app.users
  ADD COLUMN display_name text CHECK (char_length(btrim(display_name)) BETWEEN 1 AND 80),
  ADD COLUMN phone text CHECK (phone ~ '^[0-9+ ]{6,20}$'),
  ADD COLUMN avatar_key text,
  ADD COLUMN locale text CHECK (locale IN ('vi', 'en')),
  ADD COLUMN time_zone text CHECK (char_length(time_zone) BETWEEN 1 AND 64),
  ADD COLUMN preferences jsonb NOT NULL DEFAULT '{}'::jsonb
    CHECK (jsonb_typeof(preferences) = 'object')
    CHECK (octet_length(preferences::text) <= 8192);

CREATE TABLE app.user_identities (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id          uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  provider         text NOT NULL CHECK (provider IN ('google')),
  provider_user_id text NOT NULL,                  -- Google `sub`, immutable
  email_at_link    text NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_user_id)
);
CREATE INDEX ON app.user_identities (user_id);
```

**Roles and permissions** (§5, plan 70 §4), from R2:

```sql
CREATE TABLE app.permissions (
  key       text PRIMARY KEY,                      -- 'content.tests.write', ...
  group_key text NOT NULL CHECK (group_key IN ('content','teaching','people','system')),
  ordinal   smallint NOT NULL CHECK (ordinal > 0),
  in_matrix boolean NOT NULL,                      -- false for the four hidden keys
  UNIQUE (group_key, ordinal)
);

CREATE TABLE app.roles (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  builtin_key text UNIQUE CHECK (builtin_key IN ('admin','teacher','assistant','student')),
  name        text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 60),
  description text NOT NULL DEFAULT '' CHECK (length(description) <= 300),
  icon        text NOT NULL,                       -- one of the Edit role dialog's twelve
  color       text NOT NULL,                       -- one of its seven card colours
  copied_from uuid REFERENCES app.roles(id) ON DELETE SET NULL,
  revision    bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_by  uuid REFERENCES app.users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX roles_name_lower_key ON app.roles (lower(name));

CREATE TABLE app.role_permissions (
  role_id        uuid NOT NULL REFERENCES app.roles(id) ON DELETE CASCADE,
  permission_key text NOT NULL REFERENCES app.permissions(key) ON DELETE RESTRICT,
  PRIMARY KEY (role_id, permission_key)
);
```

`permissions` is plan 70 §4.1's catalogue. A migration writes the catalogue, the four built-in roles and their grants (`docs/plan/20-data-model.md` D-21). A custom role has no `builtin_key`, and no role's `builtin_key` changes. The Admin stores no grant but `learning.take_tests`: it holds every other key as the wildcard. Triggers refuse a hidden key for any role and keep `learning.take_tests` on the built-in Student, and every grant change bumps the role's `revision`. The view `app.student_like_roles` is the one definition of a strict student target (§5.4): the built-in Student, or a custom role holding nothing but `learning.take_tests`.

`users.role_id` replaces `users.role`. Until v0.9.1 (T-R3.1) both exist, and a trigger keeps them in step for the v0.7.0 binary; `role_id` is `NOT NULL … NOT VALID` until v0.9.1 validates it and drops `role` and `app.user_role`. The trigger `users_last_admin` refuses a demotion, disable or delete that would leave no active Admin. An access token carries the user's `session_epoch`, and a token below the stored value is refused; a disable and a staff password reset bump it (§5). `created_by` names the staff member who created the account; it is NULL for an account older than R2 and for a Google self-join. Details: `docs/plan/20-data-model.md` §29–§30.

**Classes and join codes** (§6):

```sql
CREATE TABLE app.classes (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  name              text NOT NULL,
  description       text,
  self_join_enabled boolean NOT NULL DEFAULT true,
  teacher_id        uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app.class_join_codes (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  class_id        uuid NOT NULL REFERENCES app.classes(id) ON DELETE CASCADE,
  code_hash       bytea NOT NULL,                  -- 1: sha256(normalized code); 2: HMAC-SHA256
  code_ciphertext bytea CHECK (code_ciphertext IS NULL OR length(code_ciphertext) = 36),
  key_id          smallint CHECK (key_id IS NULL OR key_id <> 0),
  lookup_scheme   smallint NOT NULL DEFAULT 1 CHECK (lookup_scheme IN (1, 2)),
  code_hint       text NOT NULL,                   -- last 4 chars, for the teacher's display
  expires_at      timestamptz NOT NULL,
  max_uses        integer CHECK (max_uses IS NULL OR max_uses > 0),
  uses_count      integer NOT NULL DEFAULT 0,
  revoked_at      timestamptz,
  created_by      uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (code_hash),
  CHECK ((lookup_scheme = 2) = (code_ciphertext IS NOT NULL)
     AND (lookup_scheme = 2) = (key_id IS NOT NULL))
);
-- one active code per class
CREATE UNIQUE INDEX class_join_codes_one_active
  ON app.class_join_codes (class_id)
  WHERE revoked_at IS NULL;

CREATE TABLE app.class_members (
  class_id   uuid NOT NULL REFERENCES app.classes(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  joined_via app.join_source NOT NULL,
  joined_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (class_id, user_id)
);
CREATE INDEX ON app.class_members (user_id);
```

The code is stored encrypted, so it can be read back for its teacher and for admins (D5). A code issued from v0.8.0 on is a scheme-2 row: `code_ciphertext` holds a 12-byte nonce, the 8-byte code and a 16-byte AES-256-GCM tag, sealed under `JOIN_CODE_KEY` with the class and code ids as additional data; `key_id` is the 16-bit id derived from that key; and `code_hash` is the HMAC-SHA256 of the normalized code under a lookup key derived from it. The key never reaches the database, so a dump does not hand over class access. A code issued before v0.8.0 is a scheme-1 row: its SHA-256 and no ciphertext. It still redeems, cannot be read back, and R4 rotates it (D5). `code_hash` stays the only lookup path (§6.5). Key derivation and rotation: §6.1 and `docs/plan/20-data-model.md` §33.

**Media** (§11):

```sql
CREATE TABLE app.media_assets (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  kind              app.media_kind NOT NULL,
  storage_key       text NOT NULL UNIQUE,          -- R2 object key; immutable
  mime_type         text NOT NULL,
  bytes             bigint NOT NULL CHECK (bytes > 0),
  duration_ms       integer CHECK (duration_ms IS NULL OR duration_ms > 0),
  original_filename text NOT NULL,
  checksum_sha256   bytea NOT NULL,                -- dedupe identical re-uploads
  uploaded_by       uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  owner_id          uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CHECK (kind <> 'audio' OR duration_ms IS NOT NULL)
);
CREATE INDEX ON app.media_assets (kind, created_at DESC);
```

**Question bank** — normalize; do not stuff options into `jsonb`. Options and blanks are ordered, queried, and graded against.

Migration `00032_add_option_content.sql` adds nullable `content jsonb` to
`question_options` and `test_version_options` for the bounded inline AST only.
Option identity, order, text and correctness stay relational. No grading key is
placed in the AST, and no legacy row is rewritten. Read-compatible binaries are
the rollback floor after rich writes; rolling back the migration discards rich
formatting and is only a pre-rollout development operation.

Migration `00033_add_question_content.sql` adds nullable `prompt_content jsonb`
and `explanation_content jsonb` to `questions` and `test_version_questions`.
These columns hold the bounded prose AST from §7.1; companion strings remain
exact plain projections. Migration `00034_add_question_gap_bindings.sql` enables
rich fill-blank prompts and adds nullable, question-scoped unique `gap_id` to
`question_blanks` and `test_version_blanks`. Null identifies legacy Markdown.
Bank copies and restored drafts remap gap identities and answer bindings together;
snapshots freeze them. Answer submissions remain keyed by frozen blank UUID.
Down refuses existing rich fill-blank rows instead of silently discarding bindings.
No legacy rows are rewritten, no grading keys move, and the same read-compatible rollback floor
applies after prose writes.

```sql
CREATE TABLE app.questions (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  type            app.question_type NOT NULL,
  prompt          text NOT NULL,
  media_asset_id  uuid REFERENCES app.media_assets(id) ON DELETE RESTRICT,
  media_alt       text CHECK (char_length(media_alt) BETWEEN 1 AND 1000),  -- image only; D-33
  audio_max_plays integer CHECK (audio_max_plays IS NULL OR audio_max_plays > 0),
  audio_allow_seek boolean NOT NULL DEFAULT false,
  audio_show_transcript_after boolean NOT NULL DEFAULT true,
  transcript      text,
  points          numeric(8,2) NOT NULL CHECK (points > 0),
  explanation     text,
  sample_answer   text,                            -- OQ-4; admin-only, never in a student payload
  tags            text[] NOT NULL DEFAULT '{}',
  created_by      uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  owner_id        uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE INDEX ON app.questions USING gin (tags);
CREATE INDEX ON app.questions USING gin (to_tsvector('simple', prompt));
CREATE INDEX ON app.questions (media_asset_id) WHERE media_asset_id IS NOT NULL;

CREATE TABLE app.question_options (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  question_id uuid NOT NULL REFERENCES app.questions(id) ON DELETE CASCADE,
  ordinal     smallint NOT NULL,
  text        text NOT NULL,
  is_correct  boolean NOT NULL DEFAULT false,
  UNIQUE (question_id, ordinal)
);
```

`question_blanks` and `question_blank_answers` mirror this for `fill_blank`.

**Tests and versioning** — the load-bearing decision. On publish, snapshot resolved content into version tables so editing a test can never mutate an in-flight or historical attempt:

```
tests(id, title, description, status, current_version, last_published_version, owner_id, ...)
test_versions(id, test_id, version, published_at, total_points, UNIQUE(test_id, version))
test_version_sections(id, test_version_id, ordinal, title, instructions)
test_version_questions(id, test_version_section_id, ordinal, source_question_id,
                       type, prompt, media_asset_id, media_alt, audio_max_plays, audio_allow_seek,
                       audio_show_transcript_after, transcript, points, explanation, sample_answer)
test_version_options(id, test_version_question_id, ordinal, text, is_correct)
test_version_blanks / test_version_blank_answers
```

Snapshotting into **normalized rows** rather than one `jsonb` blob keeps per-question analytics a plain SQL query later. `source_question_id` preserves the bank link without coupling to it. `media_asset_id` points at the same immutable asset — the file is never copied.

**Ownership** (D3), from R2. `tests`, `questions`, `question_groups` and `media_assets` carry `owner_id`, and `classes` carries `teacher_id`. Each is `NOT NULL REFERENCES app.users ON DELETE RESTRICT` and names the teacher the row belongs to. `created_by` (`uploaded_by` for media) stays the provenance, so a transfer moves the owner without rewriting who made the row (`docs/plan/20-data-model.md` D-22). `classes` has no `created_by`; the backfill gave every existing class the oldest active Admin, the teacher v0.7.0 showed (D-23). Assignments and Word imports belong to their `created_by`. Until v0.9.1 (T-R3.2) the owner columns are `NOT NULL … NOT VALID`, and triggers fill them for the v0.7.0 binary's inserts. Repositories scope every read and write to what the caller reaches, and another teacher's row answers as a missing one does (plan 70 §4.2). Details: `docs/plan/20-data-model.md` §31–§32.

**Per-student overrides** (R4). `assignment_student_overrides(assignment_id, student_id, closes_at, duration_minutes, extra_attempts, reason, created_by, created_at, updated_at)`, primary key `(assignment_id, student_id)`, both foreign keys `ON DELETE CASCADE`, `created_by` `ON DELETE SET NULL`. `closes_at` and `duration_minutes` are nullable and `extra_attempts` defaults to 0; a check requires at least one of the three to change something. A student's close is `greatest(least(closed_at, closes_at), override.closes_at)`: an override only ever lengthens, and one that reaches past an early close lifts it for that student alone. `shared/schedule` holds that rule as SQL and in Go, and the join that reads it.

**Assignments and attempts:**

```sql
CREATE TABLE app.attempts (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  assignment_id   uuid NOT NULL REFERENCES app.assignments(id) ON DELETE RESTRICT,
  test_version_id uuid NOT NULL REFERENCES app.test_versions(id) ON DELETE RESTRICT,
  student_id      uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  attempt_no      smallint NOT NULL CHECK (attempt_no > 0),
  status          app.attempt_status NOT NULL DEFAULT 'in_progress',
  session_id      uuid NOT NULL,                   -- takeover detection
  started_at      timestamptz NOT NULL DEFAULT now(),
  deadline_at     timestamptz NOT NULL,            -- authoritative, server-computed
  submitted_at    timestamptz,
  graded_at       timestamptz,
  score_earned    numeric(8,2),
  score_total     numeric(8,2),
  focus_loss_count integer NOT NULL DEFAULT 0,
  flagged         boolean NOT NULL DEFAULT false,
  UNIQUE (assignment_id, student_id, attempt_no),
  CHECK (deadline_at > started_at)
);
CREATE UNIQUE INDEX attempts_one_live
  ON app.attempts (assignment_id, student_id) WHERE status = 'in_progress';
CREATE INDEX ON app.attempts (assignment_id, status);
CREATE INDEX ON app.attempts (student_id, started_at DESC);

CREATE TABLE app.attempt_answers (
  attempt_id   uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  question_id  uuid NOT NULL REFERENCES app.test_version_questions(id) ON DELETE RESTRICT,
  payload      jsonb NOT NULL,
  auto_score   numeric(8,2),
  manual_score numeric(8,2),
  final_score  numeric(8,2) GENERATED ALWAYS AS (coalesce(manual_score, auto_score)) VIRTUAL,
  grader_comment text,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (attempt_id, question_id)
);

CREATE TABLE app.attempt_audio_plays (               -- §11.4
  attempt_id  uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  question_id uuid NOT NULL REFERENCES app.test_version_questions(id) ON DELETE RESTRICT,
  plays       integer NOT NULL DEFAULT 0 CHECK (plays >= 0),
  last_played_at timestamptz,
  PRIMARY KEY (attempt_id, question_id)
);

CREATE TABLE app.attempt_events (                    -- append-only, §10
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  attempt_id  uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  kind        text NOT NULL,
  occurred_at timestamptz NOT NULL,                  -- client time, offset-corrected
  received_at timestamptz NOT NULL DEFAULT now(),
  client_seq  integer NOT NULL,
  question_id uuid,
  meta        jsonb,
  UNIQUE (attempt_id, client_seq)
);
CREATE INDEX ON app.attempt_events (attempt_id, occurred_at);
```

`final_score` uses PG18 **virtual generated columns** — computed on read, no storage, never stale (the default for generated columns in 18). Grading precedence is a pure function of two columns and must never drift.

`attempt_events` uses `bigint IDENTITY` rather than a UUID: a high-volume append-only log read only by `attempt_id` is better served by a narrow sequential key. The application never updates or deletes it. An explicit privileged retention operation may delete events only after both assignment closure and server receipt are more than thirteen calendar months old, using server `received_at` and a UTC cutoff; each batch is audited. The audit log remains retained. Disabled accounts are not automatically erased. Manual requested student anonymization removes structured identity and credentials while preserving historical IDs and records; free text, old audit entries and backups require separate review (see `docs/setup/operations.md`).

### 13.4 Audit log

`audit_log(id, actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff jsonb)`. Required entries: class enrolment (§6.5), join-code generation and rotation, attempt reset/void/extend, password reset, media deletion, test publish.

Use PG18's `OLD`/`NEW` in `RETURNING` to capture the diff in the same statement as the update, rather than a read-then-write.

### 13.5 Security in the data layer

- Refresh tokens stored as SHA-256 hashes: `token_hash`, `family_id`, `user_id`, `expires_at`, `revoked_at`, `replaced_by`, `user_agent`, `ip`.
- Join codes sealed with AES-256-GCM under a key the database never holds, and found by an HMAC-SHA256 of the normalized code under a key derived from it (§13.3). Lookup by hash, compared in constant time. A code issued before v0.8.0 keeps its SHA-256 hash until R4 rotates it.
- Passwords: Argon2id (bcrypt cost ≥ 12 if unavailable).
- `sample_answer`, `transcript`, `is_correct`, and `accepted_answers` must never reach a student response. Enforce with explicit column lists — **no `SELECT *` in student-facing paths.** Add a test that asserts these keys are absent from `GET /app/attempts/:id`.
- Least-privilege DB roles: the app connects with DML on `app` only, not as owner. Migrations run as a separate role. The app role only reads `permissions`, `roles`, `role_permissions` and `student_like_roles`; R5 grants the role commands' writes. `users_last_admin` is the schema's first `SECURITY DEFINER` function: owned by the migrate role, with `search_path = pg_catalog, app, pg_temp` and EXECUTE revoked from PUBLIC (`docs/plan/20-data-model.md` §30).

### 13.6 PG18 features — use and skip

| Feature | Use? | Why |
|---|---|---|
| `uuidv7()` | **Yes**, all PKs | Time-ordered → index locality; avoids v4 fragmentation |
| Virtual generated columns | **Yes**, `final_score` | Computed on read, no storage, never stale |
| `OLD`/`NEW` in `RETURNING` | **Yes**, audit log | One statement instead of read-then-write |
| B-tree skip scan | Passive | One multicolumn index serves more shapes — do not add redundant prefix indexes preemptively |
| `NOT NULL NOT VALID` + `VALIDATE` | **Yes**, in migrations | Adds `NOT NULL` without a full-table lock |
| Async I/O | Passive | Server-side; no schema impact |
| Temporal constraints (`WITHOUT OVERLAPS`) | **No** in v1 | No business rule needs non-overlapping validity periods. Revisit for class scheduling. |
| OAuth DB authentication | **No** | That is Postgres *connection* auth, unrelated to §5's application-level Google Sign-In. Do not confuse the two. |

### 13.7 Migrations

- **goose**, SQL, forward-only, one concern per migration.
- Expand-contract for anything breaking: add nullable → backfill → start writing → `NOT NULL` → drop old. Never in one migration.
- `CREATE INDEX CONCURRENTLY` runs outside a transaction — mark those `-- +goose NO TRANSACTION`.
- Test every migration against a copy of real data before production. On Neon, a branch per migration, reset from parent between runs.
- Seed data in `seed/`, never in migrations. The exception is reference data the app cannot run without: the permission catalogue and the built-in roles (`docs/plan/20-data-model.md` D-21).

### 13.8 Query discipline

- No `SELECT *` in application code.
- Keyset pagination everywhere (`WHERE (created_at, id) < ($1,$2) ORDER BY created_at DESC, id DESC LIMIT $3`), not `OFFSET`. `uuidv7()` PKs make this natural.
- `EXPLAIN (ANALYZE, BUFFERS)` any query touching `attempts`, `attempt_answers`, or `attempt_events` before merging.
- N+1 is the default failure mode of the grading and monitor screens — one query for an attempt's answers, one query for an assignment's monitor rows.

### 13.9 Maintenance windows

`app.maintenance_windows (id, starts_at, ends_at, created_at, created_by, cancelled_at)` (migration `00053`, `docs/plan/20-data-model.md` §27).

- A window runs at most 12 hours, and windows that are not cancelled may not overlap (an exclusion constraint).
- The app role only reads the table. The operator schedules, cancels and ends windows through `cmd/maintenance`.
- Scheduling extends by the window's length every in-progress attempt that would still be running when it starts, every published, open assignment that would close inside it, and every student override of a published assignment that would close inside it. Every extension is audited as System.
- The API reads the table into a snapshot at most every 30 s, and only while requests arrive. During an active window it answers 503 (§15), and it refuses an attempt start that would run into a window.
- A start and a schedule take the same advisory lock (73819, 40), so a new attempt is either refused or extended. So do every other writer of an assignment's window or overrides: the teacher's extension, the assignment update, the reopening and the two override writes (shared, as a start does), so none of them runs between a window's scheduling and its extensions.

---

## 14. Testing & definition of done

**Unit (Vitest):** zod schemas, utils, take-test store (timer math, resume merge, submit idempotency), integrity buffering and strike counting, join-code normalization, audio plays-remaining reconciliation.

**Component (Testing Library):** each form validates and submits; guards redirect correctly; a renderer test per `QuestionType`; integrity dialog states; audio player states (idle / playing / plays exhausted / seek blocked / load error).

**E2E (Playwright + MSW or a seeded backend):**
1. Admin logs in → creates a test with one of each question type **including audio** → publishes → assigns.
2. Student logs in with **password** → starts → answers → reloads mid-test → answers persist → submits → sees result.
3. **Join flow:** anonymous visitor opens `/join/:code` → sees class name → Google sign-in (mocked GIS) → account created, enrolled, lands on `/app`.
4. **Expired code** → plain error, no account created, nothing leaked about the class.
5. Timer expiry auto-submits.
6. Tab-switch fires `tab_hidden`, warning dialog appears, event lands on the admin integrity timeline.
7. Second tab on the same attempt supersedes the first; the first goes read-only.
8. **Audio `maxPlays`:** play twice → play button disabled → **reload** → still disabled (server-authoritative).
9. Student payload assertion: `GET /app/attempts/:id` contains no `isCorrect`, `sampleAnswer`, `transcript`, or `acceptedAnswers`.

**Definition of done for any task:**
- [ ] TypeScript strict passes; no `any` without a comment
- [ ] Lint clean
- [ ] Loading / error / empty states present
- [ ] All strings via `t()`, keys in both `vi` and `en`
- [ ] Keyboard-operable
- [ ] Tests added/updated per the levels above
- [ ] Any DDL reviewed against §13 and the Neon skill
- [ ] Any new public (unauthenticated) endpoint is rate-limited and leaks nothing (§6.5)
- [ ] No new dependency without a stated reason in the PR description

---

## 15. Assumed API surface

Base `VITE_API_BASE_URL`, JSON, `Authorization: Bearer <access>`.

```
# public (unauthenticated — rate-limited, §6.5)
GET    /public/status                   → {maintenance: {startsAt, endsAt, active} | null}
                                          same answer for everyone, Cache-Control public 30 s,
                                          120/min and 2,000/h per IP
POST   /join/preview                    {joinCode} → {classId, className, teacherName}
POST   /auth/login                      {email,password} → {accessToken,user: CurrentUser}
POST   /auth/google                     {code,codeVerifier,redirectUri,joinCode?} → {accessToken,user: CurrentUser}
POST   /auth/refresh                    (cookie) → {accessToken}

# authenticated
POST   /auth/logout
GET    /auth/me                         → CurrentUser
PATCH  /auth/me                         {fullName?,displayName?|null,phone?|null,locale?,timeZone?}
                                          → CurrentUser; at least one supplied field;
                                          omitted fields unchanged; unknown zone400 VALIDATION_FAILED
PATCH  /me/preferences                 top-level UserPreferences merge → UserPreferences;
                                          nested assignmentDefaults replaces its key, {} no-op;
                                          raw/stored UTF8 cap8192 bytes,400 on excess
GET    /me/notifications?before=&limit= → {items: [Notification], nextBefore}; the caller's own,
                                          newest first (20 by default, 50 at most). Writes the caller's
                                          due-time notifications first, at most once in 5 minutes
POST   /me/notifications/read           {ids?} (1 to 100; absent means all) → 204 whatever matched
GET    /me/summary                      → {unreadNotifications}; writes the due-time notifications first
GET    /me/notification-preferences     → the five switches [{event,inApp,email}]
PUT    /me/notification-preferences     the five switches, each once → the stored switches
PUT    /me/avatar                      multipart file (PNG or JPEG, at most 2 MiB, each side 200–2048 px)
                                          → CurrentUser; stored as a 256×256 PNG, EXIF orientation applied
                                          then dropped; 413 MEDIA_TOO_LARGE, 415 MEDIA_TYPE_UNSUPPORTED |
                                          MEDIA_UNREADABLE | IMAGE_DIMENSIONS; 10 an hour per user
DELETE /me/avatar                      → CurrentUser without avatarUrl
POST   /auth/change-password            400 VALIDATION_FAILED on the rules (§5.4), 400 PASSWORD_UNCHANGED
POST   /auth/google/link                link Google to current account → CurrentUser
DELETE /auth/google/link                rejected if it would leave no login method

# teacher
GET    /teacher/dashboard?range=7d|14d|30d → Dashboard (default 14d; own teaching)
GET    /teacher/summary                → TeacherSummary {liveAssignments,answersToGrade,unreadNotifications}
GET    /teacher/tests?status=&q=&cursor=
POST   /teacher/tests | GET /:id | PATCH /:id
POST   /teacher/tests/:id/publish       → new version
POST   /teacher/tests/:id/duplicate
GET    /teacher/questions?type=&tag=&tagMatch=any|all&level=&skill=&q=&cursor=
POST   /teacher/questions | PATCH /:id | DELETE /:id
POST   /teacher/media?defaultMaxPlays=  multipart → MediaAsset (validates mime, size, duration, quota)
GET    /teacher/media?kind=&unused=&q=&cursor=   rows, facets and usage against the quota
PATCH  /teacher/media/:id               {displayName?, defaultMaxPlays?} → the library row
POST   /teacher/media/:id/replace       multipart → {asset, repointed: {questions, groups}, left: {questions, groups}}
DELETE /teacher/media/:id               409 if referenced by a published version
GET    /teacher/assignments?status=&classId=&classId=&q=&page=&limit= | POST | GET /:id | PATCH /:id
                                        rows carry questionCount; classId repeats (any of them)
                                        PATCH 409 ASSIGNMENT_LOCKED while the assignment is open and the
                                        body changes testVersionId, durationMinutes or maxAttempts
POST   /teacher/assignments/:id/extend  {minutes,notify?} → Assignment; 409 ASSIGNMENT_CLOSED when closed
POST   /teacher/assignments/:id/duplicate {classIds} → 201 Assignment, a draft; 409 TEST_NOT_PUBLISHED
GET    /teacher/assignments/:id/item-analysis → {handedIn, items:[{questionId,number,type,promptExcerpt,
                                          answered,correctRate|null}]}, hardest first
GET    /teacher/assignments/results.csv?ids=  → text/csv, BOM; 404 for any id not reached; 422 over 20000 rows
GET    /teacher/assignments/:id/student-overrides   → {items}
PUT    /teacher/assignments/:id/student-overrides   {studentIds,extendBy?|closesAt?,durationMinutes?,
                                          extraAttempts?,reason,notify?} → {items}; 422 for a student
                                          who is not a target the caller reaches; 409 ASSIGNMENT_CLOSED
                                          for extendBy on a student whose close has passed
DELETE /teacher/assignments/:id/student-overrides/:studentId
GET    /teacher/assignments/:id/attempts  → rows incl. integrity + audio summary
POST   /teacher/attempts/:id/extend     {minutes,reason}
POST   /teacher/attempts/:id/reset      {reason}
POST   /teacher/attempts/:id/void       {reason}
GET    /teacher/attempts/:id | GET /teacher/attempts/:id/events
POST   /teacher/attempts/:id/grade      {items:[{questionId,points,comment}]}
POST   /teacher/attempts/:id/finish-grading
GET    /teacher/grading/queue?mode=student|question&assignmentId=&studentId=
                                          → {groups,items,answersRemaining,studentsWaiting}; default student
GET    /teacher/students | POST | GET /:id
PATCH  /teacher/students/:id            403 FORBIDDEN when disabled is sent without
                                          people.users.manage or the subset rule refuses; 403 STUDENT_SHARED
                                          for a new email when someone else also reaches the student (§5.4)
POST   /teacher/students/:id/reset-password
                                          403 STUDENT_SHARED when someone else also reaches the student,
                                          403 FORBIDDEN when the subset rule refuses (§5.4)
GET    /teacher/classes | POST | GET /:id | PATCH /:id
DELETE /teacher/classes/:id             archived only, else 409 RESOURCE_NOT_ARCHIVED;
                                          409 RESOURCE_REFERENCED {referencedBy}
POST   /teacher/classes/:id/members | DELETE /teacher/classes/:id/members/:userId
GET    /teacher/classes/:id/join-code   → {code, hint, legacy, expiresAt, maxUses, usesCount}
                                          Cache-Control no-store; code null for a legacy code or one
                                          sealed under a key the server no longer holds; 404 without
                                          an active code
POST   /teacher/classes/:id/join-code   rotate → {code}
DELETE /teacher/classes/:id/join-code   revoke, disable self-join

# admin
DELETE /admin/users/:id                 a disabled student only, else 409 RESOURCE_NOT_ARCHIVED;
                                          403 FORBIDDEN when the subset rule refuses,
                                          409 RESOURCE_REFERENCED {referencedBy}
POST   /admin/docs-session              opens the API reference for fifteen minutes (§5.5)

# student
POST   /app/classes/join                {joinCode} → Class      (already-authed path)
GET    /app/classes
GET    /app/assignments                 → {dueNow,upcoming,completed}; a card with a live attempt
                                          carries liveAnsweredCount
GET    /app/assignments/:id
POST   /app/assignments/:id/attempts    → create or resume → Attempt + ordered questions + sessionId
                                          409 MAINTENANCE_SCHEDULED {startsAt, endsAt} for a start
                                          that would run into a window
                                          startOrResumeAttempt accepts resume; a Continue never
                                          starts an attempt (#237)
GET    /app/attempts/:id                → Attempt + questions + serverTime + audioPlays
                                       optional session names the tab's held session;
                                       a different session on a live attempt answers
                                       200 with superseded:true, that held session id,
                                       and an empty beacon token, without a write
PATCH  /app/attempts/:id/answers        {sessionId,answers:[...],events:[...]} → {savedAt, serverTime, deadlineAt}
POST   /app/attempts/:id/events         standalone flush (sendBeacon path)
POST   /app/attempts/:id/audio-play     {questionId} → {plays, maxPlays}
POST   /app/attempts/:id/submit         idempotent; 409 if already closed; 409 DEADLINE_NOT_REACHED
                                          {deadlineAt} for timer_expired more than 5 s early
GET    /app/attempts/:id/result         → Attempt + review policy + sections + questions,
                                          each question naming its sectionId
GET    /app/media/:assetId/url          → short-lived signed URL
```

**Prefixes.** `/teacher/*` holds the teaching operations, `/admin/*` the platform's, `/app/*` the student's and `/auth/*` the caller's own session and account; `/me/*` accepts only the `self` requirement. Every operation that requires a bearer token declares its permission as `x-permission` (§5). Under `/teacher/*` and `/app/*`, an id the caller does not reach answers exactly as a missing one does. The v0.7.0 paths under `/admin/*` that moved still answer until v0.9.1 (T-R3.3): the server rewrites each to its new path (`DELETE /admin/students/:id` becomes `DELETE /admin/users/:id`) and logs `legacy_admin_path`.

**`User`** adds only optional `displayName` and `avatarUrl`; it never carries
phone, locale, timeZone or preferences. Student class/intro/join-preview teacher
names use the chosen display name, falling back to full name, without a new
private field. `CurrentUser.avatarUrl` is the caller's photo as a presigned GET valid 24 hours, set by `PUT /me/avatar` and cleared by `DELETE /me/avatar` (T-R4.8); `User` leaves it unset until a list that shows other people's photos has its own task.

**`CurrentUser`** adds optional private `phone`, `locale`, `timeZone` and
`preferences` to `User`'s fields, plus `permissions`, the keys the user's role holds in catalogue order, and `workspaces`, the consoles the user may open (`teacher`, `admin`, `app`). Only a response about the caller carries it: login, Google sign-in, `GET` and `PATCH /auth/me`, and `POST /auth/google/link`. A payload about someone else, such as the student on an attempt under review, carries `User`, so one user's permissions never reach another user's payload.

`deleteClass` and `deleteUser` answer `RESOURCE_REFERENCED` with `details.referencedBy`, which names what still references the row: `assignments`, `attempts`, `audit`, `members`, `owned_content` or `other`. Later releases add values, and a client treats one it does not know as `other`. The other permanent deletes (`deleteAssignment`, `deleteTest`, `deleteTestVersion`, `deleteQuestionGroup`) answer `RESOURCE_REFERENCED` without details.

**During a maintenance window**, every route answers `503 MAINTENANCE`, with `details {startsAt, endsAt}`, `Retry-After` (seconds until the end) and a vi/en message. The exceptions are `GET`/`HEAD` `/livez`, `/healthz` and `/public/status`. The 503 comes before authentication and rate limiting, so an expired token also gets it. A `POST /auth/logout` refused this way still clears the refresh and docs cookies when it carried the refresh cookie. The family is not revoked: the device is signed out, but a copy of the token held elsewhere stays usable for as long as it is rotated, because each rotation renews the lifetime and the cleared cookie can no longer trigger reuse detection; only a password reset, a disable or a password change ends it. The same holds for a logout the limiter refuses (429) and one whose revoke fails (500).

**A request the contract does not describe** is answered in the envelope, in the caller's language and with the request id. A path nothing serves answers `404 NOT_FOUND`, without the maintenance window being read. A path the router cleans (`//auth/me`, `/a/../b`) is first redirected to the cleaned path with 307, as before. A path under a method it does not serve answers `405 METHOD_NOT_ALLOWED` with an `Allow` header naming the methods it does serve; during a window it answers the 503 as every request to that path does. `HEAD` is answered as `GET` without a body on the operations that need no token (`GET /public/status`), and on `/livez` and `/healthz` as before; on a `GET` that needs a token it answers the same 405, with or without a token. Neither answer reaches a handler or spends a rate-limit bucket.

**Body readers and refusals.** The rate limiter's body key reads first on
login, Google sign-in, join preview and in-app join; the body-limit middleware
reads next, and the contract validator reads the buffered body last. A
non-streamed body the limit middleware cannot finish reading answers
`408 REQUEST_INCOMPLETE`: the request may be sent again. The earlier body-key
reader can swallow a read error on those four operations, so a declared body
that ends early can still answer 400 instead of 408. Streamed uploads have
separate handling. Every operation taking a JSON body declares the
validator's 400 response; malformed or schema-invalid bodies are
`400 VALIDATION_FAILED`.

**The review options and the note.** `review.release` is `on_submit` or `after_close`; `review.showClassAverage` and `studentNote` are the teacher's other two choices. On `createAssignment` an omitted field takes its default (`on_submit`, false, no note); on `updateAssignment` an omitted one keeps what is stored, every other field is replaced, and `studentNote: null` clears the note. The note is trimmed by the server and stored as NULL when nothing is left; the contract counts its 500 characters before trimming. It is plain text everywhere it is drawn and is carried to a student on the assignment's detail only.

**The live lock.** While the derived status is `open` (from `opensAt` up to, not including, `closesAt`, and before an early close), `updateAssignment` refuses a change to `testVersionId`, `durationMinutes` or `maxAttempts` with `409 ASSIGNMENT_LOCKED`, whether or not anyone has started; sending the stored value is not a change. The check precedes the version check, so a scheduled or closed assignment that has attempts still answers `VERSION_LOCKED` for a changed version, and its timing stays editable.

**Release after the close.** The effective close of an assignment is the earlier of its early close and `closesAt`. Until `now` reaches it, a result whose assignment releases `after_close` is read as if the policy hid the score, the marks, the answer key and the explanations, and its grader's comments are not sent: the result's `review.showScore`, `showCorrectAnswers` and `showExplanations` are false, `review.release` and `review.showClassAverage` are as stored, and `releasesAt` is that close. A pending manual answer and a transcript keep their own gates. The student's assignment card shows no score before the same moment.

**`classAverage`** on a result is the mean, over the students who qualify, of each one's best graded attempt as earned over total, as a percent (0 to 100, rounded to two decimals). A student qualifies when they are targeted (through a class or by name) and enabled and have a graded attempt; voided, submitted-but-ungraded and timed-out-but-ungraded attempts count for nothing. It is present only when `review.showClassAverage` is on, this result is released, the assignment has closed and at least 3 students qualify, and no student is named.

**`liveAnsweredCount`** on a student's assignment card is the number of the live attempt's saved answers that say something, by the rule the engine's navigator applies (`web/src/features/take-test/answered.ts`): a choice with an option picked, a true/false with a value, a text that is not blank, and a fill-in with every blank of the frozen question filled. Blank means empty after removing the whitespace JavaScript's `trim()` removes. An answer that exists only in the browser's draft is not counted. The field is absent when there is no live attempt. The teacher's monitor applies the same rule to a row's `answeredCount`, so a saved answer the student has since cleared counts on neither screen.

**`classIds`** on a student's assignment card is every class the assignment targets that contains the student and is not archived, empty for a student targeted only by name, so a paper given to two of the student's classes can be next on both class cards; `classId` and `className` are set only when it holds exactly one class.

**The result's `sections`** are the paper's parts in test order, and every result question names its `sectionId`. Both are present under every review policy, because the attempt already showed the student its parts; the page sums a part's score from its questions' `earned`, which the policy still gates.

List endpoints return `{ items, nextCursor }` (keyset, §13.8). Student payloads never include `isCorrect`, `sampleAnswer`, `acceptedAnswers`, or `transcript` (the last only per `showTranscriptAfterSubmit`, on the result endpoint).

---

## 16. Delivery phases

Each phase ends deployable. Do not start the next with the current one red.

| Phase | Deliverable | Exit criteria |
|---|---|---|
| **0 — Scaffold** | Vite app, Tailwind, shadcn (neutral), router, i18n, API client, lint/test/CI. **Neon skill installed**; `app` schema + users/identities/classes migrations. R2 bucket + credentials. | `pnpm dev/test/build` green; `/login` renders; migrations clean |
| **1 — Auth, join, shells** | Password login, Google login, refresh + rotation, **join-code flow end-to-end**, guards, both layouts, settings | E2E 2, 3, 4 pass |
| **2 — Admin authoring** | Tests list, builder (all 5 types), question bank, **media upload + validation**, publish + version snapshot | E2E 1 passes |
| **3 — Assign & take** | Assignment create/list, student home, intro, **take-test engine**, **audio player**, **integrity capture** | E2E 5, 6, 7, 8 pass |
| **4 — Grade & results** | Monitor, attempt review + integrity timeline, grading with sample answers, result page honoring review + transcript flags | Full E2E suite green, incl. 9 |
| **5 — Hardening** | Perf budgets, a11y pass, empty/error audit, mobile QA at 360px, rate-limit verification, `EXPLAIN` review of hot queries | Lighthouse a11y ≥ 95; budgets met; no seq scans on hot paths |

P1 after v1: presigned direct-to-R2 upload, CSV import (students, questions), password self-signup with email verification, dark mode, per-student time accommodations, per-question analytics.

The next accepted milestone is [production Word exam import](quizzivy-word-import-spec.md),
implemented through [the six-stage delivery plan](plan/17-word-import.md) and
[teacher workflow](plan/18-word-import-ux.md). The supplied document defines the
target rather than current availability. Existing free-form `.docx` and `.doc`
documents, explicit answer evidence, shared context, semantic formatting,
recoverable review and atomic draft creation are required before general release.
Imported and manually authored exams must share rendering, publication and
grading contracts. Missing answers remain unknown; OCR, answer generation and
automatic publication are excluded. PDF input is limited to a text layer (D-10). Each accepted detailed
contract updates the relevant sections above and OpenAPI before implementation.

The native source intake checkpoint is configured separately from learner media.
`IMPORT_S3_BUCKET` names a separate private bucket; `IMPORT_WORK_DIR` is an absolute,
private disk directory. Both are required to enable intake. The existing S3 endpoint
and credentials are reused, with no public bucket or CDN fallback. An import, its
sources, its review and its commit belong to its creator. `scope.all` reaches every
import by id, the history lists only the caller's own (§5), and another creator's
import answers as a missing one does. The test a
commit creates, with its questions and groups, belongs to the import's creator; the
committer stays their `created_by`, the audit actor and `committed_by` (T-R2.12f).
Creator and modifying actors are retained.

Availability is reported, not assumed. `GET /teacher/imports/capabilities` answers on
every deployment:

- `intakeEnabled` is true when the import store is configured.
- `processingEnabled` is true when `IMPORT_PROCESSING_ENABLED` states that a
  worker runs beside the API. The API refuses that setting without import storage.

Without processing, `processWordImport` answers 503 `IMPORT_PROCESSING_UNAVAILABLE`
and queues nothing, so no new run waits for a worker that does not exist. A run
queued before processing was switched off waits until a worker returns or the
teacher cancels it.
Finished imports stay reviewable and committable. The client hides the feature
where intake is off. Where processing is off, it withholds new imports, retries
and reprocessing, and says why. Production runs both: O-24 was decided on 2026-09-25, and the
import worker runs there as its own Fly process group.

The R4 history presents the existing list contract as one card/grid that changes
at 960px of its outer container. Six ordered filter controls, search, page and
page size use the URL; active-import polling pauses while idle and refreshes on
resume. History returns six facets under the caller's own scope and search,
ignoring the selected status. Repeated status values are OR-ed, with a single
value still valid; Processing selects `awaiting_sources`, `queued`, `processing`
and `committing`. Each history item carries `reviewCounts {needsAction,
toConfirm}|null`, derived from the exact current review body. Machine completion,
review save and candidate adoption update counts with that body; an unadopted
candidate leaves current counts intact. An untouched historical draft remains
unknown, while computed zero is an exact zero. Facets, pagination and hydrated
items share one repeatable-read snapshot. Text sources carry their normalized
character count; filename search excludes only their stored `pasted-text.txt` name,
while title and companion-file search remain available.

The R4 upload page keeps one source-intake instance mounted across
`?source=paste` changes, preserving selected files and the typed title. T-R4.57
supplies paste editing and its real intake operation; before that release the
paste mode explicitly reports its availability and cannot start an import.
File mode retains automatic recognition, actual limits and retention, the privacy
notice and a sticky Cancel/Start footer. Leaving during upload quietly stops
intake, as before. Neither page infers provider policy from capability data.

The R4 processing page uses the deck's 720px frame and five presentation rows
mapped to the actual run stage. It retains elapsed time, attempts and retry waiting,
shows completed receipts only after their stages, and exposes no invented run-event
history. The actual exam source's role and format select the pasted-text header,
character receipt and plain-text/no-formatting receipt, independently of filename
or title. Failed text imports keep the actual error code and stage; source-invalid
and size titles and recovery instructions remain neutral about the pasted text or
its companion file. Restart opens the paste-mode destination, whose editor remains
T-R4.57. Cancellation and closed states use configured retention and finality; Ready
uses the current review's real counts, severity and conflict rules. The active-stage
spinner pauses on hover and keyboard focus and stays static under reduced motion;
retry waiting has no active spinner. This page's source title is static.

`POST /teacher/imports/{id}/sources/text` accepts a closed JSON body with
`uploadId`, `expectedRevision` and plaintext `text`, under `content.tests.write`.
It normalizes NFC and stores a private `pasted-text.txt` exam source using the
file upload's ownership, replay, revision, quota, intake-capacity and retention
rules. `ImportLimits.pasteMaxCharacters` is 100000; file `formats` stay unchanged.
The request schema rejects more than 100000 sent code points with 400. The command
rejects NUL or invalid UTF-8 with 415, and more than 100000 normalized code points
or 20000 nonblank lines with 413. Invalid UTF-8 is a command boundary assertion;
JSON decoding can replace malformed wire bytes. The default raw-body limit still
applies. A multipart `.txt` file remains unsupported.

Text extraction removes an initial BOM, normalizes line breaks and NFC, and emits
plain paragraph blocks with original line-number IDs, no formatting spans and
`TEXT_MARKS_UNAVAILABLE`. The private original and existing download, source view
and retention operations remain available. `word-pipeline-v3` fences these runs
from v2 workers; recognition remains `rules-v2` until T-R4.56. A companion answer-key
file uses the existing upload operation. T-R4.55 adds the typed client operation,
not the paste editor or T-R4.56 recognition/count corpus; T-R4.57 supplies that UI.

`/teacher/imports` creates an empty record idempotently and lists history by status,
title or current filename. A source upload accepts exactly one native `.docx`, with
bounded package/content inspection, a maximum 25 MiB compressed body and the Word
inspector's expansion/XML limits. `.doc` is accepted where `IMPORT_LEGACY_DOC` is
set. Operators set it only beside a worker that has the isolated converter; a run
without one fails with `LEGACY_CONVERSION_UNAVAILABLE`. A successful upload only
acknowledges stored source bytes.

A `.pdf` is accepted by its `%PDF-` signature. The worker reads it with PDFium
inside a WebAssembly sandbox: no filesystem, no network, 256 MiB of memory, one
minute, 60 pages. It never converts a PDF. It reads only the text layer, one
line per block. Page numbers are kept out of the exam. Lines repeated at the top
or bottom of the pages are kept out too, and the teacher confirms them, since a
repeated instruction may belong to the exam. A line holding two questions side
by side, as a two-column page produces, needs review. A PDF carries no underline,
bold or colour marks for recognition, and the draft says so in an informational
finding. A scan, a locked
file, a broken file or one over the limits fails with `PDF_NO_TEXT`,
`PDF_PROTECTED`, `PDF_INVALID` or `PDF_TOO_LARGE`, and the teacher needs another
file.

Each accepted exam/key addition or replacement creates an immutable source set,
retaining its unchanged companion and prior originals. Uploads require the current
import revision while `awaiting_sources`. Exact retries reuse their upload identity;
changed input conflicts. A durable reservation precedes each object write, and both
pending and completed bytes count towards actor/global quotas. No database transaction
is held during upload or inspection. The API admits one expanded inspection per
process. Interrupted reservations remain observable until retention removes the
import's files. Only completed sources can receive a 60-second download
URL, forced to attachment/octet-stream. Source identifiers are never media asset IDs.

Retention keeps an import's original files, artifacts and review draft as follows:

- **After commit:** 30 days, counted from the commit.
- **After cancellation:** 7 days.
- **Idle imports:** an import waiting on a teacher (`awaiting_sources`, `failed`,
  `needs_review`) that nobody has touched for 60 days, draft saves included, is
  closed as `cancelled`, flagged `closedIdle`, and its files are removed at once.
  An upload, a reservation or a draft save counts as a touch.

The API sweeps at start-up and then daily, and retries a failed sweep after an
hour. It exists wherever import storage does, with or without a worker. Each
sweep works through everything due, a batch at a time, within a five-minute
budget. An import whose removal fails is passed over for the rest of that sweep.

For each import the sweep deletes the objects first, then the draft, then sets
`files_removed_at`, and it audits the closure and the removal. Removal applies
only to committed or cancelled imports, which can never be processed again.

The history row, the source metadata, the runs and the commit record remain.
Downloads, the source view and the review answer 410 `IMPORT_FILES_REMOVED`, and
the client says the files were removed under the policy.

Native extraction retains XML order and source-bound identities for paragraph,
container, object and unassigned-content blocks. Source fragment offsets use
Unicode code points and exclude generated labels. Hidden/revised text, fields,
ancillary parts and unsupported objects remain review evidence. No formatting is
interpreted as correctness without a confirmed convention. Nested table coordinates
retain merge evidence; ambiguous grids are flagged. Selected PNG/JPEG assets may
be normalized under bounded decoding limits, but remain private until explicitly
reviewed and bound to learner content. Source blocks are separate from machine
candidate JSON and must not be exposed on student endpoints.

An internal offline converter can normalize binary DOC and render private source
pages under fixed development resource limits. Native originals remain unchanged;
converted DOCX and raster output are revalidated. Source, renderer, immutable image
and artifact identities accompany the rendition. Layout and legacy conversion
require explicit review; visual source pages do not imply a question-coordinate map.
A single Docker slot prevents orphan/retry overlap, and the container's independent
deadline remains active after worker failure. This tool is not yet public legacy
intake or production activation.

Private stage artifacts have a durable reservation before object storage, pinned
by source revision, role, run, claim and component configuration. Pending bytes
count towards separate actor/global limits. A set becomes readable only when all
its declared files are stored; completed files and sets are immutable. Current
claims may reuse completed evidence for the same source, pipeline and component,
including after an explicit retry. Incomplete evidence from an older claim is
retained for accounting but cannot be adopted by a newer worker. Conditional,
checksum-verified writes prevent changed replay from replacing stored bytes.
Source blocks and page files stay outside the bounded run-result JSON. There is
no learner access through these objects; they are removed with the import's other
files under the retention policy.

Processing requests retain their source-set revision, pipeline version and replay
identity. One queued/running request per import is permitted, with at most 50
retained requests per import and 1–10 automatic attempts per request. Claims and
heartbeats serialize capacity accounting across worker processes. Live leases
expire after a configured 1 second–5 minutes; every state/result write checks the
worker identity, fencing token, lease and source revision. Exhausted crashes become
failed runs. Retrying a failed import creates a new immutable run identity.

The offline deterministic recognizer produces `word-candidate-v1` proposals from
source-linked text, never bank questions. It distinguishes unknown, known and
conflicting choice keys; explicit option IDs survive reordering. Numbering,
sections/papers, same-line options, continuation paragraphs and explicit inline,
final or companion choice keys retain Unicode source ranges. Restarted labels and
ambiguous table associations do not authorize a guessed match. Bold/underline only
become answer evidence under an explicitly teacher-confirmed convention; key-only
marks are removed from the proposed learner prose.

Every meaningful block remains in the coverage ledger. Unassigned ranges, private
branches, uncertain structure, default points and unresolved fidelity are findings.
Source comments, hidden/revised text and fields are never automatically copied into
learner prose. This initial recognizer handles labeled choice structures; grouped
cloze, typed/written keys, saved profiles and assisted free-form recognition remain
open. A proposal is not a reviewed draft or approval to commit an assessment.

An internal runner stops cooperating processors on timeout, lease loss or shutdown.
Processing runs outside transactions. Only a current claim can store a bounded
private JSON object and transition to `needs_review`. Terminal runs cannot be
changed. Run events retain actor/worker/stage/failure codes without document contents.
The standalone `import-worker` assembles source verification, private rendition,
chunked raw extraction and deterministic recognition. It verifies object lengths
and SHA-256 before parsing, reuses completed stages pinned to source/configuration,
and stores candidate content privately. Its bounded result contains only lineage
and artifact-set identities. The API does not start this worker or require Docker.
One synchronous job per process, database lease limits, a 512 MiB Go soft-memory
target, a five-minute job deadline and the converter's separate hard limits are
conservative development defaults, not the approved production capacity envelope.
Full domain validation, public processing controls and production supervision
remain required before release.

Thuong approved two ownership/recovery policies for this milestone:

- Groups and shared materials belong to independent editable copies. Saving a
  dependent group to the bank or copying it into another test carries every
  required member/material and remaps identities; subsequent source edits or
  deletion cannot affect the copy. Relational asset references protect reused
  immutable files from deletion; reuse of bytes is not shared editable content.
- Unsent edits may be stored locally, isolated by account, for at most seven days
  from the unsent revision's creation, and cleared on logout. Recovery is explicit
  and revision-aware; local-only changes never display as server-saved. Local
  storage failure or quota exhaustion must remain visible and cannot silently
  discard unsent changes. Source/answer files are not stored in this recovery
  outbox. The group editor implements this through IndexedDB, with expiry checked
  before recovery, explicit restore/discard and a global logout fence that rejects
  older writers in other tabs. Opening a newer editor of the same item fences the
  previous local writer; it cannot overwrite or clear the newer editor's outbox.
  Section-owned group editing uses the same recovery gate and outbox. An explicit
  route exit first confirms local persistence; save-and-leave flushes both content
  and outline. Test title/outline and standalone questions are not stored locally.
  Import review and standalone authoring recovery still require integration.

---

## 17. Decisions taken inside the answers — worth a second look

All original open questions are closed. These three sub-decisions were made while implementing the answers, and are the ones most likely to want changing.

1. **Self-join is Google-only** (§6.3). Password self-signup needs verified email, which needs an email provider. If Thuong wants that in v1, add Resend/SES and an `email_verifications` table — the join flow itself is unchanged. *Decide before Phase 1.*
2. **No admin approval queue for self-join** (§6.5). Rotate-and-remove is judged sufficient for a small practice. If enrolments become high-volume or a code leaks in practice, add a `pending` membership state. *Non-blocking.*
3. **mp3 and m4a only** (§11.1). Covers every current browser with no transcoding. Adding `.ogg`/`.wav` means a transcoding pipeline or a Safari support matrix. *Decide before Phase 2.*

---

## 18. Working agreement for the AI agent

- Read this spec fully before starting any phase. Where §5, §6, §7, §10, §11, §12, or §13 is ambiguous, ask **one** precise question rather than guessing.
- Install and consult the Neon `postgres-best-practices` skill before writing any DDL or non-trivial SQL. Cite the PG18 docs section for any version-specific construct.
- Propose before adding a dependency or deviating from §2, §12, or §13.
- Any endpoint reachable without a session gets a rate limit and a leak review in the same PR.
- Small, reviewable PRs scoped to one feature folder.
- Write the `vi` string first, then `en`. Never leave English-only user-facing text.
- When touching `features/take-test/`, `features/integrity/`, or `features/media/`, run their unit tests before and after every change.
- Never remove a failing test to make CI green; fix it or flag it.
- Keep this document current: when a decision changes, edit the section and bump the version at the top.
