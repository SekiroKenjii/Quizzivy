# R4 readiness, 2026-10-07

The onboarding surveys of the tasks that were ready on 2026-10-07, against
`work/redesign-r4` at `bb4d4000`. This is a dated snapshot that informs the delegation
briefs. It is not the plan: `docs/plan/74-r4.md` stays the authority, and a task's own
"As built" supersedes its entry here. Delete this file when R4 closes.

## Backend (`senior_swe_backend`)

Read-only survey, no tests run. References are `docs/plan/74-r4.md` line numbers and paths
under `server/`. The next migration number on `work/redesign-r4` is **00092**.

### Cross-cutting

- All seven tasks edit `api/openapi.yaml`, the generated code and `permissions.golden`,
  and bump pinned literals, so their PRs conflict textually and merge serially, each
  regenerating after it brings in the integration branch:
  - `permissions_test.go:85` (operation count) and `:424` (Assistant refusals; 12 adds
    three, 20 adds one).
  - `group_authoring_test.go:63` (body budgets; 8 adds one).
  - `credential_limits_test.go:109` (20, and 18).
  - `resource_contract_test.go:13` with `openapi.yaml:63` (resource kinds; 12 adds
    `override`).
  - `isolation_world_test.go:354` `snapshotOf` (12's table).
- The `ErrorCode` enum (`openapi.yaml:479`) gains `ASSIGNMENT_LOCKED` (11),
  `ASSIGNMENT_CLOSED` (12), `IMAGE_DIMENSIONS` (8), and an unnamed code for 9's 409.
- The isolation runner requires a missing id to fail (`isolation_runner_test.go:306`) or
  an `excuse`. `/auth/*` is outside the suite. A new `/me/*` operation needs a row even
  without a uuid.
- Where a plan "Touches" list omits a file, the task below names it (ledger F-1).

### T-R4.16: publish base, usage, diff and change note (plan M, realistically L, split 16a/16b)

- **Operations.**
  - `publishTest` takes an optional `{changeNote}` and returns `testUpdatedAt`.
  - `getTest` gains `unpublishedChanges`.
  - `TestVersion` gains `assignmentCount` and `changeNote`.
  - List rows gain `assignments{live,scheduled,closed}`.
  - New `GET /teacher/tests/{id}/versions/{version}/diff?against=`
    (`workspace.teacher`, `x-resource` id=test).
- **Migration.** `test_versions.change_note`, nullable, at most 200.
- **Reuse.** `loadQuestions` (`publish_load.go:68`), `readGroupMaterials`/`Recordings`
  (`group_read.go:13-28`), `versionColumns` (`versions.go:13`).
- **A new lock-free draft loader.** `loadDraft` takes `FOR SHARE` (`publish_groups.go:13-36`),
  and PostgreSQL refuses `FOR SHARE` in a read-only transaction. The diff needs its own
  `REPEATABLE READ READ ONLY` transaction, set with `SET TRANSACTION`, because `db.Conn`
  has only `Begin` (`platform/db/context.go:19`).
- **Tests.** Diff units per kind and for a reorder; endpoint and bank-edit integration;
  `publish_snapshot_test.go` untouched; an isolation row like `deleteTestVersion`'s.
- **Open questions:**
  - Where `testUpdatedAt` lives.
  - How a change is counted.
  - Whether `assignmentCount` counts drafts, and whose assignments.
  - What `against=previous` means for version 1.

### T-R4.11: review options, note to students, live lock (plan M)

- **Operations.** `createAssignment`/`updateAssignment` gain `review.release`,
  `review.showClassAverage`, `studentNote` and a 409 `ASSIGNMENT_LOCKED`.
  `getAssignment`, `getMyAssignment` and `getAttemptResult` (`classAverage`) change.
  `maxAttempts` gets a maximum of 99.
- **Migrations.** `add_assignment_review_options` (constant defaults) and
  `add_assignments_student_note` (nullable, 1 to 500 after trim).
- **Files.**
  - `assignments`: `domain/commands.go`; `repositories/write.go:155-170`,
    `assignments.go:30`, `student.go:44`.
  - `attempts/repositories/result.go:115` (`resultRules`).
- **Tests.** The release rule, the three-student floor, the lock, the note, the leak walk,
  and the canaries at the branch point and the head. About fourteen MSW fixtures are
  swept in a mechanical commit. The web half (results page, intro) belongs to a frontend
  owner.
- **Questions for the Principal:**
  - A full-replace PATCH with optional fields must preserve an omitted field (the
    `json.RawMessage` precedent of T-R4.7), or an old tab resets them.
  - `after_close` must also gate the student card's `score` (`student.go:44`), and
    "answers" needs a definition.
  - How the client learns the release time.
  - The type of `classAverage`, and who counts toward the three.
  - The lock's precedence over `VERSION_LOCKED`.
  - One shared "effective close" helper that 12 extends (R9 reuses it, `79-r9.md:291`).

### T-R4.12: extensions and per-student overrides (plan L, realistically XL, split 12a/12b)

- **Operations.**
  - `POST /teacher/assignments/{id}/extend` (`teaching.assignments.write`).
  - `GET`/`PUT /teacher/assignments/{id}/student-overrides` and `DELETE
    …/{studentId}` (`teaching.attempts.intervene`, `x-resource-list` override, kinds
    student and override).
- **Migration.** `create_assignment_student_overrides`: composite key, `ON DELETE CASCADE`,
  an `updated_at` trigger and CHECKs. It is added to the isolation world.
- **Files.**
  - `assignments`: `schedule.go`, `student.go`, `for_student.go`.
  - `attempts`: `rulesQuery` (`store.go:34`), `CanStart`/`Deadline`
    (`support/service.go:109`, `domain/attempt.go:113`), `monitor.go`.
  - `core/maintenance/windows.go:50,107`, and `join_and_take_flow_test.go`.
- **Tests.**
  - Effective-window units.
  - A start under an override.
  - The deadline recomputed under attempt row locks, with the audit in a data-modifying
    CTE (`attempts/repositories/admin.go:20`).
  - A non-target student, isolation, the maintenance extension, reopen for one student.
  - `events_test.go` and `publish_snapshot_test.go` untouched.
- **Questions for the Principal:**
  - Does PUT merge or replace?
  - What is `extendBy`'s unit and base?
  - The effective close is `max(assignment, override)`, so an earlier override never
    restricts.
  - How does an override interact with an early close?
  - How does extend behave on a draft or a scheduled assignment?
  - The teacher must reach the student, which the suite does not catch.
  - What is the contract field for "Extended to…" on `MonitorRow`?
  - `notify` has no consumer until 10b.
  - What 422 code does a non-target student get?

### T-R4.13: duplicate, item analysis and results export (plan M)

- **Operations.**
  - `POST /teacher/assignments/{id}/duplicate` (`teaching.assignments.write`;
    `classIds`).
  - `GET …/{id}/item-analysis` (`teaching.grading`).
  - `GET /teacher/assignments/results.csv?ids=` (`teaching.grading`; streamed, BOM).
  - `listAssignments` gains `q`, `classId[]` and `questionCount`.
- **No migration.**
- **Files.**
  - `assignments`: duplicate, `narrow()`, `selectAssignment`.
  - `attempts`: item analysis, results export, `analysis.go`.
  - A new `platform/tabular`, and a zone port wired like the dashboard's
    (`adapters/zone.go`).
- **Questions:**
  - A foreign or missing id in `ids` should 404 the whole request.
  - Which attempt per student, and which class per row.
  - What status does a request over 20,000 rows get?
  - What is item analysis's denominator?
  - Duplicate copies `review_release` and `student_note`, so it follows 11.
  - What do `classId[]` semantics mean against T-R4.54?

### T-R4.8: profile photo (plan S, realistically M)

- **Operations.** `PUT /me/avatar` (multipart, `x-max-body-bytes` 2 MiB, `perActor`
  10 an hour) and `DELETE /me/avatar`, each with an isolation row. `IMAGE_DIMENSIONS` is
  a 415.
- **No migration:** `avatar_key` exists since 00085, but nothing reads or writes it.
- **Files beyond the plan.**
  - `domain.User` and its projection, `toCurrentUser` (five call sites), and wiring.
  - The storage client is built only in `wiring/media.go`, after identity, so it has to
    be shared.
  - A new `platform/imagesafe` and an identity object-store port. No new dependency.
- **Questions.** EXIF orientation is dropped, not applied. The `avatars/{userId}/` key in
  student payloads (T-R4.18's `teacherAvatarUrl`) needs a leak review.

### T-R4.9: signed-in devices (plan M)

- **Operations.** `GET /auth/sessions`, `DELETE /auth/sessions/{familyId}` and
  `POST /auth/sessions/revoke-others`. They are outside the isolation suite, so they need
  their own 404 test.
- **Migration.** `refresh_tokens.geo_label`, nullable, at most 80.
- **Files.** `identity`'s `session.go` and `refresh_store.go` (`issueSuccessor` copies
  the label), a new `platform/uaparse`, and a location-header middleware beside
  `WithRefreshCookie` (`router.go:88-89`).
- **Session effects.** Each revoke bumps the caller's epoch and calls `Forget`. The
  caller's own tab then recovers through the single-flight refresh, so
  `client.refresh.test.ts` stays untouched.
- **Questions.**
  - The 409's error code is unnamed.
  - The geo label is trusted only behind `CF-Connecting-IP`.
  - The location shown is where the user first signed in, because rotation copies the
    label.

### T-R4.20: students, bulk reset and filters (plan S)

- **Operations.**
  - `POST /teacher/students/reset-passwords` (`people.students.reset_password`; 2 a
    minute and 10 an hour per actor; joins `theCredentialMinters`).
  - `listStudents` gains `classId[]` and `mustChangePassword`.
- **Reuse.** `ResetPassword` and the unshared rule (`students_write.go:171,216`), and
  `hashSlots`.
- **Tests.**
  - `STUDENT_SHARED` lands in `failed`, and a foreign id answers `NOT_FOUND`.
  - The reset token dies, and the rate limit holds.
  - The isolation row needs an `excuse` (the missing-id case answers 200).
- **Questions.**
  - Is `classId[]` OR or AND?
  - If the client drops mid-request, committed resets lose their one-time passwords:
    decide whether the work detaches from the request context.

### Collisions and order

- **11, 12 and 13** share `assignments/repositories/{assignments,student,write}.go`, the
  HTTP handlers, the domain and the `Assignment` schemas: serialize them. 13's attempts half
  is file-disjoint from 11, 12 and 16.
- **16** stands alone; 62 follows it.
- **8, 9 and 20** share `identity/application/app.go`, `ratelimits.go` and
  `credential_limits_test.go`. They can be built in parallel worktrees but merge one at a
  time, and each later one regenerates after bringing in the integration branch.
- **Parallel worktrees are safe** for 16, 11, 20 and 9. The contract, generated code and
  pinned tests always merge serially.
- **Inputs to the schedule** (the one schedule is the ledger's): 11 introduces
  `EffectiveClose`, which 12 and 10b extend; 16 unblocks 62, 29, 30 and 31b; 13's
  attempts half is file-disjoint and can fill a gap.

## Frontend (`senior_swe_frontend`)

Read-only survey; only `git fetch` and diffs were run. References are paths under `web/`
and deck line numbers in `Quizzivy Teacher.dc.html`.

### The teacher tree today

- `layouts/TeacherShell.tsx` renders `TeacherLayout` (inside the deck scale) when the leaf
  route has a teacher handle, and `AdminLayout` otherwise. `AdminLayout` still forces
  light (`AdminLayout.tsx:88`).
- **Rebuilt to the deck:** the Dashboard, the imports list, new import and import run,
  and the assignment detail. **Still old:** tests, test detail, builder, import review,
  bank, groups, question editor, media, assignments list, new and edit, attempts,
  grading, students, classes and settings.
- **Composites available:**
  - `components/shared/data/` (`DataTable`, `Pager`, `FacetFilter`), `CardGrid`,
    `BulkActions`.
  - `form/` (`FormDialog`, `DialogShell`, `ChipInput`, `TagCombobox`, `NumberStepper`,
    `RadioCard`, `FileDrop`, fields).
  - `Sheet`, `CopyField`, `DirtyBar`, `SettingsLayout`, `ConfirmDialog`.
  - `stats/` (`KpiTile`, `StatStrip`, `ProgressBar`, `Meter`), `charts/BarChart`,
    `IconTile`, `Callout`, `LockNotice`, `EventList`, `SplitPane`, `Waveform`.
  - The hooks `useIdlePolling`, `useFileDrop`, `usePage`; `layouts/shell/` (`PageHead`,
    `useCrumbs`, `useContentWidthAtLeast`, `handle`); `ui/segmented`, `RowMenu`.
  - Test support: `tests/units/shell/support.tsx`.
- **Content widths** (sidebar 248 expanded or 60 collapsed, side padding 28): 464 or 652
  at 768, 720 or 908 at 1024, 976 or 1164 at 1280, 1136 or 1320 at 1440, and 332 in the
  drawer at 360.

### T-R4.63: content editor frame (plan M, realistically M+)

- **Outcome.** `ContentEditor` gets the deck's box, toolbar, link popover, table row,
  notice band, word count and `readOnly`. Pasted images are left out and counted.
- **References.** Deck 742-760 and 813, script 3955-4030; DG-30, 35, 36, 109, 110, 115,
  116.
- **Files.**
  - `components/shared/content/editor/` (`ContentEditor`, `ContentToolbar`,
    `PastePreview`).
  - New `TableToolbar`, `LinkPopover`, `NoticeBand`.
  - `clipboardHTML.ts`, `useContentPaste.ts`, `extensions.ts`, `content.css`.
  - The content unit tests, and `e2e/rich-question-content.spec.ts`.
- **API.** None. i18n under `contentEditor`.
- **Open points.**
  - "Touches" omits the six hosts. The recommendation is that 63 ships the new props
    with defaults and edits only the group and option hosts.
  - Every host is still an old, forced-light screen behind `VITE_RICH_QUESTION_EDITOR`,
    so the dark and five-width check needs a deck-harness case (`cases/r4-63.tsx`).
  - DG-115 is an unconfirmed default.
- **Accessibility.** The toolbar becomes one tab stop with roving arrow keys. The link
  popover traps focus and returns it on Esc. A selection survives a touch press.
- **Canaries.** `router-chunks` stays untouched.

### T-R4.57: paste a test (plan M, realistically L)

- **Outcome.** Paste mode replaces the deferred placeholder in `NewImportPage.tsx:66-70`.
- **References.** Deck 1595-1701; DG-12, 14, 15, 17, 21.
- **API.** Everything exists. The 46-case count fixture is already in CI's web set.
  `TEXT_MARKS_UNAVAILABLE` already has copy in both languages.
- **Size.** `paste.ts` ports part of a roughly 1,460-line Go grammar, and each rule is
  mutation-tested, so this is L.
- **Open points.**
  - The crumb can come from `PageHead`, which avoids editing `router.tsx` and
    `teacher-routes.test.ts` and so the collision with 27a.
  - "Tips beside the example from 900px" should be a container query.
  - `[&_button]:min-w-[168px]` (`NewImportPage.tsx:62`) contradicts DG-21.
- **Accessibility.** Ctrl or ⌘+Enter never fires during IME composition, and leaving asks
  first.

### T-R4.35: media library (plan M, realistically M+)

- **Outcome.** The deck's Media page, the Upload, Replace and Rename dialogs, and
  `useMediaUpload`.
- **References.** Deck 1168-1202, script 3383-3388 and 3484; DG-09, 63, 69, 83.
- **Files.**
  - `MediaLibraryPage` becomes `MediaPage` (`router.tsx:148`, `teacher-routes.test.ts:40`).
  - New `MediaCard`, `UploadDialog`, `ReplaceDialog`, `useMediaUpload.ts`; `UploadPanel`
    is retired.
  - The upload call in `QuestionMediaField` and `MaterialAssetDialog`.
  - `limits.ts`, `probe.ts`, and `media/api.ts`, which lacks `q`, `unused`,
    `replaceMedia` and `updateMedia`.
- **Contract gap.** "Download": `LibraryAsset.url` is a cross-origin signed URL, so
  `<a download>` cannot name the file. Either it opens in a tab, or the backend adds a
  disposition.
- **Canaries.** No `router-chunks` edit is needed: `ADMIN` already covers all of
  `features/media/`. Do not export the hook from `features/media/index.ts`, which
  `ResultPage` imports. `audio-player.test.tsx` stays untouched, and the media card's
  play button needs its own same-tick test.
- **Open points.**
  - The two former `UploadPanel` hosts need their own progress and error UI.
  - The image pre-checks are new client logic.
  - DG-63 and DG-09 override the deck's copy.

### T-R4.32: question bank (plan M)

- **References.** Deck 477-549, script 4197-4296.
- **Files.** `QuestionBankPage.tsx` (648 lines), a handle in `router.tsx:117`, and
  `tests/units/question-bank/`.
- **Contract gaps.**
  - No count for "Audio", although the plan asks for server counts.
  - No per-tag counts, which the deck's popover shows.
  - No operation to remove a tag from a row: `tagQuestions` only adds.
- **Decision left to this task by T-R4.1a.** The column thresholds: content width (the
  deck's, which overflows by up to 62px at 1024 expanded beside the 220px aside) or the
  table's own width.
- **Level groups.** The deck groups "A1–A2"; the plan wants Pre-A1 to C2.

### T-R4.27a: wizard frame, test and students (plan M, realistically M+)

- **References.** Deck 854-940; DG-65.
- **Files.**
  - New `AssignmentWizardPage` and `WizardStepper`, replacing the 925-line
    `AssignmentFormPage`.
  - `router.tsx:157,177` and `teacher-routes.test.ts:42,50`.
  - The four `?testId`/`?classId` link sites.
  - `e2e/support/live.ts:123-145` and `admin-change-requests.live.spec.ts:125-155`.
- **Main open point.** 27a ships steps 1 and 2, while Schedule, Rules and the final
  Assign are 27b, which waits for T-R4.11. Between them the integration branch would lose
  scheduling, rules, the §10.5 honest-limits text and the Assign that `assignToClass`
  drives. Proposal: 27a mounts today's schedule and rules groups as interim steps 3 and
  4, keeps a final Assign, and keeps `/:id/edit` working for published assignments.

### T-R4.46: integrity on the teacher side (plan S, high-risk)

- **Mostly built already.** The Dashboard's Flagged tile, the roster's flag tone, and
  `KIND_LOOK` flagged. `FocusLossCell` is dead code.
- **What remains.**
  - Consolidate the tones in `integrity/tones.ts`.
  - Colour the compact timeline's dots (`Timeline.tsx:99,108`).
  - There is no "submitted" event (it comes from the attempt as a prop), and no
    autosave event behind the deck's "Answers autosaved".
- **For the Principal.** Danger dots against spec §10.4 ("neutral text"); AGENTS.md D8
  appears to settle it.

### Collisions and order

- **#414** shares `router.tsx`, the `builder` locale namespace, `MarqueeText.tsx`,
  `index.css` and separate hunks of `admin-change-requests.live.spec.ts`. **#416** shares
  `router.tsx:192` and `teacher-routes.test.ts:52`, one line from 27a's and 35's rows.
- **Inputs to the schedule** (the one schedule is the ledger's). With one frontend
  engineer the order is 63 (the critical path), then 35, 46, 32, 57 and 27a. If a second
  implementer joins, 63, 46 and 35 are file-disjoint and can run in separate worktrees.
  27a goes after 35, whose rows sit beside its own in `router.tsx` and
  `teacher-routes.test.ts`, and after the interim-steps question is settled.
- **Serialize** anything editing `router.tsx` or `teacher-routes.test.ts`. Locale
  namespaces merge one at a time.

## Decisions the surveys leave open

For the Principal (approach) or the user (product), before the task that needs them:

1. T-R4.11 and 12: omitted-field preservation on PATCH, one `EffectiveClose`, how an
   override meets an early close, the PUT semantics.
2. T-R4.16: the change count, `assignmentCount`, `against=previous` for version 1.
3. T-R4.27a: the interim steps 3 and 4.
4. T-R4.32: the three contract gaps and the column thresholds.
5. T-R4.35: Download.
6. T-R4.46: danger tones on the compact timeline.
7. DG-115 and DG-110 (user).
