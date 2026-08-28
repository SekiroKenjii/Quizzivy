import { createBrowserRouter, Navigate, type RouteObject } from "react-router";
import { ErrorBoundary, NotFound } from "@/app/ErrorBoundary";

/**
 * Three route trees (§2): `/admin/*`, `/app/*`, and a small public tree.
 *
 * Every component is behind its own `lazy` import, so "a student never
 * downloads admin code and an anonymous visitor downloads neither" is a
 * property of the bundle, not an intention. router.chunks.test.ts checks that
 * against the real build output.
 *
 * Note what `lazy` can and cannot do. It supplies `Component`, `loader`,
 * `action` and `ErrorBoundary` — but NOT `children`: the route tree has to be
 * statically matchable, so paths are declared here and only the components are
 * deferred. An earlier version of this file returned `children` from `lazy`; it
 * type-checked, rendered the layout, and 404'd on every child route.
 */

/** `lazy` for a module whose default export is the route component. */
const page = (load: () => Promise<{ default: React.ComponentType }>) => async () => ({
  Component: (await load()).default,
});

const publicTree: RouteObject = {
  lazy: page(() => import("@/layouts/PublicLayout")),
  children: [
    { path: "login", lazy: page(() => import("@/features/auth/pages/LoginPage")) },
    // /join/* is built in T-1.12; the path exists so the tree is complete.
    { path: "join", lazy: page(() => import("@/features/auth/pages/LoginPage")) },
    { path: "join/:code", lazy: page(() => import("@/features/auth/pages/LoginPage")) },
  ],
};

const adminTree: RouteObject = {
  path: "admin",
  lazy: page(() => import("@/layouts/AdminLayout")),
  children: [
    { index: true, lazy: page(() => import("@/app/pages/AdminDashboardPage")) },
    { path: "tests", lazy: page(() => import("@/features/tests/pages/TestsListPage")) },
    {
      path: "question-bank",
      lazy: page(() => import("@/features/question-bank/pages/QuestionBankPage")),
    },
    { path: "media", lazy: page(() => import("@/features/media/pages/MediaLibraryPage")) },
    {
      path: "assignments",
      lazy: page(() => import("@/features/assignments/pages/AssignmentsListPage")),
    },
    { path: "students", lazy: page(() => import("@/features/students/pages/StudentsListPage")) },
    { path: "classes", lazy: page(() => import("@/features/classes/pages/ClassesListPage")) },
    { path: "settings", lazy: page(() => import("@/features/auth/pages/AdminSettingsPage")) },
  ],
};

const studentTree: RouteObject = {
  path: "app",
  lazy: page(() => import("@/layouts/StudentLayout")),
  children: [
    { index: true, lazy: page(() => import("@/features/assignments/pages/StudentHomePage")) },
    { path: "classes", lazy: page(() => import("@/features/classes/pages/StudentClassesPage")) },
    { path: "settings", lazy: page(() => import("@/features/auth/pages/StudentSettingsPage")) },
  ],
};

/**
 * The take-test route lives under `/app` but outside StudentLayout: §9 gives it
 * FocusLayout, a shell with nothing to click away to.
 */
const takeTestTree: RouteObject = {
  path: "app/attempts/:attemptId",
  lazy: page(() => import("@/layouts/FocusLayout")),
  children: [{ index: true, lazy: page(() => import("@/features/take-test/pages/TakeTestPage")) }],
};

export const router = createBrowserRouter([
  {
    ErrorBoundary,
    children: [
      { index: true, element: <Navigate to="/login" replace /> },
      publicTree,
      adminTree,
      studentTree,
      takeTestTree,
      { path: "403", lazy: page(() => import("@/app/pages/ForbiddenPage")) },
      { path: "*", element: <NotFound /> },
    ],
  },
]);
