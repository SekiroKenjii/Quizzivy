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
- Several plan "Touches" lists omit files; they are named below.

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
  - The plan's "new DG entry" already exists as DG-71.

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
  - The first Done-when bullet has already shipped.
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
  `credential_limits_test.go`. 9 and 20, and 8 and 20, can run in parallel; 8 and 9 merge in
  sequence.
- **Parallel worktrees are safe** for 16, 11, 20 and 9. The contract, generated code and
  pinned tests always merge serially.
- **Recommended order:**
  - Wave 1: 11 (it introduces `EffectiveClose`), 16, 20, 9.
  - Wave 2: 12a then 12b, 8, and 13's attempts half.
  - Wave 3: 13's assignments half, 10b, 18, 62.
- The Principal's wave plan (`ledger.md`) starts with 16 and runs 11 → 12 → 13 serially.
  Both agree that the assignments cluster is serial. The choice of first task is the
  Tech Lead's when the objective is set.

## Frontend (`senior_swe_frontend`)

Pending.
