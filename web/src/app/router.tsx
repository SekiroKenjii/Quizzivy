import { createBrowserRouter, redirect, type RouteObject } from "react-router";
import { ErrorBoundary } from "@/app/ErrorBoundary";
import NotFoundPage from "@/app/pages/NotFoundPage";
import ForbiddenPage from "@/app/pages/ForbiddenPage";
import { RequireSession } from "@/app/guards/RequireSession";
import { StudentArea, TeacherWorkspace } from "@/app/guards/RequireWorkspace";
import { HomeRedirect } from "@/app/guards/HomeRedirect";
import {
  AssignmentPapersRedirect,
  LegacyTeacherRedirect,
} from "@/app/LegacyTeacherRedirect";

/**
 * §3's three route trees: public, /teacher for the teacher, /app for students.
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
      lazy: page(() => import("@/layouts/TeacherShell")),
      children: [
        {
          index: true,
          handle: { crumb: [{ key: "teacherShell.nav.dashboard" }], width: 1320 },
          lazy: page(
            () => import("@/features/dashboard/pages/teacher/TeacherDashboardPage"),
          ),
        },
        {
          path: "tests",
          handle: { crumb: [{ key: "teacherShell.nav.tests" }], width: 1320 },
          lazy: page(() => import("@/features/tests/pages/teacher/TestsListPage")),
        },
        {
          path: "tests/:id",
          handle: {
            crumb: [
              { key: "teacherShell.nav.tests", to: "/teacher/tests" },
              { key: "tests.detail.crumb" },
            ],
            width: 1320,
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
            width: 1320,
          },
          lazy: page(() => import("@/features/tests/pages/teacher/TestBuilderPage")),
        },
        {
          path: "imports",
          lazy: page(() => import("@/features/imports/pages/teacher/ImportsGate")),
          children: [
            {
              index: true,
              handle: { crumb: [{ key: "imports.historyTitle" }], width: 1320 },
              lazy: page(
                () => import("@/features/imports/pages/teacher/ImportsListPage"),
              ),
            },
            {
              path: "new",
              handle: {
                crumb: [
                  { key: "teacherShell.nav.tests", to: "/teacher/tests" },
                  { key: "tests.importHistory", to: "/teacher/imports" },
                  { key: "imports.newTitle" },
                ],
                width: 860,
              },
              lazy: page(
                () => import("@/features/imports/pages/teacher/NewImportPage"),
              ),
            },
            {
              path: ":id",
              handle: {
                crumb: [
                  { key: "imports.historyTitle", to: "/teacher/imports" },
                  { key: "imports.detail.pageTitle" },
                ],
                width: 720,
              },
              lazy: page(
                () => import("@/features/imports/pages/teacher/ImportDetailPage"),
              ),
            },
            {
              path: ":id/review",
              handle: {
                crumb: [
                  { key: "teacherShell.nav.tests", to: "/teacher/tests" },
                  { key: "tests.importHistory", to: "/teacher/imports" },
                  { key: "imports.review.crumb" },
                ],
                width: "full",
                sidebar: "collapsed",
              },
              lazy: page(
                () => import("@/features/imports/pages/teacher/ImportReviewPage"),
              ),
            },
          ],
        },
        {
          path: "question-bank",
          handle: { crumb: [{ key: "teacherShell.nav.questionBank" }], width: 1320 },
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
            width: 1320,
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
            width: 1320,
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
            width: 1320,
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
            width: 1320,
          },
          lazy: page(
            () => import("@/features/question-bank/pages/teacher/QuestionEditorPage"),
          ),
        },
        {
          path: "media",
          handle: { crumb: [{ key: "media.title" }], width: 1320 },
          lazy: page(() => import("@/features/media/pages/teacher/MediaPage")),
        },
        {
          path: "assignments",
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
            width: 1080,
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
            width: 1320,
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
            width: 1080,
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
          lazy: page(
            () => import("@/features/attempts/pages/teacher/AttemptReviewPage"),
          ),
        },
        {
          path: "grading",
          handle: { crumb: [{ key: "teacherShell.nav.grading" }], width: 1320 },
          lazy: page(() => import("@/features/attempts/pages/teacher/GradingPage")),
        },
        {
          path: "students",
          lazy: page(
            () => import("@/features/students/pages/teacher/StudentsListPage"),
          ),
        },
        {
          path: "classes",
          lazy: page(() => import("@/features/classes/pages/teacher/ClassesListPage")),
        },
        {
          path: "classes/:id",
          lazy: page(() => import("@/features/classes/pages/teacher/ClassDetailPage")),
        },
        {
          path: "settings/:section?",
          handle: { crumb: [{ key: "nav.settings" }], width: 1080 },
          lazy: page(
            () => import("@/features/settings/pages/teacher/TeacherSettingsPage"),
          ),
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
