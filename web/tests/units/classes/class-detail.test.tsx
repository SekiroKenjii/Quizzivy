import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import ClassDetailPage from "@/features/classes/pages/teacher/ClassDetailPage";
import { Toaster } from "@/components/ui/sonner";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { assistantUser, teacherUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const CLASS_ID = "018f0000-0000-7000-8000-0000000000c1";
const MINH = "018f0000-0000-7000-8000-0000000000e1";
const HAN = "018f0000-0000-7000-8000-0000000000e2";
const LAN = "018f0000-0000-7000-8000-0000000000e3";

const klass = {
  id: CLASS_ID,
  name: "IELTS Foundation — Lớp tối T3/T5",
  description: null,
  scheduleLabel: "Tue · Thu, 17:30",
  room: "Room 2",
  studentCount: 2,
  openAssignmentCount: 1,
  archivedAt: null,
  selfJoinEnabled: true,
  joinCode: null,
  createdAt: "2026-06-01T00:00:00Z",
};

let assignmentQueries: (string | null)[] = [];
let removals: string[] = [];
let additions: unknown[] = [];
let patches: unknown[] = [];
let stored: typeof klass = klass;

function assignmentRow(over: Record<string, unknown>) {
  return {
    id: "018f0000-0000-7000-8000-0000000000d1",
    testId: "018f0000-0000-7000-8000-0000000000a1",
    testVersionId: "018f0000-0000-7000-8000-0000000000f1",
    testVersion: 3,
    testTitle: "Unit 5",
    targets: {
      classes: [{ id: CLASS_ID, name: klass.name, studentCount: 18 }],
      students: [],
    },
    publishedAt: "2026-08-27T00:00:00Z",
    updatedAt: "2026-08-27T00:00:00Z",
    window: {
      opensAt: "2020-09-07T01:00:00Z",
      closesAt: "2099-09-09T14:00:00Z",
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
      maxFocusLoss: 2,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
    status: "open",
    ...over,
  };
}

const stats = (over: Record<string, unknown>) => ({
  submittedCount: 0,
  score: null,
  flaggedCount: 0,
  activity: { live: false, lastAttemptAt: null },
  ...over,
});

beforeEach(() => {
  assignmentQueries = [];
  removals = [];
  additions = [];
  patches = [];
  stored = klass;
  useAuthStore.setState({ user: teacherUser });
  server.use(
    http.delete(`${BASE}/teacher/classes/${CLASS_ID}/members/:userId`, ({ params }) => {
      removals.push(String(params["userId"]));
      return new HttpResponse(null, { status: 204 });
    }),
    http.post(`${BASE}/teacher/classes/${CLASS_ID}/members`, async ({ request }) => {
      additions.push(await request.json());
      return new HttpResponse(null, { status: 204 });
    }),
    http.patch(`${BASE}/teacher/classes/${CLASS_ID}`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      patches.push(body);
      stored = { ...stored, ...body };
      return contractJson("/teacher/classes/{id}", "patch", 200, stored);
    }),
    http.get(`${BASE}/teacher/classes/${CLASS_ID}`, () =>
      contractJson("/teacher/classes/{id}", "get", 200, stored),
    ),
    http.get(`${BASE}/teacher/classes/${CLASS_ID}/members`, () => {
      const items = [
        {
          userId: MINH,
          fullName: "Nguyễn Đức Minh",
          email: "minh.nguyen@gmail.com",
          joinedVia: "admin",
          joinedAt: "2026-07-12T01:00:00Z",
          joinCodeHint: null,
          stats: stats({
            submittedCount: 14,
            score: { earned: 86, total: 100, pendingManual: 0 },
          }),
        },
        {
          userId: HAN,
          fullName: "Phạm Gia Hân",
          email: "han.pham@gmail.com",
          joinedVia: "join_code",
          joinedAt: "2026-08-29T01:12:00Z",
          joinCodeHint: "P9QR",
          stats: stats({}),
        },
      ].filter((member) => !removals.includes(member.userId));
      return contractJson("/teacher/classes/{id}/members", "get", 200, {
        items,
        page: 1,
        pageSize: 20,
        total: items.length,
      });
    }),
    http.get(`${BASE}/teacher/assignments`, ({ request }) => {
      assignmentQueries.push(new URL(request.url).searchParams.get("classId"));
      return contractJson("/teacher/assignments", "get", 200, {
        page: 1,
        pageSize: 100,
        total: 2,
        facets: { all: 2, draft: 0, scheduled: 0, open: 1, closed: 1 },
        items: [
          assignmentRow({
            id: "018f0000-0000-7000-8000-0000000000d2",
            testTitle: "Unit 4 — Past simple review",
            testVersion: 2,
            window: {
              opensAt: "2020-08-22T01:00:00Z",
              closesAt: "2020-08-24T14:00:00Z",
              closedAt: null,
            },
            submittedCount: 18,
            targetCount: 18,
          }),
          assignmentRow({
            id: "018f0000-0000-7000-8000-0000000000d1",
            testTitle: "Unit 5 — Present perfect & listening",
            testVersion: 3,
            submittedCount: 12,
            targetCount: 18,
          }),
        ],
      });
    }),
    http.get(`${BASE}/teacher/students`, () =>
      contractJson("/teacher/students", "get", 200, {
        items: [
          {
            id: MINH,
            email: "minh.nguyen@gmail.com",
            fullName: "Nguyễn Đức Minh",
            hasPassword: true,
            linkedProviders: [],
            mustChangePassword: false,
            createdAt: "2026-06-01T00:00:00Z",
            disabledAt: null,
            classes: [
              {
                id: CLASS_ID,
                name: klass.name,
                joinedVia: "admin",
                joinedAt: "2026-07-12T01:00:00Z",
              },
            ],
            stats: stats({}),
          },
          {
            id: HAN,
            email: "han.pham@gmail.com",
            fullName: "Phạm Gia Hân",
            hasPassword: false,
            linkedProviders: ["google"],
            mustChangePassword: false,
            createdAt: "2026-06-01T00:00:00Z",
            disabledAt: null,
            classes: [],
            stats: stats({}),
          },
        ],
        facets: { total: 2, activeLast7Days: 1 },
        page: 1,
        pageSize: 20,
        total: 2,
      }),
    ),
  );
});

afterEach(() => {
  useAuthStore.setState({ user: null });
});

function renderDetail() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [
      { path: "/teacher/classes/:id", element: <ClassDetailPage /> },
      { path: "/teacher/classes", element: <p>classes</p> },
      { path: "/teacher/students", element: <p>students</p> },
      { path: "/teacher/assignments/new", element: <p>wizard</p> },
    ],
    { initialEntries: [`/teacher/classes/${CLASS_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), router };
}

async function memberRow(name: string) {
  return (await screen.findByText(name)).closest<HTMLElement>("[role=row]")!;
}

describe("the class's head", () => {
  it("names the class with its count, and offers Add student and Assign to class", async () => {
    renderDetail();
    expect(
      await screen.findByRole("heading", { level: 1, name: klass.name }),
    ).toBeInTheDocument();
    expect(screen.getByText("2 học viên")).toBeVisible();
    expect(screen.getByRole("link", { name: "Lớp học" })).toHaveAttribute(
      "href",
      "/teacher/classes",
    );
    expect(screen.getByRole("link", { name: "Giao bài cho lớp" })).toHaveAttribute(
      "href",
      `/teacher/assignments/new?class=${CLASS_ID}`,
    );
    expect(screen.getByRole("button", { name: "Thêm học viên" })).toBeVisible();
  });

  it("says a class it cannot read is not found", async () => {
    server.use(
      http.get(`${BASE}/teacher/classes/${CLASS_ID}`, () =>
        HttpResponse.json(
          {
            error: {
              code: "NOT_FOUND",
              message: "Không tìm thấy.",
              requestId: "018f0000-0000-7000-8000-0000000000f9",
            },
          },
          { status: 404 },
        ),
      ),
    );
    renderDetail();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Không tìm thấy lớp" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Lớp này không có hoặc bạn không dạy lớp này."),
    ).toBeVisible();
  });
});

describe("the members", () => {
  it("draws the deck's rows: how they joined, what they handed in and the average", async () => {
    renderDetail();
    expect(await screen.findByRole("heading", { name: "Học viên · 2" })).toBeVisible();
    const minh = await memberRow("Nguyễn Đức Minh");
    expect(within(minh).getByText("Giáo viên thêm")).toBeInTheDocument();
    expect(within(minh).getByText("14")).toBeInTheDocument();
    expect(within(minh).getByText("86%")).not.toHaveClass("text-danger-ink");
    const han = await memberRow("Phạm Gia Hân");
    expect(within(han).getByText("Mã ••P9QR")).toBeInTheDocument();
    expect(within(han).getByText("—")).toBeInTheDocument();
    expect(
      screen.getByText("Xoá học viên khỏi lớp vẫn giữ bài đã nộp và điểm của họ."),
    ).toBeVisible();
  });

  it("removes a member after a confirm naming them and the class, then gives focus to the members' search", async () => {
    const { user } = renderDetail();
    const han = await memberRow("Phạm Gia Hân");
    const trigger = within(han).getByRole("button", {
      name: "Thao tác với Phạm Gia Hân",
    });
    await user.click(trigger);
    expect(screen.getByRole("menuitem", { name: "Xem kết quả" })).toHaveAttribute(
      "href",
      `/teacher/students?studentId=${HAN}`,
    );
    await user.click(screen.getByRole("menuitem", { name: "Xoá khỏi lớp" }));
    const dialog = await screen.findByRole("dialog", {
      name: `Xoá Phạm Gia Hân khỏi ${klass.name}?`,
    });
    expect(removals).toEqual([]);
    await user.click(within(dialog).getByRole("button", { name: "Xoá" }));
    await waitFor(() => expect(removals).toEqual([HAN]));
    expect(await screen.findByText("Đã xoá Phạm Gia Hân khỏi lớp")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(screen.queryByText("Phạm Gia Hân")).toBeNull());
    expect(screen.getByRole("searchbox", { name: "Tìm học viên" })).toHaveFocus();
  });

  it("tints a selected row and removes the selected members from the bar", async () => {
    const { user } = renderDetail();
    const han = await memberRow("Phạm Gia Hân");
    await user.click(within(han).getByRole("checkbox"));
    expect(han).toHaveAttribute("aria-selected", "true");
    expect(han).toHaveClass("bg-muted");
    expect(await memberRow("Nguyễn Đức Minh")).not.toHaveClass("bg-muted");
    await user.click(screen.getByRole("button", { name: /Xoá khỏi lớp/ }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Xác nhận 1 mục" }));
    await waitFor(() => expect(removals).toEqual([HAN]));
  });

  it("offers a reader without the right to change classes no checkboxes and no removal", async () => {
    useAuthStore.setState({ user: assistantUser });
    renderDetail();
    const han = await memberRow("Phạm Gia Hân");
    expect(within(han).queryByRole("checkbox")).toBeNull();
    expect(screen.queryByRole("button", { name: "Thêm học viên" })).toBeNull();
    expect(screen.queryByRole("menuitem", { name: "Xoá khỏi lớp" })).toBeNull();
  });
});

describe("adding students", () => {
  it("asks for at least one, shows who is already in, and adds the ones ticked", async () => {
    const { user } = renderDetail();
    const opener = await screen.findByRole("button", { name: "Thêm học viên" });
    await user.click(opener);
    const dialog = await screen.findByRole("dialog", {
      name: `Thêm học viên vào ${klass.name}`,
    });
    const minh = await within(dialog).findByRole("checkbox", {
      name: /Nguyễn Đức Minh/,
    });
    expect(minh).toHaveAttribute("aria-disabled", "true");
    expect(minh).toHaveTextContent("đã trong lớp");
    await user.click(within(dialog).getByRole("button", { name: "Thêm" }));
    expect(within(dialog).getByText("Hãy chọn ít nhất một học viên")).toBeVisible();
    await user.click(within(dialog).getByRole("checkbox", { name: /Phạm Gia Hân/ }));
    await user.click(within(dialog).getByRole("button", { name: "Thêm 1" }));
    await waitFor(() => expect(additions).toEqual([{ userId: HAN }]));
    expect(await screen.findByText("Đã thêm 1 học viên")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(opener).toHaveFocus();
  });

  it("after a partial failure keeps only the one it could not add, counts only the new ones and shows the added one as a member", async () => {
    const joined = new Set<string>();
    let lanRefused = false;
    const pupil = (id: string, fullName: string) => ({
      id,
      email: `${id.slice(-4)}@gmail.com`,
      fullName,
      hasPassword: true,
      linkedProviders: [],
      mustChangePassword: false,
      createdAt: "2026-06-01T00:00:00Z",
      disabledAt: null,
      classes: joined.has(id)
        ? [
            {
              id: CLASS_ID,
              name: klass.name,
              joinedVia: "admin",
              joinedAt: "2026-10-11T01:00:00Z",
            },
          ]
        : [],
      stats: stats({}),
    });
    server.use(
      http.post(`${BASE}/teacher/classes/${CLASS_ID}/members`, async ({ request }) => {
        const body = (await request.json()) as { userId: string };
        additions.push(body);
        if (body.userId === LAN && !lanRefused) {
          lanRefused = true;
          return HttpResponse.json(
            { error: { code: "NOT_FOUND", message: "Không tìm thấy học viên" } },
            { status: 404 },
          );
        }
        joined.add(body.userId);
        return new HttpResponse(null, { status: 204 });
      }),
      http.get(`${BASE}/teacher/students`, () =>
        contractJson("/teacher/students", "get", 200, {
          items: [pupil(HAN, "Phạm Gia Hân"), pupil(LAN, "Đỗ Ngọc Lan")],
          facets: { total: 2, activeLast7Days: 0 },
          page: 1,
          pageSize: 20,
          total: 2,
        }),
      ),
    );
    const { user } = renderDetail();
    await user.click(await screen.findByRole("button", { name: "Thêm học viên" }));
    const dialog = await screen.findByRole("dialog", {
      name: `Thêm học viên vào ${klass.name}`,
    });
    await user.click(
      await within(dialog).findByRole("checkbox", { name: /Phạm Gia Hân/ }),
    );
    await user.click(within(dialog).getByRole("checkbox", { name: /Đỗ Ngọc Lan/ }));
    await user.click(within(dialog).getByRole("button", { name: "Thêm 2" }));
    expect(await within(dialog).findByText(/Đỗ Ngọc Lan: /)).toBeInTheDocument();
    const toasts = (await screen.findAllByText("Đã thêm 1 học viên")).length;
    await waitFor(() =>
      expect(
        within(dialog).getByRole("checkbox", { name: /Phạm Gia Hân/ }),
      ).toHaveAttribute("aria-disabled", "true"),
    );
    expect(
      within(dialog).getByRole("checkbox", { name: /Phạm Gia Hân/ }),
    ).toHaveTextContent("đã trong lớp");
    expect(
      within(dialog).getByRole("checkbox", { name: /Đỗ Ngọc Lan/ }),
    ).toHaveAttribute("aria-checked", "true");
    await user.click(within(dialog).getByRole("button", { name: "Thêm 1" }));
    await waitFor(() =>
      expect(additions).toEqual([{ userId: HAN }, { userId: LAN }, { userId: LAN }]),
    );
    await waitFor(() =>
      expect(screen.getAllByText("Đã thêm 1 học viên").length).toBeGreaterThan(toasts),
    );
    expect(screen.queryByText("Đã thêm 2 học viên")).toBeNull();
  });
});

describe("the class's own words", () => {
  it("saves the name, schedule and room when something changed, and says students see the new name", async () => {
    const { user } = renderDetail();
    const card = (await screen.findByRole("heading", { name: "Lớp" })).closest(
      "section",
    )!;
    const save = within(card).getByRole("button", { name: "Lưu" });
    expect(save).toHaveAttribute("aria-disabled", "true");
    await user.clear(within(card).getByLabelText("Phòng"));
    await user.type(within(card).getByLabelText("Phòng"), "Phòng 5");
    expect(save).not.toHaveAttribute("aria-disabled");
    await user.click(save);
    await waitFor(() =>
      expect(patches).toEqual([
        {
          name: klass.name,
          description: "",
          scheduleLabel: "Tue · Thu, 17:30",
          room: "Phòng 5",
        },
      ]),
    );
    expect(
      await within(card).findByText("Đã lưu. Học viên sẽ thấy tên mới."),
    ).toBeVisible();
    expect(save).toHaveFocus();
  });
});

describe("the class's assignments", () => {
  it("lists what the class has been given, open first, with the way to the full list", async () => {
    renderDetail();
    const card = (await screen.findByRole("heading", { name: "Bài giao" })).closest(
      "section",
    )!;
    expect(await within(card).findByText("2 bài · 1 đang mở")).toBeVisible();
    const rows = await within(card).findAllByRole("row");
    expect(rows[1]).toHaveTextContent("Unit 5 — Present perfect & listening");
    expect(rows[2]).toHaveTextContent("Unit 4 — Past simple review");
    expect(within(card).getByRole("link", { name: /Xem tất cả/ })).toHaveAttribute(
      "href",
      `/teacher/assignments?classId=${CLASS_ID}`,
    );
    expect(assignmentQueries).toEqual([CLASS_ID]);
  });
});
