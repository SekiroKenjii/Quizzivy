import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import { Toaster, toast } from "@/components/ui/sonner";
import { exportResults, resultsFileName } from "@/features/assignments/api";
import AssignmentsListPage from "@/features/assignments/pages/teacher/AssignmentsListPage";
import type { PermissionKey } from "@/features/auth/permissions";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { contentWidth } from "@tests/support/contentWidth";
import { viewport } from "@tests/support/viewport";
import i18n from "@/lib/i18n";

const BASE = "http://localhost:8080";
const NOW = new Date("2026-08-29T10:00:00Z");
const A1 = "018f0000-0000-7000-8000-0000000000d1";
const A2 = "018f0000-0000-7000-8000-0000000000d2";
const CLASS_A = "018f0000-0000-7000-8000-0000000000c1";
const CLASS_B = "018f0000-0000-7000-8000-0000000000c2";

function assignment(over: Record<string, unknown> = {}) {
  return {
    id: A1,
    testId: "018f0000-0000-7000-8000-0000000000a1",
    testVersionId: "018f0000-0000-7000-8000-0000000000f1",
    testVersion: 3,
    testTitle: "Unit 5",
    targets: {
      classes: [{ id: CLASS_A, name: "IELTS Foundation", studentCount: 18 }],
      students: [],
    },
    publishedAt: "2026-08-27T00:00:00Z",
    updatedAt: "2026-08-27T00:00:00Z",
    window: {
      opensAt: "2026-08-28T00:00:00Z",
      closesAt: "2026-08-31T14:00:00Z",
      closedAt: null,
    },
    durationMinutes: 45,
    maxAttempts: 1,
    shuffleQuestions: false,
    shuffleOptions: false,
    review: {
      showScore: true,
      showCorrectAnswers: false,
      showExplanations: false,
      release: "on_submit",
      showClassAverage: false,
    },
    studentNote: null,
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
    questionCount: 40,
    pendingGradingCount: 6,
    flaggedCount: 0,
    ...over,
  };
}

function klass(id: string, name: string) {
  return {
    id,
    name,
    description: null,
    studentCount: 18,
    openAssignmentCount: 1,
    archivedAt: null,
    selfJoinEnabled: true,
    joinCode: null,
    createdAt: "2026-06-01T00:00:00Z",
  };
}

let requests: URLSearchParams[] = [];
let items: ReturnType<typeof assignment>[] = [];
let extends_: { id: string; body: unknown }[] = [];
let refuseExtend = new Set<string>();
let closes: { id: string; body: Record<string, unknown> }[] = [];
let duplicates: { id: string; body: unknown }[] = [];
let exports_: string[][] = [];
let failList = false;

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: NOW });
  contentWidth(1280);
  requests = [];
  items = [assignment()];
  extends_ = [];
  refuseExtend = new Set();
  closes = [];
  duplicates = [];
  exports_ = [];
  failList = false;
  grant(["teaching.assignments.write", "teaching.grading"]);
  server.use(
    http.get(`${BASE}/teacher/assignments`, ({ request }) => {
      requests.push(new URL(request.url).searchParams);
      if (failList)
        return contractJson("/teacher/assignments", "get", 500, {
          error: {
            code: "INTERNAL",
            message: "Lỗi máy chủ.",
            requestId: "018f0000-0000-7000-8000-0000000000ee",
          },
        });
      return contractJson("/teacher/assignments", "get", 200, {
        page: 1,
        pageSize: 20,
        total: items.length,
        items,
        facets: { all: 9, draft: 1, scheduled: 3, open: items.length, closed: 4 },
      });
    }),
    http.get(`${BASE}/teacher/classes`, () =>
      contractJson("/teacher/classes", "get", 200, {
        facets: { all: 2, joinable: 0, archived: 0, students: 36 },
        page: 1,
        pageSize: 100,
        total: 2,
        items: [klass(CLASS_A, "IELTS Foundation"), klass(CLASS_B, "TOEIC 600")],
      }),
    ),
    http.post(`${BASE}/teacher/assignments/:id/extend`, async ({ params, request }) => {
      const id = params["id"] as string;
      extends_.push({ id, body: await request.json() });
      if (refuseExtend.has(id))
        return contractJson("/teacher/assignments/{id}/extend", "post", 409, {
          error: {
            code: "ASSIGNMENT_CLOSED",
            message: "Bài giao đã đóng.",
            requestId: "018f0000-0000-7000-8000-0000000000ef",
          },
        });
      return contractJson(
        "/teacher/assignments/{id}/extend",
        "post",
        200,
        items.find((item) => item.id === id),
      );
    }),
    http.patch(`${BASE}/teacher/assignments/:id`, async ({ params, request }) => {
      const id = params["id"] as string;
      const body = (await request.json()) as Record<string, unknown>;
      closes.push({ id, body });
      return contractJson(
        "/teacher/assignments/{id}",
        "patch",
        200,
        items.find((item) => item.id === id),
      );
    }),
    http.post(
      `${BASE}/teacher/assignments/:id/duplicate`,
      async ({ params, request }) => {
        duplicates.push({ id: params["id"] as string, body: await request.json() });
        return contractJson(
          "/teacher/assignments/{id}/duplicate",
          "post",
          201,
          assignment({ id: A2, publishedAt: null, status: "draft" }),
        );
      },
    ),
    http.get(`${BASE}/teacher/assignments/results.csv`, ({ request }) => {
      exports_.push(new URL(request.url).searchParams.getAll("ids"));
      return new HttpResponse("Assignment,Student\n", {
        headers: {
          "Content-Type": "text/csv",
          "Content-Disposition": 'attachment; filename="results-20260829.csv"',
        },
      });
    }),
    http.get(`${BASE}/teacher/assignments/:id`, ({ params }) =>
      contractJson(
        "/teacher/assignments/{id}",
        "get",
        200,
        items.find((item) => item.id === params["id"]),
      ),
    ),
  );
});

afterEach(() => {
  toast.dismiss();
  vi.useRealTimers();
});

function grant(permissions: PermissionKey[]) {
  useAuthStore.getState().setSession("token", {
    ...teacherUser,
    permissions,
    workspaces: ["teacher"],
  });
}

function renderList(initial = "/teacher/assignments") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/assignments", element: <AssignmentsListPage /> },
      { path: "/teacher/assignments/new", element: <p>Giao bài mới</p> },
      { path: "/teacher/assignments/:id", element: <p>Chi tiết</p> },
    ],
    { initialEntries: [initial] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), router };
}

async function table() {
  return within(await screen.findByRole("table", { name: "Bài giao" }));
}

async function openMenu(user: ReturnType<typeof userEvent.setup>) {
  const rows = await table();
  await user.click(rows.getByRole("button", { name: "Thao tác với Unit 5" }));
  return screen.findByRole("menu");
}

describe("the assignments list", () => {
  it("opens on Live with every tab's count and the deck's row", async () => {
    renderList();
    const rows = await table();

    const tabs = screen.getByRole("group", { name: "Trạng thái bài giao" });
    expect(within(tabs).getByRole("button", { name: /Đang mở/ })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(within(tabs).getByRole("button", { name: /Đã lên lịch/ })).toHaveTextContent(
      "3",
    );
    expect(within(tabs).getByRole("button", { name: /Bản nháp/ })).toHaveTextContent(
      "1",
    );
    expect(requests[0]?.get("status")).toBe("open");

    expect(rows.getByRole("columnheader", { name: "Đóng lúc" })).toBeInTheDocument();
    expect(rows.getByRole("link", { name: /Unit 5/ })).toHaveAttribute(
      "href",
      `/teacher/assignments/${A1}`,
    );
    expect(rows.getByText("Đề v3 · 40 câu hỏi")).toBeInTheDocument();
    expect(rows.getByText("· 6 bài chờ chấm")).toBeInTheDocument();
    expect(rows.getAllByText("IELTS Foundation").length).toBeGreaterThan(0);
    expect(rows.getByText("12/19")).toBeInTheDocument();
    expect(rows.getByText("Đang mở")).toBeInTheDocument();
  });

  it("names the window column for the tab and keeps the tab in the URL", async () => {
    const { user, router } = renderList();
    await table();

    await user.click(screen.getByRole("button", { name: /Đã lên lịch/ }));
    await waitFor(() => expect(requests.at(-1)?.get("status")).toBe("scheduled"));
    expect(router.state.location.search).toBe("?status=scheduled");
    expect((await table()).getByRole("columnheader", { name: "Mở lúc" })).toBeVisible();

    await user.click(screen.getByRole("button", { name: /Đã đóng/ }));
    expect(
      (await table()).getByRole("columnheader", { name: "Đã đóng" }),
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: /Bản nháp/ }));
    expect(
      (await table()).getByRole("columnheader", { name: "Thời gian" }),
    ).toBeVisible();
  });

  it("searches on the server and filters by class, both in the URL", async () => {
    const { user, router } = renderList();
    await table();

    await user.type(screen.getByRole("searchbox"), "unit");
    await waitFor(() => expect(requests.at(-1)?.get("q")).toBe("unit"));

    await user.click(screen.getByRole("button", { name: "Lớp" }));
    await user.click(
      await screen.findByRole("menuitemcheckbox", { name: "TOEIC 600" }),
    );
    await waitFor(() => expect(requests.at(-1)?.getAll("classId")).toEqual([CLASS_B]));
    expect(new URLSearchParams(router.state.location.search).get("q")).toBe("unit");
    expect(new URLSearchParams(router.state.location.search).getAll("classId")).toEqual(
      [CLASS_B],
    );
  });

  it("trusts the window over a stale status from the server", async () => {
    items = [
      assignment({
        window: {
          opensAt: "2026-08-20T00:00:00Z",
          closesAt: "2026-08-28T20:00:00Z",
          closedAt: null,
        },
      }),
    ];
    renderList();
    const rows = await table();
    expect(rows.getByText("Đã đóng")).toBeInTheDocument();
    expect(rows.queryByText("Đang mở")).toBeNull();
  });

  it("reads a draft as a draft even while its window is current", async () => {
    items = [assignment({ publishedAt: null })];
    renderList("/teacher/assignments?status=draft");
    expect((await table()).getByText("Bản nháp")).toBeInTheDocument();
  });

  it("says there is nothing here and offers a new assignment", async () => {
    items = [];
    renderList();
    expect(await screen.findByText("Không có bài giao nào ở đây.")).toBeVisible();
    expect(screen.getAllByRole("link", { name: "Giao bài mới" })).toHaveLength(2);
  });

  it("says the list failed and retries", async () => {
    failList = true;
    const { user } = renderList();
    expect(await screen.findByText("Không tải được danh sách bài giao.")).toBeVisible();
    failList = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await table()).toBeTruthy();
  });
});

describe("the columns as the content narrows", () => {
  it("drops Assigned to, the window and Status in the deck's order, and says them inline", async () => {
    const { resize } = contentWidth(1280);
    renderList();
    let rows = await table();
    const headers = () =>
      rows
        .getAllByRole("columnheader")
        .map((header) => header.textContent?.trim() ?? "");
    expect(headers()).toEqual(
      expect.arrayContaining(["Giao cho", "Đóng lúc", "Đã nộp", "Trạng thái"]),
    );
    expect(rows.queryByText(/^IELTS Foundation · /)).toBeNull();

    resize(800);
    rows = await table();
    await waitFor(() => expect(headers()).not.toContain("Giao cho"));
    expect(headers()).toEqual(expect.arrayContaining(["Đóng lúc", "Trạng thái"]));
    expect(rows.getByText(/^IELTS Foundation · /)).toBeVisible();

    resize(600);
    await waitFor(() => expect(headers()).not.toContain("Đóng lúc"));
    expect(headers()).not.toContain("Trạng thái");
    expect(headers()).toContain("Đã nộp");

    resize(500);
    await waitFor(() => expect(headers()).not.toContain("Đã nộp"));
  });
});

describe("the row menu", () => {
  it("offers each action by the row's state", async () => {
    const { user } = renderList();
    const menu = await openMenu(user);
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent?.trim()),
    ).toEqual(["Mở", "Sửa cài đặt", "Gia hạn", "Nhân bản", "Đóng sớm"]);
    expect(within(menu).getByRole("menuitem", { name: "Sửa cài đặt" })).toHaveAttribute(
      "href",
      `/teacher/assignments/${A1}?tab=settings`,
    );
  });

  it("disables Extend on a draft and offers Delete instead of Close early", async () => {
    items = [assignment({ publishedAt: null })];
    const { user } = renderList("/teacher/assignments?status=draft");
    const menu = await openMenu(user);
    expect(within(menu).getByRole("menuitem", { name: "Gia hạn" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    expect(within(menu).getByRole("menuitem", { name: "Xoá" })).toBeVisible();
    expect(within(menu).queryByRole("menuitem", { name: "Đóng sớm" })).toBeNull();
  });

  it("shows only Open without the permission to change assignments", async () => {
    grant(["teaching.grading"]);
    const { user } = renderList();
    const menu = await openMenu(user);
    expect(
      within(menu)
        .getAllByRole("menuitem")
        .map((item) => item.textContent?.trim()),
    ).toEqual(["Mở"]);
    expect(screen.queryByRole("link", { name: "Giao bài mới" })).toBeNull();
  });

  it("extends one assignment by the chosen step and notifies by default", async () => {
    const { user } = renderList();
    const menu = await openMenu(user);
    await user.click(within(menu).getByRole("menuitem", { name: "Gia hạn" }));
    const dialog = await screen.findByRole("dialog", { name: "Gia hạn" });
    await user.click(within(dialog).getByRole("button", { name: "1 giờ" }));
    await user.click(within(dialog).getByRole("button", { name: "Gia hạn" }));

    await waitFor(() =>
      expect(extends_).toEqual([{ id: A1, body: { minutes: 60, notify: true } }]),
    );
    expect(await screen.findByText("Đã gia hạn thêm 1 giờ.")).toBeVisible();
  });

  it("duplicates as a draft for the original's classes", async () => {
    const { user } = renderList();
    const menu = await openMenu(user);
    await user.click(within(menu).getByRole("menuitem", { name: "Nhân bản" }));
    const dialog = await screen.findByRole("dialog", { name: "Nhân bản bài giao" });
    await user.click(within(dialog).getByRole("button", { name: "Tạo bản nháp" }));

    await waitFor(() =>
      expect(duplicates).toEqual([{ id: A1, body: { classIds: [CLASS_A] } }]),
    );
    expect(await screen.findByText("Đã tạo bản nháp.")).toBeVisible();
  });
});

describe("the bulk bar", () => {
  const LIVE = assignment();
  const CLOSED = assignment({
    id: A2,
    testTitle: "Unit 6",
    window: {
      opensAt: "2026-08-20T00:00:00Z",
      closesAt: "2026-08-28T20:00:00Z",
      closedAt: null,
    },
  });

  async function selectBoth(user: ReturnType<typeof userEvent.setup>) {
    const rows = await table();
    await user.click(rows.getByRole("checkbox", { name: /Chọn tất cả/ }));
  }

  it("extends each selected assignment and reports the one that failed", async () => {
    items = [LIVE, CLOSED];
    refuseExtend = new Set([A2]);
    const { user } = renderList();
    await selectBoth(user);
    await user.click(screen.getByRole("button", { name: "Gia hạn" }));
    const dialog = await screen.findByRole("dialog", { name: "Gia hạn 2 bài giao" });
    await user.click(within(dialog).getByRole("button", { name: "Gia hạn" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Unit 6 · IELTS Foundation: Bài giao đã đóng.",
    );
    expect(extends_.map((call) => call.id)).toEqual([A1, A2]);
    expect(extends_[0]?.body).toEqual({ minutes: 60, notify: true });

    refuseExtend = new Set();
    await user.click(within(dialog).getByRole("button", { name: /Thử lại 1/ }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(extends_.map((call) => call.id)).toEqual([A1, A2, A2]);
  });

  it("closes the live ones now and names the one that is not live", async () => {
    items = [LIVE, CLOSED];
    const { user } = renderList();
    await selectBoth(user);
    await user.click(screen.getByRole("button", { name: "Đóng ngay" }));
    const dialog = await screen.findByRole("dialog", { name: "Đóng ngay" });
    await user.click(within(dialog).getByRole("button", { name: /Xác nhận 2/ }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Unit 6 · IELTS Foundation: Bài giao này chưa mở hoặc đã đóng.",
    );
    expect(closes.map((call) => call.id)).toEqual([A1]);
    expect(closes[0]?.body).toMatchObject({ closeNow: true, draft: false });
  });

  it("exports the selection's results as one CSV file", async () => {
    items = [LIVE, CLOSED];
    const created = vi.fn(() => "blob:results");
    const revoked = vi.fn();
    Object.assign(URL, { createObjectURL: created, revokeObjectURL: revoked });
    const clicked = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);
    const { user } = renderList();
    await selectBoth(user);
    await user.click(screen.getByRole("button", { name: "Xuất kết quả" }));

    await waitFor(() => expect(exports_).toEqual([[A1, A2]]));
    await waitFor(() => expect(clicked).toHaveBeenCalledTimes(1));
    expect((clicked.mock.contexts[0] as HTMLAnchorElement).download).toBe(
      "results-20260829.csv",
    );
    clicked.mockRestore();
  });

  it("reports a refused export, such as its rate limit, instead of failing silently", async () => {
    items = [LIVE];
    server.use(
      http.get(`${BASE}/teacher/assignments/results.csv`, () =>
        HttpResponse.json(
          {
            error: {
              code: "RATE_LIMITED",
              message: "Bạn xuất quá nhiều lần. Thử lại sau ít phút.",
              requestId: "018f0000-0000-7000-8000-0000000000e0",
            },
          },
          { status: 429 },
        ),
      ),
    );
    const { user } = renderList();
    await selectBoth(user);
    await user.click(screen.getByRole("button", { name: "Xuất kết quả" }));
    expect(
      await screen.findByText("Bạn xuất quá nhiều lần. Thử lại sau ít phút."),
    ).toBeVisible();
  });

  it("offers Export alone to a grader who cannot change assignments", async () => {
    grant(["teaching.grading"]);
    items = [LIVE];
    const { user } = renderList();
    await selectBoth(user);
    expect(screen.getByRole("button", { name: "Xuất kết quả" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Đóng ngay" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Gia hạn" })).toBeNull();
  });
});

describe("what the selection remembers (QA-24-1)", () => {
  const EXTENDED = "2026-09-01T14:00:00.000Z";

  async function selectAll(user: ReturnType<typeof userEvent.setup>) {
    const rows = await table();
    await user.click(rows.getByRole("checkbox", { name: /Chọn tất cả/ }));
  }

  async function closeNow(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByRole("button", { name: "Đóng ngay" }));
    const dialog = await screen.findByRole("dialog", { name: "Đóng ngay" });
    await user.click(within(dialog).getByRole("button", { name: /Xác nhận 1/ }));
    await waitFor(() => expect(closes).toHaveLength(1));
  }

  it("keeps a class added elsewhere after the row was ticked", async () => {
    const { user } = renderList();
    await selectAll(user);
    items = [
      assignment({
        targets: {
          classes: [
            { id: CLASS_A, name: "IELTS Foundation", studentCount: 18 },
            { id: CLASS_B, name: "TOEIC 600", studentCount: 18 },
          ],
          students: [],
        },
      }),
    ];

    await closeNow(user);

    expect(closes[0]?.body).toMatchObject({
      closeNow: true,
      targets: { classIds: [CLASS_A, CLASS_B] },
    });
  });

  it("keeps a bulk extension made before Close now", async () => {
    server.use(
      http.post(`${BASE}/teacher/assignments/:id/extend`, ({ params }) => {
        items = items.map((item) =>
          item.id === params["id"]
            ? { ...item, window: { ...item.window, closesAt: EXTENDED } }
            : item,
        );
        return contractJson(
          "/teacher/assignments/{id}/extend",
          "post",
          200,
          items.find((item) => item.id === params["id"]),
        );
      }),
    );
    const { user } = renderList();
    await selectAll(user);
    await user.click(screen.getByRole("button", { name: "Gia hạn" }));
    const dialog = await screen.findByRole("dialog", { name: "Gia hạn" });
    await user.click(within(dialog).getByRole("button", { name: "1 ngày" }));
    await user.click(within(dialog).getByRole("button", { name: "Gia hạn" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    await closeNow(user);

    expect(closes[0]?.body).toMatchObject({
      closeNow: true,
      window: { closesAt: EXTENDED },
    });
  });
});

describe("the selection's scope (QA-34-1's rule)", () => {
  const selectedCount = () => screen.queryByText(/^Đã chọn/);

  async function selectAll(user: ReturnType<typeof userEvent.setup>, count = 1) {
    const rows = await table();
    await user.click(rows.getByRole("checkbox", { name: /Chọn tất cả/ }));
    expect(selectedCount()).toHaveTextContent(`Đã chọn ${count}`);
  }

  it("clears when the tab changes", async () => {
    const { user } = renderList();
    await selectAll(user);
    await user.click(screen.getByRole("button", { name: /Đã lên lịch/ }));
    await waitFor(() => expect(selectedCount()).toBeNull());
  });

  it("clears when the search changes", async () => {
    const { user } = renderList();
    await selectAll(user);
    await user.type(screen.getByRole("searchbox"), "unit");
    await waitFor(() => expect(selectedCount()).toBeNull());
  });

  it("clears when the class filter changes", async () => {
    const { user } = renderList();
    await selectAll(user);
    await user.click(screen.getByRole("button", { name: "Lớp" }));
    await user.click(
      await screen.findByRole("menuitemcheckbox", { name: "TOEIC 600" }),
    );
    await waitFor(() => expect(selectedCount()).toBeNull());
  });

  it("is kept across pages", async () => {
    servePages(25);
    const { user } = renderList();
    await selectAll(user, 20);
    await user.click(screen.getByRole("link", { name: "Trang sau" }));
    await waitFor(() => expect(requests.at(-1)?.get("page")).toBe("2"));
    expect(selectedCount()).toHaveTextContent("Đã chọn 20");
  });
});

function servePages(total: number) {
  const all = Array.from({ length: total }, (_, index) =>
    assignment({
      id: `018f0000-0000-7000-8000-${String(index).padStart(12, "0")}`,
      testTitle: `Unit ${index + 1}`,
    }),
  );
  server.use(
    http.get(`${BASE}/teacher/assignments`, ({ request }) => {
      const params = new URL(request.url).searchParams;
      requests.push(params);
      const page = Number(params.get("page") ?? "1");
      return contractJson("/teacher/assignments", "get", 200, {
        page,
        pageSize: 20,
        total,
        items: all.slice((page - 1) * 20, page * 20),
        facets: { all: total, draft: 0, scheduled: 0, open: total, closed: 0 },
      });
    }),
  );
}

describe("a page past the end (QA-24-5)", () => {
  it("moves to the last page instead of saying there is nothing", async () => {
    servePages(25);
    const { router } = renderList("/teacher/assignments?page=99");

    await waitFor(() => expect(router.state.location.search).toBe("?page=2"));
    expect(await screen.findByText("Unit 21")).toBeVisible();
    expect(screen.queryByText("Không có bài giao nào ở đây.")).toBeNull();
  });
});

describe("the row menu's dialogs give focus back (QA-24-2)", () => {
  const DRAFT = { publishedAt: null };

  it.each([
    ["Gia hạn", "Gia hạn", {}],
    ["Nhân bản", "Nhân bản bài giao", {}],
    ["Đóng sớm", "Đóng Unit 5 ngay?", {}],
    ["Xoá", "Xoá bài giao nháp này?", DRAFT],
  ] as const)(
    "returns to the menu button from %s, after Escape and after Cancel",
    async (item, title, over) => {
      items = [assignment(over)];
      const { user } = renderList(
        "publishedAt" in over ? "/teacher/assignments?status=draft" : undefined,
      );
      const button = (await table()).getByRole("button", {
        name: "Thao tác với Unit 5",
      });

      for (const leave of ["escape", "cancel"] as const) {
        await user.click(button);
        const menu = await screen.findByRole("menu");
        await user.click(within(menu).getByRole("menuitem", { name: item }));
        const dialog = await screen.findByRole("dialog", { name: title });
        if (leave === "escape") await user.keyboard("{Escape}");
        else await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
        await waitFor(() => expect(button).toHaveFocus());
      }
    },
  );
});

describe("in English", () => {
  afterEach(async () => {
    await i18n.changeLanguage("vi");
  });

  it("writes the planned close in English in Close early (QA-24-7)", async () => {
    await i18n.changeLanguage("en");
    const { user } = renderList();
    await user.click(
      (await screen.findByRole("table")).querySelector<HTMLElement>(
        'button[aria-label="Actions for Unit 5"]',
      )!,
    );
    const menu = await screen.findByRole("menu");
    await user.click(within(menu).getByRole("menuitem", { name: "Close early" }));
    const dialog = await screen.findByRole("dialog");

    expect(dialog).toHaveTextContent(/Monday/);
    expect(dialog).not.toHaveTextContent(/Thứ/);
  });
});

describe("Export results' toast (QA-24-8)", () => {
  it("says it is exporting, then that it is done", async () => {
    let release: () => void = () => undefined;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    Object.assign(URL, {
      createObjectURL: vi.fn(() => "blob:x"),
      revokeObjectURL: vi.fn(),
    });
    const clicked = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);
    server.use(
      http.get(`${BASE}/teacher/assignments/results.csv`, async () => {
        await held;
        return new HttpResponse("Assignment,Student\n", {
          headers: { "Content-Type": "text/csv" },
        });
      }),
    );
    const { user } = renderList();
    const rows = await table();
    await user.click(rows.getByRole("checkbox", { name: /Chọn tất cả/ }));
    await user.click(screen.getByRole("button", { name: "Xuất kết quả" }));

    expect(await screen.findByText("Đang xuất kết quả của 1 bài giao…")).toBeVisible();
    release();
    expect(await screen.findByText("Đã xuất kết quả của 1 bài giao.")).toBeVisible();
    clicked.mockRestore();
  });
});

describe("the deck's status pills (QA-24-3)", () => {
  it.each([
    ["open", {}, "bg-success-soft", "bg-success"],
    [
      "scheduled",
      {
        window: {
          opensAt: "2026-09-01T00:00:00Z",
          closesAt: "2026-09-03T00:00:00Z",
          closedAt: null,
        },
      },
      "bg-info-soft",
      "bg-info",
    ],
    [
      "closed",
      {
        window: {
          opensAt: "2026-08-20T00:00:00Z",
          closesAt: "2026-08-28T00:00:00Z",
          closedAt: null,
        },
      },
      "bg-muted",
      "bg-muted-fg",
    ],
    ["draft", { publishedAt: null }, "bg-transparent", "bg-border"],
  ] as const)(
    "draws %s with its tone and a 6px dot",
    async (status, over, tone, dot) => {
      items = [assignment(over)];
      renderList(
        status === "open" ? undefined : `/teacher/assignments?status=${status}`,
      );
      const pill = (await table())
        .getAllByRole("cell")
        .at(-2)!
        .querySelector("[data-slot=badge]")!;
      expect(pill.className).toContain(tone);
      expect(pill.className).toContain(`[&>[aria-hidden]]:${dot}`);
      expect(pill.querySelector("[aria-hidden]")).toHaveClass(
        "size-1.5",
        "rounded-full",
      );
    },
  );
});

describe("the assignments list on a phone", () => {
  it("draws cards with no checkbox, menu or bulk bar", async () => {
    viewport("phone");
    renderList();
    const list = await screen.findByRole("list", { name: "Bài giao" });
    expect(within(list).getByText("Unit 5")).toBeVisible();
    expect(within(list).getByText("Unit 5")).toHaveClass("text-base");
    expect(within(list).getByText(/IELTS Foundation · /)).toBeVisible();
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(screen.queryByRole("button", { name: /^Thao tác/ })).toBeNull();
  });
});

describe("exportResults", () => {
  it("removes the link at once but revokes the file's URL only later", async () => {
    vi.useFakeTimers({ toFake: ["Date", "setTimeout", "clearTimeout"], now: NOW });
    const revoked = vi.fn();
    Object.assign(URL, {
      createObjectURL: vi.fn(() => "blob:results"),
      revokeObjectURL: revoked,
    });
    const clicked = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);

    await exportResults([{ id: A1, testTitle: "Unit 5" }]);

    expect(clicked).toHaveBeenCalledTimes(1);
    expect(document.querySelector("a[download]")).toBeNull();
    expect(revoked).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1000);
    expect(revoked).not.toHaveBeenCalled();
    vi.advanceTimersByTime(60_000);
    expect(revoked).toHaveBeenCalledExactlyOnceWith("blob:results");
    clicked.mockRestore();
  });
});

describe("resultsFileName", () => {
  it("names an export by its assignment and day, or as results for several", () => {
    expect(resultsFileName(["Bài đọc: Đi du lịch"], NOW, "Asia/Ho_Chi_Minh")).toBe(
      "bai-doc-di-du-lich-20260829.csv",
    );
    expect(resultsFileName(["Unit 5", "Unit 6"], NOW, "Asia/Ho_Chi_Minh")).toBe(
      "results-20260829.csv",
    );
  });
});
