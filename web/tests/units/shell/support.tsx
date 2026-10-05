import type { ReactElement } from "react";
import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router";
import { http, HttpResponse } from "msw";
import type { TeacherHandle } from "@/layouts/shell/handle";
import TeacherLayout from "@/layouts/TeacherLayout";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";

type Dashboard = components["schemas"]["Dashboard"];
type TeacherSummary = components["schemas"]["TeacherSummary"];
type User = components["schemas"]["CurrentUser"];

export const BASE = "http://localhost:8080";

export const dashboardBody: Dashboard = {
  openAssignments: 3,
  awaitingGrading: 6,
  activeStudents: 0,
  flaggedAttempts: 0,
  newestFlaggedAttempt: null,
  recentAttempts: [],
  takingNow: { students: 0, assignments: 0, assignmentId: null },
  submissions: {
    days: Array.from({ length: 14 }, (_, i) => ({
      date: new Date(Date.UTC(2026, 8, 22 + i)).toISOString().slice(0, 10),
      count: 0,
    })),
    total: 0,
    averagePercent: null,
  },
  today: [],
  recentActivity: [],
};

export const DASHBOARD: TeacherHandle = {
  crumb: [{ key: "teacherShell.nav.dashboard" }],
};

export function crumbed(
  key: string,
  extra: Partial<TeacherHandle> = {},
): TeacherHandle {
  return { crumb: [{ key }], ...extra };
}

export function serveDashboard(body: Dashboard = dashboardBody) {
  const seen = { asked: 0 };
  server.use(
    http.get(`${BASE}/teacher/dashboard`, () => {
      seen.asked += 1;
      return contractJson("/teacher/dashboard", "get", 200, body);
    }),
  );
  return seen;
}

export function failDashboard() {
  const seen = { asked: 0 };
  server.use(
    http.get(`${BASE}/teacher/dashboard`, () => {
      seen.asked += 1;
      return HttpResponse.json(
        {
          error: {
            code: "INTERNAL",
            message: "Máy chủ gặp lỗi.",
            requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          },
        },
        { status: 500 },
      );
    }),
  );
  return seen;
}

export function renderRoutes(
  at: string,
  routes: RouteObject[],
  user: User = teacherUser,
) {
  useAuthStore.getState().setSession("token", user);
  const router = createMemoryRouter(routes, { initialEntries: [at] });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { router, client, ...view };
}

export function renderShell(
  at: string,
  pages: RouteObject[],
  user: User = teacherUser,
) {
  return renderRoutes(
    at,
    [{ path: "/teacher", element: <TeacherLayout />, children: pages }],
    user,
  );
}

export function home(element: ReactElement = <p>trang tổng quan</p>): RouteObject {
  return { index: true, handle: DASHBOARD, element };
}

/** summaryBody supplies the two existing sidebar figures as distinct summary fields. */
export const summaryBody: TeacherSummary = {
  liveAssignments: 3,
  answersToGrade: 6,
  unreadNotifications: 0,
};

/** serveSummary answers the actual cheap Teacher summary endpoint for rebuilt-shell public cases. */
export function serveSummary(body: TeacherSummary = summaryBody) {
  const seen = { asked: 0 };
  server.use(
    http.get(`${BASE}/teacher/summary`, () => {
      seen.asked += 1;
      return contractJson("/teacher/summary", "get", 200, body);
    }),
  );
  return seen;
}

/** failSummary preserves the existing failed-badge assertions on the actual summary endpoint. */
export function failSummary() {
  const seen = { asked: 0 };
  server.use(
    http.get(`${BASE}/teacher/summary`, () => {
      seen.asked += 1;
      return HttpResponse.json(
        {
          error: {
            code: "INTERNAL",
            message: "Máy chủ gặp lỗi.",
            requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          },
        },
        { status: 500 },
      );
    }),
  );
  return seen;
}
