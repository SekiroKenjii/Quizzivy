repo: SekiroKenjii/Quizzivy
branch: work/deck-d4
path: web/src

## Last sync
date: 2026-10-13T09:15:52Z

### Updated in this project
- Cloze decision (Q6, DG-184/185): one question per gap, shown as one screen
- Drawings: linked squares, gap card with own prompts, result rows, run cut by media; en/vi copy

## Screen map
| Project screen | Repo files |
| --- | --- |
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
- 2026-10-12T13:31:27Z (main): Page-width proposal, unified pager (DG-203), focus rings, AI panel missing states (DG-201)
