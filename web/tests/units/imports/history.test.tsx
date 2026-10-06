import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportsListPage from "@/features/imports/pages/teacher/ImportsListPage";
import type { WordImport } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { BASE, TEST_ID, capabilities, run, wordImport } from "./fixtures";
import "@/lib/i18n";

let items: WordImport[] = [];
let failing = false;
let queries: URLSearchParams[] = [];

beforeEach(() => {
  items = [];
  failing = false;
  queries = [];
  server.use(
    http.get(`${BASE}/teacher/imports`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      queries.push(query);
      if (failing) return new Response(null, { status: 503 });
      const visible = query.has("status") || query.has("q") ? [] : items;
      return contractJson("/teacher/imports", "get", 200, {
        items: visible,
        page: 1,
        pageSize: 20,
        total: visible.length,
      });
    }),
  );
});

function renderHistory(entry = "/teacher/imports") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/imports", element: <ImportsListPage /> },
      { path: "/teacher/imports/new", element: <p>new import</p> },
    ],
    { initialEntries: [entry] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), router };
}

describe("the Word import history", () => {
  it("names each row's state in words and opens what that state needs", async () => {
    items = [
      wordImport({
        id: "018f0000-0000-7000-8000-000000000101",
        title: "Đề A",
        status: "needs_review",
      }),
      wordImport({
        id: "018f0000-0000-7000-8000-000000000102",
        title: "Đề B",
        status: "processing",
        run: run(),
      }),
      wordImport({
        id: "018f0000-0000-7000-8000-000000000103",
        title: "Đề C",
        status: "committed",
        testId: TEST_ID,
      }),
      wordImport({
        id: "018f0000-0000-7000-8000-000000000104",
        title: "Đề D",
        status: "failed",
        run: run({ status: "failed", errorCode: "SOURCE_INVALID" }),
      }),
    ];
    renderHistory();
    const table = await screen.findByRole("table");
    const row = (title: string) =>
      within(table)
        .getByRole("link", { name: title })
        .closest<HTMLElement>('[role="row"]')!;

    expect(within(row("Đề A")).getByText("Sẵn sàng rà soát")).toBeInTheDocument();
    expect(
      within(row("Đề A")).getByRole("link", { name: "Tiếp tục rà soát Đề A" }),
    ).toHaveAttribute(
      "href",
      "/teacher/imports/018f0000-0000-7000-8000-000000000101/review",
    );
    expect(
      within(row("Đề B")).getByRole("link", { name: "Xem tiến trình của Đề B" }),
    ).toBeInTheDocument();
    expect(
      within(row("Đề C")).getByRole("link", { name: "Mở đề đã tạo từ Đề C" }),
    ).toHaveAttribute("href", `/teacher/tests/${TEST_ID}/edit`);
    expect(
      within(table).getByRole("link", { name: "Đề C" }),
      "the title opens the import itself, with its original files",
    ).toHaveAttribute("href", "/teacher/imports/018f0000-0000-7000-8000-000000000103");
    expect(within(row("Đề D")).getByText("Xử lý không thành công")).toBeInTheDocument();
    expect(
      within(row("Đề D")).getByRole("link", { name: "Xem lỗi của Đề D" }),
    ).toBeInTheDocument();
  });

  it("offers an upload when there are no imports at all", async () => {
    renderHistory();
    expect(await screen.findByText("Chưa có lần nhập đề nào.")).toBeInTheDocument();
    expect(
      screen.getAllByRole("link", { name: "Nhập đề từ Word/PDF" }).length,
    ).toBeGreaterThan(0);
  });

  it("keeps the history but offers no new import while processing is switched off", async () => {
    server.use(capabilities(false));
    items = [wordImport({ status: "needs_review" })];
    renderHistory();

    expect(
      await screen.findByText(
        /^Máy chủ đang tắt xử lý tài liệu nên chưa nhập được đề mới/,
      ),
    ).toBeInTheDocument();
    expect(await screen.findByRole("table")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Nhập đề từ Word/PDF" })).toBeNull();
    expect(screen.queryByRole("link", { name: "Dán đề" })).toBeNull();
  });

  it("offers to view, not continue uploading, an import waiting for files while processing is switched off", async () => {
    server.use(capabilities(false));
    items = [wordImport({ title: "Đề A", status: "awaiting_sources", sources: [] })];
    renderHistory();

    expect(
      await screen.findByRole("link", { name: "Xem chi tiết Đề A" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Tiếp tục tải tệp/ })).toBeNull();
  });

  it("offers no upload in an empty history while processing is switched off", async () => {
    server.use(capabilities(false));
    renderHistory();

    expect(await screen.findByText("Chưa có lần nhập đề nào.")).toBeInTheDocument();
    await screen.findByText(/^Máy chủ đang tắt xử lý tài liệu/);
    expect(screen.queryByRole("link", { name: "Nhập đề từ Word/PDF" })).toBeNull();
    expect(screen.queryByRole("link", { name: "Dán đề" })).toBeNull();
  });

  it("reads its filters from the URL and offers to clear them when nothing matches", async () => {
    items = [wordImport()];
    const { user, router } = renderHistory("/teacher/imports?status=failed&q=hk1");
    expect(
      await screen.findByText("Không có lần nhập nào khớp bộ lọc."),
    ).toBeInTheDocument();
    expect(queries[0]?.get("status")).toBe("failed");
    expect(queries[0]?.get("q")).toBe("hk1");
    expect(screen.getByRole("searchbox")).toHaveValue("hk1");

    await user.click(screen.getByRole("button", { name: "Xoá bộ lọc" }));
    expect(router.state.location.search).toBe("");
    expect(await screen.findByRole("table")).toBeInTheDocument();
  });

  it("keeps its controls and offers a retry when the service is unavailable", async () => {
    failing = true;
    const { user } = renderHistory();
    expect(
      await screen.findByText("Không tải được lịch sử nhập đề."),
    ).toBeInTheDocument();
    expect(screen.getByRole("searchbox")).toBeInTheDocument();

    failing = false;
    items = [wordImport()];
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(await screen.findByRole("table")).toBeInTheDocument();
  });
  it("offers six honest status filters, both entry links and resets page while preserving search and size", async () => {
    const { user, router } = renderHistory("/teacher/imports?q=hk1&page=3&size=30");
    await screen.findByText("Không có lần nhập nào khớp bộ lọc.");
    const filters = screen.getByRole("group", { name: "Lọc theo trạng thái" });
    const buttons = within(filters).getAllByRole("button");
    expect(buttons).toHaveLength(6);
    expect(buttons.map((button) => button.textContent)).toEqual([
      "Tất cả trạng thái",
      "Đang xử lý",
      "Sẵn sàng rà soát",
      "Xử lý không thành công",
      "Đã tạo bản nháp",
      "Đã huỷ",
    ]);
    expect(queries[0]?.get("page")).toBe("3");
    expect(queries[0]?.get("limit")).toBe("30");
    expect(screen.getByRole("link", { name: "Dán đề" })).toHaveAttribute(
      "href",
      "/teacher/imports/new?source=paste",
    );
    await user.click(within(filters).getByRole("button", { name: "Sẵn sàng rà soát" }));
    await waitFor(() =>
      expect(router.state.location.search).toBe("?q=hk1&size=30&status=needs_review"),
    );
    await waitFor(() => expect(queries.at(-1)?.get("status")).toBe("needs_review"));
    expect(queries.at(-1)?.get("page")).toBe("1");
    expect(queries.at(-1)?.get("limit")).toBe("30");
  });

  it("requests originals only on a menu action and reports a download refusal", async () => {
    items = [wordImport()];
    const requested: string[] = [];
    server.use(
      http.get(
        `${BASE}/teacher/imports/:id/sources/:sourceId/download`,
        ({ params }) => {
          requested.push(String(params.sourceId));
          return new Response(null, { status: 503 });
        },
      ),
    );
    const { user } = renderHistory();
    await screen.findByRole("table");
    expect(requested).toEqual([]);
    await user.click(
      screen.getByRole("button", { name: "Thao tác với Đề thi học kỳ 1" }),
    );
    expect(screen.getAllByRole("menuitem")).toHaveLength(2);
    await user.click(
      screen.getByRole("menuitem", { name: "Tải bản gốc de-thi-hk1.docx" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không tải được tệp gốc.",
    );
    expect(requested).toEqual([items[0]!.sources[0]!.id]);
  });

  it("keeps removed source metadata and disables every original download", async () => {
    items = [
      wordImport({
        status: "committed",
        testId: TEST_ID,
        filesRemovedAt: "2026-10-01T00:00:00Z",
      }),
    ];
    const { user } = renderHistory();
    await screen.findByRole("table");
    expect(screen.getByText("de-thi-hk1.docx")).toBeInTheDocument();
    expect(
      screen.getByText("Tệp đã được xoá theo chính sách lưu trữ."),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Mở đề Đề thi học kỳ 1" })).toHaveAttribute(
      "href",
      `/teacher/tests/${TEST_ID}/edit`,
    );
    await user.click(
      screen.getByRole("button", { name: "Thao tác với Đề thi học kỳ 1" }),
    );
    for (const item of screen.getAllByRole("menuitem"))
      expect(item).toHaveAttribute("data-disabled");
  });
});
