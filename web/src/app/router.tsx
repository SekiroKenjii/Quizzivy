import { createBrowserRouter, redirect, type RouteObject } from "react-router";
import { ADMIN_PATHS } from "@/app/adminPaths";
import { ErrorBoundary } from "@/app/ErrorBoundary";
import NotFoundPage from "@/app/pages/NotFoundPage";
import ForbiddenPage from "@/app/pages/ForbiddenPage";
import { RequireSession } from "@/app/guards/RequireSession";
import { RequirePermission } from "@/app/guards/RequirePermission";
import {
  AdminWorkspace,
  StudentArea,
  TeacherWorkspace,
} from "@/app/guards/RequireWorkspace";
import { HomeRedirect } from "@/app/guards/HomeRedirect";
import {
  AssignmentPapersRedirect,
  LegacyTeacherRedirect,
} from "@/app/LegacyTeacherRedirect";

/**
 * §3's four route trees: public, /teacher for the teacher, /admin for the
 * Admin console and /app for students.
 *
 * Guards are pathless routes so the tree structure states who may see what,
 * rather than each page re-checking.
 */

/** `lazy` for a module whose default export is the route component. */
const page = (load: () => Promise<{ default: React.ComponentType }>) => async () => ({
  Component: (await load()).default,
});

/**
 * The signed-out screens. Each owns the whole viewport and draws its own brand
 * frame, so there is no shared layout route above them.
 */
const authTree: RouteObject = {
  children: [
    { path: "login", lazy: page(() => import("@/features/auth/pages/LoginPage")) },
    {
      path: "forgot-password",
      lazy: page(() => import("@/features/auth/pages/ForgotPasswordPage")),
    },
    {
      path: "set-password",
      lazy: page(() => import("@/features/auth/pages/SetPasswordPage")),
    },
    {
      path: "auth/google/callback",
      lazy: page(() => import("@/features/auth/pages/GoogleCallbackPage")),
    },
    { path: "join", lazy: page(() => import("@/features/join/pages/JoinPage")) },
    { path: "join/:code", lazy: page(() => import("@/features/join/pages/JoinPage")) },
    {
      path: "join/:code/confirm",
      loader: ({ params }) =>
        redirect(`/join/${encodeURIComponent(params["code"] ?? "")}`),
    },
  ],
};

const teacherTree: RouteObject = {
  path: "teacher",
  element: <TeacherWorkspace />,
  children: [
    {
      lazy: page(() => import("@/layouts/TeacherLayout")),
      children: [
        {
          index: true,
          handle: { crumb: [{ key: "teacherShell.nav.dashboard" }] },
          lazy: page(
            () => import("@/features/dashboard/pages/teacher/TeacherDashboardPage"),
          ),
        },
        {
          path: "tests",
          handle: { crumb: [{ key: "teacherShell.nav.tests" }] },
          lazy: page(() => import("@/features/tests/pages/teacher/TestsListPage")),
        },
        {
          path: "tests/:id",
          handle: {
            crumb: [
              { key: "teacherShell.nav.tests", to: "/teacher/tests" },
              { key: "tests.detail.crumb" },
            ],
          },
          lazy: page(() => import("@/features/tests/pages/teacher/TestDetailPage")),
        },

        {
          path: "tests/:id/edit",
          handle: {
            crumb: [
              { key: "teacherShell.nav.tests", to: "/teacher/tests" },
              { key: "builder.titleLabel" },
            ],
          },
          lazy: page(() => import("@/features/tests/pages/teacher/TestBuilderPage")),
        },
        {
          path: "imports",
          lazy: page(() => import("@/features/imports/pages/teacher/ImportsGate")),
          children: [
            {
              index: true,
              handle: {
                crumb: [
                  { key: "teacherShell.nav.tests", to: "/teacher/tests" },
                  { key: "imports.historyTitle" },
                ],
              },
              lazy: page(
                () => import("@/features/imports/pages/teacher/ImportsListPage"),
              ),
            },
            {
              path: "new",
              handle: {
                crumb: [
                  { key: "teacherShell.nav.tests", to: "/teacher/tests" },
                  { key: "imports.historyTitle", to: "/teacher/imports" },
                  { key: "imports.newCrumb" },
                ],
              },
              lazy: page(
                () => import("@/features/imports/pages/teacher/ImportWorkspacePage"),
              ),
            },
            {
              path: ":id",
              handle: {
                crumb: [
                  { key: "teacherShell.nav.tests", to: "/teacher/tests" },
                  { key: "imports.historyTitle", to: "/teacher/imports" },
                  { key: "imports.detail.pageTitle" },
                ],
              },
              lazy: page(
                () => import("@/features/imports/pages/teacher/ImportWorkspacePage"),
              ),
            },
            {
              path: ":id/review",
              handle: {
                crumb: [
                  { key: "teacherShell.nav.tests", to: "/teacher/tests" },
                  { key: "imports.historyTitle", to: "/teacher/imports" },
                  { key: "imports.review.crumb" },
                ],
                width: "full",
                sidebar: "collapsed",
              },
              lazy: page(
                () => import("@/features/imports/pages/teacher/ImportReviewPage"),
              ),
            },
            {
              path: ":id/confirm",
              handle: {
                crumb: [
                  { key: "teacherShell.nav.tests", to: "/teacher/tests" },
                  { key: "imports.historyTitle", to: "/teacher/imports" },
                  { key: "imports.confirm.crumb" },
                ],
              },
              lazy: page(
                () => import("@/features/imports/pages/teacher/ImportConfirmPage"),
              ),
            },
          ],
        },
        {
          path: "question-bank",
          handle: { crumb: [{ key: "teacherShell.nav.questionBank" }] },
          lazy: page(
            () => import("@/features/question-bank/pages/teacher/QuestionBankPage"),
          ),
        },
        {
          path: "question-bank/groups",
          handle: {
            crumb: [
              { key: "teacherShell.nav.questionBank", to: "/teacher/question-bank" },
              { key: "groups.bankTitle" },
            ],
          },
          lazy: page(
            () => import("@/features/question-groups/pages/teacher/GroupsListPage"),
          ),
        },
        {
          path: "question-bank/groups/:id",
          handle: {
            crumb: [
              { key: "teacherShell.nav.questionBank", to: "/teacher/question-bank" },
              { key: "groups.bankTitle", to: "/teacher/question-bank/groups" },
              { key: "groups.one" },
            ],
          },
          lazy: page(
            () => import("@/features/question-groups/pages/teacher/GroupEditorPage"),
          ),
        },
        {
          path: "question-bank/new",
          handle: {
            crumb: [
              { key: "teacherShell.nav.questionBank", to: "/teacher/question-bank" },
              { key: "questionEditor.newTitle" },
            ],
          },
          lazy: page(
            () => import("@/features/question-bank/pages/teacher/QuestionEditorPage"),
          ),
        },
        {
          path: "question-bank/:id",
          handle: {
            crumb: [
              { key: "teacherShell.nav.questionBank", to: "/teacher/question-bank" },
              { key: "questionEditor.editTitle" },
            ],
          },
          lazy: page(
            () => import("@/features/question-bank/pages/teacher/QuestionEditorPage"),
          ),
        },
        {
          path: "media",
          handle: { crumb: [{ key: "media.title" }] },
          lazy: page(() => import("@/features/media/pages/teacher/MediaPage")),
        },
        {
          path: "assignments",
          handle: { crumb: [{ key: "teacherShell.nav.assignments" }] },
          lazy: page(
            () => import("@/features/assignments/pages/teacher/AssignmentsListPage"),
          ),
        },
        {
          path: "assignments/new",
          handle: {
            crumb: [
              { key: "teacherShell.nav.assignments", to: "/teacher/assignments" },
              { key: "assignments.new" },
            ],
          },
          lazy: page(
            () => import("@/features/assignments/pages/teacher/AssignmentWizardPage"),
          ),
        },
        {
          path: "assignments/:id",
          handle: {
            crumb: [
              { key: "teacherShell.nav.assignments", to: "/teacher/assignments" },
              { key: "assignmentDetail.title" },
            ],
          },
          lazy: page(
            () => import("@/features/assignments/pages/teacher/AssignmentDetailPage"),
          ),
        },
        {
          path: "assignments/:id/edit",
          handle: {
            crumb: [
              { key: "teacherShell.nav.assignments", to: "/teacher/assignments" },
              { key: "assignments.edit" },
            ],
          },
          lazy: page(
            () => import("@/features/assignments/pages/teacher/AssignmentWizardPage"),
          ),
        },
        {
          path: "assignments/:id/attempts",
          element: <AssignmentPapersRedirect />,
        },
        {
          path: "attempts/:id",
          handle: {
            crumb: [
              { key: "teacherShell.nav.assignments", to: "/teacher/assignments" },
              { key: "assignmentDetail.title" },
              { key: "review.title" },
            ],
          },
          lazy: page(
            () => import("@/features/attempts/pages/teacher/AttemptReviewPage"),
          ),
        },
        {
          path: "grading",
          handle: { crumb: [{ key: "teacherShell.nav.grading" }] },
          lazy: page(() => import("@/features/attempts/pages/teacher/GradingPage")),
        },
        {
          path: "students",
          handle: { crumb: [{ key: "teacherShell.nav.students" }] },
          lazy: page(() => import("@/features/students/pages/teacher/StudentsPage")),
        },
        {
          path: "classes",
          handle: { crumb: [{ key: "teacherShell.nav.classes" }] },
          lazy: page(() => import("@/features/classes/pages/teacher/ClassesPage")),
        },
        {
          path: "classes/:id",
          handle: {
            crumb: [
              { key: "teacherShell.nav.classes", to: "/teacher/classes" },
              { key: "classDetail.crumb" },
            ],
          },
          lazy: page(() => import("@/features/classes/pages/teacher/ClassDetailPage")),
        },
        {
          path: "settings/:section?",
          handle: { crumb: [{ key: "nav.settings" }] },
          lazy: page(
            () => import("@/features/settings/pages/teacher/TeacherSettingsPage"),
          ),
        },
      ],
    },
  ],
};

const ADMIN_CRUMB = { key: "adminShell.crumb" } as const;

const pending = page(() => import("@/features/admin-overview/pages/admin/PendingPage"));

/**
 * The Admin console (T-R5.20): behind the admin workspace, then the
 * permission each route's handle names, in `AdminLayout`. Every route
 * declares `crumb`, `width` (the page frame's maximum, 1320px, or 1080px for
 * API reference and Settings) and `permission`, the `x-permission` of the
 * operation its page opens with. The screens are R5's later tasks; each takes
 * its route over from `pending`.
 */
const adminTree: RouteObject = {
  path: "admin",
  element: <AdminWorkspace />,
  children: [
    {
      element: <RequirePermission />,
      children: [
        {
          lazy: page(() => import("@/layouts/AdminLayout")),
          children: [
            {
              index: true,
              handle: {
                crumb: [ADMIN_CRUMB, { key: "adminShell.nav.overview" }],
                permission: "workspace.admin",
                width: 1320,
              },
              lazy: page(
                () => import("@/features/admin-overview/pages/admin/OverviewPage"),
              ),
            },
            {
              path: "users",
              handle: {
                crumb: [ADMIN_CRUMB, { key: "adminShell.nav.users" }],
                permission: "people.users.manage",
                width: 1320,
              },
              lazy: page(() => import("@/features/users/pages/admin/UsersPage")),
            },
            {
              path: "roles",
              handle: {
                crumb: [ADMIN_CRUMB, { key: "adminShell.nav.roles" }],
                permission: "people.roles.manage",
                width: 1320,
              },
              lazy: page(() => import("@/features/roles/pages/admin/RolesPage")),
            },
            {
              path: "classes",
              handle: {
                crumb: [ADMIN_CRUMB, { key: "adminShell.nav.classes" }],
                permission: "scope.all",
                width: 1320,
              },
              lazy: page(
                () => import("@/features/classes/pages/admin/AdminClassesPage"),
              ),
            },
            {
              path: "ai",
              handle: {
                crumb: [ADMIN_CRUMB, { key: "adminShell.nav.ai" }],
                permission: "ai.credits.manage",
                width: 1320,
              },
              lazy: page(
                () => import("@/features/ai-credits/pages/admin/AiCreditsPage"),
              ),
            },
            {
              path: "audit",
              handle: {
                crumb: [ADMIN_CRUMB, { key: "adminShell.nav.audit" }],
                permission: "system.audit.read",
                width: 1320,
              },
              lazy: page(() => import("@/features/audit/pages/admin/AuditLogPage")),
            },
            {
              path: "api",
              handle: {
                crumb: [ADMIN_CRUMB, { key: "adminShell.nav.api" }],
                permission: "system.api_reference",
                width: 1080,
              },
              lazy: page(
                () => import("@/features/api-reference/pages/admin/ApiReferencePage"),
              ),
            },
            {
              path: "settings/:section?",
              handle: {
                crumb: [ADMIN_CRUMB, { key: "adminShell.nav.settings" }],
                permission: "system.settings.write",
                width: 1080,
              },
              lazy: page(
                () => import("@/features/organization/pages/admin/SystemSettingsPage"),
              ),
            },
            {
              path: "exports/:id",
              handle: {
                crumb: [
                  ADMIN_CRUMB,
                  { key: "adminShell.nav.settings", to: ADMIN_PATHS.settings },
                  { key: "adminShell.crumbExport" },
                ],
                permission: "system.data_export",
                width: 1080,
              },
              lazy: page(() => import("@/features/exports/pages/admin/DataExportPage")),
            },
          ],
        },
      ],
    },
  ],
};

/**
 * One student layout: a detail screen declares `handle.detail`, which below
 * 768px swaps the logo for a back arrow and that title and hides the tab bar.
 * From 768px the top bar is unchanged and the page draws its own way back.
 */
const studentTree: RouteObject = {
  path: "app",
  element: <StudentArea />,
  children: [
    {
      lazy: page(() => import("@/layouts/StudentLayout")),
      children: [
        {
          index: true,
          lazy: page(() => import("@/features/assignments/pages/StudentHomePage")),
        },
        {
          path: "classes",
          lazy: page(() => import("@/features/classes/pages/StudentClassesPage")),
        },
        {
          path: "assignments/:id",
          handle: { detail: { titleKey: "student.shell.test", back: "/app" } },
          lazy: page(() => import("@/features/assignments/pages/AssignmentIntroPage")),
        },
        {
          path: "settings/:section?",
          handle: { detail: { titleKey: "nav.settings", back: "/app" } },
          lazy: page(() => import("@/features/auth/pages/StudentSettingsPage")),
        },
        {
          path: "attempts/:attemptId/result",
          handle: { detail: { titleKey: "result.title", back: "/app" } },
          lazy: page(() => import("@/features/results/pages/ResultPage")),
        },
      ],
    },
  ],
};

/**
 * The take-test route lives under `/app` but outside StudentLayout: §9 gives it
 * FocusLayout, a shell with nothing to click away to.
 */
const takeTestTree: RouteObject = {
  path: "app/attempts/:attemptId",
  handle: { focus: true },
  lazy: page(() => import("@/layouts/FocusLayout")),
  children: [
    {
      index: true,
      lazy: page(() => import("@/features/take-test/pages/TakeTestPage")),
    },
  ],
};

/**
 * Everything that needs a session. A pathless route, so the guard runs once for
 * all three trees rather than being repeated -- and so a route added to any of
 * them is protected without anyone remembering to protect it.
 */
const protectedTree: RouteObject = {
  element: <RequireSession />,
  children: [
    {
      path: "change-password",
      lazy: page(() => import("@/features/auth/pages/ChangePasswordPage")),
    },
    teacherTree,
    adminTree,
    studentTree,
    takeTestTree,
  ],
};

export const router = createBrowserRouter([
  {
    ErrorBoundary,
    HydrateFallback: () => null,
    children: [
      { index: true, element: <HomeRedirect /> },
      authTree,
      { path: "admin/*", element: <LegacyTeacherRedirect /> },
      protectedTree,
      // Eager, like the guard that also renders it.
      { path: "403", element: <ForbiddenPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
