import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AssignmentDetailPage from "@/features/assignments/pages/AssignmentDetailPage";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

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
    http.get(`${BASE}/admin/assignments/${ID}`, () =>
      contractJson("/admin/assignments/{id}", "get", 200, a),
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
    http.patch(`${BASE}/admin/assignments/${ID}`, async ({ request }) => {
      patches.push(await request.json());
      return contractJson("/admin/assignments/{id}", "patch", 200, a);
    }),
  );
}

function renderDetail() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/assignments/:id", element: <AssignmentDetailPage /> },
      { path: "/admin/assignments/:id/edit", element: <p>edit form</p> },
    ],
    { initialEntries: [`/admin/assignments/${ID}`] },
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
    patches = [];
  });

  it("summarises what G-01 saved", async () => {
    serve(assignment());
    renderDetail();

    expect(
      await screen.findByText(
        "24 câu · 30 điểm · 4 câu nghe · 2 câu chấm tay · bản v3",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("IELTS Foundation · 18")).toBeInTheDocument();
    expect(screen.getByText("Phạm Gia Hân")).toBeInTheDocument();
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
      `/admin/tests/${TEST_ID}`,
    );
  });

  it("draft: can be edited or given out, and says students cannot see it", async () => {
    serve(assignment({ publishedAt: null, status: "draft" }));
    const user = renderDetail();

    expect(await screen.findByText("Nháp")).toBeInTheDocument();
    expect(screen.getByText(/Học viên chưa thấy bài này/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Chỉnh sửa" })).toHaveAttribute(
      "href",
      `/admin/assignments/${ID}/edit`,
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
      screen.getByText("Chọn lớp hoặc học viên trước khi giao."),
    ).toBeInTheDocument();
    expect(screen.getByText("Chưa chọn lớp hay học viên.")).toBeInTheDocument();
  });

  it("scheduled: close early stays visible but off, with the reason", async () => {
    serve(assignment({ status: "scheduled", window: scheduledWindow }));
    renderDetail();

    expect(await screen.findByRole("button", { name: "Đóng sớm" })).toBeDisabled();
    expect(screen.getByText("Đóng sớm bật khi bài đã mở")).toBeInTheDocument();
    expect(screen.getByText(/còn \d+ ngày \d+ giờ/)).toBeInTheDocument();
  });

  it("open: close early asks for one tick, then sends closeNow", async () => {
    serve(assignment());
    const user = renderDetail();

    await user.click(await screen.findByRole("button", { name: "Đóng sớm" }));
    const dialog = await screen.findByRole("dialog");
    const confirm = within(dialog).getByRole("button", { name: "Đóng ngay" });
    expect(confirm).toBeDisabled();
    expect(patches).toHaveLength(0);

    await user.click(within(dialog).getByRole("checkbox"));
    await user.click(confirm);
    await waitFor(() => expect(patches).toHaveLength(1));
    expect(patches[0]).toMatchObject({ closeNow: true, draft: false });
  });

  it("closed: the numbers come first and nothing invites an edit", async () => {
    serve(assignment({ status: "closed", window: pastWindow }));
    renderDetail();

    expect(await screen.findByText("Chờ chấm")).toBeInTheDocument();
    expect(screen.getByText("17")).toBeInTheDocument();
    expect(screen.getByText("/19")).toBeInTheDocument();
    expect(screen.getByText("Chưa nộp: 2")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("Rời trang quá 2 lần")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Đóng sớm" })).toBeNull();
    expect(screen.queryByRole("link", { name: "Chỉnh sửa" })).toBeNull();
  });
});
