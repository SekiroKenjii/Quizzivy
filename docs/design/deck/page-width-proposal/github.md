repo: SekiroKenjii/Quizzivy
branch: work/deck-d4
path: web/src

## Last sync
date: 2026-10-13T10:20:00Z

### Updated in this project
- Grading: empty queue (all graded / papers to finish) and "Ready to finish" card at 1280/768/360
- Answers: Review on the row (Finish inside the check); card folded below panes while answers wait; grouped by assignment

## Screen map
| Project screen | Repo files |
| --- | --- |
| Grading Finish States · page, card, check | web/src/features/attempts/pages/teacher/GradingPage.tsx, GradingQueueAside.tsx, gradingRecovery.ts, useGradingQueue.ts, features/attempts/gradingMessages.ts, components/shared/ListState.tsx, api/openapi.yaml |
| Cloze Decision · strip and squares | web/src/features/take-test/components/Navigator.tsx, docs/design/deck/Quizzivy Student.dc.html (rail, sheet) |
| Cloze Decision · gap card (take test) | web/src/features/take-test/components/QuestionSheet.tsx, take-test/worth.ts, docs/plan/74d-d4-deck-update.md (T-D4.10, S-12, S-13) |
| Cloze Decision · result | web/src/features/results/pages/ResultPage.tsx, results/components/ReviewItem.tsx, features/media/components/ReviewGroup.tsx |
| Cloze Decision · copy | web/src/lib/i18n/locales/en.json, vi.json |
| Page Width Proposal · shell | web/src/layouts/TeacherLayout.tsx, layouts/shell/Sidebar.tsx, TopBar.tsx, Breadcrumbs.tsx, WorkspaceSwitcher.tsx, PageHead.tsx, layouts/teacherNav.ts, layouts/shell/handle.ts, app/router.tsx, index.css |
| Dashboard | features/dashboard/pages/teacher/TeacherDashboardPage.tsx, components/shared/stats/KpiTile.tsx, ProgressBar.tsx, charts/BarChart.tsx |
| Assignments | features/assignments/pages/teacher/AssignmentsListPage.tsx, components/ui/segmented.tsx, components/shared/data/Pager.tsx |
| New assignment | features/assignments/components/WizardStepper.tsx |
| Settings | features/settings/pages/teacher/TeacherSettingsPage.tsx, components/shared/SettingsLayout.tsx |
| Import upload / processing | features/imports/pages/teacher/NewImportPage.tsx, features/imports/components/ProcessingPanel.tsx, components/shared/Callout.tsx |
| Imports history | docs/design/gaps.md (DG-203, work/deck-d4), components/shared/data/Pager.tsx |

## Sync history
- 2026-10-13T09:15:52Z (work/deck-d4): Cloze decision (Q6)
- 2026-10-12T13:31:27Z (main): Page-width proposal, unified pager (DG-203), focus rings, AI panel missing states (DG-201)
