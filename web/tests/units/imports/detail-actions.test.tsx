import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportDetailPage from "@/features/imports/pages/ImportDetailPage";
import type { WordImport } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  BASE,
  IMPORT_ID,
  question,
  review,
  run,
  section,
  errorBody,
  source,
  wordImport,
} from "./fixtures";
import "@/lib/i18n";

let current: WordImport;
let calls: { path: string; body: unknown }[] = [];

beforeEach(() => {
  calls = [];
  current = wordImport();
  server.use(
    http.get(`${BASE}/admin/imports/limits`, () =>
      contractJson("/admin/imports/limits", "get", 200, {
        maxBytes: 25 * 1024 * 1024,
        formats: ["docx"],
      }),
    ),
    http.get(`${BASE}/admin/imports/:id`, () =>
      contractJson("/admin/imports/{id}", "get", 200, current),
    ),
    http.post(`${BASE}/admin/imports/:id/cancel`, async ({ request }) => {
      calls.push({ path: "cancel", body: await request.json() });
      current = wordImport({
        ...current,
        status: "cancelled",
        revision: current.revision + 1,
      });
      return contractJson("/admin/imports/{id}/cancel", "post", 200, current);
    }),
    http.post(`${BASE}/admin/imports/:id/process`, async ({ request }) => {
      calls.push({ path: "process", body: await request.json() });
      current = wordImport({
        ...current,
        status: "queued",
        revision: current.revision + 1,
      });
      return contractJson("/admin/imports/{id}/process", "post", 202, current);
    }),
    http.post(`${BASE}/admin/imports/:id/sources`, ({ request }) => {
      const url = new URL(request.url);
      calls.push({ path: "sources", body: Object.fromEntries(url.searchParams) });
      current = wordImport({
        ...current,
        status: "awaiting_sources",
        revision: current.revision + 1,
      });
      return contractJson("/admin/imports/{id}/sources", "post", 201, {
        import: current,
        source: source("exam", { filename: "de-thi-sach.docx" }),
        sourceRevision: 3,
      });
    }),
  );
});

function renderDetail(
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } }),
) {
  const router = createMemoryRouter(
    [
      { path: "/admin/imports/:id", element: <ImportDetailPage /> },
      { path: "/admin/imports/:id/review", element: <p>review page</p> },
      { path: "/admin/imports/new", element: <p>new import</p> },
      { path: "/admin/imports", element: <p>history</p> },
    ],
    { initialEntries: [`/admin/imports/${IMPORT_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("closing an import that is not running", () => {
  it.each([
    ["awaiting_sources", { status: "awaiting_sources" as const, sources: [] }],
    [
      "failed",
      {
        status: "failed" as const,
        run: run({ status: "failed", errorCode: "SOURCE_INVALID" }),
      },
    ],
    ["needs_review", { status: "needs_review" as const, draftRevision: 2 }],
  ])("offers Huỷ lần nhập while %s", async (_status, over) => {
    current = wordImport(over);
    const user = renderDetail();
    await user.click(await screen.findByRole("button", { name: "Huỷ lần nhập" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/sẽ được đóng/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Huỷ lần nhập" }));

    expect(await screen.findByText("Lần nhập đã được đóng")).toBeInTheDocument();
    expect(calls).toEqual([{ path: "cancel", body: { expectedRevision: 4 } }]);
  });

  it("says a closed import with a draft keeps its review viewable", async () => {
    current = wordImport({ status: "cancelled", draftRevision: 3 });
    renderDetail();
    expect(await screen.findByText("Lần nhập đã được đóng")).toBeInTheDocument();
    expect(
      screen.getByText("Phần rà soát và tệp gốc vẫn xem được theo chính sách lưu trữ."),
    ).toBeInTheDocument();
  });

  it("warns that a closed review can no longer become a draft test", async () => {
    current = wordImport({ status: "needs_review", draftRevision: 2 });
    const user = renderDetail();
    await user.click(await screen.findByRole("button", { name: "Huỷ lần nhập" }));
    expect(
      within(await screen.findByRole("dialog")).getByText(
        /không thể tạo bản nháp đề từ phần rà soát này nữa/,
      ),
    ).toBeInTheDocument();
  });
});

describe("an import that failed", () => {
  it("retries with the answer-key paper the failed run used", async () => {
    current = wordImport({
      status: "failed",
      run: run({ status: "failed", errorCode: "PROCESSING_TIMEOUT", keyPaper: 2 }),
    });
    const user = renderDetail();
    expect(await screen.findByText(/sự cố tạm thời/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Thử xử lý lại" }));

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toMatchObject({
      path: "process",
      body: { expectedRevision: 4, keyPaper: 2 },
    });
  });

  it("offers no retry when the file itself must change, and replaces it on the same import", async () => {
    current = wordImport({
      status: "failed",
      run: run({ status: "failed", errorCode: "STORAGE_INTEGRITY_FAILED" }),
    });
    const user = renderDetail();
    expect(
      await screen.findByText(/Bản gốc đã lưu không còn đọc được/),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Thử xử lý lại" })).toBeNull();
    const start = screen.getByRole("button", { name: "Tải lên và xử lý lại" });
    expect(start, "nothing to send until a file is chosen").toBeDisabled();

    await user.upload(
      screen.getByLabelText("Tệp đề thi"),
      new File([new Uint8Array(2048)], "de-thi-sach.docx"),
    );
    await user.click(start);

    await waitFor(() =>
      expect(calls.map((call) => call.path)).toEqual(["sources", "process"]),
    );
    expect(calls[0]!.body).toMatchObject({ role: "exam", expectedRevision: "4" });
    expect(calls[1]!.body).toMatchObject({ expectedRevision: 5 });
  });
});

describe("an import back under review", () => {
  const FAILED_REPROCESS =
    "Lần xử lý lại gần nhất không hoàn tất. Bản rà soát hiện tại không thay đổi.";

  it("says a failed reprocess left the review unchanged and retries it with the same paper", async () => {
    current = wordImport({
      status: "needs_review",
      draftRevision: 3,
      run: run({ status: "failed", errorCode: "PROCESSING_TIMEOUT", keyPaper: 2 }),
    });
    const user = renderDetail();
    expect(
      await screen.findByText(FAILED_REPROCESS, { selector: "p:not([aria-live])" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/sự cố tạm thời/)).toBeNull();
    expect(screen.getByText("Mã lỗi: PROCESSING_TIMEOUT")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Thử xử lý lại lần nữa" }));

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toMatchObject({
      path: "process",
      body: { expectedRevision: 4, keyPaper: 2 },
    });
    expect(await screen.findByText("Đang xử lý lại")).toBeInTheDocument();
  });

  it("offers no retry after a reprocess that needs another file", async () => {
    current = wordImport({
      status: "needs_review",
      draftRevision: 3,
      run: run({ status: "failed", errorCode: "STORAGE_INTEGRITY_FAILED" }),
    });
    renderDetail();
    expect(
      await screen.findByText(FAILED_REPROCESS, { selector: "p:not([aria-live])" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Thử xử lý lại lần nữa" })).toBeNull();
    expect(screen.queryByText(/tải tệp lên lại/i)).toBeNull();
  });

  it.each([
    ["failed", FAILED_REPROCESS],
    [
      "cancelled",
      "Lần xử lý lại gần nhất đã được dừng. Bản rà soát hiện tại không thay đổi.",
    ],
  ] as const)(
    "announces a %s reprocess rather than a plain ready status",
    async (status, sentence) => {
      current = wordImport({
        status: "needs_review",
        draftRevision: 3,
        run: run(
          status === "failed"
            ? { status, errorCode: "PROCESSING_TIMEOUT" }
            : { status },
        ),
      });
      renderDetail();
      const live = await screen.findByText(sentence, {
        selector: "[aria-live='polite']",
      });
      expect(live).not.toHaveTextContent("Sẵn sàng rà soát");
    },
  );

  it("announces a plain ready status after a first run", async () => {
    current = wordImport({ status: "needs_review", draftRevision: 1 });
    renderDetail();
    expect(
      await screen.findByText("Sẵn sàng rà soát", { selector: "[aria-live='polite']" }),
    ).toBeInTheDocument();
  });

  it("drops a cached review once the import is ready again", async () => {
    current = wordImport({ status: "needs_review", draftRevision: 3 });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(
      ["word-import-review", IMPORT_ID],
      review([section([question({ id: "q1", label: "1" })])], []),
    );
    renderDetail(client);
    await screen.findByRole("link", { name: "Tiếp tục rà soát" });
    await waitFor(() =>
      expect(client.getQueryData(["word-import-review", IMPORT_ID])).toBeUndefined(),
    );
  });

  it("links a cancelled import that has a draft to its read-only review", async () => {
    current = wordImport({ status: "cancelled", draftRevision: 3 });
    renderDetail();
    expect(
      await screen.findByRole("link", { name: "Xem bản rà soát hiện tại" }),
    ).toHaveAttribute("href", `/admin/imports/${IMPORT_ID}/review`);
  });
});

describe("sending files from the detail page", () => {
  const unavailable = () => new Response(null, { status: 503 });

  it("keeps the intake when a replacement upload moves a failed import back to awaiting files", async () => {
    let refuse = true;
    server.use(
      http.post(`${BASE}/admin/imports/:id/process`, async ({ request }) => {
        calls.push({ path: "process", body: await request.json() });
        if (refuse) {
          refuse = false;
          return unavailable();
        }
        current = wordImport({
          ...current,
          status: "queued",
          revision: current.revision + 1,
        });
        return contractJson("/admin/imports/{id}/process", "post", 202, current);
      }),
    );
    current = wordImport({
      status: "failed",
      run: run({ status: "failed", errorCode: "STORAGE_INTEGRITY_FAILED" }),
    });
    const user = renderDetail();
    await user.upload(
      await screen.findByLabelText("Tệp đề thi"),
      new File([new Uint8Array(2048)], "de-thi-sach.docx"),
    );
    await user.click(screen.getByRole("button", { name: "Tải lên và xử lý lại" }));

    expect(await screen.findByText("Chưa bắt đầu xử lý")).toBeInTheDocument();
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    const start = screen.getByRole("button", { name: "Bắt đầu xử lý" });
    await waitFor(() => expect(start).toBeEnabled());
    await user.click(start);

    await waitFor(() =>
      expect(calls.map((call) => call.path)).toEqual(["sources", "process", "process"]),
    );
    expect(calls[1]!.body).toEqual(calls[2]!.body);
    expect(calls[2]!.body).toMatchObject({ expectedRevision: 5 });
    expect(await screen.findByText("Đang xử lý")).toBeInTheDocument();
  });

  it("closes an import at the revision its last upload produced", async () => {
    server.use(
      http.post(`${BASE}/admin/imports/:id/process`, async ({ request }) => {
        calls.push({ path: "process", body: await request.json() });
        return unavailable();
      }),
    );
    current = wordImport({ status: "awaiting_sources", sources: [] });
    const user = renderDetail();
    await user.upload(
      await screen.findByLabelText("Tệp đề thi"),
      new File([new Uint8Array(2048)], "de-thi.docx"),
    );
    await user.click(screen.getByRole("button", { name: "Bắt đầu xử lý" }));
    await screen.findByRole("alert");

    await user.click(screen.getByRole("button", { name: "Huỷ lần nhập" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Huỷ lần nhập",
      }),
    );

    await screen.findByText("Lần nhập đã được đóng");
    expect(calls.at(-1)).toEqual({ path: "cancel", body: { expectedRevision: 5 } });
  });

  it("re-reads the import after a conflict and starts it at the newer revision", async () => {
    let conflict = true;
    server.use(
      http.post(`${BASE}/admin/imports/:id/process`, async ({ request }) => {
        calls.push({ path: "process", body: await request.json() });
        if (conflict) {
          conflict = false;
          current = wordImport({ ...current, revision: current.revision + 2 });
          return contractJson(
            "/admin/imports/{id}/process",
            "post",
            409,
            errorBody(
              "IMPORT_CONFLICT",
              "Lần nhập đã thay đổi. Hãy tải lại dữ liệu trước khi tiếp tục.",
            ),
          );
        }
        current = wordImport({
          ...current,
          status: "queued",
          revision: current.revision + 1,
        });
        return contractJson("/admin/imports/{id}/process", "post", 202, current);
      }),
    );
    current = wordImport({ status: "awaiting_sources", sources: [source("exam")] });
    const user = renderDetail();
    const start = await screen.findByRole("button", { name: "Bắt đầu xử lý" });
    await waitFor(() => expect(start).toBeEnabled());
    await user.click(start);
    expect(await screen.findByText(/Hãy tải lại dữ liệu/)).toBeInTheDocument();
    await waitFor(() => expect(start).toBeEnabled());
    await user.click(start);

    await waitFor(() => expect(calls).toHaveLength(2));
    expect(calls[0]!.body).toMatchObject({ expectedRevision: 4 });
    expect(calls[1]!.body).toMatchObject({ expectedRevision: 6 });
    expect((calls[1]!.body as { requestId: string }).requestId).not.toBe(
      (calls[0]!.body as { requestId: string }).requestId,
    );
  });
});
