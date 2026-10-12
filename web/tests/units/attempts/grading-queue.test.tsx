import { beforeEach, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import GradingPage from "@/features/attempts/pages/teacher/GradingPage";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { BASE } from "./fixtures";

beforeEach(() => {
  useAuthStore.getState().setSession("token", teacherUser);
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: [],
        groups: [],
        answersRemaining: 250,
        studentsWaiting: 120,
      }),
    ),
    http.get(`${BASE}/teacher/attempts`, () =>
      contractJson("/teacher/attempts", "get", 200, {
        items: [],
        total: 0,
        page: 1,
        pageSize: 100,
      }),
    ),
  );
});

it("reports complete server counts rather than the returned prefix length", async () => {
  const router = createMemoryRouter(
    [{ path: "/teacher/grading", element: <GradingPage /> }],
    { initialEntries: ["/teacher/grading"] },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  expect(
    await screen.findByText("Còn 250 câu trả lời từ 120 học viên."),
  ).toBeInTheDocument();
});

it("the real grading leaf remains lazy and declares its deck shell metadata", async () => {
  const { router } = await import("@/app/router");
  const routes = router.routes
    .flatMap((root) => root.children ?? [])
    .flatMap((root) => root.children ?? [])
    .flatMap((root) => root.children ?? [])
    .flatMap((root) => root.children ?? []);
  const grading = routes.find((route) => route.path === "grading");
  expect(grading?.handle).toEqual({
    crumb: [{ key: "teacherShell.nav.grading" }],
    width: 1320,
  });
  expect(grading?.element).toBeUndefined();
  expect(typeof grading?.lazy).toBe("function");
});
