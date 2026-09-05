import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AssignmentFormPage from "@/features/assignments/pages/AssignmentFormPage";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const ID = "018f0000-0000-7000-8000-0000000000d1";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const VERSION_ID = "018f0000-0000-7000-8000-0000000000f1";
const CLASS_ID = "018f0000-0000-7000-8000-0000000000c1";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

const saved: components["schemas"]["Assignment"] = {
  id: ID,
  testId: TEST_ID,
  testVersionId: VERSION_ID,
  testVersion: 3,
  testTitle: "Unit 5 — Present perfect & listening",
  targets: {
    classes: [{ id: CLASS_ID, name: "IELTS Foundation", studentCount: 18 }],
    students: [],
  },
  publishedAt: "2026-08-27T00:00:00Z",
  updatedAt: "2026-08-27T02:12:00Z",
  window: {
    opensAt: "2020-09-07T01:00:00Z",
    closesAt: "2099-09-09T14:00:00Z",
    closedAt: null,
  },
  durationMinutes: 90,
  maxAttempts: 2,
  shuffleQuestions: false,
  shuffleOptions: true,
  review: { showScore: true, showCorrectAnswers: false, showExplanations: false },
  integrity: {
    requireFullscreen: false,
    blockCopyPaste: true,
    maxFocusLoss: 0,
    onLimitExceeded: "flag",
    minAwayMs: 3000,
  },
  status: "open",
};

let patches: unknown[] = [];

beforeEach(() => {
  patches = [];
  server.use(
    http.get(`${BASE}/admin/assignments/${ID}`, () =>
      contractJson("/admin/assignments/{id}", "get", 200, saved),
    ),
    http.get(`${BASE}/admin/tests/${TEST_ID}/versions`, () =>
      contractJson("/admin/tests/{id}/versions", "get", 200, {
        items: [
          {
            id: VERSION_ID,
            version: 3,
            totalPoints: 30,
            questionCount: 24,
            audioCount: 4,
            manualCount: 2,
            publishedAt: "2026-08-20T00:00:00Z",
            publishedBy: "Thuong",
          },
        ],
      }),
    ),
    http.get(`${BASE}/admin/classes`, () =>
      contractJson("/admin/classes", "get", 200, {
        items: [],
        page: 1,
        pageSize: 20,
        total: 0,
        facets: { all: 0, joinable: 0, archived: 0, students: 0 },
      }),
    ),
    http.get(`${BASE}/admin/students`, () =>
      contractJson("/admin/students", "get", 200, {
        items: [],
        page: 1,
        pageSize: 20,
        total: 0,
        facets: { total: 0, activeLast7Days: 0 },
      }),
    ),
    http.patch(`${BASE}/admin/assignments/${ID}`, async ({ request }) => {
      patches.push(await request.json());
      return contractJson("/admin/assignments/{id}", "patch", 200, saved);
    }),
  );
});

function renderEdit() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/assignments/:id/edit", element: <AssignmentFormPage /> },
      { path: "/admin/assignments/:id", element: <p>detail</p> },
    ],
    { initialEntries: [`/admin/assignments/${ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), router };
}

describe("editing an assignment", () => {
  it("loads what was saved into G-01's form and writes it back in place", async () => {
    const { user, router } = renderEdit();

    expect(
      await screen.findByText("Unit 5 — Present perfect & listening"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Chỉnh sửa bài giao" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Bỏ IELTS Foundation" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: "Thời lượng làm bài" }),
    ).toHaveTextContent("90 phút");
    expect(screen.queryByRole("button", { name: "Lưu nháp" })).toBeNull();

    await user.click(screen.getByRole("combobox", { name: "Thời lượng làm bài" }));
    await user.click(await screen.findByRole("option", { name: "60 phút" }));
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    await waitFor(() => expect(patches).toHaveLength(1));
    expect(patches[0]).toMatchObject({
      draft: false,
      testVersionId: VERSION_ID,
      durationMinutes: 60,
      targets: { classIds: [CLASS_ID], studentIds: [] },
    });
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/admin/assignments/${ID}`),
    );
  });
});
