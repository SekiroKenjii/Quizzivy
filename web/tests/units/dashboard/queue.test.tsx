import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AdminDashboardPage from "@/app/pages/AdminDashboardPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
let creations = 0;

beforeEach(() => {
  creations = 0;
  server.use(
    http.get(`${BASE}/admin/dashboard`, () =>
      contractJson("/admin/dashboard", "get", 200, {
        openAssignments: 0,
        awaitingGrading: 7,
        activeStudents: 23,
        flaggedAttempts: 0,
        recentAttempts: [],
      }),
    ),
    http.get(`${BASE}/admin/assignments`, () =>
      contractJson("/admin/assignments", "get", 200, {
        items: [],
        page: 1,
        pageSize: 10,
        total: 0,
        facets: { all: 0, draft: 0, scheduled: 0, open: 0, closed: 0 },
      }),
    ),
    http.post(`${BASE}/admin/tests`, () => {
      creations += 1;
      return contractJson("/admin/tests", "post", 201, {
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
      { path: "/admin", element: <AdminDashboardPage /> },
      { path: "/admin/tests/:id/edit", element: <p>builder</p> },
    ],
    { initialEntries: ["/admin"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("the dashboard's work queue", () => {
  it("links the tile that has work and disables the ones that do not", async () => {
    renderDashboard();

    expect(await screen.findByRole("link", { name: "Chấm" })).toHaveAttribute(
      "href",
      "/admin/grading",
    );
    expect(screen.getByRole("button", { name: "Xem" })).toBeDisabled();
    expect(screen.queryByRole("link", { name: "Xem" })).toBeNull();
    expect(screen.getByRole("button", { name: "Theo dõi" })).toBeDisabled();
  });

  it("creates a draft from A-01's Đề thi mới and opens the builder", async () => {
    const user = renderDashboard();
    await screen.findByRole("link", { name: "Chấm" });

    await user.click(screen.getByRole("button", { name: "Đề thi mới" }));

    await waitFor(() => expect(creations).toBe(1));
    expect(await screen.findByText("builder")).toBeInTheDocument();
  });
});
