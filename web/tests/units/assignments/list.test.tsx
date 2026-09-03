import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AssignmentsListPage from "@/features/assignments/pages/AssignmentsListPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const NOW = new Date("2026-08-29T10:00:00Z");

function assignment(over: Record<string, unknown> = {}) {
  return {
    id: "018f0000-0000-7000-8000-0000000000d1",
    testId: "018f0000-0000-7000-8000-0000000000a1",
    testVersionId: "018f0000-0000-7000-8000-0000000000f1",
    testVersion: 3,
    testTitle: "Unit 5",
    targets: {
      classes: [
        { id: "018f0000-0000-7000-8000-0000000000c1", name: "IELTS Foundation" },
      ],
      studentIds: [],
    },
    publishedAt: "2026-08-27T00:00:00Z",
    window: {
      opensAt: "2026-08-28T00:00:00Z",
      closesAt: "2026-08-31T14:00:00Z",
      closedAt: null,
    },
    durationMinutes: 45,
    maxAttempts: 1,
    shuffleQuestions: false,
    shuffleOptions: false,
    review: { showScore: true, showCorrectAnswers: false, showExplanations: false },
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 0,
      onLimitExceeded: "flag" as const,
      minAwayMs: 3000,
    },
    status: "open" as const,
    submittedCount: 12,
    targetCount: 19,
    flaggedCount: 0,
    ...over,
  };
}

function serve(items: ReturnType<typeof assignment>[]) {
  server.use(
    http.get(`${BASE}/admin/assignments`, () =>
      contractJson("/admin/assignments", "get", 200, {
        page: 1,
        pageSize: 50,
        total: 0,
        items,
        nextCursor: null,
      }),
    ),
  );
}

beforeEach(() => {
  // The status badge is derived from the window, so the clock is an input.
  vi.useFakeTimers({ toFake: ["Date"], now: NOW });
});

afterEach(() => {
  vi.useRealTimers();
});

/** The status tabs carry the same words as the badges, so rows are read here. */
async function rows() {
  return within(await screen.findByRole("table"));
}

function renderList() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/assignments", element: <AssignmentsListPage /> },
      { path: "/admin/assignments/new", element: <p>form</p> },
    ],
    { initialEntries: ["/admin/assignments"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

describe("the assignments list", () => {
  it("shows §8's row: test, targets, window, status, progress, flags", async () => {
    serve([assignment()]);
    renderList();

    const table = await rows();
    expect(table.getByText("Unit 5")).toBeInTheDocument();
    expect(table.getByText("v3")).toBeInTheDocument();
    expect(table.getByText("12/19")).toBeInTheDocument();
    expect(table.getByText("Đang mở")).toBeInTheDocument();
  });

  // The server sent status "open"; the window says it closed 14 hours ago.
  it("trusts the window over a stale status from the server", async () => {
    serve([
      assignment({
        status: "open",
        window: {
          opensAt: "2026-08-20T00:00:00Z",
          closesAt: "2026-08-28T20:00:00Z",
          closedAt: null,
        },
      }),
    ]);
    renderList();

    const table = await rows();
    expect(table.getByText("Đã đóng")).toBeInTheDocument();
    expect(table.queryByText("Đang mở")).toBeNull();
  });

  it("reads an early close as closed even inside the window", async () => {
    serve([
      assignment({
        window: {
          opensAt: "2026-08-28T00:00:00Z",
          closesAt: "2026-08-31T14:00:00Z",
          closedAt: "2026-08-29T08:00:00Z",
        },
      }),
    ]);
    renderList();

    expect((await rows()).getByText("Đã đóng")).toBeInTheDocument();
  });

  it("shows a dash rather than a zero for a clean assignment", async () => {
    serve([assignment({ flaggedCount: 0 })]);
    renderList();

    expect((await rows()).getByText("—")).toBeInTheDocument();
  });

  it("offers the way out when there is nothing to list", async () => {
    serve([]);
    renderList();

    expect(await screen.findByText("Chưa giao bài nào.")).toBeInTheDocument();
    expect(screen.queryByRole("table")).toBeNull();
    expect(
      screen.getAllByRole("button", { name: "Giao bài mới" }).length,
    ).toBeGreaterThan(0);
  });
});

/**
 * G-01's "Lưu nháp" (D-18 amended by migration 00022): a draft is never
 * anything else, whatever its window says. Publishing is an act by the teacher,
 * so nothing has to flip a row when a clock passes — which is what keeps
 * "no scheduler" true with a draft state in the enum.
 */
describe("a draft assignment", () => {
  it("reads as a draft even while its window is current", async () => {
    serve([
      assignment({
        publishedAt: null,
        status: "open",
        window: {
          opensAt: "2026-08-28T00:00:00Z",
          closesAt: "2026-08-31T14:00:00Z",
          closedAt: null,
        },
      }),
    ]);
    renderList();

    const table = await rows();
    expect(table.getByText("Nháp")).toBeInTheDocument();
    expect(table.queryByText("Đang mở")).toBeNull();
  });
});
