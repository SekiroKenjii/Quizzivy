import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import ImportConfirmPage from "@/features/imports/pages/teacher/ImportConfirmPage";
import type { ImportReview, WordImport } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  BASE,
  IMPORT_ID,
  TEST_ID,
  deferred,
  errorBody,
  finding,
  question,
  review,
  section,
  summary,
  text,
  wordImport,
} from "./fixtures";
import "@/lib/i18n";

interface Commit {
  requestId: string;
  draftRevision: number;
}

let commits: Commit[];
let reviewReads: number;
let current: ImportReview;
let value: WordImport;

function ready(over: Partial<ImportReview> = {}): ImportReview {
  return review(
    [
      section([
        question({
          id: "q1",
          label: "1",
          origins: {
            type: "inferred_structure",
            prompt: "source_explicit",
            options: "source_explicit",
            answer: "source_explicit",
            points: "teacher_entered",
          },
        }),
      ]),
    ],
    [],
    {
      ready: true,
      summary: summary({
        questions: 1,
        included: 1,
        totalPoints: "1.00",
        answersKnown: 1,
        answersMissing: 0,
        blocking: 0,
      }),
      ...over,
    },
  );
}

function open(): ImportReview {
  return review(
    [
      section([
        question({ id: "q1", label: "1" }),
        question({
          id: "q2",
          label: "2",
          answer: { state: "unknown", optionIds: [], evidence: [] },
        }),
        question({ id: "q3", label: "3", excluded: { reason: "Trùng câu 1" } }),
      ]),
      {
        id: "section-2",
        title: "Phần 2",
        origin: "source_explicit",
        source: [],
        items: [
          {
            group: {
              id: "group-1",
              label: "4",
              stimulus: text("The park is near."),
              gaps: [
                { gapId: "p1", questionId: "q4" },
                { gapId: "p2", questionId: "q4" },
              ],
              source: [],
              questions: [question({ id: "q4", label: "4.1" })],
            },
          },
        ],
      },
    ],
    [
      finding({
        id: "f-missing",
        code: "MISSING_ANSWER",
        severity: "blocking",
        target: "q2",
      }),
    ],
    {
      summary: summary({
        sections: 2,
        groups: 1,
        questions: 4,
        included: 3,
        excluded: 1,
        totalPoints: "3.00",
        answersKnown: 2,
        answersMissing: 1,
        blocking: 1,
      }),
    },
  );
}

function committed() {
  return contractJson("/teacher/imports/{id}/commit", "post", 200, {
    testId: TEST_ID,
    import: wordImport({ status: "committed", testId: TEST_ID, revision: 7 }),
  });
}

function serve() {
  server.use(
    http.get(`${BASE}/teacher/imports/:id`, () =>
      contractJson("/teacher/imports/{id}", "get", 200, value),
    ),
    http.get(`${BASE}/teacher/imports/:id/review`, () => {
      reviewReads += 1;
      return contractJson("/teacher/imports/{id}/review", "get", 200, current);
    }),
  );
}

function onCommit(answer: (attempt: number) => Response | Promise<Response>) {
  server.use(
    http.post(`${BASE}/teacher/imports/:id/commit`, async ({ request }) => {
      commits.push((await request.json()) as Commit);
      return answer(commits.length);
    }),
  );
}

async function renderConfirm() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/imports/:id/confirm", element: <ImportConfirmPage /> },
      { path: "/teacher/imports/:id/review", element: <p>review page</p> },
      { path: "/teacher/imports", element: <p>history</p> },
      { path: "/teacher/tests/:id/edit", element: <p>builder</p> },
    ],
    { initialEntries: [`/teacher/imports/${IMPORT_ID}/confirm`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  await screen.findByRole("heading", { level: 1 });
  return { user: userEvent.setup({ advanceTimers: vi.advanceTimersByTime }), router };
}

function row(label: string): HTMLElement {
  const term = screen.getByText(label, { selector: "dt span" });
  return term.closest<HTMLElement>("div")!;
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  commits = [];
  reviewReads = 0;
  current = ready();
  value = wordImport();
  serve();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("the summary", () => {
  it("lists what the draft will hold and links each open value into the review", async () => {
    current = open();
    await renderConfirm();

    const included = row("Câu đưa vào đề");
    expect(within(included).getByText("3 trên 4")).toBeInTheDocument();
    expect(
      within(included).getByText("Câu 3 đã loại: Trùng câu 1"),
    ).toBeInTheDocument();

    const structure = row("Phần và nhóm câu");
    expect(within(structure).getByText("2 phần · 1 nhóm")).toBeInTheDocument();
    expect(
      within(structure).getByText("Câu 4 giữ đoạn văn với 2 chỗ trống"),
    ).toBeInTheDocument();

    const answers = row("Đáp án đã đặt");
    expect(within(answers).getByText("2 trên 3")).toHaveClass("text-danger-ink");
    expect(within(answers).getByRole("link", { name: "Sửa" })).toHaveAttribute(
      "href",
      `/teacher/imports/${IMPORT_ID}/review?filter=blocking`,
    );

    const points = row("Điểm");
    expect(within(points).getByText("Chưa xác nhận")).toHaveClass("text-danger-ink");
    expect(
      within(points).getByText("1 điểm mỗi câu theo mặc định"),
    ).toBeInTheDocument();
    expect(within(points).getByRole("link", { name: "Xác nhận" })).toHaveAttribute(
      "href",
      `/teacher/imports/${IMPORT_ID}/review?filter=info`,
    );

    const media = row("Âm thanh và hình ảnh");
    expect(within(media).getByText("Không có")).not.toHaveClass("text-danger-ink");
    expect(within(media).queryByRole("link")).toBeNull();

    const still = row("Còn cần xử lý");
    expect(within(still).getByText("1")).toHaveClass("text-danger-ink");
    expect(within(still).getByRole("link", { name: "Rà soát" })).toHaveAttribute(
      "href",
      `/teacher/imports/${IMPORT_ID}/review?filter=all`,
    );

    expect(screen.getByRole("button", { name: "Tạo bản nháp đề" })).toBeDisabled();
    expect(screen.getByText("Hãy xử lý thêm 1 mục trước.")).toBeInTheDocument();
  });

  it("names points the teacher set and offers the draft once nothing is open", async () => {
    await renderConfirm();
    const points = row("Điểm");
    expect(within(points).getByText("Tổng 1 điểm")).not.toHaveClass("text-danger-ink");
    expect(within(points).getByText("1 điểm mỗi câu, do bạn đặt")).toBeInTheDocument();
    expect(within(row("Còn cần xử lý")).queryByRole("link")).toBeNull();
    expect(screen.getByRole("button", { name: "Tạo bản nháp đề" })).toBeEnabled();
    expect(
      screen.getByText("Thao tác này tạo một bản nháp. Phát hành là một bước riêng."),
    ).toBeInTheDocument();
  });

  it("does not offer the draft for an import that is no longer under review", async () => {
    value = wordImport({ status: "cancelled", draftRevision: 3 });
    await renderConfirm();
    expect(screen.getByRole("button", { name: "Tạo bản nháp đề" })).toBeDisabled();
    expect(
      screen.getByText(/Lần nhập này không còn ở bước rà soát/),
    ).toBeInTheDocument();
  });
});

describe("the preview", () => {
  it("shows every included question with answers hidden, and switches to the phone frame", async () => {
    current = open();
    const { user } = await renderConfirm();
    expect(
      screen.getByText("3 câu hỏi. Học viên không thấy đáp án và ghi chú của bạn."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Question 3")).toBeNull();
    const frame = document.querySelector("[data-preview-viewport]");
    expect(frame).toHaveAttribute("data-preview-viewport", "desktop");

    await user.click(screen.getByRole("button", { name: "Điện thoại" }));
    expect(frame).toHaveAttribute("data-preview-viewport", "phone");
  });

  it("says how many it shows when the paper cannot draw a question", async () => {
    current = review(
      [
        section([
          question({ id: "q1", label: "1" }),
          question({ id: "q2", label: "2", type: "unsupported" }),
        ]),
      ],
      [],
    );
    await renderConfirm();
    expect(
      screen.getByText(
        "Đang hiện 1 trên 2 câu hỏi. Học viên không thấy đáp án và ghi chú của bạn.",
      ),
    ).toBeInTheDocument();
  });
});

describe("creating the draft", () => {
  it("creates it and links to the builder and back to the imports", async () => {
    onCommit(() => committed());
    const { user } = await renderConfirm();
    await user.click(screen.getByRole("button", { name: "Tạo bản nháp đề" }));

    const card = await screen.findByRole("status", { name: "" });
    expect(
      within(card).getByText("Đã tạo bản nháp: Đề thi học kỳ 1"),
    ).toBeInTheDocument();
    expect(
      within(card).getByText("1 câu hỏi · 1 điểm. Đề chưa được phát hành."),
    ).toBeInTheDocument();
    expect(
      within(card).getByRole("link", { name: "Mở trong trình soạn đề" }),
    ).toHaveAttribute("href", `/teacher/tests/${TEST_ID}/edit`);
    expect(within(card).getByRole("link", { name: "Về mục Nhập đề" })).toHaveAttribute(
      "href",
      "/teacher/imports",
    );
    expect(commits).toEqual([{ requestId: expect.any(String), draftRevision: 1 }]);
  });

  it("retries a lost response with the same request", async () => {
    onCommit((attempt) => (attempt === 1 ? HttpResponse.error() : committed()));
    const { user } = await renderConfirm();
    await user.click(screen.getByRole("button", { name: "Tạo bản nháp đề" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Chưa nhận được phản hồi từ máy chủ. Thử lại sẽ không tạo đề trùng.",
    );
    await user.click(screen.getByRole("button", { name: "Thử lại" }));

    expect(
      await screen.findByText("Đã tạo bản nháp: Đề thi học kỳ 1"),
    ).toBeInTheDocument();
    expect(commits).toHaveLength(2);
    expect(commits[1]!.requestId, "a lost response replays the same request").toBe(
      commits[0]!.requestId,
    );
  });

  it("ignores a second click while the commit is pending", async () => {
    const gate = deferred<void>();
    onCommit(async () => {
      await gate.promise;
      return committed();
    });
    const { user } = await renderConfirm();
    await user.click(screen.getByRole("button", { name: "Tạo bản nháp đề" }));
    await waitFor(() => expect(commits).toHaveLength(1));
    const pending = screen.getByRole("button", { name: "Đang tạo bản nháp…" });
    expect(pending).toBeDisabled();
    await user.click(pending);

    gate.resolve();
    expect(
      await screen.findByText("Đã tạo bản nháp: Đề thi học kỳ 1"),
    ).toBeInTheDocument();
    expect(commits).toHaveLength(1);
  });

  it("re-reads a review another tab changed, and commits the newer revision as a new request", async () => {
    onCommit((attempt) => {
      if (attempt > 1) return committed();
      current = ready({ revision: 2 });
      return contractJson(
        "/teacher/imports/{id}/commit",
        "post",
        409,
        errorBody("IMPORT_CONFLICT", "Bản rà soát đã thay đổi ở nơi khác."),
      );
    });
    const { user } = await renderConfirm();
    const reads = reviewReads;
    await user.click(screen.getByRole("button", { name: "Tạo bản nháp đề" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bản rà soát đã thay đổi ở nơi khác.",
    );
    await waitFor(() => expect(reviewReads).toBeGreaterThan(reads));
    const button = screen.getByRole("button", { name: "Tạo bản nháp đề" });
    await waitFor(() => expect(button).toBeEnabled());
    await user.click(button);

    expect(
      await screen.findByText("Đã tạo bản nháp: Đề thi học kỳ 1"),
    ).toBeInTheDocument();
    expect(commits.map((commit) => commit.draftRevision)).toEqual([1, 2]);
    expect(commits[1]!.requestId).not.toBe(commits[0]!.requestId);
  });

  it("shows the draft already created for a committed import", async () => {
    value = wordImport({ status: "committed", testId: TEST_ID });
    await renderConfirm();
    expect(screen.getByText("Đã tạo bản nháp: Đề thi học kỳ 1")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Mở trong trình soạn đề" }),
    ).toHaveAttribute("href", `/teacher/tests/${TEST_ID}/edit`);
    expect(screen.queryByRole("button", { name: "Tạo bản nháp đề" })).toBeNull();
  });
});

describe("an import the page cannot show", () => {
  it("answers another teacher's import as a missing one", async () => {
    server.use(
      http.get(`${BASE}/teacher/imports/:id`, () =>
        contractJson(
          "/teacher/imports/{id}",
          "get",
          404,
          errorBody("NOT_FOUND", "Không tìm thấy."),
        ),
      ),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const router = createMemoryRouter(
      [{ path: "/teacher/imports/:id/confirm", element: <ImportConfirmPage /> }],
      { initialEntries: [`/teacher/imports/${IMPORT_ID}/confirm`] },
    );
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("Không tìm thấy lần nhập này.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Về mục Nhập đề" })).toHaveAttribute(
      "href",
      "/teacher/imports",
    );
    expect(screen.queryByRole("button", { name: "Tạo bản nháp đề" })).toBeNull();
  });

  it("says the review was removed by retention", async () => {
    value = wordImport({ status: "cancelled", filesRemovedAt: "2026-10-26T00:00:00Z" });
    server.use(
      http.get(`${BASE}/teacher/imports/:id/review`, () =>
        contractJson(
          "/teacher/imports/{id}/review",
          "get",
          410,
          errorBody("IMPORT_FILES_REMOVED", "Bản rà soát đã được xoá."),
        ),
      ),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const router = createMemoryRouter(
      [{ path: "/teacher/imports/:id/confirm", element: <ImportConfirmPage /> }],
      { initialEntries: [`/teacher/imports/${IMPORT_ID}/confirm`] },
    );
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    expect(
      await screen.findByText(
        "Bản rà soát của lượt nhập này đã được xoá theo chính sách lưu trữ.",
      ),
    ).toBeInTheDocument();
  });
});
