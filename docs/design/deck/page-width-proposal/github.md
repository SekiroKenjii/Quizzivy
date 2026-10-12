repo: SekiroKenjii/Quizzivy
branch: main
path: web/src

## Last sync
date: 2026-10-12T13:31:27Z

### Updated in this project
- Page-width proposal: one 1600px frame across eight teacher screens, 360–2560px
- Unified pager (DG-203), focus rings, AI panel missing states (DG-201)

## Screen map
| Project screen | Repo files |
| --- | --- |
| Page Width Proposal · shell | web/src/layouts/TeacherLayout.tsx, layouts/shell/Sidebar.tsx, TopBar.tsx, Breadcrumbs.tsx, WorkspaceSwitcher.tsx, PageHead.tsx, layouts/teacherNav.ts, layouts/shell/handle.ts, app/router.tsx, index.css |
| Dashboard | features/dashboard/pages/teacher/TeacherDashboardPage.tsx, components/shared/stats/KpiTile.tsx, ProgressBar.tsx, charts/BarChart.tsx |
| Assignments | features/assignments/pages/teacher/AssignmentsListPage.tsx, components/ui/segmented.tsx, components/shared/data/Pager.tsx |
| New assignment | features/assignments/components/WizardStepper.tsx |
| Settings | features/settings/pages/teacher/TeacherSettingsPage.tsx, components/shared/SettingsLayout.tsx |
| Import upload / processing | features/imports/pages/teacher/NewImportPage.tsx, features/imports/components/ProcessingPanel.tsx, components/shared/Callout.tsx |
| Imports history | docs/design/gaps.md (DG-203, work/deck-d4), components/shared/data/Pager.tsx |
| Copy | web/src/lib/i18n/locales/vi.json, en.json |
