import { createBrowserRouter, redirect, type RouteObject } from "react-router";
import { ErrorBoundary } from "@/app/ErrorBoundary";
import NotFoundPage from "@/app/pages/NotFoundPage";
import ForbiddenPage from "@/app/pages/ForbiddenPage";
import { RequireSession } from "@/app/guards/RequireSession";
import { AdminOnly, StudentArea } from "@/app/guards/RequireRole";
import { HomeRedirect } from "@/app/guards/HomeRedirect";

/**
 * §3's three route trees: public, /admin for the teacher, /app for students.
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

const adminTree: RouteObject = {
  path: "admin",
  element: <AdminOnly />,
  children: [
    {
      lazy: page(() => import("@/layouts/AdminLayout")),
      children: [
        { index: true, lazy: page(() => import("@/app/pages/AdminDashboardPage")) },
        {
          path: "tests",
          lazy: page(() => import("@/features/tests/pages/TestsListPage")),
        },
        {
          path: "tests/:id",
          lazy: page(() => import("@/features/tests/pages/TestDetailPage")),
        },

        {
          path: "tests/:id/edit",
          lazy: page(() => import("@/features/tests/pages/TestBuilderPage")),
        },
        {
          path: "imports",
          lazy: page(() => import("@/features/imports/pages/ImportsGate")),
          children: [
            {
              index: true,
              lazy: page(() => import("@/features/imports/pages/ImportsListPage")),
            },
            {
              path: "new",
              lazy: page(() => import("@/features/imports/pages/NewImportPage")),
            },
            {
              path: ":id",
              lazy: page(() => import("@/features/imports/pages/ImportDetailPage")),
            },
            {
              path: ":id/review",
              lazy: page(() => import("@/features/imports/pages/ImportReviewPage")),
            },
          ],
        },
        {
          path: "question-bank",
          lazy: page(() => import("@/features/question-bank/pages/QuestionBankPage")),
        },
        {
          path: "question-bank/groups",
          lazy: page(() => import("@/features/question-groups/pages/GroupsListPage")),
        },
        {
          path: "question-bank/groups/:id",
          lazy: page(() => import("@/features/question-groups/pages/GroupEditorPage")),
        },
        {
          path: "question-bank/new",
          lazy: page(() => import("@/features/question-bank/pages/QuestionEditorPage")),
        },
        {
          path: "question-bank/:id",
          lazy: page(() => import("@/features/question-bank/pages/QuestionEditorPage")),
        },
        {
          path: "media",
          lazy: page(() => import("@/features/media/pages/MediaLibraryPage")),
        },
        {
          path: "assignments",
          lazy: page(() => import("@/features/assignments/pages/AssignmentsListPage")),
        },
        {
          path: "assignments/new",
          lazy: page(() => import("@/features/assignments/pages/AssignmentFormPage")),
        },
        {
          path: "assignments/:id",
          lazy: page(() => import("@/features/assignments/pages/AssignmentDetailPage")),
        },
        {
          path: "assignments/:id/edit",
          lazy: page(() => import("@/features/assignments/pages/AssignmentFormPage")),
        },
        {
          path: "assignments/:id/attempts",
          lazy: page(() => import("@/features/attempts/pages/AssignmentAttemptsPage")),
        },
        {
          path: "attempts/:id",
          lazy: page(() => import("@/features/attempts/pages/AttemptReviewPage")),
        },
        {
          path: "grading",
          lazy: page(() => import("@/features/attempts/pages/GradingQueuePage")),
        },
        {
          path: "students",
          lazy: page(() => import("@/features/students/pages/StudentsListPage")),
        },
        {
          path: "classes",
          lazy: page(() => import("@/features/classes/pages/ClassesListPage")),
        },
        {
          path: "classes/:id",
          lazy: page(() => import("@/features/classes/pages/ClassDetailPage")),
        },
        {
          path: "settings/:section?",
          lazy: page(() => import("@/features/auth/pages/AdminSettingsPage")),
        },
      ],
    },
  ],
};

/**
 * One student layout (S-13): a detail screen declares `handle.detail`, which
 * below 1024px swaps the nav bar for the back arrow and, from 1024px, changes
 * nothing -- the page draws its own way back.
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
          handle: { detail: true, titleKey: "student.assignmentDetail" },
          lazy: page(() => import("@/features/assignments/pages/AssignmentIntroPage")),
        },
        {
          path: "settings/:section?",
          handle: { detail: true, titleKey: "nav.settings" },
          lazy: page(() => import("@/features/auth/pages/StudentSettingsPage")),
        },
        {
          path: "attempts/:attemptId/result",
          handle: { detail: true, titleKey: "result.title" },
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
    adminTree,
    studentTree,
    takeTestTree,
  ],
};

export const router = createBrowserRouter([
  {
    ErrorBoundary,
    children: [
      { index: true, element: <HomeRedirect /> },
      authTree,
      protectedTree,
      // Eager, like the guard that also renders it.
      { path: "403", element: <ForbiddenPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
