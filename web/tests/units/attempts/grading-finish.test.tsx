import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import GradingPage from "@/features/attempts/pages/teacher/GradingPage";
import { Toaster } from "@/components/ui/sonner";
import type {
  AttemptListRow,
  AttemptReview,
  GradingQueueItem,
} from "@/features/attempts/api";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { ASSIGNMENT_ID, BASE, CHOICE_ID, ESSAY_ID, review } from "./fixtures";

const OTHER_ASSIGNMENT = "018f0000-0000-7000-8000-0000000000d2";

type Kind = "hand" | "auto" | "blank" | "pending" | "flagged";

function paper(index: number, over: Partial<AttemptListRow> = {}): AttemptListRow {
  const n = String(index).padStart(12, "0");
  return {
    id: `018f0000-0000-7000-8000-${n}`,
    studentId: `018f0000-0000-7000-9000-${n}`,
    studentName: `Học viên ${index}`,
    assignmentId: ASSIGNMENT_ID,
    testTitle: "Reading Mock 3",
    status: "submitted",
    submittedAt: `2026-09-22T${String(index).padStart(2, "0")}:00:00Z`,
    pendingManual: 0,
    flagged: false,
    ...over,
  };
}

function reviewOf(row: AttemptListRow, kind: Kind = "hand"): AttemptReview {
  const paper = review({ essayScore: kind === "pending" ? null : 3 });
  paper.attempt.id = row.id;
  paper.attempt.assignmentId = row.assignmentId;
  paper.attempt.studentId = row.studentId;
  paper.student.id = row.studentId;
  paper.student.fullName = row.studentName;
  if (kind === "auto") paper.answers[ESSAY_ID]!.requiresManual = false;
  if (kind === "blank") {
    paper.answers[CHOICE_ID]!.answer = null;
    paper.answers[ESSAY_ID]!.answer = null;
    paper.attempt.score = { earned: 0, total: 10, pendingManual: 0 };
  }
  if (kind === "flagged")
    paper.attempt.integrity = { focusLossCount: 3, flagged: true };
  return paper;
}

const waiting: GradingQueueItem = {
  attemptId: "018f0000-0000-7000-8000-0000000000a7",
  questionId: ESSAY_ID,
  assignmentId: ASSIGNMENT_ID,
  assignmentTitle: "Writing Mock 2",
  studentId: "018f0000-0000-7000-8000-0000000000e1",
  studentName: "Phan Mỹ Duyên",
  questionNumber: 2,
  type: "short_answer",
  prompt: "Write about your morning.",
  answer: { type: "text", value: "I wake up at six." },
  points: 1,
  score: null,
  comment: null,
};

let rows: AttemptListRow[];
let kinds: Record<string, Kind>;
let queue: GradingQueueItem[];
let finished: string[];
let scanFails: boolean;
let finishFails: boolean;
let scanHold: Promise<void> | null;

beforeEach(() => {
  useAuthStore.getState().setSession("token", teacherUser);
  rows = [];
  kinds = {};
  queue = [];
  finished = [];
  scanFails = false;
  finishFails = false;
  scanHold = null;
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: queue,
        groups: queue.map((item) => ({
          key: item.studentId,
          kind: "student",
          label: item.studentName,
          sub: item.assignmentTitle,
          remaining: 1,
        })),
        answersRemaining: queue.length,
        studentsWaiting: queue.length,
      }),
    ),
    http.get(`${BASE}/teacher/attempts`, async ({ request }) => {
      if (scanHold) await scanHold;
      if (scanFails)
        return HttpResponse.json(
          {
            error: { code: "UNKNOWN", message: "Scan failed", requestId: "req-scan-1" },
          },
          { status: 500 },
        );
      const status = new URL(request.url).searchParams.get("status");
      const items = rows.filter(
        (row) => row.status === status && !finished.includes(row.id),
      );
      return contractJson("/teacher/attempts", "get", 200, {
        items,
        total: items.length,
        page: 1,
        pageSize: 100,
      });
    }),
    http.get(`${BASE}/teacher/attempts/:id`, ({ params }) => {
      const row = rows.find((candidate) => candidate.id === params.id);
      if (!row)
        return HttpResponse.json(
          { error: { code: "NOT_FOUND", message: "Not found" } },
          { status: 404 },
        );
      return contractJson(
        "/teacher/attempts/{id}",
        "get",
        200,
        reviewOf(row, kinds[row.id]),
      );
    }),
    http.post(`${BASE}/teacher/attempts/:id/finish-grading`, ({ params }) => {
      if (finishFails)
        return HttpResponse.json(
          { error: { code: "UNKNOWN", message: "Finish failed" } },
          { status: 500 },
        );
      const row = rows.find((candidate) => candidate.id === params.id)!;
      finished.push(row.id);
      return contractJson("/teacher/attempts/{id}/finish-grading", "post", 200, {
        ...reviewOf(row).attempt,
        status: "graded",
      });
    }),
  );
});

function mount(entry = "/teacher/grading") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/grading", element: <GradingPage /> },
      { path: "/teacher/attempts/:id", element: <p>Full review</p> },
    ],
    { initialEntries: [entry] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), router };
}

function card() {
  return screen.getByRole("region", { name: "Sẵn sàng hoàn tất" });
}

function hold() {
  let release = () => {};
  scanHold = new Promise<void>((resolve) => {
    release = resolve;
  });
  return () => {
    scanHold = null;
    release();
  };
}

describe("which state shows on an empty queue", () => {
  it("reads All graded only after a complete check with nothing to finish, and draws no card", async () => {
    mount();
    expect(
      await screen.findByRole("heading", { name: "Đã chấm xong tất cả" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Mọi bài nộp đến giờ đều đã được chấm và hoàn tất."),
    ).toBeInTheDocument();
    expect(screen.getByText("Không còn câu trả lời nào cần chấm.")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Sẵn sàng hoàn tất" })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Hiện mọi/ })).toBeNull();
  });

  it("says only that no answers are left while the check runs, with the card open on loading rows", async () => {
    const release = hold();
    rows = [paper(1)];
    mount();
    expect(
      await screen.findByText("Đang kiểm tra các bài còn cần hoàn tất…"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Không còn câu trả lời cần chấm" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Đã chấm xong tất cả")).toBeNull();
    expect(within(card()).getByRole("status")).toBeInTheDocument();
    release();
    expect(
      await screen.findByRole("button", { name: "Xem 1 bài" }),
    ).toBeInTheDocument();
  });

  it("after a failed check offers Try again in the empty state and the card's warning", async () => {
    scanFails = true;
    const { user } = mount();
    expect(
      await screen.findByText(
        "Chưa kiểm tra được hết các bài, nên có thể vẫn còn bài cần hoàn tất.",
      ),
    ).toBeInTheDocument();
    const warning = within(card()).getByRole("alert");
    expect(warning).toHaveTextContent(
      "Một số bài chưa tải được, nên danh sách này có thể chưa đủ.",
    );
    expect(warning).toHaveTextContent("req-scan-1");
    expect(screen.getAllByRole("button", { name: "Thử lại" })).toHaveLength(2);
    scanFails = false;
    rows = [paper(1)];
    await user.click(screen.getAllByRole("button", { name: "Thử lại" })[0]!);
    expect(
      await screen.findByRole("button", { name: "Xem 1 bài" }),
    ).toBeInTheDocument();
  });

  it("with papers to finish shows variant (b), whose action opens the oldest paper's check and focuses its Finish", async () => {
    rows = [
      paper(9, { studentName: "Mới hơn" }),
      paper(3, {
        studentName: "Cũ nhất",
        assignmentId: OTHER_ASSIGNMENT,
        testTitle: "Vocab",
      }),
    ];
    const { user } = mount();
    expect(
      await screen.findByText(
        "2 bài đã chấm nhưng chưa hoàn tất, nên học viên chưa nhận được kết quả.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Không còn câu trả lời cần chấm" }),
    ).toBeInTheDocument();
    const headers = within(card()).getAllByRole("heading", { level: 3 });
    expect(headers.map((header) => header.textContent)).toEqual([
      "Vocab· 1 bài",
      "Reading Mock 3· 1 bài",
    ]);
    await user.click(screen.getByRole("button", { name: "Xem 2 bài" }));
    const finish = await screen.findByRole("button", { name: "Hoàn tất chấm" });
    await waitFor(() => expect(finish).toHaveFocus());
    expect(
      screen.getByRole("button", { name: "Đóng", expanded: true }).closest("li"),
    ).toHaveTextContent("Cũ nhất");
  });
});

describe("the card's rows", () => {
  it("lists five papers per group, then Show more and Show fewer, and a single group has no header", async () => {
    rows = Array.from({ length: 7 }, (_, index) => paper(index + 1));
    const { user } = mount();
    await screen.findByRole("button", { name: "Xem 7 bài" });
    expect(within(card()).queryAllByRole("heading", { level: 3 })).toHaveLength(0);
    expect(within(card()).getAllByRole("button", { name: /^Xem bài/ })).toHaveLength(5);
    const more = within(card()).getByRole("button", { name: "Hiện thêm 2 bài" });
    expect(more).toHaveAttribute("aria-expanded", "false");
    await user.click(more);
    expect(within(card()).getAllByRole("button", { name: /^Xem bài/ })).toHaveLength(7);
    expect(within(card()).getByRole("button", { name: "Thu gọn" })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
  });

  it("adds the earliest hand-in date when two groups share a title", async () => {
    rows = [
      paper(1),
      paper(2, { assignmentId: OTHER_ASSIGNMENT, submittedAt: "2026-09-24T02:00:00Z" }),
    ];
    mount();
    await screen.findByRole("button", { name: "Xem 2 bài" });
    expect(
      within(card())
        .getAllByRole("heading", { level: 3 })
        .map((header) => header.textContent),
    ).toEqual(["Reading Mock 3· 22/09· 1 bài", "Reading Mock 3· 24/09· 1 bài"]);
  });

  it("shows the hand-in, Time ran out and Flagged from the list row, and keeps one check open", async () => {
    rows = [
      paper(1, { studentName: "Trần Minh Anh", status: "timed_out", flagged: true }),
      paper(2, { studentName: "Lê Hoàng Nam" }),
    ];
    const { user } = mount();
    const first = await screen.findByRole("button", {
      name: "Xem bài Reading Mock 3 của Trần Minh Anh",
    });
    const row = first.closest("li")!;
    expect(row).toHaveTextContent("Nộp lúc");
    expect(within(row).getByText("Hết giờ")).toBeInTheDocument();
    expect(within(row).getByText("Cần xem lại")).toBeInTheDocument();
    expect(first).toHaveAttribute("aria-expanded", "false");
    await user.click(first);
    expect(first).toHaveAttribute("aria-expanded", "true");
    expect(first).toHaveTextContent("Đóng");
    await user.click(
      screen.getByRole("button", { name: "Xem bài Reading Mock 3 của Lê Hoàng Nam" }),
    );
    expect(first).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.getAllByRole("button", { expanded: true, name: "Đóng" }),
    ).toHaveLength(1);
  });
});

describe("the check", () => {
  it.each([
    ["hand", "Bạn đã chấm 1 câu", "8 / 10", "Đã trả lời 2/2 câu"],
    ["auto", "Chấm tự động", "8 / 10", "Đã trả lời 2/2 câu"],
    ["blank", "Bỏ trống", "0 / 10", "Đã trả lời 0/2 câu"],
  ] as const)(
    "names a %s paper's marking, score and answered count",
    async (kind, label, score, answered) => {
      rows = [paper(1, { studentName: "Trần Minh Anh" })];
      kinds[rows[0]!.id] = kind;
      const { user } = mount();
      await user.click(
        await screen.findByRole("button", { name: /^Xem bài Reading Mock 3/ }),
      );
      const finish = await screen.findByRole("button", { name: "Hoàn tất chấm" });
      const check = finish.closest("li")!;
      expect(within(check).getByText(label)).toBeInTheDocument();
      expect(within(check).getByText(score)).toBeInTheDocument();
      expect(within(check).getByText(answered)).toBeInTheDocument();
      expect(
        within(check).getByText(
          "Hoàn tất sẽ báo cho Trần Minh Anh rằng kết quả đã sẵn sàng.",
        ),
      ).toBeInTheDocument();
      expect(within(check).queryByText(/^Hoàn tất sẽ ghi điểm 0/) !== null).toBe(
        kind === "blank",
      );
      expect(finish).toBeEnabled();
    },
  );

  it("warns on a flagged paper with its count", async () => {
    rows = [paper(1)];
    kinds[rows[0]!.id] = "flagged";
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: /^Xem bài/ }));
    expect(
      await screen.findByText(
        "Cần xem lại: đã rời bài làm 3 lần. Hãy xem toàn bộ bài trước khi hoàn tất.",
      ),
    ).toBeInTheDocument();
  });

  it("keeps Finish disabled with the waiting line when the fresh read has an answer to mark", async () => {
    rows = [paper(1)];
    kinds[rows[0]!.id] = "pending";
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: /^Xem bài/ }));
    expect(
      await screen.findByText(
        "Bài này vừa có câu trả lời chờ chấm, nên chưa thể hoàn tất.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hoàn tất chấm" })).toBeDisabled();
  });

  it("rereads the paper at Finish and refuses one that changed, without a Finish request", async () => {
    rows = [paper(1)];
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: /^Xem bài/ }));
    const finish = await screen.findByRole("button", { name: "Hoàn tất chấm" });
    kinds[rows[0]!.id] = "pending";
    await user.click(finish);
    expect(
      await screen.findByText(
        "Bài này vừa có câu trả lời chờ chấm, nên chưa thể hoàn tất.",
      ),
    ).toBeInTheDocument();
    expect(finish).toBeDisabled();
    expect(finished).toEqual([]);
  });

  it("shows a Finish failure inside the check, keeps the row, and Finish retries", async () => {
    rows = [paper(1)];
    finishFails = true;
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: /^Xem bài/ }));
    await user.click(await screen.findByRole("button", { name: "Hoàn tất chấm" }));
    const failure = await screen.findByRole("alert");
    expect(failure).toHaveTextContent(
      "Chưa hoàn tất được. Điểm đã được lưu; hãy thử lại.",
    );
    expect(failure.closest("li")).toContainElement(
      screen.getByRole("button", { name: "Hoàn tất chấm" }),
    );
    expect(screen.queryByText(/^Đã lưu điểm cho/)).toBeNull();
    finishFails = false;
    await user.click(screen.getByRole("button", { name: "Hoàn tất chấm" }));
    await waitFor(() => expect(finished).toEqual([rows[0]!.id]));
  });
});

describe("after Finish", () => {
  it("removes the row, toasts, announces the next paper and focuses its Finish; after the last, focuses the empty title", async () => {
    rows = [
      paper(1, { studentName: "Trần Minh Anh" }),
      paper(2, { studentName: "Lê Hoàng Nam" }),
    ];
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: "Xem 2 bài" }));
    await user.click(await screen.findByRole("button", { name: "Hoàn tất chấm" }));
    expect(
      await screen.findByText("Đã hoàn tất chấm bài của Trần Minh Anh."),
    ).toBeInTheDocument();
    const live = document.querySelector("[aria-live=polite].sr-only");
    expect(live).toHaveTextContent(
      "Đã hoàn tất chấm bài của Trần Minh Anh. Tiếp theo: Lê Hoàng Nam.",
    );
    expect(screen.queryByRole("button", { name: /của Trần Minh Anh$/ })).toBeNull();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Hoàn tất chấm" })).toHaveFocus(),
    );
    expect(document.activeElement!.closest("li")).toHaveTextContent("Lê Hoàng Nam");
    await user.click(screen.getByRole("button", { name: "Hoàn tất chấm" }));
    await waitFor(() =>
      expect(
        screen.getByRole("heading", { name: "Đã chấm xong tất cả" }),
      ).toHaveFocus(),
    );
    expect(finished).toEqual([rows[0]!.id, rows[1]!.id]);
  });

  it.each([
    [
      "assignment",
      `?assignment=${ASSIGNMENT_ID}`,
      "Mọi bài nộp cho Reading Mock 3 đều đã được chấm và hoàn tất.",
      "Hiện mọi bài giao",
    ],
    [
      "student",
      "?student=018f0000-0000-7000-9000-000000000001",
      "Mọi bài Trần Minh Anh đã nộp đều đã được chấm và hoàn tất.",
      "Hiện mọi học viên",
    ],
  ])(
    "on a queue narrowed to one %s names it, and Show all clears the filter and focuses the heading",
    async (key, search, hint, action) => {
      rows = [paper(1, { studentName: "Trần Minh Anh" })];
      const { user, router } = mount(`/teacher/grading${search}`);
      await user.click(await screen.findByRole("button", { name: "Xem 1 bài" }));
      await user.click(await screen.findByRole("button", { name: "Hoàn tất chấm" }));
      await waitFor(() =>
        expect(
          screen.getByRole("heading", { name: "Đã chấm xong tất cả" }),
        ).toHaveFocus(),
      );
      expect(screen.getByText(hint)).toBeInTheDocument();
      await user.click(screen.getByRole("button", { name: action }));
      await waitFor(() =>
        expect(new URLSearchParams(router.state.location.search).has(key)).toBe(false),
      );
      await waitFor(() =>
        expect(
          screen.getByRole("heading", { level: 1, name: "Chấm bài" }),
        ).toHaveFocus(),
      );
    },
  );
});

describe("while answers are waiting", () => {
  it("folds the card below the panes, and the header link unfolds it and moves focus to it", async () => {
    queue = [waiting];
    rows = [paper(1), paper(2)];
    const { user } = mount();
    const link = await screen.findByRole("button", { name: "2 bài sẵn sàng hoàn tất" });
    expect(screen.getByText("Còn 1 câu trả lời từ 1 học viên.")).toBeInTheDocument();
    const show = within(card()).getByRole("button", { name: "Hiện" });
    expect(show).toHaveAttribute("aria-expanded", "false");
    expect(within(card()).queryByRole("button", { name: /^Xem bài/ })).toBeNull();
    expect(within(card()).queryByRole("button", { name: "Làm mới" })).toBeNull();
    await user.click(link);
    expect(
      within(card()).getByRole("heading", { name: "Sẵn sàng hoàn tất" }),
    ).toHaveFocus();
    expect(within(card()).getAllByRole("button", { name: /^Xem bài/ })).toHaveLength(2);
    expect(within(card()).getByRole("button", { name: "Ẩn" })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
    expect(screen.getByRole("complementary")).toBeInTheDocument();
  });

  it("draws no card with nothing to finish, and a folded one when the check failed", async () => {
    queue = [waiting];
    const first = mount();
    await screen.findByRole("complementary");
    await waitFor(() =>
      expect(screen.queryByRole("region", { name: "Sẵn sàng hoàn tất" })).toBeNull(),
    );
    expect(screen.queryByRole("button", { name: /sẵn sàng hoàn tất$/ })).toBeNull();
    first.router.dispose();
    document.body.innerHTML = "";
    scanFails = true;
    mount();
    const folded = await screen.findByRole("region", { name: "Sẵn sàng hoàn tất" });
    expect(within(folded).getByRole("button", { name: "Hiện" })).toBeInTheDocument();
  });
});

it("Refresh reads Checking… while it runs, keeps the rows, then shows the time of the check", async () => {
  rows = [paper(1)];
  const { user } = mount();
  await screen.findByRole("button", { name: "Xem 1 bài" });
  expect(within(card()).getByText(/^Đã kiểm tra lúc \d\d:\d\d$/)).toBeInTheDocument();
  const release = hold();
  await user.click(within(card()).getByRole("button", { name: "Làm mới" }));
  const running = await within(card()).findByRole("button", { name: "Đang kiểm tra…" });
  expect(running).toBeDisabled();
  expect(within(card()).queryByText(/^Đã kiểm tra lúc/)).toBeNull();
  expect(within(card()).getByRole("button", { name: /^Xem bài/ })).toBeInTheDocument();
  release();
  expect(await within(card()).findByText(/^Đã kiểm tra lúc/)).toBeInTheDocument();
});
