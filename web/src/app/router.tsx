import { createBrowserRouter, type RouteObject } from "react-router";
import { ErrorBoundary, NotFound } from "@/app/ErrorBoundary";
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
 * The signed-out screens that own the whole viewport.
 *
 * Outside PublicLayout on purpose. That layout is §9's "logo + content" shell,
 * and these two already carry the brand themselves -- nesting them would put a
 * header above a full-height split and a `<main>` inside a `<main>`.
 */
const authTree: RouteObject = {
  children: [
    { path: "login", lazy: page(() => import("@/features/auth/pages/LoginPage")) },
    {
      path: "auth/google/callback",
      lazy: page(() => import("@/features/auth/pages/GoogleCallbackPage")),
    },
  ],
};

/**
 * §9's public shell: logo + content. §12 wants the join screens as a single
 * centered card under it -- "calm and legitimate, not a marketing page", since
 * this is the first thing a new student sees.
 */
const publicTree: RouteObject = {
  lazy: page(() => import("@/layouts/PublicLayout")),
  children: [
    { path: "join", lazy: page(() => import("@/features/join/pages/JoinPage")) },
    { path: "join/:code", lazy: page(() => import("@/features/join/pages/JoinPage")) },
    {
      path: "join/:code/confirm",
      lazy: page(() => import("@/features/join/pages/JoinConfirmPage")),
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
          path: "question-bank",
          lazy: page(() => import("@/features/question-bank/pages/QuestionBankPage")),
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
          path: "settings",
          lazy: page(() => import("@/features/auth/pages/AdminSettingsPage")),
        },
      ],
    },
  ],
};

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
      ],
    },
    {
      lazy: page(() => import("@/layouts/StudentDetailLayout")),
      children: [
        {
          path: "assignments/:id",
          handle: { titleKey: "student.assignmentDetail" },
          lazy: page(() => import("@/features/assignments/pages/AssignmentIntroPage")),
        },
        {
          path: "settings",
          handle: { titleKey: "nav.settings" },
          lazy: page(() => import("@/features/auth/pages/StudentSettingsPage")),
        },
        {
          path: "attempts/:attemptId/result",
          handle: { titleKey: "result.title" },
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
      publicTree,
      protectedTree,
      // Eager, like the guard that also renders it.
      { path: "403", element: <ForbiddenPage /> },
      { path: "*", element: <NotFound /> },
    ],
  },
]);
