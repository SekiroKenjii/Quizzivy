import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AssignmentAttemptsPage from "@/features/attempts/pages/AssignmentAttemptsPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { ASSIGNMENT_ID, ATTEMPT_ID, BASE, assignment, monitor } from "./fixtures";
import type { components } from "@/lib/api/schema";

type Row = components["schemas"]["MonitorRow"];

const VY = "018f0000-0000-7000-8000-0000000000e4";
const VY_ATTEMPT = "018f0000-0000-7000-8000-0000000000a9";

function rows(): Row[] {
  return [
    {
      studentId: "018f0000-0000-7000-8000-0000000000e3",
      fullName: "Hoàng Tiến Dũng",
      state: "not_started",
      flagged: false,
      audioOverLimit: false,
    },
    {
      studentId: VY,
      fullName: "Lê Khánh Vy",
      state: "graded",
      attemptId: VY_ATTEMPT,
      attemptNo: 1,
      startedAt: "2026-09-04T01:00:00Z",
      submittedAt: "2026-09-04T01:38:00Z",
      score: { earned: 29, total: 30, pendingManual: 0 },
      focusLossCount: 0,
      flagged: false,
      audioOverLimit: false,
    },
    {
      studentId: "018f0000-0000-7000-8000-0000000000e2",
      fullName: "Nguyễn Đức Minh",
      state: "submitted",
      attemptId: "018f0000-0000-7000-8000-0000000000a8",
      attemptNo: 2,
      startedAt: "2026-09-04T02:06:00Z",
      submittedAt: "2026-09-04T02:47:00Z",
      score: { earned: 26, total: 30, pendingManual: 2 },
      focusLossCount: 1,
      flagged: false,
      audioOverLimit: false,
    },
    {
      studentId: "018f0000-0000-7000-8000-0000000000e1",
      fullName: "Phạm Gia Hân",
      state: "timed_out",
      attemptId: ATTEMPT_ID,
      attemptNo: 1,
      startedAt: "2026-09-04T03:00:00Z",
      submittedAt: "2026-09-04T03:45:00Z",
      score: { earned: 18, total: 30, pendingManual: 0 },
      focusLossCount: 3,
      flagged: true,
      audioOverLimit: false,
    },
  ];
}

let reset: unknown = null;

function serve() {
  server.use(
    http.get(`${BASE}/admin/assignments/${ASSIGNMENT_ID}`, () =>
      contractJson(
        "/admin/assignments/{id}",
        "get",
        200,
        assignment({
          status: "closed",
          window: {
            opensAt: "2020-09-07T01:00:00Z",
            closesAt: "2020-09-09T14:00:00Z",
            closedAt: null,
          },
          submittedCount: 3,
          targetCount: 4,
          pendingGradingCount: 1,
          flaggedCount: 1,
        }),
      ),
    ),
    http.get(`${BASE}/admin/assignments/${ASSIGNMENT_ID}/attempts`, () =>
      contractJson("/admin/assignments/{id}/attempts", "get", 200, monitor(rows())),
    ),
    http.post(`${BASE}/admin/attempts/${VY_ATTEMPT}/reset`, async ({ request }) => {
      reset = await request.json();
      return contractJson("/admin/attempts/{id}/reset", "post", 200, {
        id: VY_ATTEMPT,
        assignmentId: ASSIGNMENT_ID,
        studentId: VY,
        testVersionId: "018f0000-0000-7000-8000-0000000000f1",
        attemptNo: 1,
        status: "voided",
        startedAt: "2026-09-04T01:00:00Z",
        deadlineAt: "2026-09-04T01:45:00Z",
      });
    }),
  );
}

function renderPage(search = "") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/assignments/:id/attempts", element: <AssignmentAttemptsPage /> },
      { path: "/admin/assignments/:id", element: <p>the assignment</p> },
    ],
    { initialEntries: [`/admin/assignments/${ASSIGNMENT_ID}/attempts${search}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("the papers of one assignment (G-11)", () => {
  beforeEach(() => {
    reset = null;
    serve();
  });

  it("lists one row per student in roster order, with the paper's columns", async () => {
    renderPage();

    expect(
      await screen.findByRole("heading", {
        name: "Bài làm · Unit 5 — Present perfect & listening",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("3/4 đã nộp · 1 chờ chấm · 1 cần xem lại"),
    ).toBeInTheDocument();

    const table = await screen.findByRole("table");
    const names = within(table)
      .getAllByRole("row")
      .slice(1)
      .map(
        (row) =>
          within(row).getAllByRole("cell")[0]!.querySelector("span:last-child")
            ?.textContent,
      );
    expect(names).toEqual([
      "Hoàng Tiến Dũng",
      "Lê Khánh Vy",
      "Nguyễn Đức Minh",
      "Phạm Gia Hân",
    ]);
    expect(
      within(table).getByRole("columnheader", { name: "Thời gian làm" }),
    ).toBeInTheDocument();
    expect(within(table).queryByRole("columnheader", { name: "Còn lại" })).toBeNull();

    const vy = within(table).getByRole("row", { name: /Lê Khánh Vy/ });
    expect(within(vy).getByText("1/2")).toBeInTheDocument();
    expect(within(vy).getByText("08:38, 04/09/2026")).toBeInTheDocument();
    expect(within(vy).getByText("38 phút")).toBeInTheDocument();
    expect(within(vy).getByText("29/30")).toBeInTheDocument();

    const minh = within(table).getByRole("row", { name: /Nguyễn Đức Minh/ });
    expect(within(minh).getByText("2/2")).toBeInTheDocument();
    expect(within(minh).getByRole("link", { name: "chờ chấm 2" })).toHaveAttribute(
      "href",
      "/admin/attempts/018f0000-0000-7000-8000-0000000000a8",
    );
  });

  it("the tabs are the results strip's numbers; timed out counts as handed in", async () => {
    const user = renderPage();

    await screen.findByRole("table");
    const tabs = screen.getByRole("tablist", { name: "Lọc bài làm" });
    expect(within(tabs).getByRole("tab", { name: /Tất cả/ })).toHaveTextContent("4");
    expect(within(tabs).getByRole("tab", { name: /Đã nộp/ })).toHaveTextContent("3");
    expect(within(tabs).getByRole("tab", { name: /Chờ chấm/ })).toHaveTextContent("1");
    expect(within(tabs).getByRole("tab", { name: /Cần xem lại/ })).toHaveTextContent(
      "1",
    );
    expect(within(tabs).getByRole("tab", { name: /Chưa nộp/ })).toHaveTextContent("1");

    await user.click(within(tabs).getByRole("tab", { name: /Chưa nộp/ }));
    await waitFor(() => expect(screen.getAllByRole("row")).toHaveLength(2));
    expect(screen.getByText("Hoàng Tiến Dũng")).toBeInTheDocument();

    await user.click(within(tabs).getByRole("tab", { name: /Cần xem lại/ }));
    await waitFor(() => expect(screen.getByText("Phạm Gia Hân")).toBeInTheDocument());
    expect(screen.queryByText("Lê Khánh Vy")).toBeNull();
  });

  it("opens on the tab the link asked for", async () => {
    renderPage("?tab=pending");

    await screen.findByRole("table");
    expect(screen.getByRole("tab", { name: /Chờ chấm/ })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("Nguyễn Đức Minh")).toBeInTheDocument();
    expect(screen.queryByText("Lê Khánh Vy")).toBeNull();
  });

  it("finds a student without their accents", async () => {
    const user = renderPage();

    await screen.findByRole("table");
    await user.type(screen.getByRole("searchbox"), "khanh vy");
    await waitFor(() => expect(screen.queryByText("Nguyễn Đức Minh")).toBeNull());
    expect(screen.getByText("Lê Khánh Vy")).toBeInTheDocument();

    await user.clear(screen.getByRole("searchbox"));
    await user.type(screen.getByRole("searchbox"), "zzz");
    expect(
      await screen.findByText("Không có học viên nào khớp “zzz”."),
    ).toBeInTheDocument();
  });

  it("the row menu is G-02's without the clock: view, reset, void", async () => {
    const user = renderPage();

    const table = await screen.findByRole("table");
    const vy = within(table).getByRole("row", { name: /Lê Khánh Vy/ });
    await user.click(within(vy).getByRole("button", { name: "Thao tác" }));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Xem bài làm" })).toHaveAttribute(
      "href",
      `/admin/attempts/${VY_ATTEMPT}`,
    );
    expect(
      within(menu).queryByRole("menuitem", { name: "Gia hạn thời gian" }),
    ).toBeNull();

    await user.click(within(menu).getByRole("menuitem", { name: "Cho làm lại" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Lý do"), "làm lại vì mất mạng");
    await user.click(within(dialog).getByRole("button", { name: "Cho làm lại" }));
    await waitFor(() => expect(reset).toEqual({ reason: "làm lại vì mất mạng" }));
  });
});
