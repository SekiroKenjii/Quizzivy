repo: SekiroKenjii/Quizzivy
branch: main
path: web/src

## Last sync
date: 2026-09-26T01:58:51Z
branch read: main (Word/PDF import release)

### Updated in this project
- Import renamed "Import from Word/PDF"; upload accepts .docx, .doc, .pdf with the release's limits text and PDF errors
- Upload slots match SourceIntake (exam + answer key), with a test title field and recognition note
- PDF import examples: review with PDF layout findings, failed run with PDF_NO_TEXT and error code
- History adds the Cancelled filter

## Sync history
- 2026-09-25T09:05:37Z · develop · API reference, test versions, Word import

## Screen map
| Screen | Repo files |
|---|---|
| Admin console (Quizzivy Admin.dc.html) | migrations/00004_create_users.sql, migrations/00008_create_audit_log.sql |
| Student app (Quizzivy Student.dc.html) | web/src/app/router.tsx (studentTree, takeTestTree), web/src/layouts/StudentLayout.tsx |
| System pages (Quizzivy System pages.dc.html) | web/src/app/pages/errorArt.tsx, web/src/app/pages/ErrorScreen.tsx, web/src/app/ErrorBoundary.tsx |
| Sign in (Quizzivy Sign in.dc.html) | web/src/app/router.tsx (authTree, publicTree) |
| Shell (sidebar, top bar, ⌘K) | web/src/layouts/AdminLayout.tsx, web/src/features/search/CommandPalette.tsx |
| Dashboard | web/src/app/pages/AdminDashboardPage.tsx |
| Assignments | web/src/features/assignments/pages/AssignmentsListPage.tsx |
| Assignment detail | web/src/features/assignments/pages/AssignmentDetailPage.tsx, web/src/features/attempts/components/Monitor.tsx |
| Grading | web/src/features/attempts/pages/GradingQueuePage.tsx, web/src/features/attempts/components/GradeByQuestion.tsx |
| Tests | web/src/features/tests/pages/TestsListPage.tsx |
| Test builder | web/src/features/tests/pages/TestBuilderPage.tsx, web/src/features/tests/components/OutlineTree.tsx |
| New assignment | web/src/features/assignments/pages/AssignmentFormPage.tsx, migrations/00018_create_assignments.sql |
| Question bank | web/src/features/question-bank/pages/QuestionBankPage.tsx |
| Students | web/src/features/students/pages/StudentsListPage.tsx |
| Classes | web/src/features/classes/pages/ClassesListPage.tsx |
| Class detail | web/src/features/classes/pages/ClassDetailPage.tsx, components/JoinCodePanel.tsx, ClassSettingsCard.tsx, ClassAssignmentsCard.tsx |
| Word/PDF import (Teacher: imports, importNew, importRun, importReview, importConfirm) | web/src/features/imports/pages/*, components/SourceIntake.tsx, FileSlot.tsx, ProcessingPanel.tsx, status.ts, findings.ts, locales/en.json (imports.*), docs/plan/18-word-import-ux.md |
| Test detail · versions (Teacher: testDetail) | web/src/features/tests/pages/TestDetailPage.tsx, web/src/features/tests/api.ts (develop) |
| Admin API reference (Admin: apiref) | web/src/features/auth/components/SettingsSections.tsx (ApiDocsSection), SettingsPage.tsx (develop) |
| Question editor | web/src/features/question-bank/pages/QuestionEditorPage.tsx, components/QuestionEditor.tsx, questionSchema.ts |
