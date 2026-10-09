import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TeacherDashboardPage from "@/features/dashboard/pages/teacher/TeacherDashboardPage";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
let creations = 0;

beforeEach(() => {
  creations = 0;
  useAuthStore.getState().setSession("token", teacherUser);
  server.use(
    http.get(`${BASE}/teacher/dashboard`, () =>
      contractJson("/teacher/dashboard", "get", 200, {
        openAssignments: 0,
        awaitingGrading: 7,
        activeStudents: 23,
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
      }),
    ),
    http.get(`${BASE}/teacher/assignments`, () =>
      contractJson("/teacher/assignments", "get", 200, {
        items: [],
        page: 1,
        pageSize: 10,
        total: 0,
        facets: { all: 0, draft: 0, scheduled: 0, open: 0, closed: 0 },
      }),
    ),
    http.post(`${BASE}/teacher/tests`, () => {
      creations += 1;
      return contractJson("/teacher/tests", "post", 201, {
        skills: [],
        assignments: { live: 0, scheduled: 0, closed: 0 },
        unpublishedChanges: null,
        id: "018f0000-0000-7000-8000-0000000000a1",
        title: "Đề thi chưa đặt tên",
        description: null,
        status: "draft",
        currentVersion: 0,
        totalPoints: 1,
        questionCount: 0,
        audioCount: 0,
        sections: [],
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      });
    }),
  );
});

function renderDashboard() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher", element: <TeacherDashboardPage /> },
      { path: "/teacher/tests/:id/edit", element: <p>builder</p> },
    ],
    { initialEntries: ["/teacher"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("the dashboard's work queue", () => {
  it("links the tile that has work and leaves the ones without work noninteractive", async () => {
    renderDashboard();

    expect(await screen.findByRole("link", { name: /Chờ chấm/ })).toHaveAttribute(
      "href",
      "/teacher/grading",
    );
    expect(screen.queryByRole("button", { name: "Xem" })).toBeNull();
    expect(screen.queryByRole("link", { name: "Xem" })).toBeNull();
    expect(screen.queryByRole("link", { name: /Đóng trong 24 giờ/ })).toBeNull();
  });

  it("creates a draft from Đề thi mới and opens the builder", async () => {
    const user = renderDashboard();
    await screen.findByRole("link", { name: /Chờ chấm/ });

    await user.click(screen.getByRole("button", { name: "Đề thi mới" }));

    await waitFor(() => expect(creations).toBe(1));
    expect(await screen.findByText("builder")).toBeInTheDocument();
  });
});
