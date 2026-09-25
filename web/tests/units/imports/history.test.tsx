import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportsListPage from "@/features/imports/pages/ImportsListPage";
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
    http.get(`${BASE}/admin/imports`, ({ request }) => {
      const query = new URL(request.url).searchParams;
      queries.push(query);
      if (failing) return new Response(null, { status: 503 });
      const visible = query.has("status") || query.has("q") ? [] : items;
      return contractJson("/admin/imports", "get", 200, {
        items: visible,
        page: 1,
        pageSize: 20,
        total: visible.length,
      });
    }),
  );
});

function renderHistory(entry = "/admin/imports") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/imports", element: <ImportsListPage /> },
      { path: "/admin/imports/new", element: <p>new import</p> },
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
      within(table).getByRole("link", { name: title }).closest("tr")!;

    expect(within(row("Đề A")).getByText("Sẵn sàng rà soát")).toBeInTheDocument();
    expect(
      within(row("Đề A")).getByRole("link", { name: "Tiếp tục rà soát Đề A" }),
    ).toHaveAttribute(
      "href",
      "/admin/imports/018f0000-0000-7000-8000-000000000101/review",
    );
    expect(
      within(row("Đề B")).getByRole("link", { name: "Xem tiến trình của Đề B" }),
    ).toBeInTheDocument();
    expect(
      within(row("Đề C")).getByRole("link", { name: "Mở đề đã tạo từ Đề C" }),
    ).toHaveAttribute("href", `/admin/tests/${TEST_ID}/edit`);
    expect(
      within(table).getByRole("link", { name: "Đề C" }),
      "the title opens the import itself, with its original files",
    ).toHaveAttribute("href", "/admin/imports/018f0000-0000-7000-8000-000000000103");
    expect(within(row("Đề D")).getByText("Xử lý không thành công")).toBeInTheDocument();
    expect(
      within(row("Đề D")).getByRole("link", { name: "Xem lỗi của Đề D" }),
    ).toBeInTheDocument();
  });

  it("offers an upload when there are no imports at all", async () => {
    renderHistory();
    expect(await screen.findByText("Chưa có lần nhập đề nào.")).toBeInTheDocument();
    expect(
      screen.getAllByRole("link", { name: "Nhập đề từ Word" }).length,
    ).toBeGreaterThan(0);
  });

  it("keeps the history but offers no new import while processing is switched off", async () => {
    server.use(capabilities(false));
    items = [wordImport({ status: "needs_review" })];
    renderHistory();

    expect(
      await screen.findByText(
        /^Máy chủ đang tắt xử lý tài liệu Word nên chưa nhập được đề mới/,
      ),
    ).toBeInTheDocument();
    expect(await screen.findByRole("table")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Nhập đề từ Word" })).toBeNull();
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
    await screen.findByText(/^Máy chủ đang tắt xử lý tài liệu Word/);
    expect(screen.queryByRole("link", { name: "Nhập đề từ Word" })).toBeNull();
  });

  it("reads its filters from the URL and offers to clear them when nothing matches", async () => {
    items = [wordImport()];
    const { user, router } = renderHistory("/admin/imports?status=failed&q=hk1");
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
});
