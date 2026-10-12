import { beforeEach, describe, expect, it } from "vitest";
import { act, render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportDetailPage from "@/features/imports/pages/teacher/ImportDetailPage";
import { formatDateTime } from "@/lib/i18n/datetime";
import { ProcessingPanel } from "@/features/imports/components/ProcessingPanel";
import type { ImportReview, ImportRun, WordImport } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  BASE,
  IMPORT_ID,
  capabilities,
  finding,
  question,
  review,
  run,
  section,
  summary,
  source,
  wordImport,
} from "./fixtures";
import "@/lib/i18n";

let current: WordImport;
let result: ImportReview;
let reviewFails: boolean;
let reviewReads: number;
let cancelBodies: unknown[];

beforeEach(() => {
  current = wordImport({ status: "needs_review", draftRevision: 1 });
  result = review(
    [section([question({ id: "q1", label: "1" }), question({ id: "q2", label: "2" })])],
    [],
  );
  reviewFails = false;
  reviewReads = 0;
  cancelBodies = [];
  server.use(
    capabilities(),
    http.get(`${BASE}/teacher/imports/limits`, () =>
      contractJson("/teacher/imports/limits", "get", 200, {
        maxBytes: 25 * 1024 * 1024,
        formats: ["docx", "pdf"],
      }),
    ),
    http.get(`${BASE}/teacher/imports/:id`, () =>
      contractJson("/teacher/imports/{id}", "get", 200, current),
    ),
    http.get(`${BASE}/teacher/imports/:id/review`, () => {
      reviewReads += 1;
      if (reviewFails) return new Response(null, { status: 503 });
      return contractJson("/teacher/imports/{id}/review", "get", 200, result);
    }),
    http.post(`${BASE}/teacher/imports/:id/cancel`, async ({ request }) => {
      cancelBodies.push(await request.json());
      current = wordImport({
        ...current,
        status: "cancelled",
        revision: current.revision + 1,
      });
      return contractJson("/teacher/imports/{id}/cancel", "post", 200, current);
    }),
  );
});

function renderDetail() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/imports/:id", element: <ImportDetailPage /> },
      { path: "/teacher/imports/:id/review", element: <p>review destination</p> },
      { path: "/teacher/imports", element: <p>history destination</p> },
    ],
    { initialEntries: [`/teacher/imports/${IMPORT_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { ...userEvent.setup(), router };
}

describe("deck processing stages on the public run model", () => {
  it.each([
    ["queued", []],
    ["source_validation", ["Kiểm tra tệp"]],
    ["normalization", ["Đọc nội dung"]],
    ["extraction", ["Đọc nội dung"]],
    ["recognition", ["Nhận diện cấu trúc", "Đối chiếu đáp án"]],
    ["validation", ["Kiểm tra đề"]],
    ["ready", []],
  ] satisfies [ImportRun["stage"], string[]][])(
    "maps %s without making up another pipeline phase",
    (stage, labels) => {
      render(<ProcessingPanel run={run({ stage })} />);
      const rows = within(
        screen.getByRole("list", { name: "Các bước xử lý" }),
      ).getAllByRole("listitem");
      expect(rows).toHaveLength(5);
      expect(
        rows
          .filter((row) => row.getAttribute("aria-current") === "step")
          .map((row) => row.textContent?.replace("đang thực hiện", "")),
      ).toEqual(labels);
      expect(screen.getByText("Lần thử 1/3")).toBeInTheDocument();
      expect(screen.queryByText(/%/)).toBeNull();
    },
  );
});

describe("processing result data and real actions", () => {
  it("reads the real Ready summary and informational findings, including affected-target counts", async () => {
    result = review(
      [
        section([
          question({ id: "q1", label: "1" }),
          question({ id: "q2", label: "2" }),
        ]),
      ],
      [
        finding({
          id: "note1",
          code: "UNSUPPORTED_DOCUMENT_OBJECT",
          field: "PDF_MARKS_UNAVAILABLE",
          severity: "informational",
          count: 4,
        }),
        finding({
          id: "note2",
          code: "UNSUPPORTED_DOCUMENT_OBJECT",
          field: "SEMANTIC_FORMATTING_LOSS",
          severity: "informational",
          count: 3,
        }),
      ],
      { summary: summary({ blocking: 1, needsDecision: 2 }) },
    );
    renderDetail();
    expect(
      await screen.findByText("Đã tìm thấy 2 câu hỏi với 1 đáp án trong 1 phần"),
    ).toBeInTheDocument();
    expect(screen.getByText("1 cần xử lý")).toBeInTheDocument();
    expect(screen.getByText("2 cần xác nhận")).toBeInTheDocument();
    expect(screen.getByText("2 ghi chú")).toBeInTheDocument();
    expect(screen.queryByText("7 ghi chú")).toBeNull();
    expect(screen.queryByText("Mỗi câu đều có đáp án")).toBeNull();
    expect(reviewReads).toBe(1);
    expect(screen.getByRole("link", { name: "Bắt đầu rà soát" })).toHaveAttribute(
      "href",
      `/teacher/imports/${IMPORT_ID}/review`,
    );
    expect(screen.getByRole("link", { name: "Để sau" })).toHaveAttribute(
      "href",
      "/teacher/imports",
    );
  });

  it("shows actual zero counts and omits zero decision/note chips", async () => {
    result = review(
      [],
      [finding({ id: "empty", code: "NO_QUESTIONS", severity: "blocking" })],
      {
        summary: summary({
          sections: 0,
          questions: 0,
          included: 0,
          answersKnown: 0,
          answersMissing: 0,
          blocking: 1,
          needsDecision: 0,
        }),
      },
    );
    renderDetail();
    expect(
      await screen.findByText("Đã tìm thấy 0 câu hỏi với 0 đáp án trong 0 phần"),
    ).toBeInTheDocument();
    expect(screen.getByText("1 cần xử lý")).toBeInTheDocument();
    expect(screen.queryByText(/cần xác nhận$|ghi chú$/)).toBeNull();
    expect(screen.queryByText("Mỗi câu đều có đáp án")).toBeNull();
  });

  it("shows the success pill only when no blocking or missing answer remains", async () => {
    result = review(
      [
        section([
          question({ id: "q1", label: "1" }),
          question({ id: "q2", label: "2" }),
        ]),
      ],
      [],
      { summary: summary({ answersKnown: 2, answersMissing: 0, blocking: 0 }) },
    );
    renderDetail();
    expect(await screen.findByText("Mỗi câu đều có đáp án")).toBeInTheDocument();
    expect(screen.queryByText(/cần xử lý$/)).toBeNull();
  });

  it("does not turn a failed summary read into invented zeros, and recovers on an explicit retry", async () => {
    reviewFails = true;
    const user = renderDetail();
    expect(
      await screen.findByText(
        "Không tải được tóm tắt. Bạn vẫn có thể mở bản rà soát hoặc thử lại.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Đã tìm thấy/)).toBeNull();
    expect(screen.getByRole("link", { name: "Bắt đầu rà soát" })).toBeInTheDocument();
    reviewFails = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(
      await screen.findByText("Đã tìm thấy 2 câu hỏi với 1 đáp án trong 1 phần"),
    ).toBeInTheDocument();
    expect(reviewReads).toBe(2);
  });

  it("marks cached Ready data stale after a failed return read and keeps an explicit reload", async () => {
    const user = renderDetail();
    await screen.findByText("Đã tìm thấy 2 câu hỏi với 1 đáp án trong 1 phần");
    await act(async () => {
      await user.router.navigate("/teacher/imports");
    });
    reviewFails = true;
    await act(async () => {
      await user.router.navigate(`/teacher/imports/${IMPORT_ID}`);
    });
    await screen.findByRole("button", { name: "Thử lại" });
    expect(
      screen.getByText("Đã tìm thấy 2 câu hỏi với 1 đáp án trong 1 phần"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Đã tìm thấy 0/)).toBeNull();
    expect(
      screen.getByText(
        "Không cập nhật được thông tin mới nhất; đang hiển thị dữ liệu trước đó.",
      ),
    ).toBeInTheDocument();
    reviewFails = false;
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    await waitFor(() =>
      expect(
        screen.queryByText(
          "Không cập nhật được thông tin mới nhất; đang hiển thị dữ liệu trước đó.",
        ),
      ).toBeNull(),
    );
    expect(reviewReads).toBe(3);
    expect(
      screen.getByText("Đã tìm thấy 2 câu hỏi với 1 đáp án trong 1 phần"),
    ).toBeInTheDocument();
  });

  it("uses real file/run/key metadata and the configured cancellation retention", async () => {
    current = wordImport({
      status: "processing",
      run: run({ stage: "validation", createdAt: "2026-10-06T01:00:00Z" }),
    });
    server.use(
      http.get(`${BASE}/teacher/imports/capabilities`, () =>
        contractJson("/teacher/imports/capabilities", "get", 200, {
          intakeEnabled: true,
          processingEnabled: true,
          retention: { afterCommitDays: 42, afterCancelDays: 9, idleDays: 88 },
        }),
      ),
    );
    const user = renderDetail();
    expect(
      await screen.findByRole("heading", { level: 1, name: "de-thi-hk1.docx" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Bắt đầu.*kèm dap-an-hk1.docx/)).toHaveAttribute(
      "title",
      formatDateTime(current.run!.createdAt, "vi"),
    );
    expect(screen.getByText(/Bạn có thể rời trang này/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Huỷ xử lý" }));
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/Tệp gốc được giữ thêm 9 ngày/),
    ).toBeInTheDocument();
    await user.click(
      within(dialog).getByRole("button", { name: "Huỷ và đóng lần nhập" }),
    );
    expect(await screen.findByText("Lần nhập đã được đóng")).toBeInTheDocument();
    expect(cancelBodies).toEqual([{ expectedRevision: 4 }]);
    expect(screen.queryByRole("button", { name: /Xử lý lại|Huỷ xử lý/ })).toBeNull();
    expect(reviewReads).toBe(0);
  });

  it("names a real PDF error and stopped stage while preserving its original error code", async () => {
    current = wordImport({
      status: "failed",
      sources: [
        source("exam", { filename: "scanned.pdf", format: "pdf" }),
        source("answer_key"),
      ],
      run: run({
        status: "failed",
        stage: "source_validation",
        errorCode: "PDF_NO_TEXT",
      }),
    });
    renderDetail();
    expect(
      await screen.findByText("PDF không có lớp văn bản để đọc"),
    ).toBeInTheDocument();
    expect(screen.getByText("Dừng ở “Kiểm tra tệp”")).toBeInTheDocument();
    expect(screen.getByText(/PDF_NO_TEXT/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Thay tệp đề thi" })).toHaveAttribute(
      "href",
      expect.stringMatching(/^#/),
    );
    expect(screen.queryByRole("button", { name: "Thử xử lý lại" })).toBeNull();
    expect(reviewReads).toBe(0);
  });
});
