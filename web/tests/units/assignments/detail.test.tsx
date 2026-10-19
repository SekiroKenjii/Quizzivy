import { beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
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
    review: {
      showScore: true,
      showCorrectAnswers: false,
      showExplanations: true,
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
            skills: ["grammar", "reading"],
            assignmentCount: 0,
            changeNote: null,
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
        {},
        { timeout: 5_000 },
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "IELTS Foundation" })).toBeInTheDocument();
    expect(screen.getByText("+2 học viên lẻ")).toBeInTheDocument();
    expect(screen.getByText("2 lượt · lấy điểm cao nhất")).toBeInTheDocument();
    expect(
      screen.getByText("Cho phép 2 lần, quá thì đánh dấu để xem lại"),
    ).toBeInTheDocument();
    expect(screen.getByText(/^Đảo câu hỏi trong từng phần/)).toBeInTheDocument();
    for (const title of [
      "Đề và thời gian",
      "Thời gian mở",
      "Tính toàn vẹn",
      "Kết quả",
      "Lời nhắn cho học viên",
    ])
      expect(screen.getByRole("region", { name: title })).toBeInTheDocument();
  });

  it("draws only the item analysis on the Questions tab, as the deck does", async () => {
    serve(assignment({ status: "scheduled", window: scheduledWindow }));
    renderDetail("questions");
    expect(
      await screen.findByText(
        "Các câu hỏi xếp theo mức độ học viên làm sai nhiều nhất.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Xem đề" })).toBeNull();
    expect(screen.queryByText(/Bài giao gắn với bản v/)).toBeNull();
  });

  it("links the pinned test from Settings and says which version students get", async () => {
    serve(assignment({ status: "scheduled", window: scheduledWindow }));
    renderDetail("settings");
    const timing = await screen.findByRole("region", { name: "Đề và thời gian" });
    expect(
      await within(timing).findByRole("link", {
        name: "Unit 5 — Present perfect & listening",
      }),
    ).toHaveAttribute("href", `/teacher/tests/${TEST_ID}`);
    expect(
      within(timing).getByText(
        "Bài giao gắn với bản v3. Đề có sửa sau này, học viên vẫn làm đúng bản này.",
      ),
    ).toBeInTheDocument();
  });

  it("says students took the pinned version once the assignment is closed", async () => {
    serve(assignment({ status: "closed", window: pastWindow }));
    renderDetail("settings");
    expect(
      await screen.findByText(
        "Học viên đã làm trên bản v3, kể cả khi đề hiện có bản mới hơn.",
      ),
    ).toBeInTheDocument();
  });

  it("names the test without a link when its versions cannot be read", async () => {
    serve(assignment({ status: "scheduled", window: scheduledWindow }));
    server.use(
      http.get(`${BASE}/teacher/tests/${TEST_ID}/versions`, () =>
        HttpResponse.json(
          { error: { code: "NOT_FOUND", message: "Not found", requestId: "versions" } },
          { status: 404 },
        ),
      ),
    );
    renderDetail("settings");
    const timing = await screen.findByRole("region", { name: "Đề và thời gian" });
    expect(
      await within(timing).findByText("Unit 5 — Present perfect & listening"),
    ).toBeInTheDocument();
    expect(within(timing).getByText("bản v3")).toBeInTheDocument();
    expect(within(timing).queryByRole("link")).toBeNull();
    expect(within(timing).getByText(/Bài giao gắn với bản v3\./)).toBeInTheDocument();
  });

  it("draft: can be edited or given out, and says students cannot see it", async () => {
    serve(assignment({ publishedAt: null, status: "draft" }));
    const user = renderDetail("settings");

    expect(await screen.findByText("Bản nháp")).toBeInTheDocument();
    expect(screen.getByText(/Học viên chưa thấy bài này/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Thao tác khác" }));
    expect(await screen.findByRole("menuitem", { name: "Chỉnh sửa" })).toHaveAttribute(
      "href",
      `/teacher/assignments/${ID}/edit`,
    );
    await user.keyboard("{Escape}");

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

  it("scheduled: Edit settings opens the Settings tab and never sends closeNow before the opening window", async () => {
    serve(assignment({ status: "scheduled", window: scheduledWindow }));
    const user = renderDetail();
    await user.click(await screen.findByRole("button", { name: "Thao tác khác" }));
    expect(screen.queryByRole("menuitem", { name: "Đóng sớm" })).toBeNull();
    await user.click(await screen.findByRole("menuitem", { name: "Sửa cài đặt" }));
    expect(await screen.findByRole("tab", { name: "Cài đặt" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(patches).toEqual([]);
  });

  it("open: close early asks for one tick, then sends closeNow", async () => {
    serve(assignment());
    const user = renderDetail();

    await user.click(await screen.findByRole("button", { name: "Thao tác khác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Đóng sớm" }));
    const dialog = await screen.findByRole("dialog", {
      name: /^Đóng .* ngay\?$/,
    });
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
    expect(screen.getByText("19 học viên")).toBeInTheDocument();
  });

  it("closed: derives numbers from the actual full roster and preserves students without inviting an edit", async () => {
    serve(assignment({ status: "closed", window: pastWindow }));
    renderDetail();
    const table = await screen.findByRole("table");
    expect(within(table).getByText("Phạm Gia Hân")).toBeInTheDocument();
    expect(within(table).getByText("Chưa bắt đầu")).toBeInTheDocument();
    expect(screen.getByText("Đã nộp 0/1")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Chỉnh sửa" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Đóng sớm" })).toBeNull();
    expect(screen.getByRole("button", { name: "Gia hạn" })).toBeEnabled();
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
    expect(screen.getByText("Đã nộp 1/2")).toBeInTheDocument();
    expect(screen.queryByRole("complementary")).toBeNull();
  });

  it("closed: Extend asks for a reason, then reopens from now", async () => {
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

    const extend = await screen.findByRole("button", { name: "Gia hạn" });
    await user.click(extend);
    const dialog = await screen.findByRole("dialog", { name: "Gia hạn" });
    await user.type(within(dialog).getByLabelText(/Lý do/), "mất điện cả lớp");
    await user.click(within(dialog).getByRole("button", { name: "Gia hạn" }));

    await waitFor(() => expect(sent).not.toBeNull());
    expect(sent!.reason).toBe("mất điện cả lớp");
    const inHalfAnHour = Date.now() + 30 * 60 * 1000;
    expect(Math.abs(new Date(sent!.closesAt).getTime() - inHalfAnHour)).toBeLessThan(
      60_000,
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(extend).toHaveFocus();
  });

  it("draft: Giao bài keeps focus while its request is pending", async () => {
    let release: () => void = () => undefined;
    serve(assignment({ publishedAt: null, status: "draft" }));
    server.use(
      http.patch(`${BASE}/teacher/assignments/${ID}`, async () => {
        await new Promise<void>((resolve) => {
          release = resolve;
        });
        return contractJson(
          "/teacher/assignments/{id}",
          "patch",
          200,
          assignment({ publishedAt: null, status: "draft" }),
        );
      }),
    );
    const user = renderDetail();
    const publish = await screen.findByRole("button", { name: "Giao bài" });
    publish.focus();
    await user.keyboard("{Enter}");
    await waitFor(() => expect(publish).toHaveAttribute("aria-disabled", "true"));
    expect(publish).not.toBeDisabled();
    expect(publish).toHaveFocus();
    release();
    await waitFor(() => expect(publish).not.toHaveAttribute("aria-disabled"));
    expect(publish).toHaveFocus();
  });

  it("draft: moves focus to the heading once Giao bài is gone after it succeeds", async () => {
    serve(assignment({ publishedAt: null, status: "draft" }));
    const published = assignment({ status: "scheduled", window: scheduledWindow });
    server.use(
      http.patch(`${BASE}/teacher/assignments/${ID}`, () => {
        server.use(
          http.get(`${BASE}/teacher/assignments/${ID}`, () =>
            contractJson("/teacher/assignments/{id}", "get", 200, published),
          ),
        );
        return contractJson("/teacher/assignments/{id}", "patch", 200, published);
      }),
    );
    const user = renderDetail();
    const publish = await screen.findByRole("button", { name: "Giao bài" });
    publish.focus();
    await user.keyboard("{Enter}");
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Giao bài" })).toBeNull(),
    );
    await waitFor(() =>
      expect(
        screen.getByRole("heading", { level: 1, name: published.testTitle }),
      ).toHaveFocus(),
    );
  });
});

type Row = components["schemas"]["MonitorRow"];

function student(n: number, state: Row["state"], over: Partial<Row> = {}): Row {
  const id = `018f0000-0000-7000-8000-0000000001${String(n).padStart(2, "0")}`;
  const started = state !== "not_started";
  return {
    studentId: id,
    fullName: `Học viên ${String(n).padStart(2, "0")}`,
    state,
    flagged: false,
    audioOverLimit: false,
    ...(started
      ? {
          attemptId: `018f0000-0000-7000-8000-0000000002${String(n).padStart(2, "0")}`,
          attemptNo: 1,
          startedAt: "2026-09-04T01:00:00Z",
        }
      : {}),
    ...over,
  };
}

function serveRoster(rows: Row[]) {
  serve(assignment());
  server.use(
    http.get(`${BASE}/teacher/assignments/${ID}/attempts`, () =>
      contractJson("/teacher/assignments/{id}/attempts", "get", 200, {
        serverTime: "2026-09-04T02:10:00Z",
        questionCount: 24,
        rows,
      }),
    ),
  );
}

function renderAt(search: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/assignments/:id", element: <AssignmentDetailPage /> }],
    { initialEntries: [`/teacher/assignments/${ID}${search}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), router };
}

function legend() {
  return screen.getByRole("group", { name: "Lọc học viên" });
}

function rosterNames() {
  return within(screen.getByRole("table"))
    .getAllByRole("row")
    .slice(1)
    .map((row) => row.textContent ?? "");
}

describe("the progress card", () => {
  beforeEach(() => {
    useAuthStore.getState().setSession("token", teacherUser);
  });

  it("says how many submitted, that all did, or that nobody is assigned", async () => {
    serveRoster([
      student(1, "submitted"),
      student(2, "in_progress"),
      student(3, "not_started"),
    ]);
    renderAt("");
    expect(await screen.findByText("Đã nộp 1/3")).toBeInTheDocument();
  });

  it("says All submitted once every student has handed in, timed out included", async () => {
    serveRoster([student(1, "graded"), student(2, "timed_out")]);
    renderAt("");
    expect(await screen.findByText("Đã nộp đủ 2/2")).toBeInTheDocument();
  });

  it("says Not assigned yet with nobody targeted, and draws an empty bar", async () => {
    serveRoster([]);
    renderAt("");
    expect(await screen.findByText("Chưa giao cho học viên nào")).toBeInTheDocument();
    const bar = document.querySelector<HTMLElement>('[data-slot="progress-bar"]')!;
    for (const segment of bar.children)
      expect((segment as HTMLElement).style.width).toBe("0%");
    expect(within(legend()).queryAllByRole("button")).toHaveLength(0);
  });

  it("draws submitted then in progress as shares of everyone, and the graded average", async () => {
    serveRoster([
      student(1, "submitted"),
      student(2, "graded", { score: { earned: 6, total: 8, pendingManual: 0 } }),
      student(3, "in_progress"),
      student(4, "not_started"),
      student(5, "not_started"),
      student(6, "not_started"),
      student(7, "not_started"),
      student(8, "not_started"),
    ]);
    renderAt("");
    await screen.findByText("Đã nộp 2/8");
    const bar = document.querySelector<HTMLElement>('[data-slot="progress-bar"]')!;
    expect(bar).toHaveClass("h-2", "bg-muted");
    const [submitted, progress] = [...bar.children] as HTMLElement[];
    expect(submitted).toHaveAttribute("data-group", "submitted");
    expect(submitted).toHaveClass("bg-success");
    expect(submitted!.style.width).toBe("25%");
    expect(progress).toHaveClass("bg-info");
    expect(progress!.style.width).toBe("12.5%");
    const average = screen.getByText("75%");
    expect(average).toHaveClass("text-md", "font-semibold", "text-fg");
    expect(average.parentElement).toHaveTextContent("Trung bình75%");
  });

  it("filters the roster by a legend press, opens Students, resets the page and names the filter, and a second press shows everyone", async () => {
    serveRoster([
      student(1, "submitted"),
      student(2, "timed_out"),
      student(3, "in_progress", { flagged: true, focusLossCount: 4 }),
      student(4, "not_started"),
    ]);
    const { user, router } = renderAt("?tab=questions&page=2&size=10");
    const inProgress = await within(
      await screen.findByRole("group", { name: "Lọc học viên" }),
    ).findByRole("button", { name: "Đang làm 1" });
    expect(inProgress).toHaveAttribute("aria-pressed", "false");

    await user.click(inProgress);
    await waitFor(() =>
      expect(router.state.location.search).toBe(
        "?tab=students&size=10&roster=inProgress",
      ),
    );
    expect(screen.getByRole("tab", { name: "Học viên" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      within(legend()).getByRole("button", { name: "Đang làm 1" }),
    ).toHaveAttribute("aria-pressed", "true");
    await waitFor(() => expect(rosterNames()).toHaveLength(1));
    expect(rosterNames()[0]).toContain("Học viên 03");
    expect(screen.getByRole("combobox", { name: "Lọc bài làm" })).toHaveTextContent(
      "Đang làm",
    );

    const submitted = within(legend()).getByRole("button", { name: "Đã nộp 2" });
    submitted.focus();
    await user.keyboard("{Enter}");
    await waitFor(() =>
      expect(router.state.location.search).toBe(
        "?tab=students&size=10&roster=submitted",
      ),
    );
    await waitFor(() => expect(rosterNames()).toHaveLength(2));
    expect(rosterNames().join(" ")).toContain("Học viên 02");

    await user.keyboard(" ");
    await waitFor(() =>
      expect(router.state.location.search).toBe("?tab=students&size=10"),
    );
    expect(within(legend()).getByRole("button", { name: "Đã nộp 2" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    await waitFor(() => expect(rosterNames()).toHaveLength(4));
  });

  it("counts the whole assignment, not the searched or filtered roster", async () => {
    serveRoster([
      student(1, "submitted"),
      student(2, "submitted"),
      student(3, "not_started"),
    ]);
    renderAt("?q=01&roster=notStarted");
    expect(await screen.findByText("Đã nộp 2/3")).toBeInTheDocument();
    expect(
      within(legend()).getByRole("button", { name: "Đã nộp 2" }),
    ).toBeInTheDocument();
    expect(
      within(legend()).getByRole("button", { name: "Chưa bắt đầu 1" }),
    ).toHaveAttribute("aria-pressed", "true");
  });

  it("hides an entry at zero unless it is the one selected, and marks Flagged in danger ink", async () => {
    serveRoster([student(1, "submitted"), student(2, "not_started")]);
    renderAt("");
    await screen.findByText("Đã nộp 1/2");
    expect(
      within(legend())
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Đã nộp 1", "Chưa bắt đầu 1"]);
    cleanup();

    serveRoster([student(1, "submitted"), student(2, "not_started")]);
    renderAt("?roster=flagged");
    const flagged = await within(
      await screen.findByRole("group", { name: "Lọc học viên" }),
    ).findByRole("button", { name: "Bị gắn cờ 0" });
    expect(flagged).toHaveAttribute("aria-pressed", "true");
    expect(within(flagged).getByText("0")).toHaveClass("text-danger-ink");
    expect(flagged.querySelector("svg")).toHaveClass("text-danger-ink");
    expect(
      within(legend())
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Đã nộp 1", "Chưa bắt đầu 1", "Bị gắn cờ 0"]);
  });

  it("names what a press will do in its tooltip", async () => {
    serveRoster([student(1, "submitted"), student(2, "in_progress")]);
    const { user } = renderAt("?roster=submitted");
    const submitted = await within(
      await screen.findByRole("group", { name: "Lọc học viên" }),
    ).findByRole("button", { name: "Đã nộp 1" });
    await user.hover(submitted);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Hiện tất cả học viên",
    );
    cleanup();

    serveRoster([student(1, "submitted"), student(2, "in_progress")]);
    const again = renderAt("");
    const inProgress = await within(
      await screen.findByRole("group", { name: "Lọc học viên" }),
    ).findByRole("button", { name: "Đang làm 1" });
    await again.user.hover(inProgress);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Chỉ hiện học viên đang làm",
    );
  });

  it("drops the filter when the page is opened again from the list", async () => {
    serveRoster([student(1, "submitted"), student(2, "not_started")]);
    const { router } = renderAt("?tab=students&roster=notStarted");
    await waitFor(() => expect(rosterNames()).toHaveLength(1));
    await act(() => router.navigate(`/teacher/assignments/${ID}`));
    await waitFor(() => expect(rosterNames()).toHaveLength(2));
    for (const button of within(legend()).getAllByRole("button"))
      expect(button).toHaveAttribute("aria-pressed", "false");
  });

  it("holds a skeleton while the roster loads and offers a retry when it fails", async () => {
    serve(assignment());
    let fail = true;
    server.use(
      http.get(`${BASE}/teacher/assignments/${ID}/attempts`, () =>
        fail
          ? HttpResponse.json(
              { error: { code: "INTERNAL", message: "boom", requestId: "r-1" } },
              { status: 500 },
            )
          : contractJson("/teacher/assignments/{id}/attempts", "get", 200, {
              serverTime: "2026-09-04T02:10:00Z",
              questionCount: 24,
              rows: [student(1, "submitted")],
            }),
      ),
    );
    const { user } = renderAt("?tab=settings");
    expect(
      await screen.findByRole("status", { name: "Đang tải…" }),
    ).toBeInTheDocument();
    const failed = await screen.findByText("Chưa tải được tiến độ.");
    fail = false;
    await user.click(
      within(failed.parentElement!).getByRole("button", { name: "Thử lại" }),
    );
    expect(await screen.findByText("Đã nộp đủ 1/1")).toBeInTheDocument();
  });
});
