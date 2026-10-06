import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AssignmentDetailPage from "@/features/assignments/pages/teacher/AssignmentDetailPage";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";

const BASE = "http://localhost:8080";
const ID = "018f0000-0000-7000-8000-0000000000d1";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const VERSION_ID = "018f0000-0000-7000-8000-0000000000f1";
const CLASS_ID = "018f0000-0000-7000-8000-0000000000c1";
const HAN = "018f0000-0000-7000-8000-0000000000e1";
const KHOA = "018f0000-0000-7000-8000-0000000000e2";
type Assignment = components["schemas"]["Assignment"];

function assignment(over: Partial<Assignment> = {}): Assignment {
  return {
    id: ID,
    testId: TEST_ID,
    testVersionId: VERSION_ID,
    testVersion: 3,
    testTitle: "Unit 5 — Present perfect & listening",
    targets: {
      classes: [{ id: CLASS_ID, name: "IELTS Foundation", studentCount: 18 }],
      students: [
        { id: HAN, name: "Phạm Gia Hân" },
        { id: KHOA, name: "Vũ Minh Khoa" },
      ],
    },
    publishedAt: "2026-08-27T00:00:00Z",
    updatedAt: "2026-08-27T02:12:00Z",
    window: {
      opensAt: "2020-09-07T01:00:00Z",
      closesAt: "2099-09-09T14:00:00Z",
      closedAt: null,
    },
    durationMinutes: 45,
    maxAttempts: 2,
    shuffleQuestions: true,
    shuffleOptions: true,
    review: { showScore: true, showCorrectAnswers: false, showExplanations: true },
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 2,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
    status: "open",
    submittedCount: 17,
    targetCount: 19,
    flaggedCount: 2,
    pendingGradingCount: 4,
    ...over,
  };
}

let patches: unknown[] = [];

function serve(a: Assignment) {
  server.use(
    http.get(`${BASE}/teacher/assignments/${ID}`, () =>
      contractJson("/teacher/assignments/{id}", "get", 200, a),
    ),
    http.get(`${BASE}/teacher/tests/${TEST_ID}/versions`, () =>
      contractJson("/teacher/tests/{id}/versions", "get", 200, {
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
    http.get(`${BASE}/teacher/assignments/${ID}/attempts`, () =>
      contractJson("/teacher/assignments/{id}/attempts", "get", 200, {
        serverTime: "2026-09-04T02:10:00Z",
        questionCount: 24,
        rows: [
          {
            studentId: HAN,
            fullName: "Phạm Gia Hân",
            state: "not_started",
            flagged: false,
            audioOverLimit: false,
          },
        ],
      }),
    ),
    http.patch(`${BASE}/teacher/assignments/${ID}`, async ({ request }) => {
      patches.push(await request.json());
      return contractJson("/teacher/assignments/{id}", "patch", 200, a);
    }),
  );
}

function renderDetail(tab = "students") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/assignments/:id", element: <AssignmentDetailPage /> },
      { path: "/teacher/assignments/:id/edit", element: <p>edit form</p> },
    ],
    { initialEntries: [`/teacher/assignments/${ID}?tab=${tab}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

const scheduledWindow = {
  opensAt: "2099-09-07T01:00:00Z",
  closesAt: "2099-09-09T14:00:00Z",
  closedAt: null,
};
const pastWindow = {
  opensAt: "2020-09-07T01:00:00Z",
  closesAt: "2020-09-09T14:00:00Z",
  closedAt: null,
};

describe("the assignment detail", () => {
  beforeEach(() => {
    useAuthStore.getState().setSession("token", teacherUser);
    patches = [];
  });

  it("summarises what G-01 saved", async () => {
    serve(assignment({ status: "scheduled", window: scheduledWindow }));
    renderDetail("settings");

    expect(
      await screen.findByText(
        "24 câu · 30 điểm · 4 câu nghe · 2 câu chấm tay · bản v3",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("IELTS Foundation")[0]).toBeInTheDocument();
    expect(screen.getAllByText("Phạm Gia Hân")[0]).toBeInTheDocument();
    expect(
      screen.getByText("1 học viên đã có trong lớp nên chỉ tính một lần."),
    ).toBeInTheDocument();
    expect(screen.getByText("2 lượt · lấy điểm cao nhất")).toBeInTheDocument();
    expect(
      screen.getByText("Cho phép 2 lần, quá thì đánh dấu để xem lại"),
    ).toBeInTheDocument();
    expect(screen.getByText("Có, trong từng phần")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Xem đề" })).toHaveAttribute(
      "href",
      `/teacher/tests/${TEST_ID}`,
    );
  });

  it("draft: can be edited or given out, and says students cannot see it", async () => {
    serve(assignment({ publishedAt: null, status: "draft" }));
    const user = renderDetail("settings");

    expect(await screen.findByText("Bản nháp")).toBeInTheDocument();
    expect(screen.getByText(/Học viên chưa thấy bài này/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Chỉnh sửa" })).toHaveAttribute(
      "href",
      `/teacher/assignments/${ID}/edit`,
    );

    await user.click(screen.getByRole("button", { name: "Giao bài" }));
    await waitFor(() => expect(patches).toHaveLength(1));
    expect(patches[0]).toMatchObject({
      draft: false,
      testVersionId: VERSION_ID,
      targets: { classIds: [CLASS_ID], studentIds: [HAN, KHOA] },
    });
  });

  it("draft without targets: the button waits, with the reason beside it", async () => {
    serve(
      assignment({
        publishedAt: null,
        status: "draft",
        targets: { classes: [], students: [] },
        targetCount: 0,
      }),
    );
    renderDetail();

    expect(await screen.findByRole("button", { name: "Giao bài" })).toBeDisabled();
    expect(
      screen.getByText(/Chọn lớp hoặc học viên trước khi giao\./),
    ).toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("tab", { name: "Cài đặt" }));
    expect(await screen.findByText("Chưa chọn lớp hay học viên.")).toBeInTheDocument();
  });

  it("scheduled: preserves Edit and never sends closeNow before the opening window", async () => {
    serve(assignment({ status: "scheduled", window: scheduledWindow }));
    const user = renderDetail();
    await user.click(await screen.findByRole("button", { name: "Thao tác" }));
    expect(await screen.findByRole("menuitem", { name: "Chỉnh sửa" })).toHaveAttribute(
      "href",
      `/teacher/assignments/${ID}/edit`,
    );
    expect(screen.queryByRole("menuitem", { name: "Đóng sớm" })).toBeNull();
    expect(patches).toEqual([]);
  });

  it("open: close early asks for one tick, then sends closeNow", async () => {
    serve(assignment());
    const user = renderDetail();

    await user.click(await screen.findByRole("button", { name: "Thao tác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Đóng sớm" }));
    const dialog = await screen.findByRole("dialog");
    const confirm = within(dialog).getByRole("button", { name: "Đóng ngay" });
    expect(confirm).toBeDisabled();
    expect(patches).toHaveLength(0);

    await user.click(within(dialog).getByRole("checkbox"));
    await user.click(confirm);
    await waitFor(() => expect(patches).toHaveLength(1));
    expect(patches[0]).toMatchObject({ closeNow: true, draft: false });
  });

  it("open: names who the work went to, with the class one click away", async () => {
    serve(assignment());
    renderDetail("settings");

    await screen.findByRole("tab", { name: "Cài đặt" });
    expect(screen.getByRole("link", { name: "IELTS Foundation" })).toHaveAttribute(
      "href",
      `/teacher/classes/${CLASS_ID}`,
    );
    expect(screen.getByText("+2 học viên lẻ")).toBeInTheDocument();
    expect(screen.getByText("· 19 học viên")).toBeInTheDocument();
  });

  it("closed: derives numbers from the actual full roster and preserves students without inviting an edit", async () => {
    serve(assignment({ status: "closed", window: pastWindow }));
    renderDetail();
    const table = await screen.findByRole("table");
    expect(within(table).getByText("Phạm Gia Hân")).toBeInTheDocument();
    expect(within(table).getByText("Chưa bắt đầu")).toBeInTheDocument();
    expect(screen.getByText("/ 1")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Chỉnh sửa" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Đóng sớm" })).toBeNull();
    expect(screen.getByRole("button", { name: "Gia hạn cho tất cả" })).toBeEnabled();
  });

  it("closed: keeps graded and flagged work in the roster and counts pending answers independently", async () => {
    serve(assignment({ status: "closed", window: pastWindow }));
    server.use(
      http.get(`${BASE}/teacher/assignments/${ID}/attempts`, () =>
        contractJson("/teacher/assignments/{id}/attempts", "get", 200, {
          serverTime: "2026-09-04T02:10:00Z",
          questionCount: 24,
          rows: [
            {
              studentId: HAN,
              fullName: "Phạm Gia Hân",
              state: "submitted",
              attemptId: "018f0000-0000-7000-8000-0000000000a7",
              attemptNo: 1,
              submittedAt: "2026-09-04T02:47:00Z",
              score: { earned: 22, total: 30, pendingManual: 2 },
              focusLossCount: 3,
              flagged: true,
              audioOverLimit: false,
            },
            {
              studentId: KHOA,
              fullName: "Vũ Minh Khoa",
              state: "not_started",
              flagged: false,
              audioOverLimit: false,
            },
          ],
        }),
      ),
    );
    renderDetail();
    const table = await screen.findByRole("table");
    expect(within(table).getByRole("button", { name: "Phạm Gia Hân" })).toBeEnabled();
    expect(within(table).queryByRole("button", { name: "Vũ Minh Khoa" })).toBeNull();
    expect(screen.getByRole("button", { name: "Chấm 2 câu trả lời" })).toBeEnabled();
    expect(within(table).getByRole("row", { name: /Phạm Gia Hân/ })).toHaveClass(
      "bg-danger-soft",
    );
    expect(screen.getByText("/ 2")).toBeInTheDocument();
    expect(screen.queryByRole("complementary")).toBeNull();
  });

  it("closed: Gia hạn cho tất cả asks for a moment and a reason, then reopens", async () => {
    let sent: { closesAt: string; reason: string } | null = null;
    server.use(
      http.post(`${BASE}/teacher/assignments/${ID}/reopen`, async ({ request }) => {
        sent = (await request.json()) as { closesAt: string; reason: string };
        return contractJson(
          "/teacher/assignments/{id}/reopen",
          "post",
          200,
          assignment(),
        );
      }),
    );
    serve(assignment({ status: "closed", window: pastWindow }));
    const user = renderDetail();

    await user.click(await screen.findByRole("button", { name: "Gia hạn cho tất cả" }));
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByText("Mở lại cho cả 19 học viên")).toBeInTheDocument();
    expect(within(menu).getByText(/Bước sau sẽ hỏi lý do/)).toBeInTheDocument();
    await user.click(within(menu).getByRole("menuitem", { name: "Thêm 3 ngày" }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Mở lại cho cả 19 học viên")).toBeInTheDocument();
    const confirm = within(dialog).getByRole("button", { name: "Mở lại" });
    expect(confirm).toBeDisabled();
    expect(within(dialog).queryByRole("radio")).toBeNull();
    await user.type(within(dialog).getByLabelText("Lý do"), "mất điện cả lớp");
    expect(confirm).toBeEnabled();
    await user.click(confirm);

    await waitFor(() => expect(sent).not.toBeNull());
    expect(sent!.reason).toBe("mất điện cả lớp");
    const inThreeDays = Date.now() + 3 * 24 * 60 * 60 * 1000;
    expect(Math.abs(new Date(sent!.closesAt).getTime() - inThreeDays)).toBeLessThan(
      60_000,
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });
});
