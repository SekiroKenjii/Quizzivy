import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import TeacherDashboardPage from "@/features/dashboard/pages/teacher/TeacherDashboardPage";
import { dashboardBars } from "@/features/dashboard/view";
import { setDisplayTimeZone, APP_TIME_ZONE } from "@/lib/i18n/datetime";
import { useAuthStore } from "@/stores/auth";
import { dashboard23, dashboard23Assignments } from "@tests/support/dashboard23";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import "@/lib/i18n";

afterEach(() => {
  cleanup();
  setDisplayTimeZone(APP_TIME_ZONE);
  useAuthStore.getState().clearSession();
  vi.useRealTimers();
});

it("reacts without remounting and refetches zone-scoped calendar buckets without changing URL range or duplicates", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-10-02T01:00:00Z"));
  useAuthStore
    .getState()
    .setSession("a", { ...teacherUser, displayName: "Minh Display" });
  let reads = 0;
  server.use(
    http.get("http://localhost:8080/teacher/dashboard", () => {
      reads += 1;
      return HttpResponse.json(dashboard23());
    }),
    http.get("http://localhost:8080/teacher/assignments", () =>
      HttpResponse.json(dashboard23Assignments()),
    ),
  );
  const router = createMemoryRouter(
    [{ path: "/teacher", element: <TeacherDashboardPage /> }],
    { initialEntries: ["/teacher?range=14d&x=1&x=2#keep"] },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  await waitFor(() => expect(reads).toBe(1));
  const heading = screen.getByRole("heading", { level: 1 });
  expect(heading).toHaveTextContent("Chào buổi sáng, Display");
  act(() => {
    setDisplayTimeZone("America/New_York");
  });
  expect(heading).toHaveTextContent("Chào buổi tối, Display");
  await waitFor(() => expect(reads).toBe(2));
  expect(screen.getByRole("heading", { level: 1 })).toBe(heading);
  expect(router.state.location.search).toBe("?range=14d&x=1&x=2");
  expect(router.state.location.hash).toBe("#keep");
});

it("does not shift calendar-day chart labels in the furthest western display zone", () => {
  setDisplayTimeZone("Pacific/Pago_Pago");
  const bars = dashboardBars(
    [{ date: "2026-10-02", count: 0 }],
    "vi",
    (count, date) => `${count} / ${date}`,
  );
  expect(bars[0]).toMatchObject({
    key: "2026-10-02",
    value: 0,
    title: "0 / 02/10/2026",
  });
});
