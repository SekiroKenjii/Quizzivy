import { beforeEach, describe, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportDetailPage from "@/features/imports/pages/teacher/ImportDetailPage";
import { ProcessingPanel } from "@/features/imports/components/ProcessingPanel";
import type { ImportReview, ImportRun, WordImport } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  BASE,
  IMPORT_ID,
  capabilities,
  deferred,
  errorBody,
  finding,
  question,
  review,
  run,
  section,
  source,
  summary,
  wordImport,
} from "./fixtures";
import i18n from "@/lib/i18n";

const exam = source("exam", {
  format: "text",
  characters: 4873,
  filename: "stored-original.txt",
  bytes: 9112,
});
let current: WordImport;
let result: ImportReview;
let calls: unknown[];

beforeEach(() => {
  current = wordImport({
    status: "processing",
    sources: [exam, source("answer_key")],
    run: run({ stage: "source_validation" }),
  });
  result = review([section([question({ id: "q1", label: "1" })])], [], {
    summary: summary({ blocking: 0, answersMissing: 0, answersKnown: 2 }),
  });
  calls = [];
  server.use(
    capabilities(),
    http.get(`${BASE}/teacher/imports/:id`, () =>
      contractJson("/teacher/imports/{id}", "get", 200, current),
    ),
    http.get(`${BASE}/teacher/imports/:id/review`, () =>
      contractJson("/teacher/imports/{id}/review", "get", 200, result),
    ),
    http.post(`${BASE}/teacher/imports/:id/process`, async ({ request }) => {
      calls.push(await request.json());
      current = {
        ...current,
        status: "queued",
        revision: current.revision + 1,
        run: run({ status: "queued", stage: "queued" }),
      };
      return contractJson("/teacher/imports/{id}/process", "post", 202, current);
    }),
    http.post(`${BASE}/teacher/imports/:id/cancel`, async ({ request }) => {
      calls.push(await request.json());
      current = { ...current, status: "cancelled", revision: current.revision + 1 };
      return contractJson("/teacher/imports/{id}/cancel", "post", 200, current);
    }),
  );
});

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/imports/:id", element: <ImportDetailPage /> },
      { path: "/teacher/imports/new", element: <p>paste destination</p> },
    ],
    { initialEntries: [`/teacher/imports/${IMPORT_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), client, router };
}

describe("pasted text processing on the public detail page", () => {
  it("names the actual text exam in the header, tooltip and source download while keeping its title and companion", async () => {
    mount();
    const heading = await screen.findByRole("heading", {
      level: 1,
      name: "Văn bản đã dán",
    });
    expect(within(heading).getByText("Văn bản đã dán")).toHaveAttribute(
      "title",
      "Văn bản đã dán",
    );
    expect(screen.getByText("Đề thi học kỳ 1")).toBeInTheDocument();
    expect(screen.getByText(/từ văn bản đã dán/)).toHaveTextContent("dap-an-hk1.docx");
    const originals = screen.getByRole("region", { name: "Nguồn ban đầu" });
    expect(within(originals).getByText("Văn bản đã dán")).toBeInTheDocument();
    expect(
      within(originals).getByRole("button", { name: "Tải bản gốc Văn bản đã dán" }),
    ).toBeEnabled();
    expect(
      within(originals).getByRole("button", { name: "Tải bản gốc dap-an-hk1.docx" }),
    ).toBeEnabled();
    expect(screen.queryByText(exam.filename)).toBeNull();
    expect(
      heading.parentElement?.previousElementSibling?.querySelector(
        ".lucide-clipboard-paste",
      ),
    ).not.toBeNull();
  });

  it("starts an uploaded text exam with the existing revision and key-paper command without mounting upload fields", async () => {
    current = { ...current, status: "awaiting_sources", run: run({ keyPaper: 2 }) };
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: "Bắt đầu xử lý" }));
    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toEqual({
      requestId: expect.any(String),
      expectedRevision: 4,
      keyPaper: 2,
    });
    expect(screen.queryByLabelText("Tệp đề thi")).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  it("retains the idempotent start identity while pending and after a recoverable refusal", async () => {
    current = { ...current, status: "awaiting_sources" };
    const gate = deferred<void>();
    server.use(
      http.post(`${BASE}/teacher/imports/:id/process`, async ({ request }) => {
        calls.push(await request.json());
        await gate.promise;
        return new Response(null, { status: 503 });
      }),
    );
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: "Bắt đầu xử lý" }));
    expect(await screen.findByRole("button", { name: "Đang gửi…" })).toBeDisabled();
    await act(async () => gate.resolve());
    expect(await screen.findByRole("alert")).toHaveTextContent("HTTP 503");
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));
    await waitFor(() => expect(calls).toHaveLength(2));
    expect(calls[1]).toEqual(calls[0]);
  });

  it("withholds Start while processing is unavailable", async () => {
    current = { ...current, status: "awaiting_sources" };
    server.use(capabilities(false));
    mount();
    await screen.findByText("Chưa bắt đầu xử lý");
    expect(screen.queryByRole("button", { name: "Bắt đầu xử lý" })).toBeNull();
    expect(screen.queryByLabelText("Tệp đề thi")).toBeNull();
  });

  it.each([
    ["source_validation", "Kiểm tra văn bản"],
    ["extraction", "Đọc nội dung"],
    ["recognition", "Nhận diện cấu trúc · Đối chiếu đáp án"],
    ["queued", "Đang chờ đến lượt"],
  ] satisfies [ImportRun["stage"], string][])(
    "keeps the real failed %s stage and offers paste-mode restart without file replacement",
    async (stage, label) => {
      current = {
        ...current,
        status: "failed",
        run: run({ status: "failed", stage, errorCode: "SOURCE_INVALID" }),
      };
      mount();
      expect(await screen.findByText(`Dừng ở “${label}”`)).toBeInTheDocument();
      expect(screen.getByRole("link", { name: "Nhập đề mới" })).toHaveAttribute(
        "href",
        "/teacher/imports/new?source=paste",
      );
      expect(screen.queryByRole("link", { name: "Thay tệp đề thi" })).toBeNull();
      expect(screen.queryByLabelText("Tệp đề thi")).toBeNull();
      expect(screen.getByText("Mã lỗi: SOURCE_INVALID")).toBeInTheDocument();
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(
        "Không đọc được văn bản đã dán hoặc một tệp đi kèm.",
      );
      expect(alert).toHaveTextContent("Mã lỗi: SOURCE_INVALID");
    },
  );

  it("cancels processing at its exact revision with configured pasted-text retention and final paste restart", async () => {
    server.use(
      http.get(`${BASE}/teacher/imports/capabilities`, () =>
        contractJson("/teacher/imports/capabilities", "get", 200, {
          intakeEnabled: true,
          processingEnabled: true,
          retention: { afterCommitDays: 30, afterCancelDays: 11, idleDays: 64 },
        }),
      ),
    );
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: "Huỷ xử lý" }));
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/Văn bản đã dán và tệp đi kèm được giữ thêm 11 ngày/),
    ).toBeInTheDocument();
    await user.click(
      within(dialog).getByRole("button", { name: "Huỷ và đóng lần nhập" }),
    );
    expect(await screen.findByText("Lần nhập đã được đóng")).toBeInTheDocument();
    expect(calls).toEqual([{ expectedRevision: 4 }]);
    expect(
      screen.getByText(/Văn bản đã dán và tệp đi kèm được giữ 11 ngày kể từ khi đóng/),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Nhập đề mới" })).toHaveAttribute(
      "href",
      "/teacher/imports/new?source=paste",
    );
    expect(screen.queryByRole("button", { name: "Bắt đầu xử lý" })).toBeNull();
  });

  it.each([false, true])(
    "closes a waiting text import with draft=%s using configured whole retention copy",
    async (draft) => {
      current = {
        ...current,
        status: draft ? "needs_review" : "awaiting_sources",
        draftRevision: draft ? 3 : 0,
      };
      const { user } = mount();
      await user.click(await screen.findByRole("button", { name: "Huỷ lần nhập" }));
      const dialog = await screen.findByRole("dialog");
      expect(
        within(dialog).getByText(
          draft
            ? /Phần rà soát, văn bản đã dán và tệp đi kèm vẫn xem được thêm 7 ngày/
            : /Văn bản đã dán và tệp đi kèm vẫn xem được thêm 7 ngày/,
        ),
      ).toBeInTheDocument();
      await user.click(within(dialog).getByRole("button", { name: "Huỷ lần nhập" }));
      expect(await screen.findByText("Lần nhập đã được đóng")).toBeInTheDocument();
      expect(calls).toEqual([{ expectedRevision: 4 }]);
    },
  );

  it("keeps plain retention and idle-closure choices without inventing days", async () => {
    server.use(
      http.get(`${BASE}/teacher/imports/capabilities`, () =>
        contractJson("/teacher/imports/capabilities", "get", 200, {
          intakeEnabled: true,
          processingEnabled: true,
        }),
      ),
    );
    current = { ...current, status: "cancelled" };
    const { client } = mount();
    expect(
      await screen.findByText(
        "Văn bản đã dán và tệp đi kèm vẫn được giữ theo chính sách lưu trữ.",
      ),
    ).toBeInTheDocument();
    current = { ...current, closedIdle: true };
    await act(async () =>
      client.invalidateQueries({ queryKey: ["word-import", IMPORT_ID] }),
    );
    expect(
      await screen.findByText(
        "Lượt nhập văn bản đã dán đã tự đóng vì lâu không có thay đổi.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/7 ngày/)).toBeNull();
  });

  it("keeps review retention and removal truthful for a closed text import", async () => {
    current = { ...current, status: "cancelled", draftRevision: 3 };
    const { client } = mount();
    expect(
      await screen.findByText(
        /Phần rà soát, văn bản đã dán và tệp đi kèm vẫn xem được 7 ngày/,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Xem bản rà soát hiện tại" }),
    ).toBeInTheDocument();
    current = { ...current, filesRemovedAt: "2026-10-02T03:00:00Z", closedIdle: true };
    await act(async () =>
      client.invalidateQueries({ queryKey: ["word-import", IMPORT_ID] }),
    );
    await waitFor(() =>
      expect(
        screen.queryByRole("link", { name: "Xem bản rà soát hiện tại" }),
      ).toBeNull(),
    );
    expect(screen.queryByText(/vẫn xem được 7 ngày/)).toBeNull();
    expect(screen.queryByRole("button", { name: /Tải bản gốc/ })).toBeNull();
  });

  it("downloads by the actual source id and handles retained-source 410 without losing metadata", async () => {
    let requested = "";
    server.use(
      http.get(
        `${BASE}/teacher/imports/:id/sources/:sourceId/download`,
        ({ params }) => {
          requested = String(params.sourceId);
          return contractJson(
            "/teacher/imports/{id}/sources/{sourceId}/download",
            "get",
            410,
            errorBody("IMPORT_FILES_REMOVED", "Bản gốc đã được xoá."),
          );
        },
      ),
    );
    const { user } = mount();
    await user.click(
      await screen.findByRole("button", { name: "Tải bản gốc Văn bản đã dán" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent("Bản gốc đã được xoá.");
    expect(requested).toBe(exam.id);
    expect(screen.getByRole("heading", { name: "Văn bản đã dán" })).toBeInTheDocument();
  });

  it.each([
    [0, 0, true],
    [1, 0, false],
    [0, 1, false],
  ])(
    "keeps Ready protection for blocking=%s/conflicts=%s",
    async (blocking, answersConflicting, complete) => {
      current = { ...current, status: "needs_review", draftRevision: 2 };
      result = {
        ...result,
        findings: [
          finding({
            id: "note-a",
            code: "DEFAULT_POINTS",
            severity: "informational",
            count: 8,
          }),
          finding({
            id: "note-b",
            code: "DEFAULT_POINTS",
            severity: "informational",
            count: 3,
          }),
        ],
        summary: { ...result.summary, blocking, answersConflicting },
      };
      mount();
      await screen.findByRole("link", { name: "Bắt đầu rà soát" });
      await screen.findByText("2 ghi chú");
      expect(screen.queryByRole("alert")).toBeNull();
      expect(screen.queryByText("Mỗi câu đều có đáp án") !== null).toBe(complete);
      expect(screen.queryByText("0 cần xử lý")).toBeNull();
      expect(screen.queryByText("11 ghi chú")).toBeNull();
    },
  );

  it("keeps a stale text summary and retries its real review request", async () => {
    current = { ...current, status: "needs_review", draftRevision: 2 };
    let failing = false;
    let reads = 0;
    server.use(
      http.get(`${BASE}/teacher/imports/:id/review`, () => {
        reads += 1;
        return failing
          ? new Response(null, { status: 503 })
          : contractJson("/teacher/imports/{id}/review", "get", 200, result);
      }),
    );
    const { client, user } = mount();
    await screen.findByText("Mỗi câu đều có đáp án");
    failing = true;
    await act(async () =>
      client.invalidateQueries({ queryKey: ["word-import-ready-summary", IMPORT_ID] }),
    );
    expect(screen.getByText("Mỗi câu đều có đáp án")).toBeInTheDocument();
    const retry = await screen.findByRole("button", { name: "Thử lại" });
    failing = false;
    await user.click(retry);
    await waitFor(() => expect(reads).toBe(3));
  });
});

describe("truthful pasted-text stage receipts", () => {
  it("uses the English text receipts without claiming preserved formatting", async () => {
    await i18n.changeLanguage("en");
    try {
      render(<ProcessingPanel run={run({ stage: "recognition" })} exam={exam} />);
      expect(screen.getByText("Check the text")).toBeInTheDocument();
      expect(screen.getByText("4873 characters checked")).toBeInTheDocument();
      expect(screen.getByText("Plain text · no formatting")).toBeInTheDocument();
      expect(screen.queryByText(/Formatting kept/)).toBeNull();
    } finally {
      await i18n.changeLanguage("vi");
    }
  });

  it("shows checks only after their real stage completes, preserves both recognition rows and has no fabricated zero", () => {
    const { rerender } = render(
      <ProcessingPanel run={run({ stage: "queued" })} exam={exam} />,
    );
    for (const stage of ["queued", "source_validation"] as const) {
      rerender(<ProcessingPanel run={run({ stage })} exam={exam} />);
      expect(screen.getByText("Kiểm tra văn bản")).toBeInTheDocument();
      expect(screen.queryByText("Đã kiểm tra 4873 ký tự")).toBeNull();
    }
    rerender(<ProcessingPanel run={run({ stage: "extraction" })} exam={exam} />);
    expect(screen.getByText("Đã kiểm tra 4873 ký tự")).toBeInTheDocument();
    expect(screen.queryByText("Văn bản thuần · không có định dạng")).toBeNull();
    rerender(<ProcessingPanel run={run({ stage: "recognition" })} exam={exam} />);
    expect(screen.getByText("Văn bản thuần · không có định dạng")).toBeInTheDocument();
    const rows = within(screen.getByRole("list")).getAllByRole("listitem");
    expect(
      rows
        .filter((row) => row.getAttribute("aria-current") === "step")
        .map((row) => row.textContent),
    ).toEqual([
      expect.stringContaining("Nhận diện cấu trúc"),
      expect.stringContaining("Đối chiếu đáp án"),
    ]);
    rerender(<ProcessingPanel run={run({ stage: "ready" })} exam={undefined} />);
    expect(screen.queryByText(/Đã kiểm tra/)).toBeNull();
    expect(screen.queryByText(/0 ký tự/)).toBeNull();
  });

  it("retains waiting retry semantics and omitted-metadata file behavior", () => {
    const { rerender } = render(
      <ProcessingPanel
        run={run({
          stage: "queued",
          status: "queued",
          errorCode: "PROCESSING_TIMEOUT",
        })}
        exam={exam}
      />,
    );
    expect(
      screen.getByText("Đang chờ thử lại tự động. Bạn không cần làm gì."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Đã kiểm tra/)).toBeNull();
    rerender(<ProcessingPanel run={run({ stage: "recognition" })} />);
    expect(screen.getByText("Kiểm tra tệp")).toBeInTheDocument();
    expect(screen.queryByText("Văn bản thuần · không có định dạng")).toBeNull();
  });
});

describe("source-aware text failure recovery", () => {
  it.each([
    ["SOURCE_INVALID", false],
    ["SOURCE_INVALID", true],
    ["STORAGE_INTEGRITY_FAILED", false],
    ["STORAGE_INTEGRITY_FAILED", true],
  ] as const)("keeps %s neutral with companion=%s", async (errorCode, companion) => {
    current = {
      ...current,
      status: "failed",
      sources: companion ? [exam, source("answer_key")] : [exam],
      run: run({ status: "failed", stage: "source_validation", errorCode }),
    };
    mount();
    const copy =
      errorCode === "SOURCE_INVALID"
        ? "Không đọc được văn bản đã dán hoặc một tệp đi kèm. Hãy kiểm tra các nguồn rồi bắt đầu một lần nhập mới."
        : "Không đọc được bản gốc đã lưu của văn bản đã dán hoặc một tệp đi kèm. Hãy bắt đầu một lần nhập mới từ các nguồn gốc.";
    expect(await screen.findByText(copy)).toBeInTheDocument();
    expect(screen.queryByText(/lưu thành \.docx|Hãy tải tệp lên lại/)).toBeNull();
    expect(screen.getByText(`Mã lỗi: ${errorCode}`)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Nhập đề mới" })).toHaveAttribute(
      "href",
      "/teacher/imports/new?source=paste",
    );
    expect(screen.queryByLabelText("Tệp đề thi")).toBeNull();
  });

  it.each(["SOURCE_TOO_LARGE", "STOPPED", "TEMPORARY", "INTERNAL", "UNKNOWN"])(
    "keeps %s source-neutral instead of assuming a file",
    async (errorCode) => {
      current = {
        ...current,
        status: "failed",
        run: run({ status: "failed", stage: "extraction", errorCode }),
      };
      mount();
      const heading = await screen.findByRole("heading", {
        level: 1,
        name: "Văn bản đã dán",
      });
      expect(heading).toBeInTheDocument();
      expect(
        await screen.findByText(
          /Văn bản đã dán và tệp đi kèm|Văn bản đã dán hoặc một tệp đi kèm/,
        ),
      ).toBeInTheDocument();
      expect(
        screen.queryByText(
          /Hãy tách đề hoặc giảm kích thước hình ảnh rồi tải lên lại|cùng tệp|Tệp của bạn vẫn được giữ/,
        ),
      ).toBeNull();
    },
  );

  it.each(["SOURCE_INVALID", "STORAGE_INTEGRITY_FAILED"])(
    "keeps English %s recovery source-neutral",
    async (errorCode) => {
      await i18n.changeLanguage("en");
      try {
        current = {
          ...current,
          status: "failed",
          run: run({ status: "failed", stage: "source_validation", errorCode }),
        };
        mount();
        const copy =
          errorCode === "SOURCE_INVALID"
            ? "The pasted text or a companion file could not be read. Check the sources and start a new import."
            : "The stored original of the pasted text or a companion file can no longer be read. Start a new import from the original sources.";
        expect(await screen.findByText(copy)).toBeInTheDocument();
        expect(
          screen.queryByText(/save it as \.docx|Upload the file again/),
        ).toBeNull();
      } finally {
        await i18n.changeLanguage("vi");
      }
    },
  );

  it.each(["SOURCE_INVALID", "STORAGE_INTEGRITY_FAILED", "PDF_PROTECTED"])(
    "preserves existing file-specific %s recovery",
    async (errorCode) => {
      current = {
        ...current,
        status: "failed",
        sources: [source("exam")],
        run: run({ status: "failed", stage: "source_validation", errorCode }),
      };
      mount();
      expect(
        await screen.findByText(i18n.t(`imports.runError.${errorCode}`)),
      ).toBeInTheDocument();
      expect(screen.getByRole("link", { name: "Thay tệp đề thi" })).toBeInTheDocument();
    },
  );
});

describe("pasted text failure headings", () => {
  const titles = {
    vi: {
      SOURCE_INVALID: "Không đọc được một nguồn nhập đề",
      SOURCE_TOO_LARGE: "Nguồn nhập đề vượt quá giới hạn",
    },
    en: {
      SOURCE_INVALID: "An import source could not be read",
      SOURCE_TOO_LARGE: "An import source exceeds the limits",
    },
  };
  for (const language of ["vi", "en"] as const) {
    for (const errorCode of ["SOURCE_INVALID", "SOURCE_TOO_LARGE"] as const) {
      it.each([false, true])(
        `names the source neutrally for ${errorCode} in ${language} with companion=%s`,
        async (companion) => {
          await i18n.changeLanguage(language);
          try {
            current = {
              ...current,
              status: "failed",
              sources: companion ? [exam, source("answer_key")] : [exam],
              run: run({ status: "failed", stage: "source_validation", errorCode }),
            };
            mount();
            const title = titles[language][errorCode];
            expect(await screen.findByText(title)).toBeInTheDocument();
            expect(
              screen.queryByText(i18n.t(`imports.failureTitle.${errorCode}`)),
            ).toBeNull();
            expect(
              screen.getByText(i18n.t("imports.detail.errorCode", { code: errorCode })),
            ).toBeInTheDocument();
          } finally {
            await i18n.changeLanguage("vi");
          }
        },
      );
      it(`preserves the file heading for ${errorCode} in ${language}`, async () => {
        await i18n.changeLanguage(language);
        try {
          current = {
            ...current,
            status: "failed",
            sources: [source("exam")],
            run: run({ status: "failed", stage: "source_validation", errorCode }),
          };
          mount();
          expect(
            await screen.findByText(i18n.t(`imports.failureTitle.${errorCode}`)),
          ).toBeInTheDocument();
        } finally {
          await i18n.changeLanguage("vi");
        }
      });
    }
  }
});
