import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import { useNavCounts } from "@/layouts/shell/useNavCounts";
import TeacherDashboardPage from "@/features/dashboard/pages/teacher/TeacherDashboardPage";
import { dashboardKeys } from "@/features/dashboard/keys";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { teacherUser } from "@tests/support/fixtures";
import { contractJson } from "@tests/support/contractResponse";
import {
  dashboard23,
  dashboard23Assignments,
  dashboard23Summary,
} from "@tests/support/dashboard23";
import { renderShell } from "../../units/shell/support";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
let summaryReads: number;
let homeReads: number;
let liveReads: number;
const originalVisibility = Object.getOwnPropertyDescriptor(document, "visibilityState");

function visibility(value: "visible" | "hidden") {
  Object.defineProperty(document, "visibilityState", { value, configurable: true });
  act(() => document.dispatchEvent(new Event("visibilitychange")));
}
function Counts() {
  const counts = useNavCounts();
  return <output aria-label="counts">{JSON.stringify(counts)}</output>;
}
function board(home = false) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
  });
  const router = createMemoryRouter(
    [
      {
        path: "/teacher",
        element: (
          <>
            <Counts />
            {home && <TeacherDashboardPage />}
          </>
        ),
      },
    ],
    { initialEntries: ["/teacher"] },
  );
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { ...view, client };
}
beforeEach(() => {
  summaryReads = 0;
  homeReads = 0;
  liveReads = 0;
  visibility("visible");
  useAuthStore.getState().setSession("token", teacherUser);
  server.use(
    http.get(`${BASE}/teacher/summary`, () => {
      summaryReads++;
      return contractJson("/teacher/summary", "get", 200, dashboard23Summary());
    }),
    http.get(`${BASE}/teacher/dashboard`, () => {
      homeReads++;
      return contractJson("/teacher/dashboard", "get", 200, dashboard23());
    }),
    http.get(`${BASE}/teacher/assignments`, () => {
      liveReads++;
      return contractJson("/teacher/assignments", "get", 200, dashboard23Assignments());
    }),
  );
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  if (originalVisibility)
    Object.defineProperty(document, "visibilityState", originalVisibility);
  else Reflect.deleteProperty(document, "visibilityState");
});

describe("actual summary and home cache lifecycle", () => {
  it("names actual manual answers once without substituting distinct students or papers", async () => {
    renderShell("/teacher", [
      {
        index: true,
        handle: { crumb: [{ key: "teacherShell.nav.dashboard" }] },
        element: <TeacherDashboardPage />,
      },
    ]);
    const nav = screen.getByRole("navigation", { name: "Điều hướng chính" });
    await waitFor(() =>
      expect(nav.querySelector('a[href="/teacher/grading"]')).toHaveAccessibleName(
        /^Chấm bài\s*6 câu trả lời chờ chấm$/,
      ),
    );
    expect(nav.querySelector('a[href="/teacher/grading"]')).not.toHaveAccessibleName(
      /4 học viên|2 bài|6 bài/,
    );
  });
  it("requests only summary for navigation and preserves a known live count beside denied grading", async () => {
    server.use(
      http.get(`${BASE}/teacher/summary`, () => {
        summaryReads++;
        return contractJson("/teacher/summary", "get", 200, {
          ...dashboard23Summary(),
          answersToGrade: null,
        });
      }),
    );
    board();
    await waitFor(() =>
      expect(screen.getByLabelText("counts")).toHaveTextContent(
        '{"liveAssignments":3,"toGrade":null,"unread":2}',
      ),
    );
    expect(summaryReads).toBe(1);
    expect(homeReads).toBe(0);
    expect(liveReads).toBe(0);
  });
  it("preserves both nullable counts and hides stale figures after refresh failure", async () => {
    const { client } = board();
    await waitFor(() =>
      expect(screen.getByLabelText("counts")).toHaveTextContent('"toGrade":6'),
    );
    server.use(
      http.get(`${BASE}/teacher/summary`, () =>
        contractJson("/teacher/summary", "get", 200, {
          ...dashboard23Summary(),
          liveAssignments: null,
          answersToGrade: null,
        }),
      ),
    );
    await act(() => client.invalidateQueries({ queryKey: ["admin-dashboard"] }));
    await waitFor(() =>
      expect(screen.getByLabelText("counts")).toHaveTextContent(
        '{"liveAssignments":null,"toGrade":null,"unread":2}',
      ),
    );
    server.use(
      http.get(`${BASE}/teacher/summary`, () =>
        HttpResponse.json(
          {
            error: {
              code: "INTERNAL",
              message: "Máy chủ gặp lỗi.",
              requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
            },
          },
          { status: 500 },
        ),
      ),
    );
    await act(() => client.invalidateQueries({ queryKey: ["admin-dashboard"] }));
    await waitFor(() =>
      expect(screen.getByLabelText("counts")).toHaveTextContent(/^null$/),
    );
  });
  it("disables summary and home reads and resume work without a Teacher workspace", async () => {
    useAuthStore.getState().setSession("token", { ...teacherUser, workspaces: [] });
    board(true);
    visibility("hidden");
    visibility("visible");
    act(() => window.dispatchEvent(new KeyboardEvent("keydown", { key: "a" })));
    await act(() => Promise.resolve());
    expect(summaryReads).toBe(0);
    expect(homeReads).toBe(0);
    expect(liveReads).toBe(0);
    await waitFor(() =>
      expect(screen.getByLabelText("counts")).toHaveTextContent(/^null$/),
    );
  });
  it("refreshes distinct summary and home leaves on the unchanged write-invalidation prefix", async () => {
    const { client } = board(true);
    await screen.findByRole("link", { name: /Bài bị gắn cờ/ });
    await waitFor(() => expect(summaryReads).toBe(1));
    server.use(
      http.get(`${BASE}/teacher/dashboard`, () => {
        homeReads++;
        return contractJson("/teacher/dashboard", "get", 200, {
          ...dashboard23(),
          awaitingGrading: 11,
        });
      }),
      http.get(`${BASE}/teacher/summary`, () => {
        summaryReads++;
        return contractJson("/teacher/summary", "get", 200, {
          ...dashboard23Summary(),
          answersToGrade: 11,
        });
      }),
    );
    await act(() => client.invalidateQueries({ queryKey: ["admin-dashboard"] }));
    expect(summaryReads).toBe(2);
    expect(homeReads).toBe(2);
    expect(liveReads).toBe(1);
    await waitFor(() =>
      expect(screen.getByLabelText("counts")).toHaveTextContent('"toGrade":11'),
    );
    expect(screen.getByRole("link", { name: /Chờ chấm/ })).toHaveTextContent("11");
    expect(client.getQueryData(dashboardKeys.home("14d"))).not.toEqual(
      client.getQueryData(dashboardKeys.summary),
    );
  });
  it("polls home at 30 seconds and summary at 60, suspends hidden and refreshes each once on resume", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { unmount } = board(true);
    await waitFor(() => expect(summaryReads + homeReads).toBe(2));
    await act(() => vi.advanceTimersByTimeAsync(31_000));
    await waitFor(() => expect(homeReads).toBe(2));
    expect(summaryReads).toBe(1);
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    await waitFor(() => expect(summaryReads).toBe(2));
    expect(homeReads).toBe(3);
    visibility("hidden");
    const hidden = [summaryReads, homeReads, liveReads];
    await act(() => vi.advanceTimersByTimeAsync(120_000));
    expect([summaryReads, homeReads, liveReads]).toEqual(hidden);
    visibility("visible");
    await waitFor(() =>
      expect([summaryReads, homeReads, liveReads]).toEqual(hidden.map((n) => n + 1)),
    );
    act(() => window.dispatchEvent(new KeyboardEvent("keydown", { key: "a" })));
    await act(() => vi.advanceTimersByTimeAsync(500));
    expect([summaryReads, homeReads, liveReads]).toEqual(hidden.map((n) => n + 1));
    unmount();
    const final = [summaryReads, homeReads, liveReads];
    await act(() => vi.advanceTimersByTimeAsync(120_000));
    expect([summaryReads, homeReads, liveReads]).toEqual(final);
  });
  it("stops home when idle and refreshes only once on the first returning input", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    board(true);
    await waitFor(() => expect(homeReads).toBe(1));
    await act(() => vi.advanceTimersByTimeAsync(601_000));
    const idle = [summaryReads, homeReads, liveReads];
    await act(() => vi.advanceTimersByTimeAsync(120_000));
    expect([summaryReads, homeReads, liveReads]).toEqual(idle);
    act(() => window.dispatchEvent(new Event("pointermove")));
    await waitFor(() =>
      expect([summaryReads, homeReads, liveReads]).toEqual(idle.map((n) => n + 1)),
    );
    act(() => window.dispatchEvent(new Event("pointermove")));
    expect([summaryReads, homeReads, liveReads]).toEqual(idle.map((n) => n + 1));
  });
});
